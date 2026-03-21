// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// DFRSimilarity implements the Divergence From Randomness (DFR) framework.
// DFR is a probabilistic retrieval framework based on measuring the divergence
// of the actual term distribution from a random distribution.
//
// The basic DFR formula is: score = I(n) * I(F) * tfNormalization
// where:
// - I(n) is the information content based on document frequency
// - I(F) is the information content based on term frequency
// - tfNormalization handles term frequency saturation
type DFRSimilarity struct {
	*BaseSimilarity
	basicModel BasicModel // The basic model (e.g., Poisson, Geometric)
	afterEffect AfterEffect // The after effect (e.g., Laplace, Dirichlet)
	normalization Normalization // Term frequency normalization
}

// BasicModel represents the basic model for DFR.
type BasicModel interface {
	// Score computes the basic score given term and document statistics
	Score(stats *BasicStats, tf float64) float64
	// Name returns the name of this basic model
	Name() string
}

// AfterEffect represents the after effect for DFR.
type AfterEffect interface {
	// Score computes the after effect score
	Score(stats *BasicStats, tfn float64) float64
	// Name returns the name of this after effect
	Name() string
}

// Normalization represents term frequency normalization.
type Normalization interface {
	// Tfn computes the normalized term frequency
	Tfn(stats *BasicStats, freq float64, docLen float64) float64
	// Name returns the name of this normalization
	Name() string
}

// BasicStats holds statistics needed for DFR scoring.
type BasicStats struct {
	TotalTermFreq int64   // Total term frequency in collection
	DocFreq       int     // Document frequency
	DocCount      int     // Number of documents
	AvgDocLength  float64 // Average document length
}

// NewDFRSimilarity creates a new DFRSimilarity with default components.
// Default: Poisson basic model, Laplace after effect, H2 normalization
func NewDFRSimilarity() *DFRSimilarity {
	return &DFRSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		basicModel:     NewBasicModelPoisson(),
		afterEffect:    NewAfterEffectLaplace(),
		normalization:  NewNormalizationH2(),
	}
}

// NewDFRSimilarityWithParams creates a DFRSimilarity with custom components.
func NewDFRSimilarityWithParams(basicModel BasicModel, afterEffect AfterEffect, normalization Normalization) *DFRSimilarity {
	return &DFRSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		basicModel:     basicModel,
		afterEffect:    afterEffect,
		normalization:  normalization,
	}
}

// ComputeNorm computes the norm value for a field.
func (s *DFRSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// Coord returns the coordination factor.
func (s *DFRSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}

// QueryNorm returns the query normalization value.
func (s *DFRSimilarity) QueryNorm(sumOfSquaredWeights float32) float32 {
	return 1.0 / float32(math.Sqrt(float64(sumOfSquaredWeights)))
}

// ComputeWeight computes the weight for a term.
func (s *DFRSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return NewDFRSimWeight(s, collectionStats, termStats, boost)
}

// Scorer creates a scorer for this similarity.
func (s *DFRSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewDFRSimScorer(s, collectionStats, termStats)
}

// DFRSimWeight holds the weight for DFR scoring.
type DFRSimWeight struct {
	sim             *DFRSimilarity
	collectionStats *CollectionStatistics
	termStats       *TermStatistics
	boost           float32
	basicStats      *BasicStats
}

// NewDFRSimWeight creates a new DFRSimWeight.
func NewDFRSimWeight(sim *DFRSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics, boost float32) *DFRSimWeight {
	stats := &BasicStats{}
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
		stats.TotalTermFreq = termStats.TotalTermFreq()
	}
	return &DFRSimWeight{
		sim:             sim,
		collectionStats: collectionStats,
		termStats:       termStats,
		boost:           boost,
		basicStats:      stats,
	}
}

// GetValue returns the value for this weight.
func (w *DFRSimWeight) GetValue() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *DFRSimWeight) Normalize(norm float32) {
	w.boost *= norm
}

// Scorer creates a scorer for this weight.
func (w *DFRSimWeight) Scorer() SimScorer {
	return NewDFRSimScorerWithWeight(w)
}

