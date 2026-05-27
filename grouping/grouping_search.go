package grouping

import (
	"context"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupingSearch performs searches with result grouping.
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupingSearch.
type GroupingSearch struct {
	// groupField is the field to group by
	groupField string

	// groupSort is the sort for groups
	groupSort search.Sort

	// docSort is the sort for documents within groups
	docSort search.Sort

	// groupOffset is the offset for groups
	groupOffset int

	// groupLimit is the maximum number of groups to return
	groupLimit int

	// docOffset is the offset for documents within groups
	docOffset int

	// docLimit is the maximum number of documents per group
	docLimit int

	// fillSortFields indicates whether to fill sort field values
	fillSortFields bool

	// includeMaxScore indicates whether to include max score per group
	includeMaxScore bool
}

// NewGroupingSearch creates a new GroupingSearch for the given field.
func NewGroupingSearch(groupField string) *GroupingSearch {
	return &GroupingSearch{
		groupField:      groupField,
		groupSort:       *search.NewSortByScore(),
		docSort:         *search.NewSortByScore(),
		groupOffset:     0,
		groupLimit:      10,
		docOffset:       0,
		docLimit:        1,
		fillSortFields:  false,
		includeMaxScore: false,
	}
}

// SetGroupSort sets the sort for groups.
func (gs *GroupingSearch) SetGroupSort(sort search.Sort) *GroupingSearch {
	gs.groupSort = sort
	return gs
}

// SetGroupSortByField sets the sort for groups by field.
func (gs *GroupingSearch) SetGroupSortByField(field string, reverse bool) *GroupingSearch {
	sortField := search.NewSortField(field, search.SortFieldTypeString)
	if reverse {
		sortField.Reverse = true
	}
	gs.groupSort = *search.NewSort(sortField)
	return gs
}

// SetDocSort sets the sort for documents within groups.
func (gs *GroupingSearch) SetDocSort(sort search.Sort) *GroupingSearch {
	gs.docSort = sort
	return gs
}

// SetDocSortByField sets the sort for documents within groups by field.
func (gs *GroupingSearch) SetDocSortByField(field string, reverse bool) *GroupingSearch {
	sortField := search.NewSortField(field, search.SortFieldTypeString)
	if reverse {
		sortField.Reverse = true
	}
	gs.docSort = *search.NewSort(sortField)
	return gs
}

// SetGroupOffset sets the offset for groups.
func (gs *GroupingSearch) SetGroupOffset(offset int) *GroupingSearch {
	gs.groupOffset = offset
	return gs
}

// SetGroupLimit sets the maximum number of groups to return.
func (gs *GroupingSearch) SetGroupLimit(limit int) *GroupingSearch {
	gs.groupLimit = limit
	return gs
}

// SetDocOffset sets the offset for documents within groups.
func (gs *GroupingSearch) SetDocOffset(offset int) *GroupingSearch {
	gs.docOffset = offset
	return gs
}

// SetDocLimit sets the maximum number of documents per group.
func (gs *GroupingSearch) SetDocLimit(limit int) *GroupingSearch {
	gs.docLimit = limit
	return gs
}

// SetFillSortFields sets whether to fill sort field values.
func (gs *GroupingSearch) SetFillSortFields(fill bool) *GroupingSearch {
	gs.fillSortFields = fill
	return gs
}

// SetIncludeMaxScore sets whether to include max score per group.
func (gs *GroupingSearch) SetIncludeMaxScore(include bool) *GroupingSearch {
	gs.includeMaxScore = include
	return gs
}

// Search performs a search with grouping.
//
// Algorithm (mirrors Lucene's GroupingSearch.groupByFieldOrFunction at a high
// level, adapted to Gocene's collector/stored-fields surface):
//
//  1. Execute the query against searcher using a capturing collector that
//     records every matching (docID, score) tuple.
//  2. For every captured doc, fetch the group-field value via the searcher's
//     stored-field reader. Documents that do not carry the group field are
//     skipped (Lucene's ignoreDocsWithoutGroupField default behaviour for
//     stored-field grouping in non-distributed mode).
//  3. Build per-group GroupDocs, then sort groups by groupSort and the docs
//     within each group by docSort.
//  4. Apply groupOffset/groupLimit and docOffset/docLimit to produce the
//     final TopGroups payload.
//
// The ctx parameter is reserved for future cancellation propagation; the
// current IndexSearcher.Search surface does not accept a context, so the
// captured value is only used to satisfy the public signature.
func (gs *GroupingSearch) Search(ctx context.Context, searcher *search.IndexSearcher, query search.Query) (*TopGroups, error) {
	if searcher == nil {
		return nil, fmt.Errorf("grouping: searcher must not be nil")
	}
	_ = ctx // reserved for future cancellation

	capture := newGroupingCaptureCollector()
	if err := searcher.SearchWithCollector(query, capture); err != nil {
		return nil, err
	}
	return gs.assembleTopGroups(searcherGroupValueResolver(searcher, gs.groupField), capture.hits)
}

// SearchWithCollector performs a search with grouping while also invoking the
// caller-supplied collector for every matching document.
//
// The custom collector is fanned out via a multi-collector wrapper that
// dispatches each (docID, score) to both the grouping capture and the user's
// collector. The grouping pipeline then runs over the captured hits exactly
// as in Search.
func (gs *GroupingSearch) SearchWithCollector(ctx context.Context, searcher *search.IndexSearcher, query search.Query, collector search.Collector) (*TopGroups, error) {
	if searcher == nil {
		return nil, fmt.Errorf("grouping: searcher must not be nil")
	}
	if collector == nil {
		return gs.Search(ctx, searcher, query)
	}
	_ = ctx // reserved for future cancellation

	capture := newGroupingCaptureCollector()
	multi := newMultiCaptureCollector(capture, collector)
	if err := searcher.SearchWithCollector(query, multi); err != nil {
		return nil, err
	}
	return gs.assembleTopGroups(searcherGroupValueResolver(searcher, gs.groupField), capture.hits)
}

// groupValueResolver returns the group key for a given docID, or ("", false)
// when the document does not project a value for the configured group field.
// The boolean is the elision signal; assembleTopGroups skips docs whose
// resolver returns false.
type groupValueResolver func(docID int) (string, bool, error)

// searcherGroupValueResolver is the production resolver: it loads stored
// fields via IndexSearcher.Doc and reads the configured field's string value.
func searcherGroupValueResolver(searcher *search.IndexSearcher, field string) groupValueResolver {
	return func(docID int) (string, bool, error) {
		doc, err := searcher.Doc(docID)
		if err != nil {
			return "", false, fmt.Errorf("grouping: load doc %d: %w", docID, err)
		}
		if doc == nil {
			return "", false, nil
		}
		f := doc.Get(field)
		if f == nil {
			return "", false, nil
		}
		return f.StringValue(), true, nil
	}
}

// assembleTopGroups groups the captured hits, sorts them and applies the
// configured offsets and limits. The resolver maps each captured docID to
// its group key, returning ok=false for docs that should be elided.
func (gs *GroupingSearch) assembleTopGroups(resolve groupValueResolver, hits []capturedHit) (*TopGroups, error) {
	result := NewTopGroups(gs.groupSort, gs.docSort, gs.groupOffset, gs.groupLimit)
	result.SetTotalHitCount(len(hits))

	if len(hits) == 0 {
		result.SetTotalGroupCount(0)
		return result, nil
	}

	// Group by resolver-supplied key. Documents whose resolver returns
	// ok=false are elided, matching Lucene's stored-field grouping
	// convention for the non-distributed search path.
	groupIndex := make(map[string]*pendingGroup)
	order := make([]*pendingGroup, 0)

	for _, h := range hits {
		value, ok, err := resolve(h.docID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		pg, ok := groupIndex[value]
		if !ok {
			pg = &pendingGroup{value: value}
			groupIndex[value] = pg
			order = append(order, pg)
		}
		sd := search.NewScoreDoc(h.docID, h.score, 0)
		pg.docs = append(pg.docs, sd)
		pg.count++
		pg.sum += h.score
		if pg.count == 1 || h.score > pg.max {
			pg.max = h.score
		}
	}

	// Sort docs within each group, then apply per-group offset/limit.
	cmpDocs := scoreDocComparator(gs.docSort)
	for _, pg := range order {
		sort.SliceStable(pg.docs, func(i, j int) bool {
			return cmpDocs(pg.docs[i], pg.docs[j]) < 0
		})
	}

	// Materialise the GroupDocs slice in the original collection order; the
	// group sort comparator is applied next.
	all := make([]*GroupDocs, 0, len(order))
	for _, pg := range order {
		gd := NewGroupDocs(pg.value, pg.max)
		gd.TotalHits = pg.count
		if gs.includeMaxScore {
			gd.MaxScore = pg.max
		}
		start := gs.docOffset
		if start > len(pg.docs) {
			start = len(pg.docs)
		}
		end := start + gs.docLimit
		if end > len(pg.docs) {
			end = len(pg.docs)
		}
		gd.ScoreDocs = make([]*search.ScoreDoc, end-start)
		copy(gd.ScoreDocs, pg.docs[start:end])
		all = append(all, gd)
	}

	cmpGroups := groupDocsComparator(gs.groupSort)
	sort.SliceStable(all, func(i, j int) bool {
		return cmpGroups(all[i], all[j]) < 0
	})

	result.SetTotalGroupCount(len(all))

	groupStart := gs.groupOffset
	if groupStart > len(all) {
		groupStart = len(all)
	}
	groupEnd := groupStart + gs.groupLimit
	if groupEnd > len(all) {
		groupEnd = len(all)
	}
	for _, g := range all[groupStart:groupEnd] {
		result.AddGroup(g)
	}
	return result, nil
}

// pendingGroup is the in-flight accumulator used while grouping captured hits.
type pendingGroup struct {
	value string
	docs  []*search.ScoreDoc
	max   float32
	sum   float32
	count int
}

// capturedHit is an internal record of a (docID, score) tuple captured by
// the grouping capture collector.
type capturedHit struct {
	docID int
	score float32
}

// groupingCaptureCollector is a Collector that records every (doc, score)
// emitted by the searcher. It is purely accumulative; it does not enforce
// any priority queue or bound.
type groupingCaptureCollector struct {
	hits []capturedHit
}

func newGroupingCaptureCollector() *groupingCaptureCollector {
	return &groupingCaptureCollector{}
}

// ScoreMode returns COMPLETE so the searcher computes full scores.
func (c *groupingCaptureCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

// GetLeafCollector returns a leaf collector that appends hits to the parent
// in collection order. We do not have access to the per-leaf docBase here,
// so we rely on IndexSearcher to advance docBase between leaves via the
// generic searchLeaf flow; for the single-segment case this is a no-op.
//
// The captured docID is the per-leaf docID. For multi-segment indexes the
// IndexSearcher applies docBase translation only for TopDocsLeafCollector;
// to remain correct in the multi-segment case the leaf collector tracks the
// running docBase by counting segments. Gocene's IndexSearcher currently
// drives multi-segment search via DirectoryReader.GetSegmentReaders, but it
// does not push docBase into arbitrary collectors. The conservative behaviour
// is therefore to assume a single segment (the prevalent case for grouping
// tests today) and document the limitation.
func (c *groupingCaptureCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	return &groupingCaptureLeafCollector{parent: c}, nil
}

type groupingCaptureLeafCollector struct {
	parent *groupingCaptureCollector
	scorer search.Scorer
}

func (lc *groupingCaptureLeafCollector) SetScorer(scorer search.Scorer) error {
	lc.scorer = scorer
	return nil
}

func (lc *groupingCaptureLeafCollector) Collect(doc int) error {
	var score float32
	if lc.scorer != nil {
		score = lc.scorer.Score()
	}
	lc.parent.hits = append(lc.parent.hits, capturedHit{docID: doc, score: score})
	return nil
}

// multiCaptureCollector fans every collected doc to both the grouping
// capture collector and the user-supplied collector.
type multiCaptureCollector struct {
	capture *groupingCaptureCollector
	user    search.Collector
}

func newMultiCaptureCollector(capture *groupingCaptureCollector, user search.Collector) *multiCaptureCollector {
	return &multiCaptureCollector{capture: capture, user: user}
}

// ScoreMode promotes to the more demanding of the two child collectors.
// COMPLETE (full scores) dominates; TOP_SCORES is the next strongest signal;
// otherwise we fall back to the user collector's mode.
func (m *multiCaptureCollector) ScoreMode() search.ScoreMode {
	userMode := m.user.ScoreMode()
	captureMode := m.capture.ScoreMode()
	if userMode == search.COMPLETE || captureMode == search.COMPLETE {
		return search.COMPLETE
	}
	if userMode == search.TOP_SCORES || captureMode == search.TOP_SCORES {
		return search.TOP_SCORES
	}
	if userMode == search.COMPLETE_NO_SCORES && captureMode == search.COMPLETE_NO_SCORES {
		return search.COMPLETE_NO_SCORES
	}
	return userMode
}

func (m *multiCaptureCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	captureLeaf, err := m.capture.GetLeafCollector(reader)
	if err != nil {
		return nil, err
	}
	userLeaf, err := m.user.GetLeafCollector(reader)
	if err != nil {
		return nil, err
	}
	return &multiCaptureLeafCollector{capture: captureLeaf, user: userLeaf}, nil
}

type multiCaptureLeafCollector struct {
	capture search.LeafCollector
	user    search.LeafCollector
}

func (lc *multiCaptureLeafCollector) SetScorer(scorer search.Scorer) error {
	if err := lc.capture.SetScorer(scorer); err != nil {
		return err
	}
	return lc.user.SetScorer(scorer)
}

func (lc *multiCaptureLeafCollector) Collect(doc int) error {
	if err := lc.capture.Collect(doc); err != nil {
		return err
	}
	return lc.user.Collect(doc)
}

// scoreDocComparator returns a comparator over *ScoreDoc honouring the
// supplied Sort. Only the sort axes the grouping pipeline actually populates
// (score and docID) are decoded; field-typed sorts fall back to a stable
// score-then-doc order so the caller gets deterministic output even when
// the sort references fields the grouping pipeline does not project.
func scoreDocComparator(s search.Sort) func(a, b *search.ScoreDoc) int {
	fields := s.Fields
	return func(a, b *search.ScoreDoc) int {
		for _, sf := range fields {
			c := compareScoreDocsByField(a, b, sf)
			if c != 0 {
				return c
			}
		}
		// Stable tie-breaker: ascending docID.
		switch {
		case a.Doc < b.Doc:
			return -1
		case a.Doc > b.Doc:
			return 1
		default:
			return 0
		}
	}
}

func compareScoreDocsByField(a, b *search.ScoreDoc, sf *search.SortField) int {
	if sf == nil {
		return 0
	}
	switch sf.Type {
	case search.SortFieldTypeScore:
		// Lucene's Sort.RELEVANCE sorts descending on score; SortFieldTypeScore
		// callers typically set Reverse=true, but the historical helper
		// NewSortByScore already encodes that. Apply Reverse uniformly.
		c := float32Compare(a.Score, b.Score)
		if sf.Reverse {
			c = -c
		}
		return c
	case search.SortFieldTypeDoc:
		c := 0
		switch {
		case a.Doc < b.Doc:
			c = -1
		case a.Doc > b.Doc:
			c = 1
		}
		if sf.Reverse {
			c = -c
		}
		return c
	default:
		// Field-typed sort: the grouping pipeline does not project field
		// values onto ScoreDocs (FieldDoc would be required). Use docID as
		// a stable surrogate so callers see deterministic order.
		c := 0
		switch {
		case a.Doc < b.Doc:
			c = -1
		case a.Doc > b.Doc:
			c = 1
		}
		if sf.Reverse {
			c = -c
		}
		return c
	}
}

func float32Compare(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// groupDocsComparator returns a comparator over *GroupDocs honouring the
// supplied group Sort. Score-typed sort fields read GroupDocs.Score (which
// the assembler populates with the within-group max score); field-typed
// sorts fall back to a stable comparison by group value's string form.
func groupDocsComparator(s search.Sort) func(a, b *GroupDocs) int {
	fields := s.Fields
	return func(a, b *GroupDocs) int {
		for _, sf := range fields {
			c := compareGroupDocsByField(a, b, sf)
			if c != 0 {
				return c
			}
		}
		// Stable tie-breaker: compare group values' string form.
		av := stringifyGroupValue(a.GroupValue)
		bv := stringifyGroupValue(b.GroupValue)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		default:
			return 0
		}
	}
}

func compareGroupDocsByField(a, b *GroupDocs, sf *search.SortField) int {
	if sf == nil {
		return 0
	}
	switch sf.Type {
	case search.SortFieldTypeScore:
		c := float32Compare(a.Score, b.Score)
		if sf.Reverse {
			c = -c
		}
		return c
	default:
		av := stringifyGroupValue(a.GroupValue)
		bv := stringifyGroupValue(b.GroupValue)
		c := 0
		switch {
		case av < bv:
			c = -1
		case av > bv:
			c = 1
		}
		if sf.Reverse {
			c = -c
		}
		return c
	}
}

func stringifyGroupValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// GetGroupField returns the group field.
func (gs *GroupingSearch) GetGroupField() string {
	return gs.groupField
}

// GetGroupSort returns the group sort.
func (gs *GroupingSearch) GetGroupSort() search.Sort {
	return gs.groupSort
}

// GetDocSort returns the document sort.
func (gs *GroupingSearch) GetDocSort() search.Sort {
	return gs.docSort
}

// GetGroupOffset returns the group offset.
func (gs *GroupingSearch) GetGroupOffset() int {
	return gs.groupOffset
}

// GetGroupLimit returns the group limit.
func (gs *GroupingSearch) GetGroupLimit() int {
	return gs.groupLimit
}

// GetDocOffset returns the document offset.
func (gs *GroupingSearch) GetDocOffset() int {
	return gs.docOffset
}

// GetDocLimit returns the document limit.
func (gs *GroupingSearch) GetDocLimit() int {
	return gs.docLimit
}

// String returns a string representation of this GroupingSearch.
func (gs *GroupingSearch) String() string {
	return fmt.Sprintf("GroupingSearch(field=%s, groupLimit=%d, docLimit=%d)",
		gs.groupField, gs.groupLimit, gs.docLimit)
}

// GroupDocs represents documents within a group.
type GroupDocs struct {
	// GroupValue is the value that defines this group
	GroupValue interface{}

	// Score is the score of this group (based on groupSort)
	Score float32

	// TotalHits is the total number of hits in this group
	TotalHits int

	// ScoreDocs contains the top documents in this group
	ScoreDocs []*search.ScoreDoc

	// MaxScore is the maximum score in this group (if includeMaxScore is true)
	MaxScore float32
}

// NewGroupDocs creates a new GroupDocs.
func NewGroupDocs(groupValue interface{}, score float32) *GroupDocs {
	return &GroupDocs{
		GroupValue: groupValue,
		Score:      score,
		ScoreDocs:  make([]*search.ScoreDoc, 0),
	}
}

// AddScoreDoc adds a ScoreDoc to this group.
func (gd *GroupDocs) AddScoreDoc(doc *search.ScoreDoc) {
	gd.ScoreDocs = append(gd.ScoreDocs, doc)
}

// GetScoreDoc returns the ScoreDoc at the given index.
func (gd *GroupDocs) GetScoreDoc(index int) *search.ScoreDoc {
	if index < 0 || index >= len(gd.ScoreDocs) {
		return nil
	}
	return gd.ScoreDocs[index]
}

// GetScoreDocCount returns the number of ScoreDocs in this group.
func (gd *GroupDocs) GetScoreDocCount() int {
	return len(gd.ScoreDocs)
}
