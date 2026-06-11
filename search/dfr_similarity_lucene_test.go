// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// makeBasicStats builds a populated LuceneBasicStats for DFR unit tests.
func makeBasicStats(boost float64) *LuceneBasicStats {
	s := NewLuceneBasicStats("body", boost)
	s.SetNumberOfDocuments(1000)
	s.SetNumberOfFieldTokens(50000)
	s.SetAvgFieldLength(50.0)
	s.SetDocFreq(100)
	s.SetTotalTermFreq(300)
	return s
}

// TestLuceneBasicModelG_Score cross-checks the canonical formula.
func TestLuceneBasicModelG_Score(t *testing.T) {
	stats := makeBasicStats(1.0)
	m := NewLuceneBasicModelG()
	tfn := 3.0
	ae := 1.5
	got := m.Score(stats, tfn, ae)
	F := float64(stats.TotalTermFreq() + 1)
	N := float64(stats.NumberOfDocuments())
	lambda := F / (N + F)
	A := math.Log2(lambda + 1)
	B := math.Log2((1 + lambda) / lambda)
	want := (B - (B-A)/(1+tfn)) * ae
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("G: got %v, want %v", got, want)
	}
}

// TestLuceneBasicModelIF_Score cross-checks the canonical formula.
func TestLuceneBasicModelIF_Score(t *testing.T) {
	stats := makeBasicStats(1.0)
	m := NewLuceneBasicModelIF()
	tfn := 4.0
	ae := 2.0
	got := m.Score(stats, tfn, ae)
	N := float64(stats.NumberOfDocuments())
	F := float64(stats.TotalTermFreq())
	A := math.Log2(1 + (N+1)/(F+0.5))
	want := A * ae * (1 - 1/(1+tfn))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("I(F): got %v, want %v", got, want)
	}
}

// TestLuceneBasicModelIne_Score cross-checks the canonical formula.
func TestLuceneBasicModelIne_Score(t *testing.T) {
	stats := makeBasicStats(1.0)
	m := NewLuceneBasicModelIne()
	tfn := 2.5
	ae := 1.0
	got := m.Score(stats, tfn, ae)
	N := float64(stats.NumberOfDocuments())
	F := float64(stats.TotalTermFreq())
	ne := N * (1 - math.Pow((N-1)/N, F))
	A := math.Log2((N + 1) / (ne + 0.5))
	want := A * ae * (1 - 1/(1+tfn))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("I(ne): got %v, want %v", got, want)
	}
}

// TestLuceneBasicModelIn_Score cross-checks the canonical formula.
func TestLuceneBasicModelIn_Score(t *testing.T) {
	stats := makeBasicStats(1.0)
	m := NewLuceneBasicModelIn()
	tfn := 6.0
	ae := 1.0
	got := m.Score(stats, tfn, ae)
	N := float64(stats.NumberOfDocuments())
	n := float64(stats.DocFreq())
	A := math.Log2((N + 1) / (n + 0.5))
	want := A * ae * (1 - 1/(1+tfn))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("I(n): got %v, want %v", got, want)
	}
}

// TestLuceneAfterEffectB verifies (F+1)/n where F=ttf+1, n=df+1.
func TestLuceneAfterEffectB(t *testing.T) {
	stats := makeBasicStats(1.0)
	e := NewLuceneAfterEffectB()
	got := e.ScoreTimes1pTfn(stats)
	F := float64(stats.TotalTermFreq() + 1)
	n := float64(stats.DocFreq() + 1)
	want := (F + 1) / n
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("B: got %v, want %v", got, want)
	}
}

// TestLuceneAfterEffectL_Constant verifies AfterEffectL always returns 1.
func TestLuceneAfterEffectL_Constant(t *testing.T) {
	e := NewLuceneAfterEffectL()
	if got := e.ScoreTimes1pTfn(makeBasicStats(1.0)); got != 1.0 {
		t.Fatalf("L: got %v, want 1.0", got)
	}
}

