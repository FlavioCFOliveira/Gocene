package uhighlight

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// HighlightFlag controls highlighting behavior.
type HighlightFlag int

const (
	HighlightFlagPhrases HighlightFlag = iota
	HighlightFlagMultiTermQuery
	HighlightFlagPassageRelevancyOverSpeed
	HighlightFlagWeightMatches
)

// MultivalSepChar renders UnifiedHighlighter.MULTIVAL_SEP_CHAR
// (UnifiedHighlighter.java:98), the character used to join the values of a
// multi-valued field before analysis.
const MultivalSepChar = rune(0)

const (
	defaultMaxLength                      = 10000
	defaultCacheFieldValCharsThreshold    = 524288
	defaultEnableMultiTermQuery           = true
	defaultEnableHighlightPhrasesStrictly = true
	defaultEnableWeightMatches            = true
	defaultEnableRelevancyOverSpeed       = true
	defaultMaxHighlightPassages           = -1
)

// UnifiedHighlighter is a Highlighter that can get offsets from either postings,
// term vectors, or via re-analyzing text.
type UnifiedHighlighter struct {
	searcher                    *search.IndexSearcher
	indexAnalyzer               analysis.Analyzer
	fieldInfos                  *index.FieldInfos
	fieldInfosMu                sync.RWMutex
	fieldMatcher                func(string) bool
	maskedFieldsFunc            func(string) []string
	flags                       map[HighlightFlag]struct{}
	handleMultiTermQuery        bool
	highlightPhrasesStrictly    bool
	weightMatches               bool
	passageRelevancyOverSpeed   bool
	maxLength                   int
	breakIterator               func() BreakIterator
	scorer                      *PassageScorer
	formatter                   PassageFormatter
	maxNoHighlightPassages      int
	cacheFieldValCharsThreshold int
	passageSortComparator       func(p1, p2 *Passage) int
}

// Builder for UnifiedHighlighter.
type Builder struct {
	searcher                    *search.IndexSearcher
	indexAnalyzer               analysis.Analyzer
	fieldMatcher                func(string) bool
	maskedFieldsFunc            func(string) []string
	flags                       map[HighlightFlag]struct{}
	handleMultiTermQuery        bool
	highlightPhrasesStrictly    bool
	passageRelevancyOverSpeed   bool
	weightMatches               bool
	maxLength                   int
	breakIterator               func() BreakIterator
	scorer                      *PassageScorer
	formatter                   PassageFormatter
	maxNoHighlightPassages      int
	cacheFieldValCharsThreshold int
	passageSortComparator       func(p1, p2 *Passage) int
}

func NewBuilder(searcher *search.IndexSearcher, analyzer analysis.Analyzer) *Builder {
	return &Builder{
		searcher:                    searcher,
		indexAnalyzer:               analyzer,
		handleMultiTermQuery:        defaultEnableMultiTermQuery,
		highlightPhrasesStrictly:    defaultEnableHighlightPhrasesStrictly,
		passageRelevancyOverSpeed:   defaultEnableRelevancyOverSpeed,
		weightMatches:               defaultEnableWeightMatches,
		maxLength:                   defaultMaxLength,
		breakIterator:               func() BreakIterator { return SentenceBreakIterator{} },
		scorer:                      NewPassageScorer(),
		formatter:                   NewDefaultPassageFormatter(),
		maxNoHighlightPassages:      defaultMaxHighlightPassages,
		cacheFieldValCharsThreshold: defaultCacheFieldValCharsThreshold,
		passageSortComparator: func(p1, p2 *Passage) int {
			if p1.StartOffset() < p2.StartOffset() {
				return -1
			} else if p1.StartOffset() > p2.StartOffset() {
				return 1
			}
			return 0
		},
	}
}

func NewBuilderWithoutSearcher(analyzer analysis.Analyzer) *Builder {
	return NewBuilder(nil, analyzer)
}

func (b *Builder) WithFlags(flags map[HighlightFlag]struct{}) *Builder {
	b.flags = flags
	return b
}

func (b *Builder) WithHighlightPhrasesStrictly(value bool) *Builder {
	b.highlightPhrasesStrictly = value
	return b
}

func (b *Builder) WithHandleMultiTermQuery(value bool) *Builder {
	b.handleMultiTermQuery = value
	return b
}

