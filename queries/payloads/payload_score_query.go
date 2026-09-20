// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadScoreQuery.java

package payloads

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
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
	search.BaseQuery
	wrappedQuery     spans.SpanQuery
	function         PayloadFunction
	decoder          PayloadDecoder
	includeSpanScore bool
}

// NewPayloadScoreQuery creates a PayloadScoreQuery that includes the underlying
// span scores.
func NewPayloadScoreQuery(wrappedQuery spans.SpanQuery, function PayloadFunction, decoder PayloadDecoder) *PayloadScoreQuery {
	return NewPayloadScoreQueryWithInclude(wrappedQuery, function, decoder, true)
}

// NewPayloadScoreQueryWithInclude creates a PayloadScoreQuery.
// If includeSpanScore is true, both span score and payload score are combined.
func NewPayloadScoreQueryWithInclude(wrappedQuery spans.SpanQuery, function PayloadFunction,
	decoder PayloadDecoder, includeSpanScore bool) *PayloadScoreQuery {
	return &PayloadScoreQuery{
		wrappedQuery:     wrappedQuery,
		function:         function,
		decoder:          decoder,
		includeSpanScore: includeSpanScore,
	}
}

// GetField returns the field of the wrapped query.
func (q *PayloadScoreQuery) GetField() string { return q.wrappedQuery.GetField() }

// GetWrappedQuery returns the wrapped query.
func (q *PayloadScoreQuery) GetWrappedQuery() spans.SpanQuery { return q.wrappedQuery }

// Rewrite rewrites the wrapped query and returns a new PayloadScoreQuery if
// the wrapped query changed.
func (q *PayloadScoreQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewritten, err := q.wrappedQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.wrappedQuery {
		sp, ok := rewritten.(spans.SpanQuery)
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
//
// Java: SpanWeight createWeight(IndexSearcher, ScoreMode, float) — the inner
// weight is returned unchanged when the score mode needs no scores.
func (q *PayloadScoreQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	innerWeight, err := q.wrappedQuery.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if !scoreMode.NeedsScores() {
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

// CreateSpanWeight delegates to the wrapped span query, which is what the
// payload weight scores over.
func (q *PayloadScoreQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*spans.SpanWeight, error) {
	return q.wrappedQuery.CreateSpanWeight(searcher, scoreMode, boost)
}

// Equals returns true if other is equal to this.
func (q *PayloadScoreQuery) Equals(other spi.Query) bool {
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
// ToString prints this query, with field assumed to be the default field and
// omitted. Mirrors the inherited Query.toString(String) that SpanQuery
// restates for the span family.
func (q *PayloadScoreQuery) ToString(field string) string { return q.String(field) }

func (q *PayloadScoreQuery) String(field string) string {
	return fmt.Sprintf("PayloadScoreQuery(%s, function: %T, includeSpanScore: %t)",
		q.wrappedQuery.ToString(field), q.function, q.includeSpanScore)
}

// Ensure PayloadScoreQuery implements spans.SpanQuery.
var _ spans.SpanQuery = (*PayloadScoreQuery)(nil)

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
		if lr := ctx.LeafReader(); lr != nil {
			norms, _ = lr.GetNormValues(w.field)
		}
	}

	scorer := newPayloadScoreScorer(ps, nil, norms, w.function, w.includeSpanScore)
	return search.NewDefaultScorerSupplier(scorer), nil
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
	advanced, err := sc.Iterator().Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return search.NoMatchExplanation("no matching spans"), nil
	}

	// Force the frequency/payload calculation.
	score, err := sc.Score()
	if err != nil {
		return nil, err
	}

	ps, ok := sc.(*payloadScoreScorer)
	if !ok {
		return search.MatchExplanation(score, "PayloadScoreQuery match"), nil
	}

	payloadScore := ps.getPayloadScore()
	payloadExpl := ps.getPayloadExplanation()

	if w.includeSpanScore {
		spanScore := score
		if payloadScore > 0 {
			spanScore = score / payloadScore
		}
		return search.MatchExplanationWithDetails(
			score,
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

func (ps *payloadScoreSpans) DocID() int                { return ps.inner.DocID() }
func (ps *payloadScoreSpans) Cost() int64               { return ps.inner.Cost() }
func (ps *payloadScoreSpans) DocIDRunEnd() (int, error) { return ps.inner.DocIDRunEnd() }
func (ps *payloadScoreSpans) StartPosition() int        { return ps.inner.StartPosition() }
func (ps *payloadScoreSpans) EndPosition() int          { return ps.inner.EndPosition() }
func (ps *payloadScoreSpans) Width() int                { return ps.inner.Width() }
func (ps *payloadScoreSpans) PositionsCost() float32    { return ps.inner.PositionsCost() }
func (ps *payloadScoreSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	return ps.inner.AsTwoPhaseIterator()
}
func (ps *payloadScoreSpans) Collect(collector spans.SpanCollector) error {
	return ps.inner.Collect(collector)
}
func (ps *payloadScoreSpans) NextDoc() (int, error)           { return ps.inner.NextDoc() }
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

func (s *payloadScoreScorer) Cost() int64               { return s.spans.Cost() }
func (s *payloadScoreScorer) DocIDRunEnd() (int, error) { return s.spans.DocIDRunEnd() }

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
func (s *payloadScoreScorer) Score() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	return s.scoreCurrentDoc(), nil
}

// Iterator returns the DocIdSetIterator view of this scorer: in Java the
// payload scorer iterates through its Spans.
func (s *payloadScoreScorer) Iterator() search.DocIdSetIterator {
	return &payloadScoreIterator{s: s}
}

// TwoPhaseIterator carries Scorer#twoPhaseIterator()'s default body (null).
func (s *payloadScoreScorer) TwoPhaseIterator() *search.TwoPhaseIterator { return nil }

// GetChildren carries Scorable.getChildren()'s default body (empty list).
func (s *payloadScoreScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{}, nil
}

// SmoothingScore carries Scorable.smoothingScore(int)'s default body (0f).
func (s *payloadScoreScorer) SmoothingScore(docID int) (float32, error) { return 0, nil }

// SetMinCompetitiveScore carries Scorable.setMinCompetitiveScore's empty default.
func (s *payloadScoreScorer) SetMinCompetitiveScore(minScore float32) error { return nil }

// NextDocsAndScores carries Scorer#nextDocsAndScores's default body.
func (s *payloadScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// payloadScoreIterator is the DocIdSetIterator view of payloadScoreScorer.
type payloadScoreIterator struct {
	s *payloadScoreScorer
}

func (it *payloadScoreIterator) DocID() int                 { return it.s.DocID() }
func (it *payloadScoreIterator) Cost() int64                { return it.s.Cost() }
func (it *payloadScoreIterator) NextDoc() (int, error)      { return it.s.NextDoc() }
func (it *payloadScoreIterator) Advance(t int) (int, error) { return it.s.Advance(t) }
func (it *payloadScoreIterator) DocIDRunEnd() (int, error)  { return it.s.DocIDRunEnd() }
func (it *payloadScoreIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// getSpanScore returns the underlying span score (without payload contribution).
func (s *payloadScoreScorer) getSpanScore() float32 {
	if s.simScorer == nil {
		return 0
	}
	return s.simScorer.Score104(s.freq, 1)
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

func (s *payloadScoreScorer) GetMaxScore(_ int) (float32, error) { return 1<<24 - 1, nil }

func (s *payloadScoreScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

var _ search.Scorer = (*payloadScoreScorer)(nil)

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (ps *payloadScoreSpans) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(ps, upTo, bitSet, offset)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *payloadScoreScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
