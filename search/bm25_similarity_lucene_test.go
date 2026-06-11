// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneBM25Similarity_Defaults verifies the canonical defaults.
func TestLuceneBM25Similarity_Defaults(t *testing.T) {
	s := NewLuceneBM25Similarity()
	if s.K1() != 1.2 {
		t.Fatalf("k1: got %v, want 1.2", s.K1())
	}
	if s.B() != 0.75 {
		t.Fatalf("b: got %v, want 0.75", s.B())
	}
	if !s.GetDiscountOverlaps() {
		t.Fatal("discountOverlaps must default to true")
	}
	if want := "BM25(k1=1.2,b=0.75)"; s.String() != want {
		t.Fatalf("String: got %q, want %q", s.String(), want)
	}
}

// TestLuceneBM25Similarity_IDF cross-checks the idf formula against the
// canonical reference value.
func TestLuceneBM25Similarity_IDF(t *testing.T) {
	s := NewLuceneBM25Similarity()
	got := s.Idf(10, 100) // n=10, N=100
	want := float32(math.Log(1.0 + (100.0-10.0+0.5)/(10.0+0.5)))
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("Idf: got %v, want %v", got, want)
	}
}

// TestLuceneBM25Similarity_Score_NoLengthNorm verifies the b=0 formula:
// score = boost * idf * freq / (freq + k1).
func TestLuceneBM25Similarity_Score_NoLengthNorm(t *testing.T) {
	s := NewLuceneBM25SimilarityFull(1.2, 0.0, true)
	collStats := NewCollectionStatistics("body", 100, 100, 1000, 100)
	termStats := NewTermStatistics(index.NewTerm("body", "go"), 10, 100)
	scorer := s.Scorer104(1.0, collStats, termStats)

	// At any norm, score = idf * freq / (freq + k1) because b=0.
	idf := s.Idf(10, 100)
	got := scorer.Score104(3.0, 1)
	want := idf * (1.0 - 1.0/(1.0+3.0*1.0/1.2)) // weight - weight/(1 + freq * normInverse)
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Fatalf("Score: got %v, want %v", got, want)
	}
}

// TestLuceneBM25Similarity_Monotonic_Freq verifies that score is non-
// decreasing in freq for any fixed norm.
func TestLuceneBM25Similarity_Monotonic_Freq(t *testing.T) {
	s := NewLuceneBM25Similarity()
	cs := NewCollectionStatistics("body", 1000, 800, 50000, 10000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 50, 200)
	scorer := s.Scorer104(1.0, cs, ts)
	prev := float32(-math.MaxFloat32)
	for f := float32(1); f <= 100; f += 1 {
		got := scorer.Score104(f, 64)
		if got < prev {
			t.Fatalf("monotonicity violated at freq=%v: %v < %v", f, got, prev)
		}
		prev = got
	}
}

// TestLuceneBM25Similarity_Monotonic_Norm verifies that score does not
// increase as the unsigned norm grows (Lucene's contract).
func TestLuceneBM25Similarity_Monotonic_Norm(t *testing.T) {
	s := NewLuceneBM25Similarity()
	cs := NewCollectionStatistics("body", 1000, 800, 50000, 10000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 50, 200)
	scorer := s.Scorer104(1.0, cs, ts)
	prev := float32(math.MaxFloat32)
	for n := int64(1); n < 256; n++ {
		got := scorer.Score104(5.0, n)
		if got > prev+1e-6 {
			t.Fatalf("norm monotonicity violated at norm=%d: %v > %v", n, got, prev)
		}
		prev = got
	}
}

// TestLuceneBM25Similarity_BulkScorer_Equivalent confirms the bulk path
// produces the same scores as the per-doc path.
func TestLuceneBM25Similarity_BulkScorer_Equivalent(t *testing.T) {
	s := NewLuceneBM25Similarity()
	cs := NewCollectionStatistics("body", 1000, 800, 50000, 10000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 50, 200)
	scorer := s.Scorer104(2.5, cs, ts)
	bulk := scorer.AsBulkSimScorer()
	freqs := []float32{1, 2, 3, 5, 8, 13, 21, 34}
	norms := []int64{1, 5, 25, 64, 128, 200, 240, 255}
	got := make([]float32, len(freqs))
	bulk.ScoreBulk(len(freqs), freqs, norms, got)
	for i := range freqs {
		want := scorer.Score104(freqs[i], norms[i])
		if math.Abs(float64(got[i]-want)) > 1e-6 {
			t.Fatalf("bulk[%d]: got %v, want %v", i, got[i], want)
		}
	}

// TestLuceneBM25Similarity_IllegalK1 verifies that the constructor panics
// on a NaN k1.
}
func TestLuceneBM25Similarity_IllegalK1(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for NaN k1")
		}
	}()
	NewLuceneBM25SimilarityWithParams(float32(math.NaN()), 0.5)
}

// TestLuceneBM25Similarity_IllegalB verifies that the constructor panics
// when b is outside [0, 1].
func TestLuceneBM25Similarity_IllegalB(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for b out of range")
		}
	}()
	NewLuceneBM25SimilarityWithParams(1.2, 2.0)
}

// TestLuceneBM25Similarity_Explain verifies the explain tree is well-formed.
func TestLuceneBM25Similarity_Explain(t *testing.T) {
	s := NewLuceneBM25Similarity()
	cs := NewCollectionStatistics("body", 100, 100, 1000, 100)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 10, 100)
	scorer := s.Scorer104(2.0, cs, ts)
	freq := NewExplanation(true, 3, "freq=3")
	exp := scorer.Explain104(freq, 64)
	if !exp.IsMatch() {
		t.Fatal("explain should be a match")
	}
	if len(exp.GetDetails()) < 2 {
		t.Fatalf("explain should have boost + idf + tf sub-explanations, got %d", len(exp.GetDetails()))
	}
}