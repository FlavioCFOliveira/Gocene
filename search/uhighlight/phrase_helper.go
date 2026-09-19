package uhighlight

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PhraseHelper helps the FieldOffsetStrategy with position sensitive queries.
// Mirrors org.apache.lucene.search.uhighlight.PhraseHelper.
type PhraseHelper struct {
	fieldName                string
	positionInsensitiveTerms map[string]bool
	spanQueries              map[spans.SpanQuery]bool
	willRewrite              bool
	fieldMatcher             func(string) bool
}

// NONE is a sentinel PhraseHelper for queries with no position sensitivity.
var NONE = &PhraseHelper{
	fieldName:                "_ignored_",
	positionInsensitiveTerms: make(map[string]bool),
	spanQueries:              make(map[spans.SpanQuery]bool),
	willRewrite:              true,
	fieldMatcher:             func(s string) bool { return false },
}

// NewPhraseHelper builds the helper based on the provided query and field.
func NewPhraseHelper(
	query search.Query,
	field string,
	fieldMatcher func(string) bool,
	rewriteQueryPred func(spans.SpanQuery) *bool,
	preExtractRewriteFunction func(search.Query) []search.Query,
	ignoreQueriesNeedingRewrite bool,
) *PhraseHelper {
	ph := &PhraseHelper{
		fieldName:                field,
		fieldMatcher:             fieldMatcher,
		positionInsensitiveTerms: make(map[string]bool),
		spanQueries:              make(map[spans.SpanQuery]bool),
	}

	mustRewriteHolder := false

	extractor := NewWeightedSpanTermExtractor(field)
	extractor.SetExpandMultiTermQuery(true)

	// Internal extraction logic using an anonymous-like wrapper.
	// Since Go doesn't have anonymous classes, we'll implement the custom extractor logic
	// by wrapping the base extractor.

	// We need a way to intercept extractWeightedTerms and extractWeightedSpanTerms.
	// I'll implement a wrapper or just do the extraction here.

	// Let's implement the extraction logic directly using a modified version of the extractor
	// or a helper function.

	// Actually, I'll implement a internal function that mirrors the Lucene anonymous class logic.
	err := extractForPhraseHelper(extractor, query, fieldMatcher, preExtractRewriteFunction,
		func(sq spans.SpanQuery) bool {
			res := rewriteQueryPred(sq)
			if res != nil {
				return *res
			}
			return extractor.mustRewriteQuery(sq)
		},
		func(sq spans.SpanQuery) {
			// If this span query isn't for this field, skip it.
			fieldNames := make(map[string]bool)
			extractor.collectSpanQueryFields(sq, fieldNames)
			for fn := range fieldNames {
				if !fieldMatcher(fn) {
					return
				}
			}

			mustRewrite := extractor.mustRewriteQuery(sq)
			if ignoreQueriesNeedingRewrite && mustRewrite {
				return
			}
			if mustRewrite {
				mustRewriteHolder = true
			}
			ph.spanQueries[sq] = true
		},
		func(termBytes []byte) {
			ph.positionInsensitiveTerms[string(termBytes)] = true
		},
		&mustRewriteHolder)

	if err != nil {
		// In Lucene this is wrapped in a RuntimeException.
		panic(err)
	}

	ph.willRewrite = mustRewriteHolder
	return ph
}

