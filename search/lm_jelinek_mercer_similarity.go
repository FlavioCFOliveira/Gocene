// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// LMJelinekMercerSimilarity implements language modeling with Jelinek-Mercer smoothing.
// This is a linear interpolation smoothing method that combines document and
// collection language models.
//
// The formula:
// P(w|d) = lambda * P(w|d) + (1 - lambda) * P(w|C)
// Score = log(P(w|d))
// where:
// - lambda is the interpolation parameter (0 < lambda < 1)
// - P(w|d) = tf / docLen (document probability)
// - P(w|C) is collection probability
//
// Jelinek-Mercer smoothing interpolates between the document model and
// collection model with fixed lambda regardless of document length.
type LMJelinekMercerSimilarity struct {
	*BaseSimilarity
	lambda float64 // Interpolation parameter (default: 0.7)
}

// NewLMJelinekMercerSimilarity creates a new LMJelinekMercerSimilarity with default parameters.
func NewLMJelinekMercerSimilarity() *LMJelinekMercerSimilarity {
	return &LMJelinekMercerSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		lambda:         0.7,
	}
}

// NewLMJelinekMercerSimilarityWithParams creates an LMJelinekMercerSimilarity with custom parameters.
func NewLMJelinekMercerSimilarityWithParams(lambda float64) *LMJelinekMercerSimilarity {
	if lambda < 0 {
		lambda = 0.0
	}
	if lambda > 1 {
		lambda = 1.0
	}
	return &LMJelinekMercerSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		lambda:         lambda,
	}
}

// Lambda returns the lambda parameter.
func (s *LMJelinekMercerSimilarity) Lambda() float64 { return s.lambda }

// ComputeNorm computes the norm value for a field.
func (s *LMJelinekMercerSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *LMJelinekMercerSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *LMJelinekMercerSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *LMJelinekMercerSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewLMJelinekMercerSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *LMJelinekMercerSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewLMJelinekMercerSimScorer(s, collectionStats, termStats)
}

// LMJelinekMercerSimWeight holds the weight for LM Jelinek-Mercer scoring.
type LMJelinekMercerSimWeight struct {
	sim             *LMJelinekMercerSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	collectionProb  float64 // P(w|C) - probability of term in collection
}

// NewLMJelinekMercerSimWeight creates a new LMJelinekMercerSimWeight.
func NewLMJelinekMercerSimWeight(sim *LMJelinekMercerSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *LMJelinekMercerSimWeight {
	// Calculate collection probability: P(w|C) = F / collectionSize
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10 // Small epsilon to avoid log(0)
	}

	return &LMJelinekMercerSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		collectionProb:  collectionProb,
	}
}

// GetValue returns the value for this weight.
func (w *LMJelinekMercerSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *LMJelinekMercerSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *LMJelinekMercerSimWeight) Scorer() SimScorer {
	return NewLMJelinekMercerSimScorerWithWeight(w)
}

// LMJelinekMercerSimScorer is a scorer for LMJelinekMercerSimilarity.
type LMJelinekMercerSimScorer struct {
	*BaseSimScorer
	similarity     *LMJelinekMercerSimilarity
	weight         *LMJelinekMercerSimWeight
	lambda         float64
	collectionProb float64
}

// NewLMJelinekMercerSimScorer creates a new LMJelinekMercerSimScorer.
func NewLMJelinekMercerSimScorer(similarity *LMJelinekMercerSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *LMJelinekMercerSimScorer {
	// Calculate collection probability
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10
	}

	return &LMJelinekMercerSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     similarity,
		lambda:         similarity.lambda,
		collectionProb: collectionProb,
	}
}

// NewLMJelinekMercerSimScorerWithWeight creates a new LMJelinekMercerSimScorer with weight.
func NewLMJelinekMercerSimScorerWithWeight(weight *LMJelinekMercerSimWeight) *LMJelinekMercerSimScorer {
	return &LMJelinekMercerSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     weight.sim,
		weight:         weight,
		lambda:         weight.sim.lambda,
		collectionProb: weight.collectionProb,
	}
}

// Score calculates the LM Jelinek-Mercer score.
// Score = log(lambda * P(w|d) + (1 - lambda) * P(w|C))
// where P(w|d) = tf / docLen
func (s *LMJelinekMercerSimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 {
		return 0
	}

	tf := float64(freq)

	// Document length (simplified - use average document length)
	docLen := 1.0
	if s.weight != nil && s.weight.collectionStats != nil && s.weight.collectionStats.DocCount() > 0 {
		totalTerms := float64(s.weight.collectionStats.SumTotalTermFreq())
		docCount := float64(s.weight.collectionStats.DocCount())
		if docCount > 0 {
			docLen = totalTerms / docCount
		}
	}

	if docLen <= 0 {
		docLen = 1.0
	}

	// P(w|d) = tf / docLen
	docProb := tf / docLen

	// Jelinek-Mercer smoothing: lambda * P(w|d) + (1 - lambda) * P(w|C)
	smoothedProb := s.lambda*docProb + (1.0-s.lambda)*s.collectionProb

	if smoothedProb <= 0 {
		return 0
	}

	score := math.Log(smoothedProb)

	// Apply boost
	if s.weight != nil {
		score *= float64(s.weight.boost)
	}

	return float32(score)
}

// Ensure LMJelinekMercerSimilarity implements Similarity
var _ Similarity = (*LMJelinekMercerSimilarity)(nil)

// Ensure LMJelinekMercerSimScorer implements SimScorer
var _ SimScorer = (*LMJelinekMercerSimScorer)(nil)