// TestLuceneNormalizations cross-checks each formula.
func TestLuceneNormalizations(t *testing.T) {
	stats := makeBasicStats(1.0)
	tf := 3.0
	length := 100.0
	avgfl := stats.AvgFieldLength()

	t.Run("H1", func(t *testing.T) {
		n := NewLuceneNormalizationH1()
		want := tf * float64(n.GetC()) * (avgfl / length)
		if got := n.Tfn(stats, tf, length); math.Abs(got-want) > 1e-9 {
			t.Fatalf("H1: got %v, want %v", got, want)
		}
	})
	t.Run("H2", func(t *testing.T) {
		n := NewLuceneNormalizationH2()
		want := tf * math.Log2(1+float64(n.GetC())*avgfl/length)
		if got := n.Tfn(stats, tf, length); math.Abs(got-want) > 1e-9 {
			t.Fatalf("H2: got %v, want %v", got, want)
		}
	})
	t.Run("H3", func(t *testing.T) {
		n := NewLuceneNormalizationH3WithMu(500)
		mu := 500.0
		want := (tf + mu*((float64(stats.TotalTermFreq())+1)/(float64(stats.NumberOfFieldTokens())+1))) / (length + mu) * mu
		if got := n.Tfn(stats, tf, length); math.Abs(got-want) > 1e-9 {
			t.Fatalf("H3: got %v, want %v", got, want)
		}
	})
	t.Run("Z", func(t *testing.T) {
		// z is stored as float32 internally — promoting it back to
		// float64 in the test would compare different bit patterns. Use
		// the same float32 promotion the implementation does.
		n := NewLuceneNormalizationZWithZ(0.40)
		want := tf * math.Pow(avgfl/length, float64(float32(0.40)))
		if got := n.Tfn(stats, tf, length); math.Abs(got-want) > 1e-9 {
			t.Fatalf("Z: got %v, want %v", got, want)
		}
	})
	t.Run("None", func(t *testing.T) {
		n := NewLuceneNoNormalization()
		if got := n.Tfn(stats, tf, length); got != tf {
			t.Fatalf("None: got %v, want %v", got, tf)
		}
	})
}

// TestLuceneNormalizationH2_IllegalCPanics defends the validation.
func TestLuceneNormalizationH2_IllegalCPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for NaN c")
		}
	}()
	NewLuceneNormalizationH2WithC(float32(math.NaN()))
}

// TestLuceneNormalizationZ_OutOfRangePanics defends the (0,0.5) check.
func TestLuceneNormalizationZ_OutOfRangePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for z >= 0.5")
		}
	}()
	NewLuceneNormalizationZWithZ(0.5)
}

// TestLuceneDFRSimilarity_NilParamsPanic verifies nil checks.
func TestLuceneDFRSimilarity_NilParamsPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil component")
		}
	}()
	NewLuceneDFRSimilarity(nil, NewLuceneAfterEffectL(), NewLuceneNoNormalization())
}

// TestLuceneDFRSimilarity_Score_IneL_H2 wires a canonical DFR triple and
// cross-checks the end-to-end Score104 against an independent computation.
func TestLuceneDFRSimilarity_Score_IneL_H2(t *testing.T) {
	bm := NewLuceneBasicModelIne()
	ae := NewLuceneAfterEffectL()
	norm := NewLuceneNormalizationH2()
	sim := NewLuceneDFRSimilarity(bm, ae, norm)

	cs := NewCollectionStatistics("body", 1000, 1000, 50000, 1000)
	ts := NewTermStatistics(index.NewTerm("body", "go"), 100, 300)
	scorer := sim.Scorer104(2.0, cs, ts)
	freq := float32(4)
	norm64 := int64(64) // arbitrary norm byte

	got := scorer.Score104(freq, norm64)

	// Recompute with raw helpers.
	stats := NewLuceneBasicStats("body", 2.0)
	sim.FillBasicStats(stats, cs, ts)
	docLen := basicSimScorerLength(norm64)
	tfn := norm.Tfn(stats, float64(freq), docLen)
	aev := ae.ScoreTimes1pTfn(stats)
	want := float32(2.0 * bm.Score(stats, tfn, aev))
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Fatalf("score: got %v, want %v", got, want)
	}

// TestLuceneDFRSimilarity_String verifies the canonical "DFR" + codes
// concatenation.
func TestLuceneDFRSimilarity_String(t *testing.T) {
	sim := NewLuceneDFRSimilarity(
		NewLuceneBasicModelG(),
		NewLuceneAfterEffectB(),
		NewLuceneNormalizationH2(),
	)
	if got, want := sim.String(), "DFR GB2"; got != want {
		t.Fatalf("String: got %q, want %q", got, want)
	}
}