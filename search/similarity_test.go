// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MockSimilarity is a simple similarity for testing
type MockSimilarity struct {
	*BaseSimilarity
	score float32
}

func NewMockSimilarity(score float32) *MockSimilarity {
	return &MockSimilarity{
		BaseSimilarity: NewBaseSimilarity(),
		score:          score,
	}
}

func (s *MockSimilarity) Scorer(stats interface{}) SimScorer {
	return &MockSimScorer{score: s.score}
}

type MockSimScorer struct {
	*BaseSimScorer
	score float32
}

func (s *MockSimScorer) Score(doc int, freq float32) float32 {
	return s.score
}

// PerFieldSimilarityWrapper mock
type PerFieldSimilarityWrapper struct {
	*BaseSimilarity
	fooSim Similarity
	barSim Similarity
}

func (s *PerFieldSimilarityWrapper) Scorer(stats interface{}) SimScorer {
	// In a real implementation, we would look at the field in stats
	// For this test, we'll just return a mock that depends on some state
	return nil
}

func TestSimilarity_Basics(t *testing.T) {
	sim := NewMockSimilarity(10.0)
	scorer := sim.Scorer(nil)
	if scorer.Score(0, 1.0) != 10.0 {
		t.Errorf("Expected score 10.0, got %f", scorer.Score(0, 1.0))
	}
}

func TestSimilarity_BM25(t *testing.T) {
	sim := NewBM25Similarity()
	// Currently Scorer returns nil in BaseSimilarity, so this is just a placeholder
	// until we implement full BM25 weighting and scoring
	collStats := NewCollectionStatistics("field", 100, 50, 1000, 500)
	termStats := NewTermStatistics(index.NewTerm("field", "value"), 10, 5)
	if sim.Scorer(collStats, termStats) != nil {
		t.Errorf("Expected nil scorer for now")
	}
}
