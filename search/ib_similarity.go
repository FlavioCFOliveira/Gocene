// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// IBSimilarity implements the Information-Based (IB) similarity model.
// IB models are based on information theory and measure the divergence
// between the actual term distribution and a reference distribution.
//
// The IB framework uses three components:
// - Distribution: The probability distribution model (e.g., Poisson, Geometric)
// - Lambda: The parameter of the distribution
// - Normalization: Document length normalization
type IBSimilarity struct {
	*BaseSimilarity
	distribution  DistributionModel // The probability distribution
	normalization NormalizationIB   // Term frequency normalization
}

// DistributionModel represents a probability distribution for IB.
type DistributionModel interface {
	// Score computes the score given term frequency and lambda
	Score(tf float64, lambda float64) float64
	// Name returns the name of this distribution
	Name() string
}

// NormalizationIB represents term frequency normalization for IB.
type NormalizationIB interface {
	// Normalize computes the normalized term frequency
	Normalize(stats *IBStats, tf float64, docLen float64) float64
	// Name returns the name of this normalization
	Name() string
}

// IBStats holds statistics needed for IB scoring.
type IBStats struct {
	TotalTermFreq int64   // Total term frequency in collection
	DocFreq       int     // Document frequency
	DocCount      int     // Number of documents
	AvgDocLength  float64 // Average document length
	Lambda        float64 // Distribution parameter
}

// NewIBSimilarity creates a new IBSimilarity with default components.
// Default: Poisson distribution with H2 normalization
func NewIBSimilarity() *IBSimilarity {
	return &IBSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		distribution:   NewDistributionPoisson(),
		normalization:  NewIBNormalizationH2(),
	}
}

// NewIBSimilarityWithParams creates an IBSimilarity with custom components.
func NewIBSimilarityWithParams(distribution DistributionModel, normalization NormalizationIB) *IBSimilarity {
	return &IBSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		distribution:   distribution,
		normalization:  normalization,
	}
}

// ComputeNorm computes the norm value for a field.
func (s *IBSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *IBSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *IBSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *IBSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewIBSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *IBSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewIBSimScorer(s, collectionStats, termStats)
}

// IBSimWeight holds the weight for IB scoring.
type IBSimWeight struct {
	sim             *IBSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	ibStats         *IBStats
}

// NewIBSimWeight creates a new IBSimWeight.
func NewIBSimWeight(sim *IBSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *IBSimWeight {
	stats := &IBStats{}
	if collectionStats != nil {
		stats.DocCount = collectionStats.DocCount()
		stats.TotalTermFreq = collectionStats.SumTotalTermFreq()
		// Calculate average document length
		if collectionStats.DocCount() > 0 {
			stats.AvgDocLength = float64(collectionStats.SumTotalTermFreq()) / float64(collectionStats.DocCount())
		}
	}
	if termStats != nil {
		stats.DocFreq = termStats.DocFreq()
		if termStats.TotalTermFreq() > 0 {
			stats.TotalTermFreq = termStats.TotalTermFreq()
		}
	}
	// Lambda = F / N (collection term frequency / number of documents)
	if stats.DocCount > 0 && stats.TotalTermFreq > 0 {
		stats.Lambda = float64(stats.TotalTermFreq) / float64(stats.DocCount)
	}

	return &IBSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		ibStats:         stats,
	}
}

// GetValue returns the value for this weight.
func (w *IBSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *IBSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *IBSimWeight) Scorer() SimScorer {
	return NewIBSimScorerWithWeight(w)
}

// IBSimScorer is a scorer for IBSimilarity.
type IBSimScorer struct {
	*BaseSimScorer
	similarity *IBSimilarity
	weight     *IBSimWeight
	ibStats    *IBStats
}

// NewIBSimScorer creates a new IBSimScorer.
func NewIBSimScorer(similarity *IBSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *IBSimScorer {
	stats := &IBStats{}
	if collectionStats != nil {
		stats.DocCount = collectionStats.DocCount()
		stats.TotalTermFreq = collectionStats.SumTotalTermFreq()
		if collectionStats.DocCount() > 0 {
			stats.AvgDocLength = float64(collectionStats.SumTotalTermFreq()) / float64(collectionStats.DocCount())
		}
	}
	if termStats != nil {
		stats.DocFreq = termStats.DocFreq()
		if termStats.TotalTermFreq() > 0 {
			stats.TotalTermFreq = termStats.TotalTermFreq()
		}
	}
	// Lambda = F / N
	if stats.DocCount > 0 && stats.TotalTermFreq > 0 {
		stats.Lambda = float64(stats.TotalTermFreq) / float64(stats.DocCount)
	}

	return &IBSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		ibStats:       stats,
	}
}

// NewIBSimScorerWithWeight creates a new IBSimScorer with weight.
func NewIBSimScorerWithWeight(weight *IBSimWeight) *IBSimScorer {
	return &IBSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    weight.sim,
		weight:        weight,
		ibStats:       weight.ibStats,
	}
}

// Score calculates the IB score.
// Score uses information-based divergence between actual and reference distributions
func (s *IBSimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 || s.ibStats == nil {
		return 0
	}

	// Document length (simplified)
	docLen := s.ibStats.AvgDocLength
	if docLen == 0 {
		docLen = 1.0
	}

	// Normalize term frequency
	tfNorm := s.similarity.normalization.Normalize(s.ibStats, float64(freq), docLen)

	// Apply distribution model
	score := s.similarity.distribution.Score(tfNorm, s.ibStats.Lambda)

	// Apply boost
	if s.weight != nil {
		score *= float64(s.weight.boost)
	}

	return float32(score)
}