func extractForPhraseHelper(
	extractor *WeightedSpanTermExtractor,
	query search.Query,
	fieldMatcher func(string) bool,
	preExtractRewriteFunction func(search.Query) []search.Query,
	rewriteQueryPred func(spans.SpanQuery) bool,
	onSpanQuery func(spans.SpanQuery),
	onWeightedTerm func([]byte),
	mustRewriteHolder *bool,
) error {
	// We need to implement the recursive extraction.
	var extract func(search.Query, float32) error
	extract = func(q search.Query, boost float32) error {
		if bq, ok := q.(*search.BoostQuery); ok {
			return extract(bq.Query(), boost*bq.Boost())
		}

		if bq, ok := q.(*search.BooleanQuery); ok {
			for _, clause := range bq.Clauses() {
				if !clause.IsProhibited() {
					if err := extract(clause.Query(), boost); err != nil {
						return err
					}
				}
			}
			return nil
		}

		if pq, ok := q.(*search.PhraseQuery); ok {
			terms := pq.GetTerms()
			if len(terms) == 1 {
				sq := spans.NewSpanTermQuery(terms[0])
				onSpanQuery(sq)
				return nil
			}
			// Convert PhraseQuery to SpanNearQuery as Lucene does.
			clauses := make([]spans.SpanQuery, len(terms))
			for i, term := range terms {
				clauses[i] = spans.NewSpanTermQuery(term)
			}
			posGaps := 0
			positions := pq.GetPositions()
			if len(positions) >= 2 {
				posGaps = positions[len(positions)-1] - positions[0] - len(positions) + 1
				if posGaps < 0 {
					posGaps = 0
				}
			}
			inOrder := (pq.GetSlop() == 0)
			sp, err := spans.NewSpanNearQuery(clauses, pq.GetSlop()+posGaps, inOrder)
			if err != nil {
				return err
			}
			onSpanQuery(sp)
			return nil
		}

		if idv, ok := q.(*search.IndexOrDocValuesQuery); ok {
			iq := idv.GetIndexQuery()
			if iq != nil {
				return extract(iq, boost)
			}
			return nil
		}

		if _, ok := q.(*search.TermQuery); ok {
			q.Visit(newPositionInsensitiveTermVisitor(fieldMatcher, onWeightedTerm))
			return nil
		}
		if _, ok := q.(*search.SynonymQuery); ok {
			q.Visit(newPositionInsensitiveTermVisitor(fieldMatcher, onWeightedTerm))
			return nil
		}

		if sq, ok := q.(spans.SpanQuery); ok {
			onSpanQuery(sq)
			return nil
		}

		if csq, ok := q.(*search.ConstantScoreQuery); ok {
			inner := csq.GetQuery()
			if inner != nil {
				return extract(inner, boost)
			}
			return nil
		}

		if ctq, ok := q.(*queries.CommonTermsQuery); ok {
			ctq.Visit(newPositionInsensitiveTermVisitor(fieldMatcher, onWeightedTerm))
			return nil
		}

		if dmq, ok := q.(*search.DisjunctionMaxQuery); ok {
			for _, clause := range dmq.Disjuncts() {
				if err := extract(clause, boost); err != nil {
					return err
				}
			}
			return nil
		}

		if mpq, ok := q.(*search.MultiPhraseQuery); ok {
			// MultiPhraseQuery is converted to SpanNearQuery of SpanOrQueries.
			termArrays := mpq.GetTermArrays()
			positions := mpq.GetPositions()
			if len(positions) > 0 {
				maxPos := positions[len(positions)-1]
				for _, p := range positions {
					if p > maxPos {
						maxPos = p
					}
				}
				disjunctLists := make([][]spans.SpanQuery, maxPos+1)
				distinctPos := 0
				for i, termArray := range termArrays {
					pos := positions[i]
					if disjunctLists[pos] == nil {
						disjunctLists[pos] = make([]spans.SpanQuery, 0, len(termArray))
						distinctPos++
					}
					for _, term := range termArray {
						disjunctLists[pos] = append(disjunctLists[pos], spans.NewSpanTermQuery(term))
					}
				}
				posGaps := 0
				posIdx := 0
				clauses := make([]spans.SpanQuery, distinctPos)
				for _, disjuncts := range disjunctLists {
					if disjuncts != nil {
						or, err := spans.NewSpanOrQuery(disjuncts...)
						if err != nil {
							return err
						}
						clauses[posIdx] = or
						posIdx++
					} else {
						posGaps++
					}
				}
				if len(clauses) == 1 {
					onSpanQuery(clauses[0])
				} else {
					slop := mpq.GetSlop()
					inOrder := (slop == 0)
					sp, err := spans.NewSpanNearQuery(clauses, slop+posGaps, inOrder)
					if err != nil {
						return err
					}
					onSpanQuery(sp)
				}
			}
			return nil
		}

		if _, ok := q.(*search.MatchAllDocsQuery); ok {
			return nil
		}

		if fsq, ok := q.(*function.FunctionScoreQuery); ok {
			return extract(fsq.GetWrappedQuery(), boost)
		}

		// PhraseHelper overrides isQueryUnsupported to answer true for every
		// class (MTQ is processed separately in MultiTermHighlighting.java), so
		// Java never reaches the rewrite fallback below from here. The fallback
		// is kept because WeightedSpanTermExtractor.extract declares it.
		if _, ok := q.(*search.MultiTermQuery); ok {
			return nil
		}

		// Final fallback: rewrite and recurse.
		leafContext, err := extractor.getLeafContext()
		if err != nil {
			return err
		}
		defer extractor.closeInternalReader()
		reader := leafContext.Reader()

		var rewritten search.Query
		if mtq, ok := q.(*search.MultiTermQuery); ok {
			rewritten, err = search.ScoringBooleanRewrite.Rewrite(search.NewIndexSearcher(reader), mtq)
		} else {
			rewritten, err = q.Rewrite(search.NewIndexSearcher(reader))
		}
		if err != nil {
			return err
		}

		if rewritten != q {
			return extract(rewritten, boost)
		}

		return nil
	}

	return extract(query, 1.0)
}

