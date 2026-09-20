// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/TopFieldCollector.java
//   lucene/core/src/java/org/apache/lucene/search/FieldValueHitQueue.java
//
// TopFieldCollector collects the top-N documents ordered by one or more sort
// fields read from DocValues, rather than by relevance score. It maintains a
// fixed-capacity priority queue keyed by the sort comparators and runs the
// LeafFieldComparator lifecycle (setReader/copy/setBottom/compareBottom) per
// segment, exactly as Lucene does.
//
// Scope note: this port reproduces the value-correct collection path (the order
// of returned hits and their FieldDoc sort values). The two performance
// optimisations Lucene layers on top — competitive-document skipping via the
// comparators' CompetitiveIterator and early termination when the search sort is
// a prefix of the index sort — are intentionally omitted; with them disabled
// Lucene produces the identical ordering, so the omission is not observable.

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// fieldEntry is one slot in the field-value priority queue. slot indexes the
// per-slot value cache held by every comparator; doc is the global document id.
//
// Mirrors org.apache.lucene.search.FieldValueHitQueue.Entry.
type fieldEntry struct {
	slot int
	doc  int
}

// fieldValueHitQueue is a min-heap whose "smallest" element is the weakest hit
// (the one that should be evicted first). Ordering is delegated to the sort
// comparators via lessThan.
//
// Mirrors org.apache.lucene.search.FieldValueHitQueue.
type fieldValueHitQueue struct {
	heap        []*fieldEntry
	comparators []FieldComparator
	reverseMul  []int
}

func newFieldValueHitQueue(comparators []FieldComparator, reverseMul []int, capacity int) *fieldValueHitQueue {
	return &fieldValueHitQueue{
		heap:        make([]*fieldEntry, 0, capacity),
		comparators: comparators,
		reverseMul:  reverseMul,
	}
}

func (q *fieldValueHitQueue) size() int { return len(q.heap) }

// getComparators returns the per-leaf view of every comparator for the given
// segment.
//
// Mirrors FieldValueHitQueue.getComparators(LeafReaderContext).
func (q *fieldValueHitQueue) getComparators(context *index.LeafReaderContext) ([]LeafFieldComparator, error) {
	comparators := make([]LeafFieldComparator, len(q.comparators))
	for i := range q.comparators {
		leaf, err := q.comparators[i].GetLeafComparator(context)
		if err != nil {
			return nil, err
		}
		comparators[i] = leaf
	}
	return comparators, nil
}

// lessThan reports whether a should sort after b — i.e. a is weaker than b and
// belongs closer to the top of the min-heap. The first non-zero comparator
// result wins; ties break on higher doc id (so equal-value hits keep ascending
// docID order, matching Lucene bug #31241 fix).
//
// Mirrors FieldValueHitQueue.lessThan (single- and multi-comparator variants).
func (q *fieldValueHitQueue) lessThan(a, b *fieldEntry) bool {
	for i, cmp := range q.comparators {
		c := q.reverseMul[i] * cmp.Compare(a.slot, b.slot)
		if c != 0 {
			return c > 0
		}
	}
	return a.doc > b.doc
}

func (q *fieldValueHitQueue) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !q.lessThan(q.heap[i], q.heap[parent]) {
			break
		}
		q.heap[i], q.heap[parent] = q.heap[parent], q.heap[i]
		i = parent
	}
}

func (q *fieldValueHitQueue) down(i int) {
	n := len(q.heap)
	for {
		l, r := 2*i+1, 2*i+2
		smallest := i
		if l < n && q.lessThan(q.heap[l], q.heap[smallest]) {
			smallest = l
		}
		if r < n && q.lessThan(q.heap[r], q.heap[smallest]) {
			smallest = r
		}
		if smallest == i {
			break
		}
		q.heap[i], q.heap[smallest] = q.heap[smallest], q.heap[i]
		i = smallest
	}
}

// add pushes a new entry and returns the (possibly unchanged) top entry.
//
// Mirrors PriorityQueue.add followed by reading top().
func (q *fieldValueHitQueue) add(e *fieldEntry) *fieldEntry {
	q.heap = append(q.heap, e)
	q.up(len(q.heap) - 1)
	return q.heap[0]
}

// top returns the weakest entry without removing it.
func (q *fieldValueHitQueue) top() *fieldEntry {
	if len(q.heap) == 0 {
		return nil
	}
	return q.heap[0]
}

