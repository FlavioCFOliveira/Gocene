// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LucenePerFieldSimilarityGetter is the per-field Similarity resolver — a
// function value replacing Java's abstract `Similarity get(String name)`.
type LucenePerFieldSimilarityGetter func(field string) LuceneSimilarity

// LucenePerFieldSimilarityWrapper mirrors org.apache.lucene.search.
// similarities.PerFieldSimilarityWrapper from Lucene 10.4.0. It dispatches
// computeNorm and Scorer104 to a per-field Similarity returned by Get.
//
// The legacy [PerFieldSimilarityWrapper] struct (map-backed, no getter
// function) is preserved untouched.
type LucenePerFieldSimilarityWrapper struct {
	get LucenePerFieldSimilarityGetter
}

// NewLucenePerFieldSimilarityWrapper returns a wrapper backed by the given
// getter. The getter must return a non-nil Similarity for every queried
// field; if it returns nil we fall back to a noop scorer for that field.
func NewLucenePerFieldSimilarityWrapper(getter LucenePerFieldSimilarityGetter) *LucenePerFieldSimilarityWrapper {
	if getter == nil {
		panic("LucenePerFieldSimilarityWrapper: getter must not be nil")
	}
	return &LucenePerFieldSimilarityWrapper{get: getter}
}

// Get returns the per-field Similarity. Exposed for parity with Java's
// abstract `Similarity get(String name)`.
func (s *LucenePerFieldSimilarityWrapper) Get(field string) LuceneSimilarity {
	return s.get(field)
}

// GetDiscountOverlaps satisfies LuceneSimilarity. There is no obvious
// answer at the wrapper level — Java does not override it — so we mirror
// the no-arg Similarity default of true.
func (s *LucenePerFieldSimilarityWrapper) GetDiscountOverlaps() bool { return true }

// ComputeNormFromInvertState delegates to the per-field Similarity, keyed
// by the FieldInvertState's field name.
func (s *LucenePerFieldSimilarityWrapper) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	if state == nil {
		return 1
	}
	sim := s.get(state.Name())
	if sim == nil {
		return DefaultComputeNormFromInvertState(state, true)
	}
	return sim.ComputeNormFromInvertState(state)
}

// Scorer104 delegates to the per-field Similarity, keyed by the
// CollectionStatistics' field name.
func (s *LucenePerFieldSimilarityWrapper) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer {
	if collectionStats == nil {
		return &noopLuceneSimScorer{}
	}
	sim := s.get(collectionStats.Field())
	if sim == nil {
		return &noopLuceneSimScorer{}
	}
	return sim.Scorer104(boost, collectionStats, termStats...)
}

// Compile-time guarantee.
var _ LuceneSimilarity = (*LucenePerFieldSimilarityWrapper)(nil)