// GetSpanQueries renders PhraseHelper.getSpanQueries()
// (PhraseHelper.java:201), which returns the Set<SpanQuery> field itself.
func (p *PhraseHelper) GetSpanQueries() map[spans.SpanQuery]bool {
	return p.spanQueries
}

// HasPositionSensitivity reports whether any registered term comes from a phrase.
func (p *PhraseHelper) HasPositionSensitivity() bool {
	return len(p.spanQueries) > 0
}

// WillRewrite reports whether the query needs rewriting.
func (p *PhraseHelper) WillRewrite() bool {
	return p.willRewrite
}

// GetAllPositionInsensitiveTerms renders
// PhraseHelper.getAllPositionInsensitiveTerms() (PhraseHelper.java:220),
// which copies the Set<BytesRef> to an array and sorts it.
func (p *PhraseHelper) GetAllPositionInsensitiveTerms() []*util.BytesRef {
	result := make([]*util.BytesRef, 0, len(p.positionInsensitiveTerms))
	for t := range p.positionInsensitiveTerms {
		result = append(result, util.NewBytesRef([]byte(t)))
	}
	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(bytesRefValue(result[i]), bytesRefValue(result[j])) < 0
	})
	return result
}

// CreateOffsetsEnumsForSpans produces a number of OffsetsEnum into the results
// param. Mirrors
// PhraseHelper.createOffsetsEnumsForSpans(LeafReader, int, List<OffsetsEnum>)
// (PhraseHelper.java:240). Java appends to the caller's List; Go slices are
// values, so results is a pointer to the caller's slice.
func (p *PhraseHelper) CreateOffsetsEnumsForSpans(leafReader index.LeafReader, docID int, results *[]OffsetsEnum) error {
	// wrap reader to a single field.
	reader := newSingleFieldWithOffsetsFilterLeafReader(leafReader, p.fieldName)
	searcher := search.NewIndexSearcher(reader)
	searcher.SetQueryCache(nil)

	readerContext, err := reader.GetContext()
	if err != nil {
		return err
	}
	leafContext, ok := readerContext.(*index.LeafReaderContext)
	if !ok {
		return fmt.Errorf("uhighlight: expected a LeafReaderContext, got %T", readerContext)
	}

	// Get the array of matching spans from the spanQueries
	spansPriorityQueue := newSpansPQ(len(p.spanQueries))
	for q := range p.spanQueries {
		rewritten, err := searcher.Rewrite(q)
		if err != nil {
			return err
		}
		weight, err := searcher.CreateWeight(rewritten, search.ScoreModeCompleteNoScores, 1)
		if err != nil {
			return err
		}
		scorer, err := weight.Scorer(leafContext)
		if err != nil {
			return err
		}
		if scorer == nil {
			continue
		}
		if twoPhaseIterator := scorer.TwoPhaseIterator(); twoPhaseIterator != nil {
			doc, err := twoPhaseIterator.Approximation().Advance(docID)
			if err != nil {
				return err
			}
			matches, err := twoPhaseIterator.Matches()
			if err != nil {
				return err
			}
			if doc != docID || !matches {
				continue
			}
		} else {
			doc, err := scorer.Iterator().Advance(docID)
			if err != nil {
				return err
			}
			if doc != docID {
				continue
			}
		}

		spanScorer, ok := scorer.(*spans.SpanScorer)
		if !ok {
			return fmt.Errorf("uhighlight: expected a SpanScorer, got %T", scorer)
		}
		matchSpans := spanScorer.GetSpans()
		start, err := matchSpans.NextStartPosition()
		if err != nil {
			return err
		}
		if start != spans.NoMorePositions {
			spansPriorityQueue.Push(matchSpans)
		}
	}

	// Iterate the Spans in the PriorityQueue, collecting as we go.
	collector := p.newOffsetSpanCollector()
	for spansPriorityQueue.Len() > 0 {
		matchSpans := spansPriorityQueue.Pop()
		if err := matchSpans.Collect(collector); err != nil {
			return err
		}
		start, err := matchSpans.NextStartPosition()
		if err != nil {
			return err
		}
		if start != spans.NoMorePositions {
			spansPriorityQueue.Push(matchSpans)
		}
	}

	for _, oe := range collector.termToOffsetsEnums {
		*results = append(*results, oe)
	}
	return nil
}