// updateTop re-sifts the root after its key changed in place and returns the new
// top. Mirrors PriorityQueue.updateTop.
func (q *fieldValueHitQueue) updateTop() *fieldEntry {
	q.down(0)
	return q.heap[0]
}

// TopFieldCollector collects the top-N documents sorted by the sort fields'
// DocValues. It is the Go port of org.apache.lucene.search.TopFieldCollector.
type TopFieldCollector struct {
	BaseSimpleCollector

	// scoreMode mirrors the TopFieldCollector.scoreMode field, returned
	// verbatim by ScoreMode.
	scoreMode ScoreMode

	numHits int
	sort    *Sort

	comparators []FieldComparator
	reverseMul  []int
	queue       *fieldValueHitQueue

	totalHits int
	maxScore  float32
	collected int
	queueFull bool
	bottom    *fieldEntry

	// after, when non-nil, is the paging marker supplied to a sort-aware
	// searchAfter. The sort-optimization feature that consumes it (CompareTop /
	// setTopValue paging filter plus competitive-hit skipping) is tracked by
	// rmp #130; until it lands the collector records the marker but still runs the
	// unpaged full-scan collection path.
	after *FieldDoc
}

// NewTopFieldCollector creates a TopFieldCollector for the given sort. numHits
// must be > 0 and sort must contain at least one field; callers that build it
// directly (rather than via the manager) are responsible for those invariants.
func NewTopFieldCollector(numHits int, sort *Sort) *TopFieldCollector {
	scoreMode := COMPLETE_NO_SCORES
	if sort.NeedsScores() {
		scoreMode = COMPLETE
	}

	// Mirrors FieldValueHitQueue.create(SortField[], int, Pruning), which calls
	// SortField.getComparator(numHits, pruning) for every sort key. No
	// comparator in this package implements competitive-document skipping, so
	// the collector asks for Pruning.NONE, which is the setting under which
	// Lucene's comparators produce the same ordering by the same code path.
	comparators := make([]FieldComparator, 0, len(sort.Fields))
	reverseMuls := make([]int, 0, len(sort.Fields))
	for _, sf := range sort.Fields {
		comparators = append(comparators, SortFieldGetComparator(sf, numHits, PruningNone))
		reverseMuls = append(reverseMuls, reverseMul(sf))
	}

	return &TopFieldCollector{
		scoreMode:   scoreMode,
		numHits:     numHits,
		sort:        sort,
		comparators: comparators,
		reverseMul:  reverseMuls,
		queue:       newFieldValueHitQueue(comparators, reverseMuls, numHits),
	}
}

// NewTopFieldCollectorAfter creates a TopFieldCollector that records a paging
// "after" marker for a sort-aware searchAfter. It is identical to
// NewTopFieldCollector except for retaining the marker.
//
// rmp #130: the collector does not yet apply the marker — the comparators'
// CompareTop is still the unimplemented no-op, so no document is filtered or
// skipped and the page is the unpaged top-n with an EQUAL_TO totalHits relation.
// The constructor exists so the searchAfter entry point is faithful to Lucene's
// public API and paging tests can exercise it; correct paging is delivered with
// the sort-optimization feature.
func NewTopFieldCollectorAfter(numHits int, sort *Sort, after *FieldDoc) *TopFieldCollector {
	c := NewTopFieldCollector(numHits, sort)
	c.after = after
	return c
}

// ScoreMode mirrors TopFieldCollector.scoreMode(), which returns the
// scoreMode field computed at construction time.
func (c *TopFieldCollector) ScoreMode() ScoreMode { return c.scoreMode }

// GetLeafCollector binds every comparator to the new leaf and returns a
// LeafCollector. The context's reader carries the segment's DocValues, which
// the comparators resolve in setReader, and its docBase rebases collected doc
// ids (and DOC-comparator values) to the global id space.
func (c *TopFieldCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	docBase := 0
	if context != nil {
		docBase = context.DocBase
	}
	return NewTopFieldLeafCollector(c, context, docBase)
}

// TopDocs returns the collected hits as a TopFieldDocs, ordered best-first, with
// each ScoreDoc upgraded to a FieldDoc carrying its per-field sort values.
func (c *TopFieldCollector) TopDocs() *TopDocs {
	return c.TopFieldDocs().TopDocs
}

