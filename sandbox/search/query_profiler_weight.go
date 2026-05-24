// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerWeight.
package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryProfilerWeight wraps a search.Weight to measure how long it takes to
// build a Scorer (BuildScorer timing), count matching documents (Count timing),
// and to return a QueryProfilerScorer that records iteration timings.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerWeight.
type QueryProfilerWeight struct {
	in      search.Weight
	profile *QueryProfilerBreakdown
}

// newQueryProfilerWeight wraps in with profiling timers sourced from profile.
func newQueryProfilerWeight(in search.Weight, profile *QueryProfilerBreakdown) *QueryProfilerWeight {
	return &QueryProfilerWeight{in: in, profile: profile}
}

// GetQuery returns the parent query of the wrapped weight.
func (w *QueryProfilerWeight) GetQuery() search.Query {
	return w.in.GetQuery()
}

// Explain delegates to the inner weight.
func (w *QueryProfilerWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return w.in.Explain(ctx, doc)
}

// Count times the count call via TimingTypeCount and delegates to the inner weight.
func (w *QueryProfilerWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	timer := w.profile.GetTimer(TimingTypeCount)
	timer.Start()
	defer timer.Stop()
	return w.in.Count(ctx)
}

// ScorerSupplier times the initial build via TimingTypeBuildScorer, then returns
// a profiling ScorerSupplier whose Get and Cost calls are also timed.
// Returns nil if the inner weight returns nil.
func (w *QueryProfilerWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	timer := w.profile.GetTimer(TimingTypeBuildScorer)
	timer.Start()
	inner, err := w.in.ScorerSupplier(ctx)
	timer.Stop()
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, nil
	}
	return &queryProfilerScorerSupplier{
		inner:   inner,
		timer:   timer,
		profile: w.profile,
	}, nil
}

// Scorer delegates via ScorerSupplier with zero lead cost.
func (w *QueryProfilerWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

// BulkScorer delegates via Scorer; uses the default bulk scoring path so that
// time can be attributed to individual operations.
func (w *QueryProfilerWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// IsCacheable always returns false; profiled weights must not be cached.
func (w *QueryProfilerWeight) IsCacheable(_ *index.LeafReaderContext) bool {
	return false
}

// Matches delegates to the inner weight.
func (w *QueryProfilerWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	return w.in.Matches(ctx, doc)
}

var _ search.Weight = (*QueryProfilerWeight)(nil)

// queryProfilerScorerSupplier is a search.ScorerSupplier that wraps an inner
// supplier and times Get and Cost via the BuildScorer timer.
type queryProfilerScorerSupplier struct {
	inner   search.ScorerSupplier
	timer   *QueryProfilerTimer
	profile *QueryProfilerBreakdown
}

// Get times the scorer construction and returns a QueryProfilerScorer.
func (s *queryProfilerScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	s.timer.Start()
	scorer, err := s.inner.Get(leadCost)
	s.timer.Stop()
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return newQueryProfilerScorer(scorer, s.profile), nil
}

// Cost times the cost computation via the BuildScorer timer and delegates.
func (s *queryProfilerScorerSupplier) Cost() int64 {
	s.timer.Start()
	c := s.inner.Cost()
	s.timer.Stop()
	return c
}

// SetTopLevelScoringClause delegates to the inner supplier.
func (s *queryProfilerScorerSupplier) SetTopLevelScoringClause() {
	s.inner.SetTopLevelScoringClause()
}

var _ search.ScorerSupplier = (*queryProfilerScorerSupplier)(nil)