func (b *Builder) WithPassageRelevancyOverSpeed(value bool) *Builder {
	b.passageRelevancyOverSpeed = value
	return b
}

func (b *Builder) WithWeightMatches(value bool) *Builder {
	b.weightMatches = value
	return b
}

func (b *Builder) WithMaxLength(value int) *Builder {
	if value < 0 || value == 2147483647 {
		panic("maxLength must be < Integer.MAX_VALUE")
	}
	b.maxLength = value
	return b
}

func (b *Builder) WithBreakIterator(value func() BreakIterator) *Builder {
	b.breakIterator = value
	return b
}

func (b *Builder) WithFieldMatcher(value func(string) bool) *Builder {
	b.fieldMatcher = value
	return b
}

func (b *Builder) WithMaskedFieldsFunc(value func(string) []string) *Builder {
	b.maskedFieldsFunc = value
	return b
}

func (b *Builder) WithScorer(value *PassageScorer) *Builder {
	b.scorer = value
	return b
}

func (b *Builder) WithFormatter(value PassageFormatter) *Builder {
	b.formatter = value
	return b
}

func (b *Builder) WithMaxNoHighlightPassages(value int) *Builder {
	b.maxNoHighlightPassages = value
	return b
}

func (b *Builder) WithCacheFieldValCharsThreshold(value int) *Builder {
	b.cacheFieldValCharsThreshold = value
	return b
}

func (b *Builder) WithPassageSortComparator(value func(p1, p2 *Passage) int) *Builder {
	b.passageSortComparator = value
	return b
}

func (b *Builder) Build() *UnifiedHighlighter {
	uh := &UnifiedHighlighter{
		searcher:                    b.searcher,
		indexAnalyzer:               b.indexAnalyzer,
		maxLength:                   b.maxLength,
		breakIterator:               b.breakIterator,
		fieldMatcher:                b.fieldMatcher,
		maskedFieldsFunc:            b.maskedFieldsFunc,
		scorer:                      b.scorer,
		formatter:                   b.formatter,
		maxNoHighlightPassages:      b.maxNoHighlightPassages,
		cacheFieldValCharsThreshold: b.cacheFieldValCharsThreshold,
		passageSortComparator:       b.passageSortComparator,
	}
	uh.flags = uh.evaluateFlags(b)
	return uh
}

func (uh *UnifiedHighlighter) evaluateFlags(b *Builder) map[HighlightFlag]struct{} {
	if b.flags != nil {
		return b.flags
	}
	flags := make(map[HighlightFlag]struct{})
	if b.handleMultiTermQuery {
		flags[HighlightFlagMultiTermQuery] = struct{}{}
	}
	if b.highlightPhrasesStrictly {
		flags[HighlightFlagPhrases] = struct{}{}
	}
	if b.passageRelevancyOverSpeed {
		flags[HighlightFlagPassageRelevancyOverSpeed] = struct{}{}
	}

	_, hasMTQ := flags[HighlightFlagMultiTermQuery]
	_, hasPhrases := flags[HighlightFlagPhrases]
	_, hasRelevancy := flags[HighlightFlagPassageRelevancyOverSpeed]

	if hasMTQ && hasPhrases && hasRelevancy && b.weightMatches {
		flags[HighlightFlagWeightMatches] = struct{}{}
	}
	return flags
}

// Highlight highlights the top passages from a single field.
//
// Mirrors UnifiedHighlighter.highlight(String, Query, TopDocs)
// (UnifiedHighlighter.java:747).
func (uh *UnifiedHighlighter) Highlight(field string, query search.Query, topDocs *search.TopDocs) ([]string, error) {
	return uh.HighlightMaxPassages(field, query, topDocs, 1)
}

// HighlightMaxPassages highlights the top-N passages from a single field.
//
// Mirrors UnifiedHighlighter.highlight(String, Query, TopDocs, int)
// (UnifiedHighlighter.java:766). Go has no overloading, so the maxPassages
// parameter is named in the method name.
func (uh *UnifiedHighlighter) HighlightMaxPassages(field string, query search.Query, topDocs *search.TopDocs, maxPassages int) ([]string, error) {
	res, err := uh.HighlightFieldsMaxPassages([]string{field}, query, topDocs, []int{maxPassages})
	if err != nil {
		return nil, err
	}
	return res[field], nil
}

