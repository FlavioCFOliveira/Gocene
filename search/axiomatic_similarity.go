// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// AxiomaticSimilarity implements the axiomatic retrieval model.
// This is based on the theory that a good retrieval function should satisfy
// certain axioms about term weighting and document scoring.
type AxiomaticSimilarity struct {
	s float64 // parameter for controlling document length normalization
	k float64 // parameter for term frequency saturation
}

// NewAxiomaticSimilarity creates a new AxiomaticSimilarity with default parameters.
func NewAxiomaticSimilarity() *AxiomaticSimilarity {
	return &AxiomaticSimilarity{
		s: 0.5,
		k: 1.0,
	}
}

// NewAxiomaticSimilarityWithParams creates a new AxiomaticSimilarity with custom parameters.
func NewAxiomaticSimilarityWithParams(s, k float64) *AxiomaticSimilarity {
	return &AxiomaticSimilarity{
		s: s,
		k: k,
	}
}

// Coord returns the coordination factor.
func (sim *AxiomaticSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (sim *AxiomaticSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeNorm computes the normalization value for a document.
func (sim *AxiomaticSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	// Axiomatic length normalization
	// In a full implementation, stats would contain document length
	return 1.0
}

// ComputeWeight computes the weight for a term.
func (sim *AxiomaticSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewAxiomaticSimWeight(sim, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (sim *AxiomaticSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewAxiomaticSimScorer(sim, collectionStats, termStats)
}

// AxiomaticSimWeight is the weight for AxiomaticSimilarity.
type AxiomaticSimWeight struct {
	sim             *AxiomaticSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
}

// NewAxiomaticSimWeight creates a new AxiomaticSimWeight.
func NewAxiomaticSimWeight(sim *AxiomaticSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *AxiomaticSimWeight {
	return &AxiomaticSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
	}
}

// GetValue returns the value for this weight.
func (w *AxiomaticSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *AxiomaticSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *AxiomaticSimWeight) Scorer() SimScorer {
	return NewAxiomaticSimScorerWithWeight(w)
}

// AxiomaticSimScorer is a scorer for AxiomaticSimilarity.
type AxiomaticSimScorer struct {
	*BaseSimScorer
	sim    *AxiomaticSimilarity
	weight *AxiomaticSimWeight
}

// NewAxiomaticSimScorer creates a new AxiomaticSimScorer.
func NewAxiomaticSimScorer(sim *AxiomaticSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *AxiomaticSimScorer {
	return &AxiomaticSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		sim:           sim,
	}
}

// NewAxiomaticSimScorerWithWeight creates a new AxiomaticSimScorer with weight.
func NewAxiomaticSimScorerWithWeight(weight *AxiomaticSimWeight) *AxiomaticSimScorer {
	return &AxiomaticSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		sim:           weight.sim,
		weight:        weight,
	}
}

// Score calculates the axiomatic score.
// Score = boost * tf / (k + tf) * log((N + 1) / (df + 1))
func (s *AxiomaticSimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 {
		return 0
	}

	k := s.sim.k
	tf := float64(freq)

	// Term frequency component (saturation)
	tfComponent := tf / (k + tf)

	// IDF component
	idf := 1.0
	if s.weight != nil && s.weight.termStats != nil && s.weight.collectionStats != nil {
		df := float64(s.weight.termStats.DocFreq())
		N := float64(s.weight.collectionStats.DocCount())
		if df > 0 {
			idf = math.Log((N + 1) / (df + 1))
		}
	}

	// Final score
	score := float32(tfComponent * idf)
	if s.weight != nil {
		score *= s.weight.boost
	}

	return score
}

// Ensure AxiomaticSimilarity implements Similarity
var _ Similarity = (*AxiomaticSimilarity)(nil)

// Ensure AxiomaticSimScorer implements SimScorer
var _ SimScorer = (*AxiomaticSimScorer)(nil)