// singleFieldWithOffsetsFilterLeafReader restricts a LeafReader to one field
// and forces its postings to carry offsets. Mirrors the private static class
// PhraseHelper.SingleFieldWithOffsetsFilterLeafReader (PhraseHelper.java:301).
type singleFieldWithOffsetsFilterLeafReader struct {
	*index.FilterLeafReader
	fieldName string
}

func newSingleFieldWithOffsetsFilterLeafReader(in index.LeafReader, fieldName string) *singleFieldWithOffsetsFilterLeafReader {
	return &singleFieldWithOffsetsFilterLeafReader{
		FilterLeafReader: index.NewFilterLeafReader(in),
		fieldName:        fieldName,
	}
}

// GetFieldInfos mirrors the override that throws UnsupportedOperationException.
func (r *singleFieldWithOffsetsFilterLeafReader) GetFieldInfos() *index.FieldInfos {
	panic("uhighlight: SingleFieldWithOffsetsFilterLeafReader.getFieldInfos is unsupported")
}

// Terms always reads the single wrapped field and ensures the underlying
// PostingsEnum returns offsets.
func (r *singleFieldWithOffsetsFilterLeafReader) Terms(field string) (index.Terms, error) {
	in, err := r.FilterLeafReader.Terms(r.fieldName)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, nil
	}
	return &offsetForcingTerms{FilterTerms: index.NewFilterTerms(in)}, nil
}

// GetNormValues always reads the single wrapped field.
func (r *singleFieldWithOffsetsFilterLeafReader) GetNormValues(field string) (index.NumericDocValues, error) {
	return r.FilterLeafReader.GetNormValues(r.fieldName)
}

// GetCoreCacheHelper mirrors the override returning null.
func (r *singleFieldWithOffsetsFilterLeafReader) GetCoreCacheHelper() index.CacheHelper { return nil }

// GetReaderCacheHelper mirrors the override returning null.
func (r *singleFieldWithOffsetsFilterLeafReader) GetReaderCacheHelper() index.CacheHelper { return nil }