// HighlightFields highlights the top passages from multiple fields.
//
// Mirrors UnifiedHighlighter.highlightFields(String[], Query, TopDocs)
// (UnifiedHighlighter.java:796).
func (uh *UnifiedHighlighter) HighlightFields(fields []string, query search.Query, topDocs *search.TopDocs) (map[string][]string, error) {
	maxPassages := make([]int, len(fields))
	for i := range maxPassages {
		maxPassages[i] = 1
	}
	return uh.HighlightFieldsMaxPassages(fields, query, topDocs, maxPassages)
}

// HighlightFieldsMaxPassages highlights the top-N passages from multiple
// fields.
//
// Mirrors UnifiedHighlighter.highlightFields(String[], Query, TopDocs, int[])
// (UnifiedHighlighter.java:828). Go has no overloading, so the per-field
// maxPassages parameter is named in the method name.
func (uh *UnifiedHighlighter) HighlightFieldsMaxPassages(fields []string, query search.Query, topDocs *search.TopDocs, maxPassages []int) (map[string][]string, error) {
	scoreDocs := topDocs.ScoreDocs
	docids := make([]int, len(scoreDocs))
	for i := range docids {
		docids[i] = scoreDocs[i].Doc
	}
	return uh.HighlightFieldsForDocIDs(fields, query, docids, maxPassages)
}

// HighlightFieldsForDocIDs highlights the top-N passages from multiple fields,
// for the provided document IDs.
//
// Mirrors UnifiedHighlighter.highlightFields(String[], Query, int[], int[])
// (UnifiedHighlighter.java:854). Go has no overloading, so the docids
// parameter is named in the method name.
func (uh *UnifiedHighlighter) HighlightFieldsForDocIDs(fieldsIn []string, query search.Query, docidsIn []int, maxPassagesIn []int) (map[string][]string, error) {
	objs, err := uh.highlightFieldsAsObjects(fieldsIn, query, docidsIn, maxPassagesIn)
	if err != nil {
		return nil, err
	}

	snippets := make(map[string][]string)
	for field, snippetObjects := range objs {
		snippetStrings := make([]string, len(snippetObjects))
		for i, snippet := range snippetObjects {
			if snippet != nil {
				snippetStrings[i] = fmt.Sprintf("%v", snippet)
			}
		}
		snippets[field] = snippetStrings
	}
	return snippets, nil
}

