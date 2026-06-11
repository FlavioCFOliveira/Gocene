// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLuceneDistributionLL_Score verifies the canonical formula.
func TestLuceneDistributionLL_Score(t *testing.T) {
	d := NewLuceneDistributionLL()
	got := d.Score(makeBasicStats(1.0), 5.0, 0.2)
	want := -math.Log(0.2 / (5.0 + 0.2))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("LL: got %v, want %v", got, want)
	}
}

// TestLuceneDistributionSPL_LambdaNot1 sanity-checks SPL at a typical
// lambda value.
func TestLuceneDistributionSPL_LambdaNot1(t *testing.T) {
	d := NewLuceneDistributionSPL()
	got := d.Score(makeBasicStats(1.0), 4.0, 0.3)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("SPL produced non-finite score: %v", got)
	}
	if got <= 0 {
		t.Fatalf("SPL at lambda=0.3 tfn=4 should be positive, got %v", got)
	}
}

// TestLuceneDistributionSPL_NonDecreasing verifies score is monotone in tfn.
func TestLuceneDistributionSPL_NonDecreasing(t *testing.T) {
	d := NewLuceneDistributionSPL()
	stats := makeBasicStats(1.0)
	prev := math.Inf(-1)
	for tfn := 0.0; tfn < 100; tfn += 1 {
		got := d.Score(stats, tfn, 0.2)
		if got < prev-1e-9 {
			t.Fatalf("SPL non-monotone at tfn=%v: %v < %v", tfn, got, prev)
		}
		prev = got
	}
}

// TestLuceneLambdaDF cross-checks LambdaDF = (n+1)/(N+1).
func TestLuceneLambdaDF(t *testing.T) {
	stats := makeBasicStats(1.0)
	got := NewLuceneLambdaDF().Lambda(stats)
	want := float32(float64(stats.DocFreq()+1) / float64(stats.NumberOfDocuments()+1))
	if got != want {
		t.Fatalf("LambdaDF: got %v, want %v", got, want)
	}
}

// TestLuceneLambdaTTF cross-checks LambdaTTF = (F+1)/(N+1).
func TestLuceneLambdaTTF(t *testing.T) {
	stats := makeBasicStats(1.0)
	got := NewLuceneLambdaTTF().Lambda(stats)
	want := float32(float64(stats.TotalTermFreq()+1) / float64(stats.NumberOfDocuments()+1))
	if got != want {
		t.Fatalf("LambdaTTF: got %v, want %v", got, want)
	}
}

// TestLuceneLambdaDF_AvoidOne checks that lambda is perturbed away from 1.
func TestLuceneLambdaDF_AvoidOne(t *testing.T) {
	stats := NewLuceneBasicStats("body", 1.0)
	stats.SetDocFreq(10)
	stats.SetNumberOfDocuments(10)
	got := NewLuceneLambdaDF().Lambda(stats)
	if got == 1 {
		t.Fatal("LambdaDF must perturb lambda away from 1")
	}
}

// TestLuceneIBSimilarity_Score_LL_DF_H2 wires a canonical IB triple and
// cross-checks Score104 against an independent computation.
func TestLuceneIBSimilarity_Score_LL_DF_H2(t *testing.T) {
	dist := NewLuceneDistributionLL()
	lam := NewLuceneLambdaDF()
	norm := NewLuceneNormalizationH2()
	sim := NewLuceneIBSimilarity(dist, lam, norm)

	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(2.0, cs, ts)
	freq := float32(4)
	encoded := int64(64)

	got := scorer.Score104(freq, encoded)

	stats := NewLuceneBasicStats("body", 2.0)
	sim.FillBasicStats(stats, cs, ts)
	docLen := basicSimScorerLength(encoded)
	tfn := norm.Tfn(stats, float64(freq), docLen)
	want := float32(2.0 * dist.Score(stats, tfn, float64(lam.Lambda(stats))))
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Fatalf("score: got %v, want %v", got, want)
	}
}

// TestLuceneIBSimilarity_String verifies the canonical toString.
func TestLuceneIBSimilarity_String(t *testing.T) {
	sim := NewLuceneIBSimilarity(
		NewLuceneDistributionLL(),
		NewLuceneLambdaDF(),
		NewLuceneNormalizationH2(),
	)
	if got, want := sim.String(), "IB LL-D2"; got != want {
		t.Fatalf("String: got %q, want %q", got, want)
	}

// TestLuceneIBSimilarity_NilParamsPanic defends the nil check.
}
func TestLuceneIBSimilarity_NilParamsPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil component")
		}
	}()
	NewLuceneIBSimilarity(nil, NewLuceneLambdaDF(), NewLuceneNoNormalization())
}