// offsetForcingTerms renders the anonymous FilterTerms of
// SingleFieldWithOffsetsFilterLeafReader.terms(String).
type offsetForcingTerms struct {
	*index.FilterTerms
}

// Iterator wraps the delegate's TermsEnum so that every postings request asks
// for offsets.
func (t *offsetForcingTerms) Iterator() (index.TermsEnum, error) {
	in, err := t.FilterTerms.Iterator()
	if err != nil {
		return nil, err
	}
	return &offsetForcingTermsEnum{FilterTermsEnum: index.NewFilterTermsEnum(in)}, nil
}

// offsetForcingTermsEnum renders the anonymous FilterTermsEnum of
// SingleFieldWithOffsetsFilterLeafReader.terms(String).
type offsetForcingTermsEnum struct {
	*index.FilterTermsEnum
}

// Postings adds PostingsEnum.OFFSETS to the requested flags. Java's override
// takes the reuse PostingsEnum too; Gocene's index.TermsEnum.Postings does not
// declare that parameter.
func (e *offsetForcingTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return e.FilterTermsEnum.Postings(flags | index.PostingsFlagOffsets)
}

// positionInsensitiveTermVisitor renders the anonymous QueryVisitor declared
// inside the extractWeightedTerms override of PhraseHelper's anonymous
// WeightedSpanTermExtractor (PhraseHelper.java:149).
type positionInsensitiveTermVisitor struct {
	search.EmptyQueryVisitorBase
	fieldMatcher   func(string) bool
	onWeightedTerm func([]byte)
}

func newPositionInsensitiveTermVisitor(fieldMatcher func(string) bool, onWeightedTerm func([]byte)) *positionInsensitiveTermVisitor {
	return &positionInsensitiveTermVisitor{fieldMatcher: fieldMatcher, onWeightedTerm: onWeightedTerm}
}

// AcceptField mirrors the anonymous visitor's acceptField.
func (v *positionInsensitiveTermVisitor) AcceptField(field string) bool {
	return v.fieldMatcher(field)
}

// GetSubVisitor keeps the walk on this visitor, as QueryVisitor's default body
// does.
func (v *positionInsensitiveTermVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	return v
}

// ConsumeTerms mirrors the anonymous visitor's consumeTerms, which adds every
// term's bytes to positionInsensitiveTerms.
func (v *positionInsensitiveTermVisitor) ConsumeTerms(query search.Query, terms ...*index.Term) {
	for _, term := range terms {
		v.onWeightedTerm(term.Bytes.Bytes[term.Bytes.Offset : term.Bytes.Offset+term.Bytes.Length])
	}
}

// Internal priority queue for Spans.
type spansPQ struct {
	data []spans.Spans
}

func newSpansPQ(cap int) *spansPQ {
	return &spansPQ{data: make([]spans.Spans, 0, cap)}
}

func (pq *spansPQ) Len() int { return len(pq.data) }
func (pq *spansPQ) Push(s spans.Spans) {
	pq.data = append(pq.data, s)
	// simplistic sort for now, in real implementation we should use a heap.
	// however, we only care about startPosition.
	for i := len(pq.data) - 1; i > 0 && pq.data[i].StartPosition() < pq.data[i-1].StartPosition(); i-- {
		pq.data[i], pq.data[i-1] = pq.data[i-1], pq.data[i]
	}
}
func (pq *spansPQ) Pop() spans.Spans {
	res := pq.data[0]
	pq.data = pq.data[1:]
	return res
}

// offsetSpanCollector renders the inner class PhraseHelper.OffsetSpanCollector
// (PhraseHelper.java:347).
type offsetSpanCollector struct {
	phraseHelper       *PhraseHelper
	termToOffsetsEnums map[string]*SpanCollectedOffsetsEnum
}