func (uh *UnifiedHighlighter) highlightFieldsAsObjects(fieldsIn []string, query search.Query, docIdsIn []int, maxPassagesIn []int) (map[string][]interface{}, error) {
	if len(fieldsIn) < 1 {
		return nil, errors.New("fieldsIn must not be empty")
	}
	if len(fieldsIn) != len(maxPassagesIn) {
		return nil, errors.New("invalid number of maxPassagesIn")
	}
	if uh.searcher == nil {
		return nil, errors.New("This method requires that an indexSearcher was passed in the constructor")
	}

	docIds := make([]int, len(docIdsIn))
	docInIndexes := make([]int, len(docIds))
	copyAndSortDocIdsWithIndex(docIdsIn, docIds, docInIndexes)

	fields := make([]string, len(fieldsIn))
	maxPassages := make([]int, len(maxPassagesIn))
	copyAndSortFieldsWithMaxPassages(fieldsIn, maxPassagesIn, fields, maxPassages)

	queryTerms := extractTerms(query)
	fieldHighlighters := make([]*FieldHighlighter, len(fields))
	numTermVectors := 0
	numPostings := 0
	for f := 0; f < len(fields); f++ {
		fh := uh.getFieldHighlighter(fields[f], query, queryTerms, maxPassages[f])
		fieldHighlighters[f] = fh

		switch fh.GetOffsetSource() {
		case OffsetSourceTermVectors:
			numTermVectors++
		case OffsetSourcePostings:
			numPostings++
		case OffsetSourcePostingsWithTermVectors:
			numTermVectors++
			numPostings++
		}
	}

	cacheCharsThreshold := uh.calculateOptimalCacheCharsThreshold(numTermVectors, numPostings)

	var indexReaderWithTermVecCache index.IndexReaderInterface
	if numTermVectors >= 2 {
		indexReaderWithTermVecCache = uh.searcher.GetIndexReader()
	}

	highlightDocsInByField := make([][]interface{}, len(fields))
	for i := range highlightDocsInByField {
		highlightDocsInByField[i] = make([]interface{}, len(docIds))
	}

	docIdIter := asDocIdSetIterator(docIds)
	for batchDocIdx := 0; batchDocIdx < len(docIds); {
		fieldValsByDoc := uh.loadFieldValues(fields, docIdIter, cacheCharsThreshold)

		for fieldIdx := 0; fieldIdx < len(fields); fieldIdx++ {
			resultByDocIn := highlightDocsInByField[fieldIdx]
			fh := fieldHighlighters[fieldIdx]
			for docIdx := batchDocIdx; docIdx-batchDocIdx < len(fieldValsByDoc); docIdx++ {
				docId := docIds[docIdx]
				content := fieldValsByDoc[docIdx-batchDocIdx][fieldIdx]
				if content == "" {
					continue
				}

				indexReader := uh.searcher.GetIndexReader()
				if fh.GetOffsetSource() == OffsetSourceTermVectors && indexReaderWithTermVecCache != nil {
					indexReader = indexReaderWithTermVecCache
				}

				var leafReader index.LeafReader
				if asLeaf, ok := indexReader.(index.LeafReader); ok {
					leafReader = asLeaf
				} else {
					leaves, err := indexReader.Leaves()
					if err != nil {
						return nil, err
					}
					leafCtx := leaves[index.ReaderUtilSubIndexLeaves(docId, leaves)]
					leafReader = leafCtx.LeafReader()
					docId -= leafCtx.DocBase // adjust 'doc' to be within this leaf reader
				}

				docInIndex := docInIndexes[docIdx] // original input order
				snippet, err := fh.HighlightFieldForDoc(leafReader, docId, content)
				if err != nil {
					return nil, err
				}
				resultByDocIn[docInIndex] = snippet
			}
		}
		batchDocIdx += len(fieldValsByDoc)
	}

	resultMap := make(map[string][]interface{})
	for f := 0; f < len(fields); f++ {
		resultMap[fields[f]] = highlightDocsInByField[f]
	}
	return resultMap, nil
}

func (uh *UnifiedHighlighter) calculateOptimalCacheCharsThreshold(numTermVectors, numPostings int) int {
	if numPostings == 0 && numTermVectors == 0 {
		return 0
	} else if numTermVectors >= 2 {
		return 0
	} else {
		return uh.cacheFieldValCharsThreshold
	}
}

func copyAndSortFieldsWithMaxPassages(fieldsIn []string, maxPassagesIn []int, fields []string, maxPassages []int) {
	copy(fields, fieldsIn)
	copy(maxPassages, maxPassagesIn)

	type pair struct {
		f string
		m int
	}
	pairs := make([]pair, len(fields))
	for i := range fields {
		pairs[i] = pair{fields[i], maxPassages[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].f < pairs[j].f
	})
	for i := range pairs {
		fields[i] = pairs[i].f
		maxPassages[i] = pairs[i].m
	}
}

func copyAndSortDocIdsWithIndex(docIdsIn []int, docIds []int, docInIndexes []int) {
	copy(docIds, docIdsIn)
	for i := range docInIndexes {
		docInIndexes[i] = i
	}

	type pair struct {
		id  int
		idx int
	}
	pairs := make([]pair, len(docIds))
	for i := range docIds {
		pairs[i] = pair{docIds[i], docInIndexes[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].id < pairs[j].id
	})
	for i := range pairs {
		docIds[i] = pairs[i].id
		docInIndexes[i] = pairs[i].idx
	}
}

func (uh *UnifiedHighlighter) HighlightWithoutSearcher(field string, query search.Query, content string, maxPassages int) (interface{}, error) {
	if uh.searcher != nil {
		return nil, errors.New("highlightWithoutSearcher should only be called on a UnifiedHighlighter without an IndexSearcher")
	}
	if content == "" {
		return nil, errors.New("content is required")
	}
	queryTerms := extractTerms(query)
	return uh.getFieldHighlighter(field, query, queryTerms, maxPassages).HighlightFieldForDoc(nil, -1, content)
}

