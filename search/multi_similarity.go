// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
)

// MultiSimilarity combines multiple similarities using weighted sum.
// This allows combining different scoring approaches (e.g., BM25 + DFR)
// to potentially achieve better retrieval performance.
type MultiSimilarity struct {
	*BaseSimilarity
	similarities []Similarity
	weights      []float64
}

// NewMultiSimilarity creates a new MultiSimilarity with equal weights.
func NewMultiSimilarity(similarities []Similarity) *MultiSimilarity {
	if len(similarities) == 0 {
		panic("MultiSimilarity requires at least one similarity")
	}

	// Equal weights
	weights := make([]float64, len(similarities))
	weight := 1.0 / float64(len(similarities))
	for i := range weights {
		weights[i] = weight
	}

	return &MultiSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		similarities:   similarities,
		weights:        weights,
	}
}

// NewMultiSimilarityWithWeights creates a new MultiSimilarity with custom weights.
func NewMultiSimilarityWithWeights(similarities []Similarity, weights []float64) *MultiSimilarity {
	if len(similarities) == 0 {
		panic("MultiSimilarity requires at least one similarity")
	}
	if len(similarities) != len(weights) {
		panic("Number of similarities must equal number of weights")
	}

	return &MultiSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		similarities:   similarities,
		weights:        weights,
	}
}

// Similarities returns the underlying similarities.
func (s *MultiSimilarity) Similarities() []Similarity {
	return s.similarities
}

// Weights returns the weights for each similarity.
func (s *MultiSimilarity) Weights() []float64 {
	return s.weights
}

// ComputeNorm computes the norm value for a field.
// Uses weighted average of norms from component similarities.
func (s *MultiSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	totalWeight := 0.0
	sumNorms := 0.0

	for i, sim := range s.similarities {
		norm := float64(sim.ComputeNorm(field, stats))
		sumNorms += s.weights[i] * norm
		totalWeight += s.weights[i]
	}

	if totalWeight > 0 {
		return float32(sumNorms / totalWeight)
	}
	return 1.0
}

// Coord returns the coordination factor.
// Uses the first similarity's coordination.
func (s *MultiSimilarity) Coord(overlap, maxOverlap int) float32 {
	if len(s.similarities) > 0 {
		return s.similarities[0].Coord(overlap, maxOverlap)
	}
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *MultiSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *MultiSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewMultiSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *MultiSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewMultiSimScorer(s, collectionStats, termStats)
}

// MultiSimWeight holds the weight for MultiSimilarity scoring.
type MultiSimWeight struct {
	sim             *MultiSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	weights         []SimWeight // Weights from component similarities
}

// NewMultiSimWeight creates a new MultiSimWeight.
func NewMultiSimWeight(sim *MultiSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *MultiSimWeight {
	// Create weights from component similarities
	weights := make([]SimWeight, len(sim.similarities))
	for i, s := range sim.similarities {
		weights[i] = s.ComputeWeight(boost, collectionStats, termStats)
	}

	return &MultiSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		weights:         weights,
	}
}

// GetValue returns the value for this weight.
func (w *MultiSimWeight) GetValue() float32 {
	// Weighted average of component weights
	totalValue := 0.0
	totalWeight := 0.0
	for i, weight := range w.weights {
		totalValue += w.sim.weights[i] * float64(weight.GetValue())
		totalWeight += w.sim.weights[i]
	}
	if totalWeight > 0 {
		return float32(totalValue / totalWeight)
	}
	return w.boost
}

// Normalize normalizes this weight.
func (w *MultiSimWeight) Normalize(norm float32) {
	w.boost *= norm
	for _, weight := range w.weights {
		weight.Normalize(norm)
	}
}

// Scorer creates a scorer for this weight.
func (w *MultiSimWeight) Scorer() SimScorer {
	return NewMultiSimScorerWithWeight(w)
}

// MultiSimScorer is a scorer for MultiSimilarity.
type MultiSimScorer struct {
	*BaseSimScorer
	similarity *MultiSimilarity
	weight     *MultiSimWeight
	scorers    []SimScorer // Scorers from component similarities
}

// NewMultiSimScorer creates a new MultiSimScorer.
func NewMultiSimScorer(similarity *MultiSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *MultiSimScorer {
	// Create scorers from component similarities
	scorers := make([]SimScorer, len(similarity.similarities))
	for i, s := range similarity.similarities {
		scorers[i] = s.Scorer(collectionStats, termStats)
	}

	return &MultiSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		scorers:       scorers,
	}
}

// NewMultiSimScorerWithWeight creates a new MultiSimScorer with weight.
func NewMultiSimScorerWithWeight(weight *MultiSimWeight) *MultiSimScorer {
	// Create scorers from component weights
	scorers := make([]SimScorer, len(weight.weights))
	for i, w := range weight.weights {
		scorers[i] = w.Scorer()
	}

	return &MultiSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    weight.sim,
		weight:        weight,
		scorers:       scorers,
	}
}

// Score calculates the MultiSimilarity score.
// Score is the weighted sum of component scores.
func (s *MultiSimScorer) Score(doc int, freq float32) float32 {
	if len(s.scorers) == 0 {
		return 0
	}

	// Calculate weighted sum of scores
	totalScore := 0.0
	totalWeight := 0.0
	for i, scorer := range s.scorers {
		score := float64(scorer.Score(doc, freq))
		totalScore += s.similarity.weights[i] * score
		totalWeight += s.similarity.weights[i]
	}

	if totalWeight > 0 {
		return float32(totalScore / totalWeight)
	}
	return 0
}

// Ensure MultiSimilarity implements Similarity
var _ Similarity = (*MultiSimilarity)(nil)

// Ensure MultiSimScorer implements SimScorer
var _ SimScorer = (*MultiSimScorer)(nil)