// DFRSimScorer is a scorer for DFRSimilarity.
type DFRSimScorer struct {
	*BaseSimScorer
	similarity *DFRSimilarity
	weight     *DFRSimWeight
	basicStats *BasicStats
}

// NewDFRSimScorer creates a new DFRSimScorer.
func NewDFRSimScorer(similarity *DFRSimilarity, collectionStats *CollectionStatistics, termStats *TermStatistics) *DFRSimScorer {
	stats := &BasicStats{}
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
	return &DFRSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    similarity,
		basicStats:    stats,
	}
}

// NewDFRSimScorerWithWeight creates a new DFRSimScorer with weight.
func NewDFRSimScorerWithWeight(weight *DFRSimWeight) *DFRSimScorer {
	return &DFRSimScorer{
		BaseSimScorer: NewBaseSimScorer(),
		similarity:    weight.sim,
		weight:        weight,
		basicStats:    weight.basicStats,
	}
}

// Score calculates the DFR score.
// Score = boost * basicModelScore * afterEffect * normalization
func (s *DFRSimScorer) Score(doc int, freq float32) float32 {
	if freq == 0 || s.basicStats == nil {
		return 0
	}

	// Document length (simplified - assume average)
	docLen := s.basicStats.AvgDocLength
	if docLen == 0 {
		docLen = 1.0
	}

	// Normalize term frequency
	tfn := s.similarity.normalization.Tfn(s.basicStats, float64(freq), docLen)

	// Apply after effect
	afterEffectScore := s.similarity.afterEffect.Score(s.basicStats, tfn)

	// Apply basic model
	basicScore := s.similarity.basicModel.Score(s.basicStats, afterEffectScore)

	// Final score
	score := basicScore
	if s.weight != nil {
		score *= float64(s.weight.boost)
	}

	return float32(score)
}

// Ensure DFRSimilarity implements Similarity
var _ Similarity = (*DFRSimilarity)(nil)

// Ensure DFRSimScorer implements SimScorer
var _ SimScorer = (*DFRSimScorer)(nil)

// ============================================================================
// Basic Model Implementations
// ============================================================================

// BasicModelPoisson implements the Poisson basic model.
type BasicModelPoisson struct{}

// NewBasicModelPoisson creates a new Poisson basic model.
func NewBasicModelPoisson() *BasicModelPoisson {
	return &BasicModelPoisson{}
}

// Score computes the Poisson score: -log(F / (N + F))
// where F is total term frequency and N is collection size
func (m *BasicModelPoisson) Score(stats *BasicStats, tf float64) float64 {
	F := float64(stats.TotalTermFreq)
	N := float64(stats.DocCount)
	if F == 0 || N == 0 {
		return 0
	}
	// Poisson approximation: -log(F / (N + F))
	return -math.Log(F / (N + F))
}

// Name returns the name of this basic model.
func (m *BasicModelPoisson) Name() string {
	return "Poisson"
}

// BasicModelGeometric implements the Geometric basic model.
type BasicModelGeometric struct{}

// NewBasicModelGeometric creates a new Geometric basic model.
func NewBasicModelGeometric() *BasicModelGeometric {
	return &BasicModelGeometric{}
}

// Score computes the Geometric score.
func (m *BasicModelGeometric) Score(stats *BasicStats, tf float64) float64 {
	F := float64(stats.TotalTermFreq)
	N := float64(stats.DocCount)
	if F == 0 || N == 0 {
		return 0
	}
	// Geometric approximation
	return -math.Log(F / (N + F + 1.0))
}

// Name returns the name of this basic model.
func (m *BasicModelGeometric) Name() string {
	return "Geometric"
}

// ============================================================================
// After Effect Implementations
// ============================================================================

// AfterEffectLaplace implements Laplace after effect.
type AfterEffectLaplace struct{}

// NewAfterEffectLaplace creates a new Laplace after effect.
func NewAfterEffectLaplace() *AfterEffectLaplace {
	return &AfterEffectLaplace{}
}

