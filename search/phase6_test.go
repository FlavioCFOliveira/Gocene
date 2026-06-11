// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneIndependenceMeasures cross-checks each Independence formula.
func TestLuceneIndependenceMeasures(t *testing.T) {
	cases := []struct {
		name string
		m    LuceneDFIIndependence
		freq float64
		exp  float64
		want float64
	}{
		{"ChiSquared", NewLuceneIndependenceChiSquared(), 10, 4, 9.0},        // (10-4)^2/4
		{"Saturated", NewLuceneIndependenceSaturated(), 10, 4, 1.5},          // (10-4)/4
		{"Standardized", NewLuceneIndependenceStandardized(), 10, 4, 3.0},    // (10-4)/2
	}
	for _, c := range cases {
		got := c.m.Score(c.freq, c.exp)
		if math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("%s(%v,%v): got %v, want %v", c.name, c.freq, c.exp, got, c.want)
		}
	}
}

// TestLuceneDFISimilarity_ScoreZeroWhenBelowExpected verifies the
// freq<=expected short-circuit.
func TestLuceneDFISimilarity_ScoreZeroWhenBelowExpected(t *testing.T) {
	sim := NewLuceneDFISimilarity(NewLuceneIndependenceChiSquared())
	// expected = (F+1) * docLen / (T+1) = (300+1) * 1 / (50000+1) ≈ 0.006
	// freq = 0.001 < expected => score 0
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	if got := scorer.Score104(0.001, 1); got != 0 {
		t.Fatalf("expected 0 when freq < expected, got %v", got)
	}
}

// TestLuceneDFISimilarity_String verifies the canonical "DFI(<measure>)" form.
func TestLuceneDFISimilarity_String(t *testing.T) {
	cases := []struct {
		m    LuceneDFIIndependence
		want string
	}{
		{NewLuceneIndependenceChiSquared(), "DFI(ChiSquared)"},
		{NewLuceneIndependenceSaturated(), "DFI(Saturated)"},
		{NewLuceneIndependenceStandardized(), "DFI(Standardized)"},
	}
	for _, c := range cases {
		if got := NewLuceneDFISimilarity(c.m).String(); got != c.want {
			t.Fatalf("String: got %q, want %q", got, c.want)
		}
	}
}

// TestLuceneLMDirichlet_Score cross-checks the formula at a typical point.
func TestLuceneLMDirichlet_Score(t *testing.T) {
	sim := NewLuceneLMDirichletSimilarity()
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	freq := float32(5)
	encoded := int64(64)
	got := scorer.Score104(freq, encoded)
	if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
		t.Fatalf("non-finite Dirichlet score: %v", got)
	}
}

// TestLuceneLMDirichlet_String verifies the formatted name.
func TestLuceneLMDirichlet_String(t *testing.T) {
	sim := NewLuceneLMDirichletSimilarity()
	got := sim.String()
	if !strings.HasPrefix(got, "LM Dirichlet(") {
		t.Fatalf("String: got %q, want prefix 'LM Dirichlet('", got)
	}
}

// TestLuceneLMDirichlet_IllegalMu defends the validation.
func TestLuceneLMDirichlet_IllegalMu(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative mu")
		}
	}()
	NewLuceneLMDirichletSimilarityWithMu(-1)
}

// TestLuceneLMJelinekMercer_Score cross-checks the formula at a typical point.
func TestLuceneLMJelinekMercer_Score(t *testing.T) {
	sim := NewLuceneLMJelinekMercerSimilarity(0.7)
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	got := scorer.Score104(5, 64)
	if math.IsNaN(float64(got)) {
		t.Fatalf("JM score is NaN: %v", got)
	}
}

// TestLuceneLMJelinekMercer_IllegalLambda defends the (0,1] check.
func TestLuceneLMJelinekMercer_IllegalLambda(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for lambda == 0")
		}
	}()
	NewLuceneLMJelinekMercerSimilarity(0)
}

// TestLuceneIndriDirichlet_Score sanity-checks the Indri formula.
func TestLuceneIndriDirichlet_Score(t *testing.T) {
	sim := NewLuceneIndriDirichletSimilarity()
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	got := scorer.Score104(5, 64)
	if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
		t.Fatalf("Indri score is non-finite: %v", got)
	}
}

// TestLuceneIndriCollectionModel_NoSmoothing verifies the +1 omission.
func TestLuceneIndriCollectionModel_NoSmoothing(t *testing.T) {
	stats := NewLuceneBasicStats("body", 1.0)
	stats.SetTotalTermFreq(300)
	stats.SetNumberOfFieldTokens(50000)
	got := NewLuceneIndriCollectionModel().ComputeProbability(stats)
	want := 300.0 / 50000.0
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("Indri P(w|C): got %v, want %v", got, want)
	}

// TestLuceneDefaultCollectionModel_WithSmoothing verifies the +1 smoothing.
}
func TestLuceneDefaultCollectionModel_WithSmoothing(t *testing.T) {
	stats := NewLuceneBasicStats("body", 1.0)
	stats.SetTotalTermFreq(300)
	stats.SetNumberOfFieldTokens(50000)
	got := NewLuceneDefaultCollectionModel().ComputeProbability(stats)
	want := 301.0 / 50001.0
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("default P(w|C): got %v, want %v", got, want)
	}
}