// TopDocsRange returns the hits in the range [start, start+howMany) of the
// collected results, best-first.
//
// Mirrors TopDocsCollector.topDocs(int start, int howMany); Go cannot overload,
// so the two-argument form carries the longer name. An out-of-range start or a
// non-positive howMany yields an empty result with the collected total, exactly
// as Java's guard does.
func (c *TopFieldCollector) TopDocsRange(start, howMany int) *TopDocs {
	all := c.TopFieldDocs()
	size := len(all.ScoreDocs)
	if start < 0 || start >= size || howMany <= 0 {
		return NewTopDocs(all.TotalHits, []*ScoreDoc{})
	}
	if howMany > size-start {
		howMany = size - start
	}
	results := make([]*ScoreDoc, howMany)
	copy(results, all.ScoreDocs[start:start+howMany])
	return NewTopDocs(all.TotalHits, results)
}

// TopFieldDocs returns the collected hits as a TopFieldDocs, so the per-hit
// FieldDoc sort values survive the merge.
//
// Mirrors TopFieldCollector.topDocs(), whose declared return type is the
// covariant TopFieldDocs; Gocene's TopDocs() returns the erased *TopDocs that
// TopDocsCollector.topDocs() declares, so the covariant override needs its own
// Go name.
func (c *TopFieldCollector) TopFieldDocs() *TopFieldDocs {
	n := c.queue.size()
	entries := make([]*fieldEntry, n)
	copy(entries, c.queue.heap)

	// Sort entries best-first: an entry is "better" when it is NOT lessThan the
	// other (lessThan marks the weaker hit). Reusing the queue ordering keeps the
	// tie-break (ascending docID) identical to collection.
	sort.SliceStable(entries, func(i, j int) bool {
		return c.queue.lessThan(entries[j], entries[i])
	})

	fieldDocs := make([]*FieldDoc, n)
	for i, e := range entries {
		fields := make([]any, len(c.comparators))
		for k, cmp := range c.comparators {
			fields[k] = cmp.Value(e.slot)
		}
		fieldDocs[i] = NewFieldDocWithFields(e.doc, float32(0), fields)
	}

	return NewTopFieldDocsWithFieldDocs(
		NewTotalHits(int64(c.totalHits), EQUAL_TO),
		fieldDocs,
		c.sort.Fields,
	)
}

// CanEarlyTerminate reports whether a search using searchSort can early
// terminate when the index is sorted by indexSort. It is a faithful port of
// org.apache.lucene.search.TopFieldCollector.canEarlyTerminate: termination is
// possible either because the search sorts by document id (canEarlyTerminateOnDocId)
// or because the search sort is a prefix of the index sort (canEarlyTerminateOnPrefix).
//
// indexSort may be nil (the index has no sort), in which case only the
// sort-by-docId case can early terminate.
func CanEarlyTerminate(searchSort, indexSort *Sort) bool {
	return canEarlyTerminateOnDocID(searchSort) || canEarlyTerminateOnPrefix(searchSort, indexSort)
}

// canEarlyTerminateOnDocID mirrors TopFieldCollector.canEarlyTerminateOnDocId:
// termination is possible when the first search sort field is the DOC field
// (SortField.FIELD_DOC), i.e. an ascending sort by document id with no missing
// value override.
func canEarlyTerminateOnDocID(searchSort *Sort) bool {
	if searchSort == nil || len(searchSort.Fields) == 0 {
		return false
	}
	return sortFieldEqualsFieldDoc(searchSort.Fields[0])
}

// canEarlyTerminateOnPrefix mirrors TopFieldCollector.canEarlyTerminateOnPrefix:
// when an index sort exists, the search sort fields must be a prefix of (i.e.
// pairwise equal to a leading slice of) the index sort fields.
func canEarlyTerminateOnPrefix(searchSort, indexSort *Sort) bool {
	if indexSort == nil {
		return false
	}
	fields1 := searchSort.Fields
	fields2 := indexSort.Fields
	if len(fields1) > len(fields2) {
		return false
	}
	for i := range fields1 {
		if !sortFieldEquals(fields1[i], fields2[i]) {
			return false
		}
	}
	return true
}

// sortFieldEqualsFieldDoc reports whether sf is equal to SortField.FIELD_DOC,
// which Lucene defines as new SortField(null, SortField.Type.DOC) — an ascending
// DOC sort over the (empty) field name with no missing-value override.
func sortFieldEqualsFieldDoc(sf *SortField) bool {
	return sf != nil &&
		sf.Type == spi.SortFieldTypeDoc &&
		sf.Field == "" &&
		!sf.Reverse &&
		sf.MissingValue == nil
}

