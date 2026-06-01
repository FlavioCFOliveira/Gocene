// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FunctionScoreQuery wraps another query and substitutes (or modifies) its
// score with a [DoubleValuesSource]. Documents for which the source yields
// no value are scored as 0.
//
// Go port of org.apache.lucene.queries.function.FunctionScoreQuery.
type FunctionScoreQuery struct {
	inner  search.Query
	source DoubleValuesSource
}

// NewFunctionScoreQuery wraps inner with source.
func NewFunctionScoreQuery(inner search.Query, source DoubleValuesSource) *FunctionScoreQuery {
	return &FunctionScoreQuery{inner: inner, source: source}
}

// GetWrappedQuery returns the wrapped query.
func (q *FunctionScoreQuery) GetWrappedQuery() search.Query { return q.inner }

// GetSource returns the wrapping DoubleValuesSource.
func (q *FunctionScoreQuery) GetSource() DoubleValuesSource { return q.source }

// BoostByValue returns a FunctionScoreQuery that multiplies inner's score
// by the value of boost. Missing values fall through to 1.0 so the inner
// score is preserved.
func BoostByValue(inner search.Query, boost DoubleValuesSource) *FunctionScoreQuery {
	return NewFunctionScoreQuery(inner, &multiplicativeBoostValuesSource{boost: boost})
}

// String renders the canonical Lucene-style description.
func (q *FunctionScoreQuery) String() string {
	return fmt.Sprintf("FunctionScoreQuery(%v, scored by %s)", q.inner, q.source.Description())
}

// Rewrite delegates to the inner query and rebuilds the FunctionScoreQuery
// when the inner rewrite changes identity.
func (q *FunctionScoreQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten == q.inner {
		return q, nil
	}
	return NewFunctionScoreQuery(rewritten, q.source), nil
}

// Clone returns a defensive copy.
func (q *FunctionScoreQuery) Clone() search.Query {
	return &FunctionScoreQuery{inner: q.inner.Clone(), source: q.source}
}

// Equals checks value equality.
func (q *FunctionScoreQuery) Equals(other search.Query) bool {
	o, ok := other.(*FunctionScoreQuery)
	if !ok || o == nil {
		return false
	}
	return q.inner.Equals(o.inner) && q.source.Equals(o.source)
}

// HashCode combines inner and source hashes.
func (q *FunctionScoreQuery) HashCode() int {
	return int(combineHash(int32(q.inner.HashCode()), q.source.HashCode()))
}

// VisitorVisitable is the optional contract implemented by queries that
// support the QueryVisitor pattern. The search.Query interface does not
// (yet) include Visit, so we type-assert when descending into wrapped
// queries.
type VisitorVisitable interface {
	Visit(visitor search.QueryVisitor)
}

// Visit drills into the inner query as a MUST sub-clause, mirroring Java.
// Inner queries that do not implement [VisitorVisitable] are skipped.
func (q *FunctionScoreQuery) Visit(visitor search.QueryVisitor) {
	sub := visitor.GetSubVisitor(search.MUST, q)
	if v, ok := q.inner.(VisitorVisitable); ok {
		v.Visit(sub)
	} else {
		sub.VisitLeaf(q.inner)
	}
}

// CreateWeight produces the score-substituting weight.
func (q *FunctionScoreQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	innerNeedsScores := needsScores && q.source.NeedsScores()
	innerWeight, err := q.inner.CreateWeight(searcher, innerNeedsScores, 1)
	if err != nil {
		return nil, err
	}
	if !needsScores {
		return innerWeight, nil
	}
	rewritten, err := q.source.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	return &functionScoreWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		inner:      innerWeight,
		source:     rewritten,
		boost:      boost,
	}, nil
}

// functionScoreWeight implements search.Weight for FunctionScoreQuery.
type functionScoreWeight struct {
	*search.BaseWeight
	query  *FunctionScoreQuery
	inner  search.Weight
	source DoubleValuesSource
	boost  float32
}

func (w *functionScoreWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.inner.IsCacheable(ctx) && w.source.IsCacheable(ctx)
}

func (w *functionScoreWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	in, err := w.inner.Scorer(ctx)
	if err != nil || in == nil {
		return in, err
	}
	scores, err := w.source.GetValues(ctx, scorerAsDoubleValues(in))
	if err != nil {
		return nil, err
	}
	return &functionScoreScorer{inner: in, values: scores, boost: w.boost}, nil
}

func (w *functionScoreWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewScorerSupplierAdapter(scorer), nil
}

func (w *functionScoreWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *functionScoreWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	return w.inner.Matches(ctx, doc)
}

func (w *functionScoreWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return w.inner.Count(ctx)
}

func (w *functionScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	innerExpl, err := w.inner.Explain(ctx, doc)
	if err != nil {
		return nil, err
	}
	if !innerExpl.IsMatch() {
		return innerExpl, nil
	}
	scores, err := w.source.GetValues(ctx, EmptyDoubleValues)
	if err != nil {
		return nil, err
	}
	var value float64
	expl, err := w.source.Explain(ctx, doc, innerExpl)
	if err != nil {
		return nil, err
	}
	ok, err := scores.AdvanceExact(doc)
	if err != nil {
		return nil, err
	}
	if ok {
		v, err := scores.DoubleValue()
		if err != nil {
			return nil, err
		}
		value = v
		if value < 0 {
			value = 0
			truncated := search.MatchExplanation(0, "truncated score, max of:")
			truncated.AddDetail(search.NewExplanation(true, 0, "minimum score"))
			truncated.AddDetail(expl)
			expl = truncated
		} else if math.IsNaN(value) {
			value = 0
			nanWrap := search.MatchExplanation(0, "score, computed as (score == NaN ? 0 : score) since NaN is an illegal score from:")
			nanWrap.AddDetail(expl)
			expl = nanWrap
		}
	} else {
		value = 0
	}
	if !expl.IsMatch() {
		envelope := search.MatchExplanation(0,
			fmt.Sprintf("weight(%v) using default score of 0 because the function produced no value:", w.query))
		envelope.AddDetail(expl)
		return envelope, nil
	}
	if w.boost != 1 {
		boosted := search.MatchExplanation(float32(value*float64(w.boost)),
			fmt.Sprintf("weight(%v), product of:", w.query))
		boosted.AddDetail(search.NewExplanation(true, w.boost, "boost"))
		boosted.AddDetail(expl)
		return boosted, nil
	}
	wrapper := search.MatchExplanation(expl.GetValue(),
		fmt.Sprintf("weight(%v), result of:", w.query))
	wrapper.AddDetail(expl)
	return wrapper, nil
}

