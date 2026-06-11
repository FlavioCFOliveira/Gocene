// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadScoreQuery.java

package payloads

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PayloadScoreQuery uses a PayloadFunction to modify the score of a wrapped
// SpanQuery.
//
// Mirrors org.apache.lucene.queries.payloads.PayloadScoreQuery.
type PayloadScoreQuery struct {
	search.BaseSpanQuery
	wrappedQuery     search.SpanQuery
	function         PayloadFunction
	decoder          PayloadDecoder
	includeSpanScore bool
}

// NewPayloadScoreQuery creates a PayloadScoreQuery that includes the underlying
// span scores.
func NewPayloadScoreQuery(wrappedQuery search.SpanQuery, function PayloadFunction, decoder PayloadDecoder) *PayloadScoreQuery {
	return NewPayloadScoreQueryWithInclude(wrappedQuery, function, decoder, true)
}

// NewPayloadScoreQueryWithInclude creates a PayloadScoreQuery.
// If includeSpanScore is true, both span score and payload score are combined.
func NewPayloadScoreQueryWithInclude(wrappedQuery search.SpanQuery, function PayloadFunction,
	decoder PayloadDecoder, includeSpanScore bool) *PayloadScoreQuery {
	return &PayloadScoreQuery{
		BaseSpanQuery:    *search.NewBaseSpanQuery(wrappedQuery.GetField()),
		wrappedQuery:     wrappedQuery,
		function:         function,
		decoder:          decoder,
		includeSpanScore: includeSpanScore,
	}
}

// GetField returns the field of the wrapped query.
func (q *PayloadScoreQuery) GetField() string { return q.wrappedQuery.GetField() }

// GetWrappedQuery returns the wrapped query.
func (q *PayloadScoreQuery) GetWrappedQuery() search.SpanQuery { return q.wrappedQuery }

// Rewrite rewrites the wrapped query and returns a new PayloadScoreQuery if
// the wrapped query changed.
func (q *PayloadScoreQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.wrappedQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten != q.wrappedQuery {
		sp, ok := rewritten.(search.SpanQuery)
		if !ok {
			return nil, fmt.Errorf("PayloadScoreQuery.Rewrite: inner rewrite returned non-SpanQuery %T", rewritten)
		}
		return NewPayloadScoreQueryWithInclude(sp, q.function, q.decoder, q.includeSpanScore), nil
	}
	return q, nil
}

// Visit visits the query tree. This method is accessed via duck-type assertion
// (interface{ Visit(search.QueryVisitor) }) by callers such as IndexSearcher.
func (q *PayloadScoreQuery) Visit(visitor search.QueryVisitor) {
	if v, ok := q.wrappedQuery.(interface{ Visit(search.QueryVisitor) }); ok {
		v.Visit(visitor.GetSubVisitor(search.MUST, q))
	}
}

// CreateWeight creates a Weight for this query.
func (q *PayloadScoreQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	innerWeight, err := q.wrappedQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	if !needsScores {
		return innerWeight, nil
	}
	return &payloadScoreWeight{
		BaseWeight:       search.NewBaseWeight(q),
		innerWeight:      innerWeight,
		field:            q.GetField(),
		function:         q.function,
		decoder:          q.decoder,
		includeSpanScore: q.includeSpanScore,
	}, nil
}

// Clone returns a copy of this query.
func (q *PayloadScoreQuery) Clone() search.Query {
	return &PayloadScoreQuery{
		BaseSpanQuery:    *search.NewBaseSpanQuery(q.GetField()),
		wrappedQuery:     q.wrappedQuery.Clone().(search.SpanQuery),
		function:         q.function,
		decoder:          q.decoder,
		includeSpanScore: q.includeSpanScore,
	}
}

// Equals returns true if other is equal to this.
func (q *PayloadScoreQuery) Equals(other search.Query) bool {
	o, ok := other.(*PayloadScoreQuery)
	if !ok {
		return false
	}
	return q.wrappedQuery.Equals(o.wrappedQuery) &&
		q.includeSpanScore == o.includeSpanScore
}