// Score computes the Laplace after effect: (tfn + 1) / (docFreq + 1)
func (e *AfterEffectLaplace) Score(stats *BasicStats, tfn float64) float64 {
	n := float64(stats.DocFreq)
	if n == 0 {
		n = 1.0
	}
	return (tfn + 1.0) / (n + 1.0)
}

// Name returns the name of this after effect.
func (e *AfterEffectLaplace) Name() string {
	return "Laplace"
}

// AfterEffectDirichlet implements Dirichlet after effect.
type AfterEffectDirichlet struct {
	mu float64 // Smoothing parameter
}

// NewAfterEffectDirichlet creates a new Dirichlet after effect.
func NewAfterEffectDirichlet(mu float64) *AfterEffectDirichlet {
	if mu <= 0 {
		mu = 2000.0 // Default value
	}
	return &AfterEffectDirichlet{mu: mu}
}

// Score computes the Dirichlet after effect.
func (e *AfterEffectDirichlet) Score(stats *BasicStats, tfn float64) float64 {
	return (tfn + e.mu*float64(stats.TotalTermFreq)/float64(stats.DocCount)) / (tfn + e.mu)
}

// Name returns the name of this after effect.
func (e *AfterEffectDirichlet) Name() string {
	return "Dirichlet"
}

// ============================================================================
// Normalization Implementations
// ============================================================================

// NormalizationH2 implements H2 normalization (pivoted normalization).
type NormalizationH2 struct {
	s float64 // Normalization parameter
}

// NewNormalizationH2 creates a new H2 normalization.
func NewNormalizationH2() *NormalizationH2 {
	return &NormalizationH2{s: 1.0}
}

// NewNormalizationH2WithParam creates a new H2 normalization with custom parameter.
func NewNormalizationH2WithParam(s float64) *NormalizationH2 {
	return &NormalizationH2{s: s}
}

// Tfn computes the H2 normalized term frequency.
// Formula: tfn = tf * log(1.0 + s * avgDocLength / docLength)
func (n *NormalizationH2) Tfn(stats *BasicStats, freq float64, docLen float64) float64 {
	avgDocLen := stats.AvgDocLength
	if avgDocLen == 0 {
		avgDocLen = 1.0
	}
	if docLen == 0 {
		docLen = avgDocLen
	}
	return freq * math.Log(1.0+n.s*avgDocLen/docLen)
}

// Name returns the name of this normalization.
func (n *NormalizationH2) Name() string {
	return "H2"
}

// NormalizationH1 implements H1 normalization.
type NormalizationH1 struct {
	s float64 // Normalization parameter
}

// NewNormalizationH1 creates a new H1 normalization.
func NewNormalizationH1() *NormalizationH1 {
	return &NormalizationH1{s: 1.0}
}

// NewNormalizationH1WithParam creates a new H1 normalization with custom parameter.
func NewNormalizationH1WithParam(s float64) *NormalizationH1 {
	return &NormalizationH1{s: s}
}

// Tfn computes the H1 normalized term frequency.
// Formula: tfn = tf * (1.0 + s * avgDocLength) / (1.0 + s * docLength)
func (n *NormalizationH1) Tfn(stats *BasicStats, freq float64, docLen float64) float64 {
	avgDocLen := stats.AvgDocLength
	if avgDocLen == 0 {
		avgDocLen = 1.0
	}
	return freq * (1.0 + n.s*avgDocLen) / (1.0 + n.s*docLen)
}

// Name returns the name of this normalization.
func (n *NormalizationH1) Name() string {
	return "H1"
}

// NormalizationNoOp implements no-op normalization (returns tf unchanged).
type NormalizationNoOp struct{}

// NewNormalizationNoOp creates a new no-op normalization.
func NewNormalizationNoOp() *NormalizationNoOp {
	return &NormalizationNoOp{}
}

// Tfn returns the term frequency unchanged.
func (n *NormalizationNoOp) Tfn(stats *BasicStats, freq float64, docLen float64) float64 {
	return freq
}

// Name returns the name of this normalization.
func (n *NormalizationNoOp) Name() string {
	return "NoNormalization"
}
