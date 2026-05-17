// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneAxiomatic_VariantStrings verifies the variant codes.
func TestLuceneAxiomatic_VariantStrings(t *testing.T) {
	cases := []struct {
		sim  *LuceneAxiomaticSimilarity
		want string
	}{
		{NewLuceneAxiomaticF1EXPDefault(), "F1EXP"},
		{NewLuceneAxiomaticF1LOGDefault(), "F1LOG"},
		{NewLuceneAxiomaticF2EXPDefault(), "F2EXP"},
		{NewLuceneAxiomaticF2LOGDefault(), "F2LOG"},
		{NewLuceneAxiomaticF3EXPDefault(0.25, 1), "F3EXP"},
		{NewLuceneAxiomaticF3LOG(0.25, 1), "F3LOG"},
	}
	for _, c := range cases {
		if got := c.sim.String(); got != c.want {
			t.Fatalf("String: got %q, want %q", got, c.want)
		}
	}
}

// TestLuceneAxiomatic_IllegalSPanic defends s validation.
func TestLuceneAxiomatic_IllegalSPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for s > 1")
		}
	}()
	NewLuceneAxiomaticF1EXP(1.5, 0.35)
}

// TestLuceneAxiomatic_IllegalKPanic defends k validation.
func TestLuceneAxiomatic_IllegalKPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for k > 1")
		}
	}()
	NewLuceneAxiomaticF1EXP(0.25, 1.5)
}

// TestLuceneAxiomatic_IllegalQueryLenPanic defends queryLen validation.
func TestLuceneAxiomatic_IllegalQueryLenPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for queryLen < 0")
		}
	}()
	NewLuceneAxiomaticF3LOG(0.25, -1)
}

// TestLuceneAxiomatic_F1EXP_Score cross-checks the formula at a sample point.
func TestLuceneAxiomatic_F1EXP_Score(t *testing.T) {
	sim := NewLuceneAxiomaticF1EXPDefault()
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	got := scorer.Score104(5, 64)
	// Recompute by hand.
	stats := NewLuceneBasicStats("body", 1.0)
	sim.FillBasicStats(stats, cs, ts)
	docLen := basicSimScorerLength(64)
	tf := axiomaticTFGrowth(stats, 5, docLen)
	ln := axiomaticLNWithGrowth(0.25)(stats, 5, docLen)
	idf := axiomaticIDFPow(0.35)(stats, 5, docLen)
	want := float32(tf * ln * 1.0 * idf)
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Fatalf("F1EXP: got %v, want %v", got, want)
	}
}

// TestLuceneAxiomatic_F3LOG_FlooredAtZero verifies the F3 gamma negative
// component is floored at zero.
func TestLuceneAxiomatic_F3LOG_FlooredAtZero(t *testing.T) {
	sim := NewLuceneAxiomaticF3LOG(1.0, 100) // exaggerate gamma
	cs := NewCollectionStatistics("body", 10, 10, 10, 1)
	ts := NewTermStatistics(index.NewTerm("body", "x"), 1, 1)
	scorer := sim.Scorer104(1.0, cs, ts)
	// Long doc + large s + large queryLen => gamma dominates negative => floored
	got := scorer.Score104(1, 255)
	if got < 0 {
		t.Fatalf("Score must be floored at zero, got %v", got)
	}
}

// TestLuceneAxiomatic_F2EXP_Score cross-checks F2EXP at a sample point.
func TestLuceneAxiomatic_F2EXP_Score(t *testing.T) {
	sim := NewLuceneAxiomaticF2EXPDefault()
	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(1.0, cs, ts)
	got := scorer.Score104(5, 64)
	if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
		t.Fatalf("F2EXP non-finite score: %v", got)
	}
	if got < 0 {
		t.Fatalf("F2EXP unexpectedly negative: %v", got)
	}
}

// TestLuceneAxiomatic_Accessors verifies S/K/QueryLen surface.
func TestLuceneAxiomatic_Accessors(t *testing.T) {
	sim := NewLuceneAxiomaticF3EXP(0.4, 7, 0.5)
	if sim.S() != 0.4 {
		t.Fatalf("S: got %v, want 0.4", sim.S())
	}
	if sim.K() != 0.5 {
		t.Fatalf("K: got %v, want 0.5", sim.K())
	}
	if sim.QueryLen() != 7 {
		t.Fatalf("QueryLen: got %d, want 7", sim.QueryLen())
	}
}