func (p *PhraseHelper) newOffsetSpanCollector() *offsetSpanCollector {
	return &offsetSpanCollector{
		phraseHelper:       p,
		termToOffsetsEnums: make(map[string]*SpanCollectedOffsetsEnum),
	}
}

// CollectLeaf mirrors OffsetSpanCollector.collectLeaf(PostingsEnum, int, Term).
func (c *offsetSpanCollector) CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error {
	if !c.phraseHelper.fieldMatcher(term.Field) {
		return nil
	}
	termBytes := term.Bytes.Bytes[term.Bytes.Offset : term.Bytes.Offset+term.Bytes.Length]
	key := string(termBytes)
	offsetsEnum, ok := c.termToOffsetsEnums[key]
	if !ok {
		// If it's pos insensitive we handle it outside of PhraseHelper.
		// term.field() is from the Query.
		if c.phraseHelper.positionInsensitiveTerms[key] {
			return nil
		}
		freq, err := postings.Freq()
		if err != nil {
			return err
		}
		offsetsEnum = NewSpanCollectedOffsetsEnum(termBytes, freq)
		c.termToOffsetsEnums[key] = offsetsEnum
	}
	startOffset, err := postings.StartOffset()
	if err != nil {
		return err
	}
	endOffset, err := postings.EndOffset()
	if err != nil {
		return err
	}
	offsetsEnum.Add(startOffset, endOffset)
	return nil
}

// Reset mirrors OffsetSpanCollector.reset(): called when at a new position. We
// don't care.
func (c *offsetSpanCollector) Reset() {}

var _ spans.SpanCollector = (*offsetSpanCollector)(nil)

// SpanCollectedOffsetsEnum implements OffsetsEnum for spans.
type SpanCollectedOffsetsEnum struct {
	term         []byte
	startOffsets []int
	endOffsets   []int
	numPairs     int
	enumIdx      int
}

func NewSpanCollectedOffsetsEnum(term []byte, freq int) *SpanCollectedOffsetsEnum {
	return &SpanCollectedOffsetsEnum{
		term:         term,
		startOffsets: make([]int, freq),
		endOffsets:   make([]int, freq),
		enumIdx:      -1,
	}
}

func (oe *SpanCollectedOffsetsEnum) Add(start, end int) {
	pairIdx := oe.numPairs - 1
	for ; pairIdx >= 0; pairIdx-- {
		if oe.startOffsets[pairIdx] == start && oe.endOffsets[pairIdx] == end {
			return
		} else if oe.startOffsets[pairIdx] < start {
			break
		}
	}
	shiftLen := oe.numPairs - (pairIdx + 1)
	if shiftLen > 0 {
		copy(oe.startOffsets[pairIdx+2:], oe.startOffsets[pairIdx+1:])
		copy(oe.endOffsets[pairIdx+2:], oe.endOffsets[pairIdx+1:])
	}
	oe.startOffsets[pairIdx+1] = start
	oe.endOffsets[pairIdx+1] = end
	oe.numPairs++
}

func (oe *SpanCollectedOffsetsEnum) NextPosition() (bool, error) {
	oe.enumIdx++
	return oe.enumIdx < oe.numPairs, nil
}

func (oe *SpanCollectedOffsetsEnum) Freq() (int, error) {
	return oe.numPairs, nil
}

func (oe *SpanCollectedOffsetsEnum) GetTerm() ([]byte, error) {
	return oe.term, nil
}

func (oe *SpanCollectedOffsetsEnum) StartOffset() (int, error) {
	return oe.startOffsets[oe.enumIdx], nil
}

func (oe *SpanCollectedOffsetsEnum) EndOffset() (int, error) {
	return oe.endOffsets[oe.enumIdx], nil
}

// Close mirrors OffsetsEnum.close(), whose body is empty and which
// SpanCollectedOffsetsEnum does not override.
func (oe *SpanCollectedOffsetsEnum) Close() error { return nil }

var _ OffsetsEnum = (*SpanCollectedOffsetsEnum)(nil)
