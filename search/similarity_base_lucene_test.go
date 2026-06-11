// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLuceneBasicStats_RoundTrip verifies every getter/setter pair.
func TestLuceneBasicStats_RoundTrip(t *testing.T) {
	s := NewLuceneBasicStats("title", 2.5)
	if s.Field() != "title" {
		t.Fatalf("Field: got %q, want %q", s.Field(), "title")
	}
	if s.Boost() != 2.5 {
		t.Fatalf("Boost: got %f, want 2.5", s.Boost())
	}
	s.SetNumberOfDocuments(100)
	s.SetNumberOfFieldTokens(5000)
	s.SetAvgFieldLength(50)
	s.SetDocFreq(20)
	s.SetTotalTermFreq(80)
	if s.NumberOfDocuments() != 100 {
		t.Fatalf("NumberOfDocuments: got %d", s.NumberOfDocuments())
	}
	if s.NumberOfFieldTokens() != 5000 {
		t.Fatalf("NumberOfFieldTokens: got %d", s.NumberOfFieldTokens())
	}
	if s.AvgFieldLength() != 50 {
		t.Fatalf("AvgFieldLength: got %f", s.AvgFieldLength())
	}
	if s.DocFreq() != 20 {
		t.Fatalf("DocFreq: got %d", s.DocFreq())
	}
	if s.TotalTermFreq() != 80 {
		t.Fatalf("TotalTermFreq: got %d", s.TotalTermFreq())
	}
}

// TestLuceneSimLog2 verifies the log2 helper against math.Log2 over a
// representative range.
func TestLuceneSimLog2(t *testing.T) {
	for _, x := range []float64{1, 2, 4, 8, 16, 1024, 0.5, 1e-9} {
		got := LuceneSimLog2(x)
		want := math.Log2(x)
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("LuceneSimLog2(%g): got %v, want %v", x, got, want)
		}
	}
}

// TestLuceneSimLengthTable_RoundTrip verifies the length table matches
// SmallFloat.Byte4ToInt for all 256 byte values.
func TestLuceneSimLengthTable_RoundTrip(t *testing.T) {
	for b := 0; b < 256; b++ {
		got := luceneSimLengthTable[b]
		want := float32(util.Byte4ToInt(byte(b)))
		if got != want {
			t.Fatalf("luceneSimLengthTable[%d]: got %v, want %v", b, got, want)
		}
	}
}

// TestLuceneSimilarityBase_FillBasicStats checks that
// FillBasicStats copies the same fields the Java reference does.
func TestLuceneSimilarityBase_FillBasicStats(t *testing.T) {
	sim := NewLuceneSimilarityBase(
		func(_ *LuceneBasicStats, freq, _ float64) float64 { return freq },
		nil,
		nil,
	)
	collStats := NewCollectionStatistics("body", 1000, 800, 50000, 10000)
	termStats := NewTermStatistics(index.NewTerm("body", "go"), 200, 350)
	stats := sim.NewStats("body", 1.0)
	sim.FillBasicStats(stats, collStats, termStats)

	if stats.NumberOfDocuments() != 800 {
		t.Fatalf("numberOfDocuments: got %d, want 800", stats.NumberOfDocuments())
	}
	if stats.NumberOfFieldTokens() != 50000 {
		t.Fatalf("numberOfFieldTokens: got %d, want 50000", stats.NumberOfFieldTokens())
	}
	if stats.AvgFieldLength() != 62.5 { // 50000/800
		t.Fatalf("avgFieldLength: got %v, want 62.5", stats.AvgFieldLength())
	}
	if stats.DocFreq() != 200 {
		t.Fatalf("docFreq: got %d, want 200", stats.DocFreq())
	}
	if stats.TotalTermFreq() != 350 {
		t.Fatalf("totalTermFreq: got %d, want 350", stats.TotalTermFreq())
	}
}

// TestLuceneSimilarityBase_Scorer104_Single verifies that Scorer104 returns
// the per-term BasicSimScorer when len(termStats) == 1 — mirroring the
// Java early return.
func TestLuceneSimilarityBase_Scorer104_Single(t *testing.T) {
	sim := NewLuceneSimilarityBase(
		func(stats *LuceneBasicStats, freq, docLen float64) float64 {
			return freq * float64(stats.DocFreq()) / docLen
		},
		nil,
		nil,
	)
	collStats := NewCollectionStatistics("body", 100, 80, 800, 200)
	termStats := NewTermStatistics(index.NewTerm("body", "go"), 10, 25)
	sc := sim.Scorer104(1.0, collStats, termStats)
	if _, ok := sc.(*basicSimScorerLucene); !ok {
		t.Fatalf("expected *basicSimScorerLucene, got %T", sc)
	}
}

// TestLuceneSimilarityBase_Scorer104_Multi verifies the multi-term path
// returns a multiSimScorerLucene whose Score104 sums the per-term scores.
func TestLuceneSimilarityBase_Scorer104_Multi(t *testing.T) {
	sim := NewLuceneSimilarityBase(
		func(_ *LuceneBasicStats, freq, _ float64) float64 { return freq },
		nil,
		nil,
	)
	collStats := NewCollectionStatistics("body", 100, 80, 800, 200)
	ts1 := NewTermStatistics(index.NewTerm("body", "a"), 5, 10)
	ts2 := NewTermStatistics(index.NewTerm("body", "b"), 7, 14)
	sc := sim.Scorer104(1.0, collStats, ts1, ts2)
	multi, ok := sc.(*multiSimScorerLucene)
	if !ok {
		t.Fatalf("expected *multiSimScorerLucene, got %T", sc)
	}
	got := multi.Score104(3.0, 1)
	want := float32(6.0) // 3.0 + 3.0
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("Score104(3,1): got %v, want %v", got, want)
	}
}

// TestLuceneSimilarityBase_Scorer104_Empty checks the no-op fallback.
func TestLuceneSimilarityBase_Scorer104_Empty(t *testing.T) {
	sim := NewLuceneSimilarityBase(
		func(_ *LuceneBasicStats, _, _ float64) float64 { return 1 },
		nil, nil,
	)
	collStats := NewCollectionStatistics("body", 100, 80, 800, 200)
	sc := sim.Scorer104(1.0, collStats)
	if _, ok := sc.(*noopLuceneSimScorer); !ok {
		t.Fatalf("expected *noopLuceneSimScorer, got %T", sc)
	}
	if got := sc.Score104(10, 1); got != 0 {
		t.Fatalf("no-op score: got %v, want 0", got)
	}

// TestLuceneSimilarityBase_NilScorePanics verifies the safety check at
// construction time.
func TestLuceneSimilarityBase_NilScorePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil score function")
		}
	}()
	NewLuceneSimilarityBase(nil, nil, nil)
}

// TestLuceneSimilarityBase_ComputeNormFromInvertState verifies that the
// canonical norm encoder is dispatched through the base.
func TestLuceneSimilarityBase_ComputeNormFromInvertState(t *testing.T) {
	sim := NewLuceneSimilarityBaseWithDiscount(true,
		func(_ *LuceneBasicStats, freq, _ float64) float64 { return freq },
		nil, nil)
	state := index.NewFieldInvertStateFull(10, "f", index.IndexOptionsDocsAndFreqs,
		0, 20, 5, 0, 0, 0)
	got := sim.ComputeNormFromInvertState(state)
	want := DefaultComputeNormFromInvertState(state, true)
	if got != want {
		t.Fatalf("delegated norm: got %d, want %d", got, want)
	}
}