// Ensure IBSimilarity implements Similarity
var _ Similarity = (*IBSimilarity)(nil)

// Ensure IBSimScorer implements SimScorer
var _ SimScorer = (*IBSimScorer)(nil)

// ============================================================================
// Distribution Model Implementations
// ============================================================================

// DistributionPoisson implements the Poisson distribution model.
type DistributionPoisson struct{}

// NewDistributionPoisson creates a new Poisson distribution.
func NewDistributionPoisson() *DistributionPoisson {
	return &DistributionPoisson{}
}

// Score computes the Poisson information score.
// Score = -log(P(X >= tf | lambda)) where X ~ Poisson(lambda)
func (d *DistributionPoisson) Score(tf float64, lambda float64) float64 {
	if lambda <= 0 {
		return 0
	}
	// Simplified Poisson: -log(1 - P(X < tf))
	// Using approximation: log(1 + tf / lambda)
	return math.Log(1.0 + tf/lambda)
}

// Name returns the name of this distribution.
func (d *DistributionPoisson) Name() string {
	return "Poisson"
}

// DistributionGeometric implements the Geometric distribution model.
type DistributionGeometric struct{}

// NewDistributionGeometric creates a new Geometric distribution.
func NewDistributionGeometric() *DistributionGeometric {
	return &DistributionGeometric{}
}

// Score computes the Geometric information score.
func (d *DistributionGeometric) Score(tf float64, lambda float64) float64 {
	if lambda <= 0 {
		return 0
	}
	p := 1.0 / (1.0 + lambda) // Geometric parameter
	return -math.Log(math.Pow(1.0-p, tf) * p)
}

// Name returns the name of this distribution.
func (d *DistributionGeometric) Name() string {
	return "Geometric"
}

// DistributionBernoulli implements the Bernoulli distribution model.
type DistributionBernoulli struct{}

// NewDistributionBernoulli creates a new Bernoulli distribution.
func NewDistributionBernoulli() *DistributionBernoulli {
	return &DistributionBernoulli{}
}

// Score computes the Bernoulli information score.
func (d *DistributionBernoulli) Score(tf float64, lambda float64) float64 {
	if lambda <= 0 || tf <= 0 {
		return 0
	}
	// Simplified Bernoulli score
	return tf * math.Log(1.0+lambda)
}

// Name returns the name of this distribution.
func (d *DistributionBernoulli) Name() string {
	return "Bernoulli"
}

// ============================================================================
// Normalization Implementations for IB
// ============================================================================

// IBNormalizationH2 implements H2 normalization for IB.
type IBNormalizationH2 struct {
	s float64 // Normalization parameter
}

// NewIBNormalizationH2 creates a new H2 normalization for IB.
func NewIBNormalizationH2() *IBNormalizationH2 {
	return &IBNormalizationH2{s: 1.0}
}

// NewIBNormalizationH2WithParam creates a new H2 normalization with custom parameter.
func NewIBNormalizationH2WithParam(s float64) *IBNormalizationH2 {
	return &IBNormalizationH2{s: s}
}

// Normalize computes the H2 normalized term frequency.
func (n *IBNormalizationH2) Normalize(stats *IBStats, tf float64, docLen float64) float64 {
	avgDocLen := stats.AvgDocLength
	if avgDocLen == 0 {
		avgDocLen = 1.0
	}
	if docLen == 0 {
		docLen = avgDocLen
	}
	return tf * math.Log(1.0+n.s*avgDocLen/docLen)
}

// Name returns the name of this normalization.
func (n *IBNormalizationH2) Name() string {
	return "H2"
}

// IBNormalizationH1 implements H1 normalization for IB.
type IBNormalizationH1 struct {
	s float64 // Normalization parameter
}

// NewIBNormalizationH1 creates a new H1 normalization for IB.
func NewIBNormalizationH1() *IBNormalizationH1 {
	return &IBNormalizationH1{s: 1.0}
}

// NewIBNormalizationH1WithParam creates a new H1 normalization with custom parameter.
func NewIBNormalizationH1WithParam(s float64) *IBNormalizationH1 {
	return &IBNormalizationH1{s: s}
}

// Normalize computes the H1 normalized term frequency.
func (n *IBNormalizationH1) Normalize(stats *IBStats, tf float64, docLen float64) float64 {
	avgDocLen := stats.AvgDocLength
	if avgDocLen == 0 {
		avgDocLen = 1.0
	}
	return tf * (1.0 + n.s*avgDocLen) / (1.0 + n.s*docLen)
}

// Name returns the name of this normalization.
func (n *IBNormalizationH1) Name() string {
	return "H1"
}

// IBNormalizationNoOp implements no-op normalization for IB.
type IBNormalizationNoOp struct{}

// NewIBNormalizationNoOp creates a new no-op normalization for IB.
func NewIBNormalizationNoOp() *IBNormalizationNoOp {
	return &IBNormalizationNoOp{}
}

// Normalize returns the term frequency unchanged.
func (n *IBNormalizationNoOp) Normalize(stats *IBStats, tf float64, docLen float64) float64 {
	return tf
}

// Name returns the name of this normalization.
func (n *IBNormalizationNoOp) Name() string {
	return "NoNormalization"
}