func (uh *UnifiedHighlighter) getFieldHighlighter(field string, query search.Query, allTerms map[*index.Term]struct{}, maxPassages int) *FieldHighlighter {
	maskedFields := uh.getMaskedFields(field)
	var fieldOffsetStrategy FieldOffsetStrategy
	if len(maskedFields) == 0 {
		components := uh.getHighlightComponents(field, query, allTerms)
		offsetSource := uh.getOptimizedOffsetSource(components)
		fieldOffsetStrategy = uh.getOffsetStrategy(offsetSource, components)
	} else {
		strategies := make([]FieldOffsetStrategy, 0, len(maskedFields)+1)
		for _, maskedField := range maskedFields {
			components := uh.getHighlightComponents(maskedField, query, allTerms)
			offsetSource := uh.getOptimizedOffsetSource(components)
			strategies = append(strategies, uh.getOffsetStrategy(offsetSource, components))
		}
		components := uh.getHighlightComponents(field, query, allTerms)
		offsetSource := uh.getOptimizedOffsetSource(components)
		strategies = append(strategies, uh.getOffsetStrategy(offsetSource, components))
		fieldOffsetStrategy = NewMultiFieldsOffsetStrategy(strategies)
	}

	return NewFieldHighlighter(
		field,
		fieldOffsetStrategy,
		NewSplittingBreakIterator(uh.getBreakIterator(field), MultivalSepChar),
		uh.getScorer(field),
		maxPassages,
		uh.getMaxNoHighlightPassages(field),
		uh.getFormatter(field),
		uh.getPassageSortComparator(field),
	)
}

func (uh *UnifiedHighlighter) getHighlightComponents(field string, query search.Query, allTerms map[*index.Term]struct{}) *UHComponents {
	fieldMatcher := uh.getFieldMatcher(field)
	highlightFlags := uh.getFlags(field)
	phraseHelper := uh.getPhraseHelper(field, query, highlightFlags)
	queryHasUnrecognizedPart := uh.hasUnrecognizedQuery(fieldMatcher, query)

	var terms []*util.BytesRef
	var automata []*LabelledCharArrayMatcher

	_, hasWeightMatches := highlightFlags[HighlightFlagWeightMatches]
	if !hasWeightMatches || !queryHasUnrecognizedPart {
		terms = filterExtractedTerms(fieldMatcher, allTerms)
		automata = uh.getAutomata(field, query, highlightFlags)
	}

	return NewUHComponents(
		field,
		fieldMatcher,
		query,
		terms,
		phraseHelper,
		automata,
		queryHasUnrecognizedPart,
		highlightFlags,
	)
}

// hasUnrecognizedQuery reports whether part of the query, other than the
// extracted terms and automata, is a leaf the highlighter does not know.
// Mirrors UnifiedHighlighter.hasUnrecognizedQuery(Predicate, Query).
func (uh *UnifiedHighlighter) hasUnrecognizedQuery(fieldMatcher func(string) bool, query search.Query) bool {
	v := &unrecognizedQueryVisitor{fieldMatcher: fieldMatcher}
	query.Visit(v)
	return v.hasUnknownLeaf
}

// unrecognizedQueryVisitor renders the anonymous QueryVisitor Java declares
// inside UnifiedHighlighter.hasUnrecognizedQuery.
type unrecognizedQueryVisitor struct {
	search.EmptyQueryVisitorBase
	fieldMatcher   func(string) bool
	hasUnknownLeaf bool
}

// AcceptField mirrors the anonymous visitor's acceptField: checking
// hasUnknownLeaf is a trick to exit early.
func (v *unrecognizedQueryVisitor) AcceptField(field string) bool {
	return !v.hasUnknownLeaf && v.fieldMatcher(field)
}

// GetSubVisitor keeps the walk on this visitor, as QueryVisitor's default
// body does.
func (v *unrecognizedQueryVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	return v
}

// VisitLeaf mirrors the anonymous visitor's visitLeaf.
func (v *unrecognizedQueryVisitor) VisitLeaf(query search.Query) {
	if !CanExtractAutomataFromLeafQuery(query) {
		if _, ok := query.(*search.MatchAllDocsQuery); !ok {
			if _, ok := query.(*search.MatchNoDocsQuery); !ok {
				v.hasUnknownLeaf = true
			}
		}
	}
}