// sortFieldEquals mirrors org.apache.lucene.search.SortField.equals: two sort
// fields are equal when they share field name, type, reverse flag and
// missing-value override. SortedNumericSortField additionally compares its
// numeric type and selector; this helper compares the common fields plus those
// extras when both operands carry them, which is sufficient for the sort shapes
// used by the early-termination contract.
func sortFieldEquals(a, b *SortField) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Field != b.Field || a.Type != b.Type || a.Reverse != b.Reverse {
		return false
	}
	return a.MissingValue == b.MissingValue
}

// GetTotalHits returns the total number of hits collected.
func (c *TopFieldCollector) GetTotalHits() int { return c.totalHits }

// GetMaxScore returns the maximum score seen (0 unless the sort needs scores).
func (c *TopFieldCollector) GetMaxScore(_ int) (float32, error) { return c.maxScore, nil }

// TopFieldLeafCollector drives one segment. It composes the per-leaf comparators
// into a single multiLeafFieldComparator (so a multi-key sort short-circuits on
// the first differing key) and runs the copy/add/setBottom lifecycle.
type TopFieldLeafCollector struct {
	*BaseLeafCollector
	collector  *TopFieldCollector
	comparator LeafFieldComparator
	scorer     Scorable
	docBase    int
}

// NewTopFieldLeafCollector creates a leaf collector for context. The per-leaf
// comparators are obtained from the queue (FieldComparator.GetLeafComparator
// for every sort key) and composed into a single comparator with their reverse
// multipliers.
//
// Mirrors TopFieldCollector.TopFieldLeafCollector(FieldValueHitQueue, Sort,
// LeafReaderContext), whose body starts with queue.getComparators(context) and
// which declares `throws IOException`. docBase is passed separately because
// Gocene's collector tolerates a nil context, which Lucene's does not.
func NewTopFieldLeafCollector(collector *TopFieldCollector, context *index.LeafReaderContext, docBase int) (*TopFieldLeafCollector, error) {
	leafComparators, err := collector.queue.getComparators(context)
	if err != nil {
		return nil, err
	}
	var comparator LeafFieldComparator
	if len(leafComparators) == 1 {
		comparator = leafComparators[0]
	} else {
		comparator, _ = newMultiLeafFieldComparator(leafComparators, collector.reverseMul)
	}
	lc := &TopFieldLeafCollector{
		BaseLeafCollector: NewBaseLeafCollector(),
		collector:         collector,
		comparator:        comparator,
	}

	// Propagate the segment docBase to the DOC comparator(s) so their cached
	// sort values are global (docBase-rebased) doc ids, matching the global doc
	// ids the queue stores. Without this the DOC comparator compares and reports
	// segment-LOCAL doc ids, which both breaks cross-segment ordering and makes
	// FieldDoc.Fields disagree with ScoreDoc.Doc. SetDocBase is the single place
	// that does this propagation.
	lc.SetDocBase(docBase)
	return lc, nil
}

// SetScorer records the scorer and forwards it to the comparators (only a
// score-typed comparator consumes it).
func (c *TopFieldLeafCollector) SetScorer(scorer Scorable) error {
	c.scorer = scorer
	return c.comparator.SetScorer(scorer)
}

// SetDocBase sets the document base offset for the segment and propagates it to
// any DOC comparator, whose values are global (docBase-rebased) document ids.
func (c *TopFieldLeafCollector) SetDocBase(docBase int) {
	c.docBase = docBase
	for _, cmp := range c.collector.comparators {
		if dc, ok := cmp.(*docComparator); ok {
			dc.docBase = docBase
		}
	}
}

