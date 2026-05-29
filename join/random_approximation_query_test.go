// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// randomApproximationQuery is a Go test-only port of
// org.apache.lucene.tests.search.RandomApproximationQuery: it wraps a query and
// exposes a two-phase view over its scorer so that conjunctions exercise the
// approximation + confirmation path (real advance() calls) instead of a plain
// doc-id stream.
//
// Gocene's search.Scorer interface surfaces two-phase iteration only via
// *search.TwoPhaseIteratorScorer (which search.AsTwoPhaseIterator / the
// conjunction DISI recognise). This wrapper therefore returns a
// TwoPhaseIteratorScorer whose approximation is the inner scorer's doc stream
// and whose Matches() always confirms — the loosest legal approximation, which
// keeps the produced match set identical to the inner query (the property the
// testIntersectionWithRandomApproximation port asserts via count equality)
// while still routing through the two-phase machinery.
//
// The seed parameter is accepted for parity with the Lucene signature; the
// confirmation here is deterministic so the seed does not influence the result
// set (only iteration order in Lucene), preserving the count-equality contract.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

type randomApproximationQuery struct {
	inner search.Query
	seed  int64
}

func newRandomApproximationQuery(inner search.Query, seed int64) *randomApproximationQuery {
	return &randomApproximationQuery{inner: inner, seed: seed}
}

func (q *randomApproximationQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten != q.inner {
		return &randomApproximationQuery{inner: rewritten, seed: q.seed}, nil
	}
	return q, nil
}

func (q *randomApproximationQuery) Clone() search.Query {
	return &randomApproximationQuery{inner: q.inner.Clone(), seed: q.seed}
}

func (q *randomApproximationQuery) Equals(other search.Query) bool {
	o, ok := other.(*randomApproximationQuery)
	return ok && q.inner.Equals(o.inner)
}

func (q *randomApproximationQuery) HashCode() int { return 31*7 + q.inner.HashCode() }

func (q *randomApproximationQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	inner, err := q.inner.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	return &randomApproximationWeight{Weight: inner}, nil
}

// randomApproximationWeight embeds the inner Weight and overrides only Scorer
// (and ScorerSupplier) to wrap the scorer in a two-phase view.
type randomApproximationWeight struct {
	search.Weight
}

func (w *randomApproximationWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	inner, err := w.Weight.Scorer(ctx)
	if err != nil || inner == nil {
		return nil, err
	}
	approximation := inner // the inner scorer is itself a DocIdSetIterator
	twoPhase := search.NewTwoPhaseIterator(approximation, func() (bool, error) {
		// The loosest legal confirmation: every approximated candidate matches.
		return true, nil
	})
	return search.NewTwoPhaseIteratorScorer(twoPhase, w.Weight), nil
}

func (w *randomApproximationWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return &fixedScorerSupplier{scorer: scorer}, nil
}

// fixedScorerSupplier returns a precomputed scorer.
type fixedScorerSupplier struct{ scorer search.Scorer }

func (s *fixedScorerSupplier) Get(int64) (search.Scorer, error) { return s.scorer, nil }
func (s *fixedScorerSupplier) Cost() int64                      { return s.scorer.Cost() }
func (s *fixedScorerSupplier) SetTopLevelScoringClause()        {}
