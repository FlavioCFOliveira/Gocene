// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LuceneMultiSimilarity mirrors org.apache.lucene.search.similarities.
// MultiSimilarity from Lucene 10.4.0. It implements the CombSUM method:
// each sub-Similarity scores the same (freq, norm) and the scores are
// summed.
//
// The legacy [MultiSimilarity] struct (weighted average) is preserved
// for backwards compatibility; LuceneMultiSimilarity is byte-equivalent to
// the Java reference and should be used for canonical scoring.
type LuceneMultiSimilarity struct {
	sims []LuceneSimilarity
}

// NewLuceneMultiSimilarity wraps the given Similarity slice. The slice
// must contain at least one entry — ComputeNormFromInvertState consults
// sims[0].
func NewLuceneMultiSimilarity(sims []LuceneSimilarity) *LuceneMultiSimilarity {
	if len(sims) == 0 {
		panic("LuceneMultiSimilarity: at least one similarity is required")
	}
	cp := make([]LuceneSimilarity, len(sims))
	copy(cp, sims)
	return &LuceneMultiSimilarity{sims: cp}
}

// Sims returns a copy of the underlying similarities for inspection. The
// returned slice is independent of the wrapper's internal storage.
func (s *LuceneMultiSimilarity) Sims() []LuceneSimilarity {
	out := make([]LuceneSimilarity, len(s.sims))
	copy(out, s.sims)
	return out
}

// GetDiscountOverlaps mirrors Java: there is no explicit override on
// MultiSimilarity; sims[0] is the source of truth for index-time norm
// configuration.
func (s *LuceneMultiSimilarity) GetDiscountOverlaps() bool {
	return s.sims[0].GetDiscountOverlaps()
}

// ComputeNormFromInvertState delegates to sims[0], matching Java.
func (s *LuceneMultiSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	return s.sims[0].ComputeNormFromInvertState(state)
}

// Scorer104 builds a per-sub-similarity scorer and wraps them in a
// LuceneMultiSimScorer.
func (s *LuceneMultiSimilarity) Scorer104(boost float32, collectionStats *CollectionStatistics, termStats ...*TermStatistics) LuceneSimScorer {
	subs := make([]LuceneSimScorer, len(s.sims))
	for i, sim := range s.sims {
		subs[i] = sim.Scorer104(boost, collectionStats, termStats...)
	}
	return NewLuceneMultiSimScorer(subs)
}

// LuceneMultiSimScorer mirrors MultiSimilarity.MultiSimScorer. The Java
// class is package-private; we export it because Gocene packages live in
// the same namespace and need it for cross-module composition.
type LuceneMultiSimScorer struct {
	subScorers []LuceneSimScorer
}

// NewLuceneMultiSimScorer constructs a MultiSimScorer from the given
// sub-scorers. The slice is copied to insulate the scorer from caller
// mutation.
func NewLuceneMultiSimScorer(subScorers []LuceneSimScorer) *LuceneMultiSimScorer {
	cp := make([]LuceneSimScorer, len(subScorers))
	copy(cp, subScorers)
	return &LuceneMultiSimScorer{subScorers: cp}
}

// Score104 sums the per-sub-scorer scores into a double internally then
// demotes to float32 — matching Java's `(float) sum` cast.
func (s *LuceneMultiSimScorer) Score104(freq float32, norm int64) float32 {
	var sum float64
	for _, sc := range s.subScorers {
		sum += float64(sc.Score104(freq, norm))
	}
	return float32(sum)
}

// AsBulkSimScorer returns the default bulk wrapper.
func (s *LuceneMultiSimScorer) AsBulkSimScorer() BulkSimScorer {
	return NewDefaultBulkSimScorer(s)
}

// Explain104 returns the canonical "sum of:" tree with one sub-explanation
// per sub-scorer.
func (s *LuceneMultiSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	score := s.Score104(freq.GetValue(), norm)
	root := NewExplanation(true, score, "sum of:")
	for _, sc := range s.subScorers {
		root.AddDetail(sc.Explain104(freq, norm))
	}
	return root
}

// Compile-time guarantees.
var (
	_ LuceneSimilarity = (*LuceneMultiSimilarity)(nil)
	_ LuceneSimScorer  = (*LuceneMultiSimScorer)(nil)
)
