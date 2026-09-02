// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

// TestBM25Similarity_Basics_Deprecated tests the deprecated BM25Similarity alias.
// New code should use LuceneBM25Similarity directly.
func TestBM25Similarity_Basics_Deprecated(t *testing.T) {
	sim := NewLuceneBM25Similarity()
	if sim.K1() != 1.2 {
		t.Errorf("Expected default K1 to be 1.2, got %f", sim.K1())
	}
	if sim.B() != 0.75 {
		t.Errorf("Expected default B to be 0.75, got %f", sim.B())
	}

	sim2 := NewLuceneBM25SimilarityWithParams(1.5, 0.5)
	if sim2.K1() != 1.5 {
		t.Errorf("Expected K1 to be 1.5, got %f", sim2.K1())
	}
	if sim2.B() != 0.5 {
		t.Errorf("Expected B to be 0.5, got %f", sim2.B())
	}
}

// TestBM25Similarity_IDF_Deprecated tests IDF calculation.
func TestBM25Similarity_IDF_Deprecated(t *testing.T) {
	sim := NewLuceneBM25Similarity()

	// idf = log(1 + (N - n + 0.5) / (n + 0.5))
	// N=100, n=10
	// idf = log(1 + (100 - 10 + 0.5) / (10 + 0.5)) = log(1 + 90.5 / 10.5) = log(1 + 8.6190476) = log(9.6190476) ≈ 2.26376
	idf := sim.Idf(10, 100)
	expected := math.Log(1 + (100-10+0.5)/(10+0.5))
	if math.Abs(float64(idf-float32(expected))) > 1e-6 {
		t.Errorf("Expected IDF %f, got %f", expected, idf)
	}
}

func TestBM25Similarity_IllegalParams_Deprecated(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewLuceneBM25SimilarityWithParams should have panicked with illegal k1")
		}
	}()
	NewLuceneBM25SimilarityWithParams(-1, 0.75)
}

func TestBM25Similarity_NaNParameters_Deprecated(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for NaN k1")
		}
	}()
	NewLuceneBM25SimilarityWithParams(float32(math.NaN()), 0.75)
}