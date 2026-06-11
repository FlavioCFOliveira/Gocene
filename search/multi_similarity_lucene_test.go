// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneMultiSimilarity_CombSum verifies that the score is the sum of
// the per-sub-similarity scores.
func TestLuceneMultiSimilarity_CombSum(t *testing.T) {
	a := NewRawTFSimilarity()
	b := NewLuceneBooleanSimilarity()
	m := NewLuceneMultiSimilarity([]LuceneSimilarity{a, b})

	cs := NewCollectionStatistics("body", 100, 80, 800, 200)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 10, 25)
	scorer := m.Scorer104(1.0, cs, ts)
	got := scorer.Score104(3.0, 64)
	// RawTF returns boost*freq = 3, Boolean returns boost = 1. Sum = 4.
	want := float32(4.0)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("Score: got %v, want %v", got, want)
	}
}

// TestLuceneMultiSimilarity_ComputeNormDelegatesToZeroth verifies that the
// norm comes from sims[0], not from a vote.
func TestLuceneMultiSimilarity_ComputeNormDelegatesToZeroth(t *testing.T) {
	a := NewLuceneBM25SimilarityFull(1.2, 0.75, true)
	b := NewLuceneBM25SimilarityFull(1.2, 0.75, false) // different flag
	m := NewLuceneMultiSimilarity([]LuceneSimilarity{a, b})
	state := index.NewFieldInvertStateFull(10, "f", index.IndexOptionsDocsAndFreqs,
		0, 10, 4, 0, 0, 0)
	got := m.ComputeNormFromInvertState(state)
	want := a.ComputeNormFromInvertState(state)
	if got != want {
		t.Fatalf("norm should come from sims[0]: got %d, want %d", got, want)
	}

// TestLuceneMultiSimilarity_EmptyPanics defends the contract.
func TestLuceneMultiSimilarity_EmptyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty sims slice")
		}
	}()
	NewLuceneMultiSimilarity(nil)
}

// TestLuceneMultiSimilarity_Explain verifies that the explanation tree
// contains one detail per sub-scorer.
func TestLuceneMultiSimilarity_Explain(t *testing.T) {
	m := NewLuceneMultiSimilarity([]LuceneSimilarity{
		NewRawTFSimilarity(),
		NewLuceneBooleanSimilarity(),
		NewRawTFSimilarity(),
	})
	cs := NewCollectionStatistics("body", 100, 80, 800, 200)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 10, 25)
	scorer := m.Scorer104(1.0, cs, ts)
	exp := scorer.Explain104(NewExplanation(true, 3, "freq"), 1)
	if got := len(exp.GetDetails()); got != 3 {
		t.Fatalf("expected 3 sub-explanations, got %d", got)
	}
}