// filterExtractedTerms strips the field off every matching term and sorts the
// remaining bytes. Mirrors
// UnifiedHighlighter.filterExtractedTerms(Predicate, Set)
// (UnifiedHighlighter.java:1207).
func filterExtractedTerms(fieldMatcher func(string) bool, queryTerms map[*index.Term]struct{}) []*util.BytesRef {
	// Strip off the redundant field and sort the remaining terms
	var filteredTerms []*util.BytesRef
	for term := range queryTerms {
		if fieldMatcher(term.Field) {
			filteredTerms = append(filteredTerms, term.Bytes)
		}
	}
	sort.Slice(filteredTerms, func(i, j int) bool {
		return util.BytesRefCompare(filteredTerms[i], filteredTerms[j]) < 0
	})
	return filteredTerms
}

func (uh *UnifiedHighlighter) getPhraseHelper(field string, query search.Query, highlightFlags map[HighlightFlag]struct{}) *PhraseHelper {
	_, useWeightMatchesIter := highlightFlags[HighlightFlagWeightMatches]
	if useWeightMatchesIter {
		return NONE // will be handled by Weight.matches which always considers phrases
	}

	_, highlightPhrasesStrictly := highlightFlags[HighlightFlagPhrases]
	_, handleMultiTermQuery := highlightFlags[HighlightFlagMultiTermQuery]

	if highlightPhrasesStrictly {
		return NewPhraseHelper(
			query,
			field,
			uh.getFieldMatcher(field),
			uh.requiresRewrite,
			uh.preSpanQueryRewrite,
			!handleMultiTermQuery,
		)
	}
	return NONE
}

func (uh *UnifiedHighlighter) getAutomata(field string, query search.Query, highlightFlags map[HighlightFlag]struct{}) []*LabelledCharArrayMatcher {
	_, hasPhrases := highlightFlags[HighlightFlagPhrases]
	_, hasWeightMatches := highlightFlags[HighlightFlagWeightMatches]

	lookInSpan := !hasPhrases || hasWeightMatches

	_, handleMTQ := highlightFlags[HighlightFlagMultiTermQuery]
	if handleMTQ {
		return ExtractAutomata(query, uh.getFieldMatcher(field), lookInSpan)
	}
	return nil
}

func (uh *UnifiedHighlighter) getOptimizedOffsetSource(components *UHComponents) OffsetSource {
	offsetSource := uh.getOffsetSource(components.Field)

	mtqOrRewrite := components.Automata == nil || len(components.Automata) > 0 ||
		(components.PhraseHelper != nil && components.PhraseHelper.WillRewrite()) ||
		components.HasUnrecognizedQueryPart

	if !mtqOrRewrite && components.Terms != nil && len(components.Terms) == 0 {
		return OffsetSourceNoneNeeded
	}

	switch offsetSource {
	case OffsetSourcePostings:
		if mtqOrRewrite {
			return OffsetSourceAnalysis
		}
	case OffsetSourcePostingsWithTermVectors:
		if !mtqOrRewrite {
			return OffsetSourcePostings
		}
	}
	return offsetSource
}

func (uh *UnifiedHighlighter) getOffsetStrategy(offsetSource OffsetSource, components *UHComponents) FieldOffsetStrategy {
	switch offsetSource {
	case OffsetSourceAnalysis:
		if components.PhraseHelper == nil || !components.PhraseHelper.HasPositionSensitivity() {
			_, hasRelevancy := components.HighlightFlags[HighlightFlagPassageRelevancyOverSpeed]
			_, hasWeightMatches := components.HighlightFlags[HighlightFlagWeightMatches]
			if !hasRelevancy && !hasWeightMatches {
				return NewTokenStreamOffsetStrategy(components, uh.indexAnalyzer)
			}
		}
		return NewMemoryIndexOffsetStrategy(components, uh.indexAnalyzer)
	case OffsetSourceNoneNeeded:
		return NoOpOffsetStrategyINSTANCE
	case OffsetSourceTermVectors:
		return NewTermVectorOffsetStrategy(components)
	case OffsetSourcePostings:
		return NewPostingsOffsetStrategy(components)
	case OffsetSourcePostingsWithTermVectors:
		return NewPostingsWithTermVectorsOffsetStrategy(components)
	}
	panic("Unrecognized offset source")
}

