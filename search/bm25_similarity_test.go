// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

func TestBM25Similarity_Basics(t *testing.T) {
	sim := NewBM25Similarity()
	if sim.K1() != 1.2 {
		t.Errorf("Expected default K1 to be 1.2, got %f", sim.K1())
	}
	if sim.B() != 0.75 {
		t.Errorf("Expected default B to be 0.75, got %f", sim.B())
	}

	sim2 := NewBM25SimilarityWithParams(1.5, 0.5)
	if sim2.K1() != 1.5 {
		t.Errorf("Expected K1 to be 1.5, got %f", sim2.K1())
	}
	if sim2.B() != 0.5 {
		t.Errorf("Expected B to be 0.5, got %f", sim2.B())
	}
}

func TestBM25Similarity_ScoreBM25(t *testing.T) {
	sim := NewBM25Similarity()

	// Test case: freq=2, docLength=10, avgDocLength=10, idf=2.0
	// k1=1.2, b=0.75
	// norm = (1 - 0.75) + 0.75 * (10 / 10) = 0.25 + 0.75 = 1.0
	// tfComponent = 2 / (2 + 1.2 * 1.0) = 2 / 3.2 = 0.625
	// score = 2.0 * 0.625 = 1.25
	score := sim.ScoreBM25(2.0, 10.0, 10.0, 2.0)
	expected := 1.25
	if math.Abs(score-expected) > 1e-9 {
		t.Errorf("Expected score %f, got %f", expected, score)
	}

	// Test case with different b
	sim = NewBM25SimilarityWithParams(1.2, 0.0) // No length normalization
	// norm = (1 - 0) + 0 * (20 / 10) = 1.0
	// tfComponent = 2 / (2 + 1.2 * 1.0) = 0.625
	score = sim.ScoreBM25(2.0, 20.0, 10.0, 2.0)
	if math.Abs(score-expected) > 1e-9 {
		t.Errorf("Expected score %f with b=0, got %f", expected, score)
	}
}

func TestBM25Similarity_IDF(t *testing.T) {
	sim := NewBM25Similarity()

	// idf = log(1 + (N - n + 0.5) / (n + 0.5))
	// N=100, n=10
	// idf = log(1 + (100 - 10 + 0.5) / (10 + 0.5)) = log(1 + 90.5 / 10.5) = log(1 + 8.6190476) = log(9.6190476) ≈ 2.26376
	idf := sim.InverseDocumentFrequency(100, 10)
	expected := math.Log(1 + (100-10+0.5)/(10+0.5))
	if math.Abs(idf-expected) > 1e-9 {
		t.Errorf("Expected IDF %f, got %f", expected, idf)
	}
}

func TestBM25Similarity_IllegalParams(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewBM25SimilarityWithParams should have panicked with illegal k1")
		}
	}()
	NewBM25SimilarityWithParams(-1, 0.75)
}

func TestBM25Similarity_NaNParameters(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for NaN k1")
		}
	}()
	NewBM25SimilarityWithParams(math.NaN(), 0.75)
}
