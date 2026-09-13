package uhighlight

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/memory"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// WeightedSpanTermExtractor is used to extract WeightedSpanTerms from a Query
// based on whether Terms from the Query are contained in a supplied TokenStream.
// Mirrors org.apache.lucene.search.highlight.WeightedSpanTermExtractor.
type WeightedSpanTermExtractor struct {
	fieldName            string
	tokenStream          analysis.TokenStream
	defaultField         string
	expandMultiTermQuery bool
	cachedTokenStream    bool
	wrapToCaching        bool
	maxDocCharsToAnalyze int
	usePayloads          bool
	internalReader       index.LeafReader
}

func NewWeightedSpanTermExtractor(defaultField string) *WeightedSpanTermExtractor {
	return &WeightedSpanTermExtractor{
		defaultField:         defaultField,
		wrapToCaching:        true,
		maxDocCharsToAnalyze: 10000, // Default value from Lucene
	}
}

// Extract fills a map with WeightedSpanTerms using the terms from the supplied Query.
func (w *WeightedSpanTermExtractor) Extract(query search.Query, boost float32, terms map[string]*WeightedSpanTerm) error {
	if bq, ok := query.(*search.BoostQuery); ok {
		return w.Extract(bq.GetQuery(), boost*bq.GetBoost(), terms)
	}

	if bq, ok := query.(*search.BooleanQuery); ok {
		for _, clause := range bq.Clauses {
			if !clause.IsProhibited() {
				if err := w.Extract(clause.Query(), boost, terms); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if pq, ok := query.(*search.PhraseQuery); ok {
		termsList := pq.GetTerms()
		if len(termsList) == 1 {
			return w.extractWeightedSpanTerms(terms, spans.NewSpanTermQuery(termsList[0]), boost)
		}

		clauses := make([]spans.SpanQuery, len(termsList))
		for i, term := range termsList {
			clauses[i] = spans.NewSpanTermQuery(term)
		}

		positionGaps := 0
		positions := pq.GetPositions()
		if len(positions) >= 2 {
			positionGaps = positions[len(positions)-1] - positions[0] - len(positions) + 1
			if positionGaps < 0 {
				positionGaps = 0
			}
		}

		inOrder := (pq.GetSlop() == 0)
		sp, err := spans.NewSpanNearQuery(clauses, pq.GetSlop()+positionGaps, inOrder)
		if err != nil {
			return err
		}
		return w.extractWeightedSpanTerms(terms, sp, boost)
	}

	if idv, ok := query.(*search.IndexOrDocValuesQuery); ok {
		indexQuery := idv.GetIndexQuery()
		if indexQuery != nil {
			return w.Extract(indexQuery, boost, terms)
		}
		return nil
	}

	if _, ok := query.(*search.TermQuery); ok {
		return w.extractWeightedTerms(terms, query, boost)
	}
	if _, ok := query.(*search.SynonymQuery); ok {
		return w.extractWeightedTerms(terms, query, boost)
	}

	if sq, ok := query.(spans.SpanQuery); ok {
		return w.extractWeightedSpanTerms(terms, sq, boost)
	}

	if csq, ok := query.(*search.ConstantScoreQuery); ok {
		q := csq.GetQuery()
		if q != nil {
			return w.Extract(q, boost, terms)
		}
		return nil
	}

	if ctq, ok := query.(*search.CommonTermsQuery); ok {
		return w.extractWeightedTerms(terms, ctq, boost)
	}

	if dmq, ok := query.(*search.DisjunctionMaxQuery); ok {
		for _, clause := range dmq.Clauses {
			if err := w.Extract(clause, boost, terms); err != nil {
				return err
			}
		}
		return nil
	}

	if mpq, ok := query.(*search.MultiPhraseQuery); ok {
		termArrays := mpq.GetTermArrays()
		positions := mpq.GetPositions()
		if len(positions) > 0 {
			maxPosition := positions[len(positions)-1]
			for _, p := range positions {
				if p > maxPosition {
					maxPosition = p
				}
			}

			disjunctLists := make([][]spans.SpanQuery, maxPosition+1)
			distinctPositions := 0
			for i, termArray := range termArrays {
				pos := positions[i]
				if disjunctLists[pos] == nil {
					disjunctLists[pos] = make([]spans.SpanQuery, 0, len(termArray))
					distinctPositions++
				}
				for _, term := range termArray {
					disjunctLists[pos] = append(disjunctLists[pos], spans.NewSpanTermQuery(term))
				}
			}

			positionGaps := 0
			position := 0
			clauses := make([]spans.SpanQuery, distinctPositions)
			for _, disjuncts := range disjunctLists {
				if disjuncts != nil {
					or, err := spans.NewSpanOrQuery(disjuncts...)
					if err != nil {
						return err
					}
					clauses[position] = or
					position++
				} else {
					positionGaps++
				}
			}

			if len(clauses) == 1 {
				return w.extractWeightedSpanTerms(terms, clauses[0], boost)
			}
			slop := mpq.GetSlop()
			inOrder := (slop == 0)
			sp, err := spans.NewSpanNearQuery(clauses, slop+positionGaps, inOrder)
			if err != nil {
				return err
			}
			return w.extractWeightedSpanTerms(terms, sp, boost)
		}
		return nil
	}

	if _, ok := query.(*search.MatchAllDocsQuery); ok {
		return nil
	}

	if fsq, ok := query.(*search.FunctionScoreQuery); ok {
		return w.Extract(fsq.GetWrappedQuery(), boost, terms)
	}

	if w.isQueryUnsupported(query) {
		return nil
	}

	if mtq, ok := query.(*search.MultiTermQuery); ok {
		if !w.expandMultiTermQuery || !w.fieldNameComparator(mtq.GetField()) {
			return nil
		}
	}

	// For unknown queries, we rewrite them using the temporary MemoryIndex.
	reader, err := w.getLeafContext()
	if err != nil {
		return err
	}
	defer w.closeInternalReader()

	var rewritten search.Query
	if mtq, ok := query.(*search.MultiTermQuery); ok {
		rewritten = search.RewriteMultiTermQuery(search.NewIndexSearcher(reader), mtq, search.ScoringBooleanRewrite)
	} else {
		rewritten = query.Rewrite(search.NewIndexSearcher(reader))
	}

	if rewritten != query {
		return w.Extract(rewritten, boost, terms)
	}

	return w.extractUnknownQuery(query, terms)
}

func (w *WeightedSpanTermExtractor) isQueryUnsupported(query search.Query) bool {
	// Spatial queries do not support highlighting.
	// In Gocene, we can check the type or a specific interface.
	return false
}

func (w *WeightedSpanTermExtractor) extractUnknownQuery(query search.Query, terms map[string]*WeightedSpanTerm) error {
	return nil
}

func (w *WeightedSpanTermExtractor) extractWeightedSpanTerms(terms map[string]*WeightedSpanTerm, spanQuery spans.SpanQuery, boost float32) error {
	queryFieldNames := make(map[string]bool)
	w.collectSpanQueryFields(spanQuery, queryFieldNames)

	if w.fieldName != "" && !queryFieldNames[w.fieldName] && (w.defaultField == "" || !queryFieldNames[w.defaultField]) {
		return nil
	}

	searcher := search.NewIndexSearcher(w.getLeafContextMust())
	searcher.SetQueryCache(nil)

	query := spanQuery
	if w.mustRewriteQuery(spanQuery) {
		query = searcher.Rewrite(spanQuery).(spans.SpanQuery)
	}

	nonWeightedTerms := make(map[string]*index.Term)
	query.Visit(search.TermCollector(nonWeightedTerms))
	if len(nonWeightedTerms) == 0 {
		return nil
	}

	spanPositions := make([]*PositionSpan, 0)
	context, err := w.getLeafContext()
	if err != nil {
		return err
	}

	weight := searcher.CreateWeight(query, search.ScoreModeCompleteNoScores, 1.0)
	spans := weight.GetSpans(context, index.SpanWeightPositions)
	if spans == nil {
		return nil
	}

	acceptDocs := context.GetReader().GetLiveDocs()
	for spans.NextDoc() != index.NoMoreDocs {
		if acceptDocs != nil && !acceptDocs.Get(spans.DocID()) {
			continue
		}
		for spans.NextStartPosition() != index.NoMorePositions {
			spanPositions = append(spanPositions, NewPositionSpan(spans.StartPosition(), spans.EndPosition()-1))
		}
	}

	if len(spanPositions) == 0 {
		return nil
	}

	for _, queryTerm := range nonWeightedTerms {
		if w.fieldNameComparator(queryTerm.Field) {
			weightedSpanTerm, ok := terms[queryTerm.Text]
			if !ok {
				weightedSpanTerm = NewWeightedSpanTerm(boost, queryTerm.Text)
				weightedSpanTerm.AddPositionSpans(spanPositions)
				weightedSpanTerm.PositionSensitive = true
				terms[queryTerm.Text] = weightedSpanTerm
			} else {
				if len(spanPositions) > 0 {
					weightedSpanTerm.AddPositionSpans(spanPositions)
				}
			}
		}
	}

	return nil
}

func (w *WeightedSpanTermExtractor) extractWeightedTerms(terms map[string]*WeightedSpanTerm, query search.Query, boost float32) error {
	nonWeightedTerms := make(map[string]*index.Term)
	searcher := search.NewIndexSearcher(w.getLeafContextMust())
	searcher.Rewrite(query).Visit(search.TermCollector(nonWeightedTerms))

	for _, queryTerm := range nonWeightedTerms {
		if w.fieldNameComparator(queryTerm.Field) {
			terms[queryTerm.Text] = NewWeightedSpanTerm(boost, queryTerm.Text)
		}
	}
	return nil
}

func (w *WeightedSpanTermExtractor) fieldNameComparator(fieldNameToCheck string) bool {
	return w.fieldName == "" || w.fieldName == fieldNameToCheck || (w.defaultField != "" && w.defaultField == fieldNameToCheck)
}

func (w *WeightedSpanTermExtractor) getLeafContext() (index.IndexReaderContext, error) {
	if w.internalReader == nil {
		cacheIt := w.wrapToCaching && !w.isCachingTokenFilter()

		if w.isTokenStreamFromTermVector() {
			cacheIt = false
			termVectorTerms := w.getTokenStreamFromTermVector().GetTermVectorTerms()
			if termVectorTerms.HasPositions() && termVectorTerms.HasOffsets() {
				w.internalReader = index.NewTermVectorLeafReader("shadowed_field", termVectorTerms)
			}
		}

		if w.internalReader == nil {
			indexer := memory.NewMemoryIndex()
			if cacheIt {
				w.tokenStream = analysis.NewCachingTokenFilter(analysis.NewOffsetLimitTokenFilter(w.tokenStream, w.maxDocCharsToAnalyze))
				w.cachedTokenStream = true
				indexer.AddField("shadowed_field", w.tokenStream)
			} else {
				indexer.AddField("shadowed_field", analysis.NewOffsetLimitTokenFilter(w.tokenStream, w.maxDocCharsToAnalyze))
			}
			searcher, err := indexer.CreateSearcher()
			if err != nil {
				return nil, err
			}
			w.internalReader = searcher.GetTopReaderContext().GetReader()
		}

		w.internalReader = index.NewDelegatingLeafReader(w.internalReader)
	}
	return w.internalReader.GetContext()
}

func (w *WeightedSpanTermExtractor) getLeafContextMust() index.IndexReaderContext {
	ctx, err := w.getLeafContext()
	if err != nil {
		panic(err)
	}
	return ctx
}

func (w *WeightedSpanTermExtractor) closeInternalReader() {
	if w.internalReader != nil {
		w.internalReader.Close()
		w.internalReader = nil
	}
}

func (w *WeightedSpanTermExtractor) collectSpanQueryFields(spanQuery spans.SpanQuery, fieldNames map[string]bool) {
	if sq, ok := spanQuery.(*search.FieldMaskingSpanQuery); ok {
		w.collectSpanQueryFields(sq.GetMaskedQuery(), fieldNames)
	} else if sq, ok := spanQuery.(*spans.SpanFirstQuery); ok {
		w.collectSpanQueryFields(sq.GetMatch(), fieldNames)
	} else if sq, ok := spanQuery.(*spans.SpanNearQuery); ok {
		for _, clause := range sq.GetClauses() {
			w.collectSpanQueryFields(clause, fieldNames)
		}
	} else if sq, ok := spanQuery.(*spans.SpanNotQuery); ok {
		w.collectSpanQueryFields(sq.GetInclude(), fieldNames)
	} else if sq, ok := spanQuery.(*spans.SpanOrQuery); ok {
		for _, clause := range sq.GetClauses() {
			w.collectSpanQueryFields(clause, fieldNames)
		}
	} else {
		fieldNames[spanQuery.GetField()] = true
	}
}

func (w *WeightedSpanTermExtractor) mustRewriteQuery(spanQuery spans.SpanQuery) bool {
	if !w.expandMultiTermQuery {
		return false
	}
	if sq, ok := spanQuery.(*search.FieldMaskingSpanQuery); ok {
		return w.mustRewriteQuery(sq.GetMaskedQuery())
	}
	if sq, ok := spanQuery.(*spans.SpanFirstQuery); ok {
		return w.mustRewriteQuery(sq.GetMatch())
	}
	if sq, ok := spanQuery.(*spans.SpanNearQuery); ok {
		for _, clause := range sq.GetClauses() {
			if w.mustRewriteQuery(clause) {
				return true
			}
		}
		return false
	}
	if sq, ok := spanQuery.(*spans.SpanNotQuery); ok {
		return w.mustRewriteQuery(sq.GetInclude()) || w.mustRewriteQuery(sq.GetExclude())
	}
	if sq, ok := spanQuery.(*spans.SpanOrQuery); ok {
		for _, clause := range sq.GetClauses() {
			if w.mustRewriteQuery(clause) {
				return true
			}
		}
		return false
	}
	if _, ok := spanQuery.(*spans.SpanTermQuery); ok {
		return false
	}
	return true
}

// Helper methods for TokenStream type checking.
func (w *WeightedSpanTermExtractor) isCachingTokenFilter() bool {
	_, ok := w.tokenStream.(*analysis.CachingTokenFilter)
	return ok
}

func (w *WeightedSpanTermExtractor) isTokenStreamFromTermVector() bool {
	_, ok := w.tokenStream.(*analysis.TokenStreamFromTermVector)
	return ok
}

func (w *WeightedSpanTermExtractor) getTokenStreamFromTermVector() *analysis.TokenStreamFromTermVector {
	return w.tokenStream.(*analysis.TokenStreamFromTermVector)
}
