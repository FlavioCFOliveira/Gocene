// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// LMDirichletSimilarity implements language modeling with Dirichlet smoothing.
// This is one of the most effective smoothing methods for language models
// in information retrieval.
//
// The formula:
// Score = log((tf + mu * P(w|C)) / (docLen + mu))
// where:
// - tf is term frequency in document
// - mu is Dirichlet smoothing parameter (default: 2000)
// - P(w|C) is collection language model probability
// - docLen is document length
//
// Dirichlet smoothing is a Bayesian approach that interpolates between
// the document model and the collection model based on document length.
type LMDirichletSimilarity struct {
	*BaseSimilarity
	mu float64 // Dirichlet smoothing parameter
}

// NewLMDirichletSimilarity creates a new LMDirichletSimilarity with default parameters.
func NewLMDirichletSimilarity() *LMDirichletSimilarity {
	return &LMDirichletSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		mu:             2000.0,
	}
}

// NewLMDirichletSimilarityWithParams creates an LMDirichletSimilarity with custom parameters.
func NewLMDirichletSimilarityWithParams(mu float64) *LMDirichletSimilarity {
	if mu <= 0 {
		mu = 2000.0
	}
	return &LMDirichletSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		mu:             mu,
	}
}

// Mu returns the mu parameter.
func (s *LMDirichletSimilarity) Mu() float64 { return s.mu }

// ComputeNorm computes the norm value for a field.
func (s *LMDirichletSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *LMDirichletSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *LMDirichletSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *LMDirichletSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewLMDirichletSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *LMDirichletSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewLMDirichletSimScorer(s, collectionStats, termStats)
}

// LMDirichletSimWeight holds the weight for LM Dirichlet scoring.
type LMDirichletSimWeight struct {
	sim             *LMDirichletSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	collectionProb  float64 // P(w|C) - probability of term in collection
}

// NewLMDirichletSimWeight creates a new LMDirichletSimWeight.
func NewLMDirichletSimWeight(sim *LMDirichletSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *LMDirichletSimWeight {
	// Calculate collection probability: P(w|C) = F / collectionSize
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10 // Small epsilon to avoid issues
	}

	return &LMDirichletSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		collectionProb:  collectionProb,
	}
}

// GetValue returns the value for this weight.
func (w *LMDirichletSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *LMDirichletSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *LMDirichletSimWeight) Scorer() SimScorer {
	return NewLMDirichletSimScorerWithWeight(w)
}

// LMDirichletSimScorer is a scorer for LMDirichletSimilarity.
type LMDirichletSimScorer struct {
	*BaseSimScorer
	similarity     *LMDirichletSimilarity
	weight         *LMDirichletSimWeight
	mu             float64
	collectionProb float64
}

// NewLMDirichletSimScorer creates a new LMDirichletSimScorer.
func NewLMDirichletSimScorer(similarity *LMDirichletSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *LMDirichletSimScorer {
	// Calculate collection probability
	collectionProb := 0.0
	if collectionStats != nil && collectionStats.SumTotalTermFreq() > 0 && termStats != nil {
		collectionProb = float64(termStats.TotalTermFreq()) / float64(collectionStats.SumTotalTermFreq())
	}
	if collectionProb == 0 {
		collectionProb = 1e-10
	}

	return &LMDirichletSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     similarity,
		mu:             similarity.mu,
		collectionProb: collectionProb,
	}
}

// NewLMDirichletSimScorerWithWeight creates a new LMDirichletSimScorer with weight.
func NewLMDirichletSimScorerWithWeight(weight *LMDirichletSimWeight) *LMDirichletSimScorer {
	return &LMDirichletSimScorer{
		BaseSimScorer:  NewBaseSimScorer(),
		similarity:     weight.sim,
		weight:         weight,
		mu:             weight.sim.mu,
		collectionProb: weight.collectionProb,
	}
}

// Score calculates the LM Dirichlet score.
// Score = log((tf + mu * P(w|C)) / (docLen + mu))
func (s *LMDirichletSimScorer) Score(doc int, freq float32) float32 {
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

	// Dirichlet smoothing: log((tf + mu * P(w|C)) / (docLen + mu))
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

// Ensure LMDirichletSimilarity implements Similarity
var _ Similarity = (*LMDirichletSimilarity)(nil)

// Ensure LMDirichletSimScorer implements SimScorer
var _ SimScorer = (*LMDirichletSimScorer)(nil)
