package uhighlight

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PhraseHelper helps the FieldOffsetStrategy with position sensitive queries.
// Mirrors org.apache.lucene.search.uhighlight.PhraseHelper.
type PhraseHelper struct {
	fieldName                string
	positionInsensitiveTerms map[string]bool
	spanQueries              map[search.SpanQuery]bool
	willRewrite              bool
	fieldMatcher             func(string) bool
}

// NONE is a sentinel PhraseHelper for queries with no position sensitivity.
var NONE = &PhraseHelper{
	fieldName:                "_ignored_",
	positionInsensitiveTerms: make(map[string]bool),
	spanQueries:              make(map[search.SpanQuery]bool),
	willRewrite:              true,
	fieldMatcher:             func(s string) bool { return false },
}

// NewPhraseHelper builds the helper based on the provided query and field.
func NewPhraseHelper(
	query search.Query,
	field string,
	fieldMatcher func(string) bool,
	rewriteQueryPred func(search.SpanQuery) *bool,
	preExtractRewriteFunction func(search.Query) []search.Query,
	ignoreQueriesNeedingRewrite bool,
) *PhraseHelper {
	ph := &PhraseHelper{
		fieldName:                field,
		fieldMatcher:             fieldMatcher,
		positionInsensitiveTerms: make(map[string]bool),
		spanQueries:              make(map[search.SpanQuery]bool),
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
		func(sq search.SpanQuery) bool {
			res := rewriteQueryPred(sq)
			if res != nil {
				return *res
			}
			return extractor.MustRewriteQuery(sq)
		},
		func(sq search.SpanQuery) {
			// If this span query isn't for this field, skip it.
			fieldNames := make(map[string]bool)
			extractor.CollectSpanQueryFields(sq, fieldNames)
			for fn := range fieldNames {
				if !fieldMatcher(fn) {
					return
				}
			}

			mustRewrite := extractor.MustRewriteQuery(sq)
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
	rewriteQueryPred func(search.SpanQuery) bool,
	onSpanQuery func(search.SpanQuery),
	onWeightedTerm func([]byte),
	mustRewriteHolder *bool,
) error {
	// We need to implement the recursive extraction.
	var extract func(search.Query, float32) error
	extract = func(q search.Query, boost float32) error {
		if bq, ok := q.(*search.BoostQuery); ok {
			return extract(bq.GetQuery(), boost*bq.GetBoost())
		}

		if bq, ok := q.(*search.BooleanQuery); ok {
			for _, clause := range bq.Clauses {
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
				sq := search.NewSpanTermQuery(terms[0])
				onSpanQuery(sq)
				return nil
			}
			// Convert PhraseQuery to SpanNearQuery as Lucene does.
			clauses := make([]search.SpanQuery, len(terms))
			for i, term := range terms {
				clauses[i] = search.NewSpanTermQuery(term)
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
			sp := search.NewSpanNearQuery(clauses, pq.GetSlop()+posGaps, inOrder)
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
			q.Visit(search.TermCollector(func(field string, term []byte) {
				if fieldMatcher(field) {
					onWeightedTerm(term)
				}
			}))
			return nil
		}
		if _, ok := q.(*search.SynonymQuery); ok {
			q.Visit(search.TermCollector(func(field string, term []byte) {
				if fieldMatcher(field) {
					onWeightedTerm(term)
				}
			}))
			return nil
		}

		if sq, ok := q.(search.SpanQuery); ok {
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

		if ctq, ok := q.(*search.CommonTermsQuery); ok {
			ctq.Visit(search.TermCollector(func(field string, term []byte) {
				if fieldMatcher(field) {
					onWeightedTerm(term)
				}
			}))
			return nil
		}

		if dmq, ok := q.(*search.DisjunctionMaxQuery); ok {
			for _, clause := range dmq.Clauses {
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
				disjunctLists := make([][]search.SpanQuery, maxPos+1)
				distinctPos := 0
				for i, termArray := range termArrays {
					pos := positions[i]
					if disjunctLists[pos] == nil {
						disjunctLists[pos] = make([]search.SpanQuery, 0, len(termArray))
						distinctPos++
					}
					for _, term := range termArray {
						disjunctLists[pos] = append(disjunctLists[pos], search.NewSpanTermQuery(term))
					}
				}
				posGaps := 0
				posIdx := 0
				clauses := make([]search.SpanQuery, distinctPos)
				for _, disjuncts := range disjunctLists {
					if disjuncts != nil {
						clauses[posIdx] = search.NewSpanOrQuery(disjuncts)
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
					sp := search.NewSpanNearQuery(clauses, slop+posGaps, inOrder)
					onSpanQuery(sp)
				}
			}
			return nil
		}

		if _, ok := q.(*search.MatchAllDocsQuery); ok {
			return nil
		}

		if fsq, ok := q.(*search.FunctionScoreQuery); ok {
			return extract(fsq.GetWrappedQuery(), boost)
		}

		// Handle multi-term queries if enabled.
		if mtq, ok := q.(*search.MultiTermQuery); ok {
			// In PhraseHelper, MTQ is processed separately (MultiTermHighlighting.java).
			// So we return true (unsupported) here.
			return nil
		}

		// Final fallback: rewrite and recurse.
		reader, err := extractor.GetLeafContext()
		if err != nil {
			return err
		}
		defer extractor.CloseInternalReader()

		var rewritten search.Query
		if mtq, ok := q.(*search.MultiTermQuery); ok {
			rewritten = search.RewriteMultiTermQuery(search.NewIndexSearcher(reader), mtq, search.ScoringBooleanRewrite)
		} else {
			rewritten = q.Rewrite(search.NewIndexSearcher(reader))
		}

		if rewritten != q {
			return extract(rewritten, boost)
		}

		return nil
	}

	return extract(query, 1.0)
}

// HasPositionSensitivity reports whether any registered term comes from a phrase.
func (p *PhraseHelper) HasPositionSensitivity() bool {
	return len(p.spanQueries) > 0
}

// WillRewrite reports whether the query needs rewriting.
func (p *PhraseHelper) WillRewrite() bool {
	return p.willRewrite
}

// GetAllPositionInsensitiveTerms returns the terms that are position-insensitive (sorted).
func (p *PhraseHelper) GetAllPositionInsensitiveTerms() [][]byte {
	res := make([][]byte, 0, len(p.positionInsensitiveTerms))
	for t := range p.positionInsensitiveTerms {
		res = append(res, []byte(t))
	}
	util.SortBytesRefs(res)
	return res
}

// CreateOffsetsEnumsForSpans produces a number of OffsetsEnum into the results param.
func (p *PhraseHelper) CreateOffsetsEnumsForSpans(leafReader index.LeafReader, docID int, results []*OffsetsEnum) error {
	// wrap reader to a single field.
	reader := index.NewSingleFieldWithOffsetsFilterLeafReader(leafReader, p.fieldName)
	searcher := search.NewIndexSearcher(reader)
	searcher.SetQueryCache(nil)

	spansPQ := newSpansPQ(len(p.spanQueries))
	for q := range p.spanQueries {
		weight := searcher.CreateWeight(searcher.Rewrite(q), search.ScoreModeCompleteNoScores, 1.0)
		scorer := weight.GetScorer(reader.GetContext())
		if scorer == nil {
			continue
		}

		// Use TwoPhaseIterator to check for matches.
		if tpi, ok := scorer.TwoPhaseIterator(); ok {
			if tpi.Approximation().Advance(docID) != docID || !tpi.Matches() {
				continue
			}
		} else if scorer.Iterator().Advance(docID) != docID {
			continue
		}

		spans := scorer.GetSpans()
		if spans != nil && spans.NextStartPosition() != index.NoMorePositions {
			spansPQ.Push(spans)
		}
	}

	collector := newOffsetSpanCollector(p.fieldMatcher)
	for spansPQ.Len() > 0 {
		spans := spansPQ.Pop()
		spans.Collect(collector)
		if spans.NextStartPosition() != index.NoMorePositions {
			spansPQ.Push(spans)
		}
	}

	for _, oe := range collector.termToOffsetsEnums {
		results = append(results, oe)
	}
	return nil
}

// Internal priority queue for Spans.
type spansPQ struct {
	data []*index.Spans
}

func newSpansPQ(cap int) *spansPQ {
	return &spansPQ{data: make([]*index.Spans, 0, cap)}
}

func (pq *spansPQ) Len() int { return len(pq.data) }
func (pq *spansPQ) Push(s *index.Spans) {
	pq.data = append(pq.data, s)
	// simplistic sort for now, in real implementation we should use a heap.
	// however, we only care about startPosition.
	for i := len(pq.data) - 1; i > 0 && pq.data[i].StartPosition() < pq.data[i-1].StartPosition(); i-- {
		pq.data[i], pq.data[i-1] = pq.data[i-1], pq.data[i]
	}
}
func (pq *spansPQ) Pop() *index.Spans {
	res := pq.data[0]
	pq.data = pq.data[1:]
	return res
}

// Internal collector for spans.
type offsetSpanCollector struct {
	fieldMatcher func(string) bool
	termToOffsetsEnums map[string]*OffsetsEnum
}

func newOffsetSpanCollector(fm func(string) bool) *offsetSpanCollector {
	return &offsetSpanCollector{
		fieldMatcher: fm,
		termToOffsetsEnums: make(map[string]*OffsetsEnum),
	}
}

func (c *offsetSpanCollector) CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error {
	if !c.fieldMatcher(term.Field) {
		return nil
	}
	bytes := term.Text()
	oe, ok := c.termToOffsetsEnums[string(bytes)]
	if !ok {
		oe = NewSpanCollectedOffsetsEnum(bytes, postings.Freq())
		c.termToOffsetsEnums[string(bytes)] = oe
	}
	oe.Add(postings.StartOffset(), postings.EndOffset())
	return nil
}

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

func (oe *SpanCollectedOffsetsEnum) NextPosition() bool {
	oe.enumIdx++
	return oe.enumIdx < oe.numPairs
}

func (oe *SpanCollectedOffsetsEnum) Freq() int {
	return oe.numPairs
}

func (oe *SpanCollectedOffsetsEnum) GetTerm() []byte {
	return oe.term
}

func (oe *SpanCollectedOffsetsEnum) StartOffset() int {
	return oe.startOffsets[oe.enumIdx]
}

func (oe *SpanCollectedOffsetsEnum) EndOffset() int {
	return oe.endOffsets[oe.enumIdx]
}
