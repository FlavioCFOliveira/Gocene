package uhighlight

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
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
	scorer                      PassageScorer
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
	scorer                      PassageScorer
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
		breakIterator:               func() BreakIterator { return NewSentenceBreakIterator() },
		scorer:                      NewPassageScorer(),
		formatter:                   NewDefaultPassageFormatter(),
		maxNoHighlightPassages:      defaultMaxHighlightPassages,
		cacheFieldValCharsThreshold: defaultCacheFieldValCharsThreshold,
		passageSortComparator: func(p1, p2 *Passage) int {
			if p1.StartOffset < p2.StartOffset {
				return -1
			} else if p1.StartOffset > p2.StartOffset {
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

func (b *Builder) WithScorer(value PassageScorer) *Builder {
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

func (uh *UnifiedHighlighter) Highlight(field string, query search.Query, topDocs *search.TopDocs) ([]string, error) {
	return uh.Highlight(field, query, topDocs, 1)
}

func (uh *UnifiedHighlighter) Highlight(field string, query search.Query, topDocs *search.TopDocs, maxPassages int) ([]string, error) {
	res, err := uh.HighlightFields([]string{field}, query, topDocs, []int{maxPassages})
	if err != nil {
		return nil, err
	}
	return res[field], nil
}

func (uh *UnifiedHighlighter) HighlightFields(fields []string, query search.Query, topDocs *search.TopDocs) (map[string][]string, error) {
	maxPassages := make([]int, len(fields))
	for i := range maxPassages {
		maxPassages[i] = 1
	}
	return uh.HighlightFields(fields, query, topDocs, maxPassages)
}

func (uh *UnifiedHighlighter) HighlightFields(fields []string, query search.Query, topDocs *search.TopDocs, maxPassages []int) (map[string][]string, error) {
	scoreDocs := topDocs.ScoreDocs
	docids := make([]int, len(scoreDocs))
	for i := range docids {
		docids[i] = scoreDocs[i].Doc
	}
	return uh.HighlightFields(fields, query, docids, maxPassages)
}

func (uh *UnifiedHighlighter) HighlightFields(fieldsIn []string, query search.Query, docidsIn []int, maxPassagesIn []int) (map[string][]string, error) {
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

	var indexReaderWithTermVecCache *index.IndexReader
	if numTermVectors >= 2 {
		indexReaderWithTermVecCache = uh.searcher.IndexReader()
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
				if content == nil {
					continue
				}

				indexReader := uh.searcher.IndexReader()
				if fh.GetOffsetSource() == OffsetSourceTermVectors && indexReaderWithTermVecCache != nil {
					indexReader = indexReaderWithTermVecCache
				}

				var leafReader index.LeafReader
				leaves, err := indexReader.Leaves()
				if err != nil {
					return nil, err
				}
				leafCtx := leaves[index.ReaderUtilSubIndex(docId, leaves)]
				leafReader = leafCtx.Reader()
				adjDocId := docId - leafCtx.DocBase

				docInIndex := docInIndexes[docIdx]
				var docContext any
				switch fh.GetOffsetSource() {
				case OffsetSourcePostings:
					docContext = uh.buildPostingsDocContext(leafReader, adjDocId, fields[fieldIdx], queryTerms)
				case OffsetSourcePostingsWithTermVectors:
					docContext = uh.buildPostingsDocContext(leafReader, adjDocId, fields[fieldIdx], queryTerms)
				default:
					docContext = nil
				}
				snippet, err := fh.HighlightFieldForDoc(docContext, string(content))
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

func (uh *UnifiedHighlighter) buildPostingsDocContext(leafReader index.LeafReader, docId int, field string, terms map[*util.BytesRef]struct{}) *PostingsDocContext {
	ctx := &PostingsDocContext{
		TermFreqsInDoc: make(map[string]int),
	}
	for term := range terms {
		pe, err := leafReader.GetPostings(field, term)
		if err != nil || pe == nil {
			continue
		}
		defer pe.Close()
		if !pe.Advance(docId) {
			continue
		}

		freq := pe.Freq()
		ctx.TermFreqsInDoc[term.String()] = freq

		startOffsets := pe.StartOffsets()
		endOffsets := pe.EndOffsets()
		if len(startOffsets) == 0 || len(endOffsets) == 0 {
			continue
		}

		entry := PostingsEntry{
			Term:         term.String(),
			StartOffsets: startOffsets,
			EndOffsets:   endOffsets,
		}
		ctx.Entries = append(ctx.Entries, entry)
	}
	return ctx
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
	return uh.getFieldHighlighter(field, query, queryTerms, maxPassages).HighlightFieldForDoc(nil, -1, content), nil
}

func (uh *UnifiedHighlighter) getFieldHighlighter(field string, query search.Query, allTerms map[*util.BytesRef]struct{}, maxPassages int) *FieldHighlighter {
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

func (uh *UnifiedHighlighter) getHighlightComponents(field string, query search.Query, allTerms map[*util.BytesRef]struct{}) *UHComponents {
	fieldMatcher := uh.getFieldMatcher(field)
	highlightFlags := uh.getFlags(field)
	phraseHelper := uh.getPhraseHelper(field, query, highlightFlags)
	queryHasUnrecognizedPart := uh.hasUnrecognizedQuery(fieldMatcher, query)

	var terms []*util.BytesRef
	var automata []LabelledCharArrayMatcher

	_, hasWeightMatches := highlightFlags[HighlightFlagWeightMatches]
	if !hasWeightMatches || !queryHasUnrecognizedPart {
		terms = filterExtractedTerms(fieldMatcher, allTerms)
		automata = uh.getAutomata(field, query, highlightFlags)
	}

	return &UHComponents{
		Field:                     field,
		FieldMatcher:              fieldMatcher,
		Query:                     query,
		Terms:                     terms,
		PhraseHelper:              phraseHelper,
		Automata:                  automata,
		QueryHasUnrecognizedQuery: queryHasUnrecognizedPart,
		HighlightFlags:            highlightFlags,
	}
}

func (uh *UnifiedHighlighter) hasUnrecognizedQuery(fieldMatcher func(string) bool, query search.Query) bool {
	hasUnknownLeaf := false
	query.Visit(func(field string) bool {
		if hasUnknownLeaf {
			return false
		}
		return fieldMatcher(field)
	}, func(q search.Query) {
		if !CanExtractAutomataFromLeafQuery(q) {
			if _, ok := q.(*search.MatchAllDocsQuery); !ok {
				if _, ok := q.(*search.MatchNoDocsQuery); !ok {
					hasUnknownLeaf = true
				}
			}
		}
	})
	return hasUnknownLeaf
}

func filterExtractedTerms(fieldMatcher func(string) bool, queryTerms map[*util.BytesRef]struct{}) []*util.BytesRef {
	var filtered []*util.BytesRef
	for term := range queryTerms {
		if fieldMatcher(term.String()) {
			filtered = append(filtered, term)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Compare(filtered[j]) < 0
	})
	return filtered
}

func (uh *UnifiedHighlighter) getPhraseHelper(field string, query search.Query, highlightFlags map[HighlightFlag]struct{}) *PhraseHelper {
	_, useWeightMatchesIter := highlightFlags[HighlightFlagWeightMatches]
	if useWeightMatchesIter {
		return nil
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
	return nil
}

func (uh *UnifiedHighlighter) getAutomata(field string, query search.Query, highlightFlags map[HighlightFlag]struct{}) []LabelledCharArrayMatcher {
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
		components.QueryHasUnrecognizedQuery

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
		return NoOpOffsetStrategy{}
	case OffsetSourceTermVectors:
		return NewTermVectorOffsetStrategy(components)
	case OffsetSourcePostings:
		return NewPostingsOffsetStrategy(components)
	case OffsetSourcePostingsWithTermVectors:
		return NewPostingsWithTermVectorsOffsetStrategy(components)
	}
	panic("Unrecognized offset source")
}

func (uh *UnifiedHighlighter) requiresRewrite(spanQuery search.Query) *bool {
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

func (v *LimitedStoredFieldVisitor) StringField(fieldInfo index.FieldInfo, value string) {
	if v.currentField < 0 {
		return
	}
	if value == "" {
		return
	}

	curValue := v.values[v.currentField]
	if curValue == "" {
		limit := v.maxLength
		if len(value) < limit {
			limit = len(value)
		}
		v.values[v.currentField] = value[:limit]
		return
	}

	lengthBudget := v.maxLength - len(curValue)
	if lengthBudget <= 0 {
		return
	}

	sep := string(v.valueSeparator)
	valToAppend := value
	if len(value) > lengthBudget-1 {
		valToAppend = value[:lengthBudget-1]
	}
	v.values[v.currentField] = curValue + sep + valToAppend
}

func (v *LimitedStoredFieldVisitor) NeedsField(fieldInfo index.FieldInfo) bool {
	v.currentField = sort.SearchStrings(v.fields, fieldInfo.Name())
	if v.currentField == len(v.fields) || v.fields[v.currentField] != fieldInfo.Name() {
		return false
	}
	curVal := v.values[v.currentField]
	if curVal != "" && len(curVal) >= v.maxLength {
		return false
	}
	return true
}

func (v *LimitedStoredFieldVisitor) GetValuesByField() []string {
	return v.values
}

func extractTerms(query search.Query) map[*util.BytesRef]struct{} {
	collector := search.NewTermCollectorVisitor()
	query.Visit(collector)

	terms := make(map[*util.BytesRef]struct{})
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

func (uh *UnifiedHighlighter) getScorer(field string) PassageScorer {
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
		if fieldInfo.IndexOptions == index.DocsAndFreqsAndPositionsAndOffsets {
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
		uh.fieldInfos = uh.searcher.IndexReader().GetFieldInfos()
	}
	return uh.fieldInfos.FieldInfoByName(field)
}