func (uh *UnifiedHighlighter) requiresRewrite(spanQuery spans.SpanQuery) *bool {
	return nil
}

func (uh *UnifiedHighlighter) preSpanQueryRewrite(query search.Query) []search.Query {
	return nil
}

func asDocIdSetIterator(sortedDocIds []int) *docIdSetIterator {
	return &docIdSetIterator{sortedDocIds: sortedDocIds, idx: -1}
}

type docIdSetIterator struct {
	sortedDocIds []int
	idx          int
}

func (d *docIdSetIterator) DocID() int {
	if d.idx < 0 || d.idx >= len(d.sortedDocIds) {
		return -1
	}
	return d.sortedDocIds[d.idx]
}

func (d *docIdSetIterator) NextDoc() int {
	d.idx++
	return d.DocID()
}

func (d *docIdSetIterator) Cost() int64 {
	cost := int64(len(d.sortedDocIds) - (d.idx + 1))
	if cost < 0 {
		return 0
	}
	return cost
}

func (uh *UnifiedHighlighter) loadFieldValues(fields []string, docIter *docIdSetIterator, cacheCharsThreshold int) [][]string {
	var docListOfFields [][]string

	visitor := uh.newLimitedStoredFieldVisitor(fields)
	storedFields, err := uh.searcher.StoredFields()
	if err != nil {
		return nil
	}
	sumChars := 0

	for {
		docId := docIter.NextDoc()
		if docId == -1 {
			break
		}
		visitor.Init()
		storedFields.Document(docId, visitor)
		valuesByField := visitor.GetValuesByField()
		docListOfFields = append(docListOfFields, valuesByField)
		for _, val := range valuesByField {
			sumChars += len(val)
		}
		if cacheCharsThreshold == 0 || sumChars > cacheCharsThreshold {
			break
		}
	}
	return docListOfFields
}

func (uh *UnifiedHighlighter) newLimitedStoredFieldVisitor(fields []string) *LimitedStoredFieldVisitor {
	return &LimitedStoredFieldVisitor{
		fields:         fields,
		valueSeparator: MultivalSepChar,
		maxLength:      uh.maxLength,
	}
}

type LimitedStoredFieldVisitor struct {
	fields         []string
	valueSeparator rune
	maxLength      int
	values         []string
	currentField   int
}

func (v *LimitedStoredFieldVisitor) Init() {
	v.values = make([]string, len(v.fields))
	v.currentField = -1
}

// StringField renders LimitedStoredFieldVisitor.stringField(FieldInfo, String)
// (UnifiedHighlighter.java:1439-1468). The field selection has already been
// performed by NeedsField, which the stored-fields reader invokes first.
func (v *LimitedStoredFieldVisitor) StringField(fieldInfo *index.FieldInfo, value string) error {
	if v.currentField < 0 {
		return errors.New("uhighlight: LimitedStoredFieldVisitor.StringField called before NeedsField selected a field")
	}

	curValue := v.values[v.currentField]
	if curValue == "" {
		limit := v.maxLength
		if len(value) < limit {
			limit = len(value)
		}
		v.values[v.currentField] = value[:limit]
		return nil
	}

	lengthBudget := v.maxLength - len(curValue)
	if lengthBudget <= 0 {
		return nil
	}

	sep := string(v.valueSeparator)
	valToAppend := value
	if len(value) > lengthBudget-1 {
		valToAppend = value[:lengthBudget-1]
	}
	v.values[v.currentField] = curValue + sep + valToAppend
	return nil
}

// BinaryField renders StoredFieldVisitor.binaryField(FieldInfo, byte[]), whose
// body in Java is empty because LimitedStoredFieldVisitor does not override it.
func (v *LimitedStoredFieldVisitor) BinaryField(*index.FieldInfo, []byte) error { return nil }

// IntField renders the empty StoredFieldVisitor.intField(FieldInfo, int).
func (v *LimitedStoredFieldVisitor) IntField(*index.FieldInfo, int) error { return nil }

// LongField renders the empty StoredFieldVisitor.longField(FieldInfo, long).
func (v *LimitedStoredFieldVisitor) LongField(*index.FieldInfo, int64) error { return nil }

