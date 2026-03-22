// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// IndriSimilarity implements the Indri retrieval model.
// Indri is a language modeling approach used in the Lemur toolkit,
// combining Dirichlet smoothing with a probabilistic framework.
//
// The Indri scoring formula:
// Score = log((tf + mu * P(w|C)) / (docLen + mu))
// where:
// - tf is term frequency in document
// - mu is smoothing parameter
// - P(w|C) is collection probability of term
// - docLen is document length
type IndriSimilarity struct {
	*BaseSimilarity
	mu float64 // Smoothing parameter (default: 2500)
}

// NewIndriSimilarity creates a new IndriSimilarity with default parameters.
func NewIndriSimilarity() *IndriSimilarity {
	return &IndriSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		mu:             2500.0,
	}
}

// NewIndriSimilarityWithParams creates an IndriSimilarity with custom parameters.
func NewIndriSimilarityWithParams(mu float64) *IndriSimilarity {
	if mu <= 0 {
		mu = 2500.0
	}
	return &IndriSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		mu:             mu,
	}
}

// Mu returns the mu parameter.
func (s *IndriSimilarity) Mu() float64 { return s.mu }

// ComputeNorm computes the norm value for a field.
func (s *IndriSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *IndriSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *IndriSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *IndriSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewIndriSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *IndriSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewIndriSimScorer(s, collectionStats, termStats)
}

// IndriSimWeight holds the weight for Indri scoring.
type IndriSimWeight struct {
	sim             *IndriSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	collectionProb  float64 // P(w|C) - probability of term in collection
}

// NewIndriSimWeight creates a new IndriSimWeight.
func NewIndriSimWeight(sim *IndriSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *IndriSimWeight {
	// Calculate collection probability: P(w|C) = F / collectionSize
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil && termStats.TotalTermFreq() > 0 {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10 // Small epsilon to avoid log(0)
	}

	return &IndriSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		collectionProb:  collectionProb,
	}
}

// GetValue returns the value for this weight.
func (w *IndriSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *IndriSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *IndriSimWeight) Scorer() SimScorer {
	return NewIndriSimScorerWithWeight(w)
}

// IndriSimScorer is a scorer for IndriSimilarity.
type IndriSimScorer struct {
	*BaseSimScorer
	similarity     *IndriSimilarity
	weight         *IndriSimWeight
	mu             float64
	collectionProb float64
}

// NewIndriSimScorer creates a new IndriSimScorer.
func NewIndriSimScorer(similarity *IndriSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *IndriSimScorer {
	// Calculate collection probability
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil && termStats.TotalTermFreq() > 0 {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10
	}

	return &IndriSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     similarity,
		mu:             similarity.mu,
		collectionProb: collectionProb,
	}
}

// NewIndriSimScorerWithWeight creates a new IndriSimScorer with weight.
func NewIndriSimScorerWithWeight(weight *IndriSimWeight) *IndriSimScorer {
	return &IndriSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     weight.sim,
		weight:         weight,
		mu:             weight.sim.mu,
		collectionProb: weight.collectionProb,
	}
}

// Score calculates the Indri score.
// Score = log((tf + mu * P(w|C)) / (docLen + mu))
func (s *IndriSimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 {
		return 0
	}

	tf := float64(freq)

	// Document length (simplified - assume average length)
	docLen := 1.0
	if s.weight != nil && s.weight.collectionStats != nil && s.weight.collectionStats.DocCount() > 0 {
		totalTerms := float64(s.weight.collectionStats.SumTotalTermFreq())
		docCount := float64(s.weight.collectionStats.DocCount())
		if docCount > 0 {
			docLen = totalTerms / docCount
		}
	}

	// Indri formula: log((tf + mu * P(w|C)) / (docLen + mu))
	numerator := tf + s.mu*s.collectionProb
	denominator := docLen + s.mu

	if denominator <= 0 {
		return 0
	}

	score := math.Log(numerator / denominator)

	// Apply boost
	if s.weight != nil {
		score *= float64(s.weight.boost)
	}

	return float32(score)
}

// Ensure IndriSimilarity implements Similarity
var _ Similarity = (*IndriSimilarity)(nil)

// Ensure IndriSimScorer implements SimScorer
var _ SimScorer = (*IndriSimScorer)(nil)
