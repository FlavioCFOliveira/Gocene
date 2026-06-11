// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneClassicSimilarity_Idf verifies the canonical formula
// log((docCount+1)/(docFreq+1)) + 1.
func TestLuceneClassicSimilarity_Idf(t *testing.T) {
	s := NewLuceneClassicSimilarity()
	got := s.idf(10, 100)
	want := float32(math.Log(101.0/11.0) + 1.0)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("idf: got %v, want %v", got, want)
	}
}

// TestLuceneClassicSimilarity_Tf verifies tf(freq) = sqrt(freq).
func TestLuceneClassicSimilarity_Tf(t *testing.T) {
	s := NewLuceneClassicSimilarity()
	for _, f := range []float32{1, 4, 9, 16, 25} {
		got := s.tf(f)
		want := float32(math.Sqrt(float64(f)))
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("tf(%v): got %v, want %v", f, got, want)
		}
	}
}

// TestLuceneClassicSimilarity_LengthNorm verifies lengthNorm = 1/sqrt(n).
func TestLuceneClassicSimilarity_LengthNorm(t *testing.T) {
	s := NewLuceneClassicSimilarity()
	for _, n := range []int{1, 4, 16, 64, 256} {
		got := s.lengthNorm(n)
		want := float32(1.0 / math.Sqrt(float64(n)))
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("lengthNorm(%d): got %v, want %v", n, got, want)
		}
	}
}

// TestLuceneClassicSimilarity_Scorer cross-checks the end-to-end score
// against an independent computation of tf * idf * boost * normTable[norm].
func TestLuceneClassicSimilarity_Scorer(t *testing.T) {
	s := NewLuceneClassicSimilarity()
	cs := NewCollectionStatistics("body", 100, 100, 1000, 100)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 10, 100)
	scorer := s.Scorer104(2.0, cs, ts)
	got := scorer.Score104(4.0, 64)
	idf := s.idf(10, 100)
	tf := s.tf(4.0)
	normLen := luceneTFIDFLengthTable[64]
	norm := s.lengthNorm(normLen)
	want := 2.0 * idf * tf * norm
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Fatalf("Score: got %v, want %v (tf=%v idf=%v norm=%v)", got, want, tf, idf, norm)
	}

// TestLuceneClassicSimilarity_String verifies the canonical name.
}
func TestLuceneClassicSimilarity_String(t *testing.T) {
	if got := NewLuceneClassicSimilarity().String(); got != "ClassicSimilarity" {
		t.Fatalf("String: got %q, want ClassicSimilarity", got)
	}
}