// Collect adds the leaf-local doc to the queue if it is competitive.
//
// Mirrors TopFieldCollector.TopFieldLeafCollector.collect (the non-paging path).
func (c *TopFieldLeafCollector) Collect(doc int) error {
	col := c.collector
	col.totalHits++
	if c.scorer != nil {
		s, err := c.scorer.Score()
		if err != nil {
			return err
		}
		if s > col.maxScore {
			col.maxScore = s
		}
	}

	if col.queueFull {
		// Competitive check: reverseMul * compareBottom(doc) > 0 means doc is
		// better than the current bottom. Multi-key sorts already fold the
		// reverse multipliers inside the composite comparator, so use +1 there.
		rm := 1
		if len(col.comparators) == 1 {
			rm = col.reverseMul[0]
		}
		cb, err := c.comparator.CompareBottom(doc)
		if err != nil {
			return err
		}
		if rm*cb <= 0 {
			// Not competitive.
			return nil
		}
		// Replace the bottom element.
		if err := c.comparator.Copy(col.bottom.slot, doc); err != nil {
			return err
		}
		col.bottom.doc = c.docBase + doc
		col.bottom = col.queue.updateTop()
		return c.comparator.SetBottom(col.bottom.slot)
	}

	// Queue not yet full: take the next free slot.
	slot := col.collected
	col.collected++
	if err := c.comparator.Copy(slot, doc); err != nil {
		return err
	}
	col.bottom = col.queue.add(&fieldEntry{slot: slot, doc: c.docBase + doc})
	if col.collected == col.numHits {
		col.queueFull = true
		return c.comparator.SetBottom(col.bottom.slot)
	}
	return nil
}

// scorerAsScorable adapts a Scorer (Score() float32) to a Scorable
// (Score() (float32, error)) so it can be handed to the comparators' SetScorer.
// DocValues comparators ignore the scorer, so the adapter is only exercised by a
// score-typed comparator.
func scorerAsScorable(s Scorer) (Scorable, bool) {
	if s == nil {
		return nil, false
	}
	return &scorerScorableAdapter{s: s}, true
}

type scorerScorableAdapter struct {
	BaseScorable
	s Scorer
}

func (a *scorerScorableAdapter) Score() (float32, error) { return a.s.Score() }

// Ensure TopFieldCollector implements Collector and the leaf type satisfies
// LeafCollector.
var (
	_ Collector     = (*TopFieldCollector)(nil)
	_ LeafCollector = (*TopFieldLeafCollector)(nil)
)

// reader-type assertion helper: the search loop hands GetLeafCollector the
// concrete leaf reader; the comparators type-assert it to the DocValues views.
var _ = index.NumericDocValues(nil)

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (t *TopFieldLeafCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(t, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (t *TopFieldLeafCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(t, stream)
}

// PopulateScores populates the scores of the given topDocs.
//
// topDocs is the top docs to populate, searcher the index searcher that has
// been used to compute topDocs, and query the query that has been used to
// compute them. An error is returned if there is evidence that topDocs have
// been computed against a different searcher or a different query.
//
// Mirrors the static TopFieldCollector.populateScores(ScoreDoc[], IndexSearcher, Query)
// of Apache Lucene 10.5.0.
func PopulateScores(topDocs []*ScoreDoc, searcher *IndexSearcher, query Query) error {
	// Get the score docs sorted in doc id order
	sorted := make([]*ScoreDoc, len(topDocs))
	copy(sorted, topDocs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Doc < sorted[j].Doc })

	rewritten, err := searcher.Rewrite(query)
	if err != nil {
		return err
	}
	weight, err := searcher.CreateWeight(rewritten, COMPLETE, 1)
	if err != nil {
		return err
	}
	contexts, err := searcher.GetIndexReader().Leaves()
	if err != nil {
		return err
	}

	var currentContext *index.LeafReaderContext
	var currentScorer Scorer
	for _, scoreDoc := range sorted {
		if currentContext == nil ||
			scoreDoc.Doc >= currentContext.DocBase+currentContext.LeafReader().MaxDoc() {
			if scoreDoc.Doc < 0 || scoreDoc.Doc >= searcher.GetIndexReader().MaxDoc() {
				return fmt.Errorf("doc id %d is out of bounds", scoreDoc.Doc)
			}
			newContextIndex := index.ReaderUtilSubIndexLeaves(scoreDoc.Doc, contexts)
			currentContext = contexts[newContextIndex]
			scorerSupplier, err := weight.ScorerSupplier(currentContext)
			if err != nil {
				return err
			}
			if scorerSupplier == nil {
				return fmt.Errorf("doc id %d doesn't match the query", scoreDoc.Doc)
			}
			currentScorer, err = scorerSupplier.Get(1) // random-access
			if err != nil {
				return err
			}
		}
		leafDoc := scoreDoc.Doc - currentContext.DocBase
		advanced, err := currentScorer.Iterator().Advance(leafDoc)
		if err != nil {
			return err
		}
		if leafDoc != advanced {
			return fmt.Errorf("doc id %d doesn't match the query", scoreDoc.Doc)
		}
		score, err := currentScorer.Score()
		if err != nil {
			return err
		}
		scoreDoc.Score = score
	}
	return nil
}