// HashCode returns a hash code for this query.
func (q *PayloadScoreQuery) HashCode() int {
	h := classHash()
	h = 31*h + q.wrappedQuery.HashCode()
	if q.includeSpanScore {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	return h
}

// classHash returns a hash component unique to the query type.
func classHash() int {
	return 53219871 // arbitrary constant
}

// String returns a string representation.
func (q *PayloadScoreQuery) String(field string) string {
	return fmt.Sprintf("PayloadScoreQuery(%s, function: %T, includeSpanScore: %t)",
		q.wrappedQuery.String(field), q.function, q.includeSpanScore)
}

// Ensure PayloadScoreQuery implements search.SpanQuery.
var _ search.SpanQuery = (*PayloadScoreQuery)(nil)

// --- Weight implementation ---

type payloadScoreWeight struct {
	*search.BaseWeight
	innerWeight      search.Weight
	field            string
	function         PayloadFunction
	decoder          PayloadDecoder
	includeSpanScore bool
}

func (w *payloadScoreWeight) getSpans(ctx *index.LeafReaderContext) (spans.Spans, error) {
	sp, ok := w.innerWeight.(spansProvider)
	if !ok {
		return nil, nil
	}
	return sp.GetSpans(ctx, spans.PostingsPayloads)
}

// ScorerSupplier returns a ScorerSupplier for the given context.
func (w *payloadScoreWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	if w.field == "" {
		return nil, nil
	}

	innerSpans, err := w.getSpans(ctx)
	if err != nil {
		return nil, err
	}
	if innerSpans == nil {
		return nil, nil
	}

	ps := newPayloadScoreSpans(innerSpans, w.decoder, w.function)

	var norms index.NumericDocValues
	if ctx != nil {
		if lr, ok := ctx.LeafReader().(*index.LeafReader); ok && lr != nil {
			norms, _ = lr.GetNormValues(w.field)
		}
	}

	scorer := newPayloadScoreScorer(ps, nil, norms, w.function, w.includeSpanScore)
	return search.NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation for the given document.
func (w *payloadScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil || supplier == nil {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	sc, err := supplier.Get(0)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	advanced, err := sc.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return search.NoMatchExplanation("no matching spans"), nil
	}

	sc.Score() // force frequency/payload calculation

	ps, ok := sc.(*payloadScoreScorer)
	if !ok {
		return search.MatchExplanation(sc.Score(), "PayloadScoreQuery match"), nil
	}

	payloadScore := ps.getPayloadScore()
	payloadExpl := ps.getPayloadExplanation()

	if w.includeSpanScore {
		spanScore := sc.Score()
		if payloadScore > 0 {
			spanScore = sc.Score() / payloadScore
		}
		return search.MatchExplanationWithDetails(
			sc.Score(),
			"PayloadSpanQuery, product of:",
			search.MatchExplanation(spanScore, "span score"),
			payloadExpl,
		), nil
	}
	return payloadExpl, nil
}

// IsCacheable returns true if the inner weight is cacheable.
func (w *payloadScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.innerWeight.IsCacheable(ctx)
}

// Count returns -1.
func (w *payloadScoreWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil.
func (w *payloadScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

func (w *payloadScoreWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

var _ search.Weight = (*payloadScoreWeight)(nil)

// --- Payload-collecting Spans ---

// payloadScoreSpans wraps inner spans and accumulates payload scores during
// position traversal. It accepts all positions (like FilterSpans with
// AcceptStatus.YES in Java), and collects payload data via its SpanCollector
// implementation.
type payloadScoreSpans struct {
	inner        spans.Spans
	decoder      PayloadDecoder
	function     PayloadFunction
	payloadsSeen int
	payloadScore float32
}

func newPayloadScoreSpans(inner spans.Spans, decoder PayloadDecoder, function PayloadFunction) *payloadScoreSpans {
	return &payloadScoreSpans{
		inner:    inner,
		decoder:  decoder,
		function: function,
	}
}

func (ps *payloadScoreSpans) DocID() int    { return ps.inner.DocID() }
func (ps *payloadScoreSpans) Cost() int64   { return ps.inner.Cost() }
func (ps *payloadScoreSpans) DocIDRunEnd() int      { return ps.inner.DocIDRunEnd() }
func (ps *payloadScoreSpans) StartPosition() int    { return ps.inner.StartPosition() }
func (ps *payloadScoreSpans) EndPosition() int      { return ps.inner.EndPosition() }
func (ps *payloadScoreSpans) Width() int            { return ps.inner.Width() }
func (ps *payloadScoreSpans) PositionsCost() float32 { return ps.inner.PositionsCost() }
func (ps *payloadScoreSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	return ps.inner.AsTwoPhaseIterator()
}
func (ps *payloadScoreSpans) Collect(collector spans.SpanCollector) error {
	return ps.inner.Collect(collector)
}
func (ps *payloadScoreSpans) NextDoc() (int, error) { return ps.inner.NextDoc() }
func (ps *payloadScoreSpans) Advance(target int) (int, error) { return ps.inner.Advance(target) }
func (ps *payloadScoreSpans) NextStartPosition() (int, error) { return ps.inner.NextStartPosition() }

// DoStartCurrentDoc resets payload tracking for a new document.
func (ps *payloadScoreSpans) DoStartCurrentDoc() error {
	ps.payloadsSeen = 0
	ps.payloadScore = 0
	return ps.inner.DoStartCurrentDoc()
}

// DoCurrentSpans collects payload scores for the current position.
func (ps *payloadScoreSpans) DoCurrentSpans() error {
	return ps.inner.Collect(ps)
}

// CollectLeaf implements spans.SpanCollector. It accumulates payload scores.
func (ps *payloadScoreSpans) CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error {
	payloadBytes, err := postings.GetPayload()
	if err != nil {
		return err
	}
	var payload *util.BytesRef
	if payloadBytes != nil {
		payload = util.NewBytesRef(payloadBytes)
	}
	payloadFactor := ps.decoder.ComputePayloadFactor(payload)
	ps.payloadScore = ps.function.CurrentScore(
		ps.inner.DocID(),
		"",
		ps.inner.StartPosition(),
		ps.inner.EndPosition(),
		ps.payloadsSeen,
		ps.payloadScore,
		payloadFactor,
	)
	ps.payloadsSeen++
	return nil
}

// Reset implements spans.SpanCollector.
func (ps *payloadScoreSpans) Reset() {}

var _ spans.Spans = (*payloadScoreSpans)(nil)
var _ spans.SpanCollector = (*payloadScoreSpans)(nil)

// --- Scorer ---

// payloadScoreScorer implements search.Scorer for payload-scored spans.
type payloadScoreScorer struct {
	spans            *payloadScoreSpans
	simScorer        search.SimScorer
	norms            index.NumericDocValues
	function         PayloadFunction
	includeSpanScore bool
	freq             float32
	lastDoc          int
}

func newPayloadScoreScorer(spans *payloadScoreSpans, simScorer search.SimScorer,
	norms index.NumericDocValues, function PayloadFunction,
	includeSpanScore bool) *payloadScoreScorer {
	return &payloadScoreScorer{
		spans:            spans,
		simScorer:        simScorer,
		norms:            norms,
		function:         function,
		includeSpanScore: includeSpanScore,
		lastDoc:          -1,
	}
}

func (s *payloadScoreScorer) DocID() int { return s.spans.DocID() }

func (s *payloadScoreScorer) NextDoc() (int, error) {
	doc, err := s.spans.NextDoc()
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	s.lastDoc = -1
	return doc, nil
}

func (s *payloadScoreScorer) Advance(target int) (int, error) {
	doc, err := s.spans.Advance(target)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	s.lastDoc = -1
	return doc, nil
}

func (s *payloadScoreScorer) Cost() int64      { return s.spans.Cost() }
func (s *payloadScoreScorer) DocIDRunEnd() int { return s.spans.DocIDRunEnd() }

// setFreqCurrentDoc accumulates sloppy frequency and triggers payload collection.
func (s *payloadScoreScorer) setFreqCurrentDoc() error {
	s.freq = 0
	if err := s.spans.DoStartCurrentDoc(); err != nil {
		return err
	}
	pos, err := s.spans.NextStartPosition()
	if err != nil {
		return err
	}
	if pos == spans.NoMorePositions {
		return nil
	}
	for {
		if s.simScorer != nil {
			s.freq += 1.0 / (1.0 + float32(s.spans.Width()))
		} else {
			s.freq = 1.0
		}
		if err := s.spans.DoCurrentSpans(); err != nil {
			return err
		}
		next, err := s.spans.NextStartPosition()
		if err != nil {
			return err
		}
		if next == spans.NoMorePositions {
			break
		}
	}
	return nil
}

func (s *payloadScoreScorer) ensureFreq() error {
	cur := s.DocID()
	if s.lastDoc != cur {
		if err := s.setFreqCurrentDoc(); err != nil {
			return err
		}
		s.lastDoc = cur
	}
	return nil
}

// Score returns the combined score for the current document.
func (s *payloadScoreScorer) Score() float32 {
	if err := s.ensureFreq(); err != nil {
		return 0
	}
	return s.scoreCurrentDoc()
}

// getSpanScore returns the underlying span score (without payload contribution).
func (s *payloadScoreScorer) getSpanScore() float32 {
	if s.simScorer == nil {
		return 0
	}
	return s.simScorer.Score(s.DocID(), s.freq)
}

// getPayloadScore returns the payload-derived score.
func (s *payloadScoreScorer) getPayloadScore() float32 {
	score := s.function.DocScore(s.DocID(), "", s.spans.payloadsSeen, s.spans.payloadScore)
	if score < 0 || math.IsNaN(float64(score)) {
		return 0
	}
	return score
}

// getPayloadExplanation returns an explanation of the payload score.
func (s *payloadScoreScorer) getPayloadExplanation() search.Explanation {
	expl := s.function.Explain(s.DocID(), "", s.spans.payloadsSeen, s.spans.payloadScore)
	if expl.GetValue() < 0 {
		return search.MatchExplanationWithDetails(
			0,
			"truncated score, max of:",
			search.MatchExplanation(0, "minimum score"),
			expl,
		)
	}
	if math.IsNaN(float64(expl.GetValue())) {
		return search.MatchExplanationWithDetails(
			0,
			"payload score, computed as (score == NaN ? 0 : score) since NaN is an illegal score from:",
			expl,
		)
	}
	return expl
}

// scoreCurrentDoc computes the final score.
func (s *payloadScoreScorer) scoreCurrentDoc() float32 {
	if s.includeSpanScore {
		return s.getSpanScore() * s.getPayloadScore()
	}
	return s.getPayloadScore()
}

func (s *payloadScoreScorer) GetMaxScore(_ int) float32 { return 1<<24 - 1 }

func (s *payloadScoreScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

var _ search.Scorer = (*payloadScoreScorer)(nil)