// FloatField renders the empty StoredFieldVisitor.floatField(FieldInfo, float).
func (v *LimitedStoredFieldVisitor) FloatField(*index.FieldInfo, float32) error { return nil }

// DoubleField renders the empty StoredFieldVisitor.doubleField(FieldInfo, double).
func (v *LimitedStoredFieldVisitor) DoubleField(*index.FieldInfo, float64) error { return nil }

// NeedsField renders LimitedStoredFieldVisitor.needsField(FieldInfo)
// (UnifiedHighlighter.java:1470-1481): the field name is binary-searched in
// the requested fields; a field already filled to maxLength yields STOP when
// only one field was requested and NO otherwise.
func (v *LimitedStoredFieldVisitor) NeedsField(fieldInfo *index.FieldInfo) (index.StoredFieldVisitorStatus, error) {
	name := fieldInfo.Name()
	v.currentField = sort.SearchStrings(v.fields, name)
	if v.currentField == len(v.fields) || v.fields[v.currentField] != name {
		v.currentField = -1
		return index.StoredFieldVisitorStatusNo, nil
	}
	curVal := v.values[v.currentField]
	if curVal != "" && len(curVal) >= v.maxLength {
		if len(v.fields) == 1 {
			return index.StoredFieldVisitorStatusStop, nil
		}
		return index.StoredFieldVisitorStatusNo, nil
	}
	return index.StoredFieldVisitorStatusYes, nil
}

func (v *LimitedStoredFieldVisitor) GetValuesByField() []string {
	return v.values
}

// extractTerms collects every index term the query exposes. Mirrors
// UnifiedHighlighter.extractTerms(Query) (UnifiedHighlighter.java:481), which
// returns a Set<Term>.
func extractTerms(query search.Query) map[*index.Term]struct{} {
	collector := search.NewTermCollectorVisitor()
	query.Visit(collector)

	terms := make(map[*index.Term]struct{})
	for _, t := range collector.Terms {
		terms[t] = struct{}{}
	}
	return terms
}

func (uh *UnifiedHighlighter) getFieldMatcher(field string) func(string) bool {
	if uh.fieldMatcher != nil {
		return uh.fieldMatcher
	}
	return func(qf string) bool {
		return field == qf
	}
}

func (uh *UnifiedHighlighter) getMaskedFields(field string) []string {
	if uh.maskedFieldsFunc == nil {
		return nil
	}
	return uh.maskedFieldsFunc(field)
}

func (uh *UnifiedHighlighter) getFlags(field string) map[HighlightFlag]struct{} {
	return uh.flags
}

func (uh *UnifiedHighlighter) getBreakIterator(field string) BreakIterator {
	return uh.breakIterator()
}

func (uh *UnifiedHighlighter) getScorer(field string) *PassageScorer {
	return uh.scorer
}

func (uh *UnifiedHighlighter) getFormatter(field string) PassageFormatter {
	return uh.formatter
}

func (uh *UnifiedHighlighter) getPassageSortComparator(field string) func(p1, p2 *Passage) int {
	return uh.passageSortComparator
}

func (uh *UnifiedHighlighter) getMaxNoHighlightPassages(field string) int {
	return uh.maxNoHighlightPassages
}

func (uh *UnifiedHighlighter) getOffsetSource(field string) OffsetSource {
	fieldInfo := uh.getFieldInfo(field)
	if fieldInfo != nil {
		if fieldInfo.IndexOptions() == index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets {
			if fieldInfo.HasTermVectors() {
				return OffsetSourcePostingsWithTermVectors
			}
			return OffsetSourcePostings
		}
		if fieldInfo.HasTermVectors() {
			return OffsetSourceTermVectors
		}
	}
	return OffsetSourceAnalysis
}

func (uh *UnifiedHighlighter) getFieldInfo(field string) *index.FieldInfo {
	if uh.searcher == nil {
		return nil
	}
	uh.fieldInfosMu.Lock()
	defer uh.fieldInfosMu.Unlock()
	if uh.fieldInfos == nil {
		fieldInfos, err := index.FieldInfosGetMergedFieldInfos(uh.searcher.GetIndexReader())
		if err != nil {
			return nil
		}
		uh.fieldInfos = fieldInfos
	}
	return uh.fieldInfos.FieldInfoByName(field)
}
