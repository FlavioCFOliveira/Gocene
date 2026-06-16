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

func (s *MockSimScorer) Score(doc int, freq float32, norm int64) float32 {
	return s.score
}

func TestSimilarity_Basics(t *testing.T) {
	sim := NewMockSimilarity(10.0)
	scorer := sim.Scorer(nil)
	if scorer.Score(0, 1.0, 1) != 10.0 {
		t.Errorf("Expected score 10.0, got %f", scorer.Score(0, 1.0, 1))
	}

}
func TestSimilarity_BM25(t *testing.T) {
	sim := NewBM25Similarity()
	collStats := NewCollectionStatistics("field", 100, 50, 1000, 500)
	termStats := NewTermStatistics(index.NewTerm("field", "value"), 10, 5)
	scorer := sim.Scorer(collStats, termStats)
	if scorer == nil {
		t.Fatalf("Expected non-nil BM25 scorer")
	}
	// score for freq=1, average norm=1 should be positive
	if scorer.Score(0, 1.0, 1) <= 0 {
		t.Errorf("Expected positive BM25 score, got %f", scorer.Score(0, 1.0, 1))
	}
}