// functionScoreScorer wraps an inner search.Scorer and replaces its score
// with the [DoubleValuesSource] output.
type functionScoreScorer struct {
	inner  search.Scorer
	values DoubleValues
	boost  float32
}

func (s *functionScoreScorer) DocID() int                 { return s.inner.DocID() }
func (s *functionScoreScorer) NextDoc() (int, error)      { return s.inner.NextDoc() }
func (s *functionScoreScorer) Advance(t int) (int, error) { return s.inner.Advance(t) }
func (s *functionScoreScorer) Cost() int64                { return s.inner.Cost() }
func (s *functionScoreScorer) DocIDRunEnd() int           { return s.inner.DocIDRunEnd() }
func (s *functionScoreScorer) GetMaxScore(_ int) float32  { return float32(math.Inf(1)) }

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Lucene's FunctionScoreQuery
// scorer overrides only getMaxScore (returning Float.MAX_VALUE) and inherits
// the advanceShallow default, so the whole remaining list is one block.
func (s *functionScoreScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

func (s *functionScoreScorer) Score() float32 {
	ok, err := s.values.AdvanceExact(s.inner.DocID())
	if err != nil || !ok {
		return 0
	}
	v, err := s.values.DoubleValue()
	if err != nil || v < 0 || math.IsNaN(v) {
		return 0
	}
	return float32(v * float64(s.boost))
}

// scorerAsDoubleValues adapts a search.Scorer.Score() snapshot into a
// DoubleValues view, mirroring DoubleValuesSource.fromScorer().
func scorerAsDoubleValues(scorer search.Scorer) DoubleValues {
	return &scorerDoubleValues{scorer: scorer}
}

type scorerDoubleValues struct {
	scorer search.Scorer
}

func (s *scorerDoubleValues) DoubleValue() (float64, error)    { return float64(s.scorer.Score()), nil }
func (s *scorerDoubleValues) AdvanceExact(_ int) (bool, error) { return true, nil }

// multiplicativeBoostValuesSource multiplies the upstream score by a
// per-doc boost value; missing values map to 1 (preserves score).
type multiplicativeBoostValuesSource struct {
	boost DoubleValuesSource
}

func (m *multiplicativeBoostValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error) {
	inner, err := m.boost.GetValues(ctx, scores)
	if err != nil {
		return nil, err
	}
	return &multiplicativeBoostValues{scores: scores, boost: DoubleValuesWithDefault(inner, 1)}, nil
}
func (m *multiplicativeBoostValuesSource) NeedsScores() bool { return true }
func (m *multiplicativeBoostValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool {
	return m.boost.IsCacheable(ctx)
}
func (m *multiplicativeBoostValuesSource) Rewrite(s *search.IndexSearcher) (DoubleValuesSource, error) {
	rewritten, err := m.boost.Rewrite(s)
	if err != nil {
		return nil, err
	}
	return &multiplicativeBoostValuesSource{boost: rewritten}, nil
}
func (m *multiplicativeBoostValuesSource) Equals(other DoubleValuesSource) bool {
	o, ok := other.(*multiplicativeBoostValuesSource)
	if !ok || o == nil {
		return false
	}
	return m.boost.Equals(o.boost)
}
func (m *multiplicativeBoostValuesSource) HashCode() int32 { return m.boost.HashCode() }
func (m *multiplicativeBoostValuesSource) Description() string {
	return "boost(" + m.boost.Description() + ")"
}
func (m *multiplicativeBoostValuesSource) Explain(ctx *index.LeafReaderContext, doc int, scoreExplanation search.Explanation) (search.Explanation, error) {
	if !scoreExplanation.IsMatch() {
		return scoreExplanation, nil
	}
	boostExpl, err := m.boost.Explain(ctx, doc, scoreExplanation)
	if err != nil {
		return nil, err
	}
	if !boostExpl.IsMatch() {
		return scoreExplanation, nil
	}
	root := search.MatchExplanation(scoreExplanation.GetValue()*boostExpl.GetValue(), "product of:")
	root.AddDetail(scoreExplanation)
	root.AddDetail(boostExpl)
	return root, nil
}

type multiplicativeBoostValues struct {
	scores DoubleValues
	boost  DoubleValues
}

func (m *multiplicativeBoostValues) DoubleValue() (float64, error) {
	s, err := m.scores.DoubleValue()
	if err != nil {
		return 0, err
	}
	b, err := m.boost.DoubleValue()
	if err != nil {
		return 0, err
	}
	return s * b, nil
}

func (m *multiplicativeBoostValues) AdvanceExact(doc int) (bool, error) {
	return m.boost.AdvanceExact(doc)
}

var (
	_ search.Query  = (*FunctionScoreQuery)(nil)
	_ search.Weight = (*functionScoreWeight)(nil)
	_ search.Scorer = (*functionScoreScorer)(nil)
)
