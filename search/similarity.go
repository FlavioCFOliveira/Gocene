// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Similarity is the base class for scoring implementations.
type Similarity interface {
	// ComputeNorm computes the norm value for a field.
	ComputeNorm(field string, stats interface{}) float32

	// ComputeWeight computes the weight for a query.
	ComputeWeight(queryWeight float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight

	// Scorer creates a SimScorer for scoring documents given collection and term stats.
	Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer

	// Coord returns the coordination factor for matching terms.
	Coord(overlap, maxOverlap int) float32
}

// BaseSimilarity provides common functionality.
type BaseSimilarity struct{}

// NewBaseSimilarity creates a new BaseSimilarity.
func NewBaseSimilarity() *BaseSimilarity {
	return &BaseSimilarity{}
}

// ComputeNorm computes the norm value.
func (s *BaseSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	return 1.0
}

// ComputeWeight computes the weight.
func (s *BaseSimilarity) ComputeWeight(boost float32, collectionStats *CollectionStatistics, termStats *TermStatistics) SimWeight {
	return nil
}

// Scorer creates a SimScorer.
func (s *BaseSimilarity) Scorer(collectionStats *CollectionStatistics, termStats *TermStatistics) SimScorer {
	return NewBaseSimScorer()
}

// Coord returns the coordination factor.
func (s *BaseSimilarity) Coord(overlap, maxOverlap int) float32 {
	return float32(overlap) / float32(maxOverlap)
}
