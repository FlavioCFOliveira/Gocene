// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestWANDScorer.java

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// doTestScalingFactor asserts that f scaled by WANDScoreScalingFactor(f) is
// in [2^23, 2^24).
// Mirrors TestWANDScorer.doTestScalingFactor.
func doTestScalingFactor(t *testing.T, f float32) {
	t.Helper()
	sf := search.WANDScoreScalingFactor(f)
	scaled := math.Ldexp(float64(f), sf)
	lo := float64(int64(1) << (search.FloatMantissaBits - 1))
	hi := float64(int64(1) << search.FloatMantissaBits)
	if scaled < lo || scaled >= hi {
		t.Errorf("f=%v: scaled=%v, want [%v, %v)", f, scaled, lo, hi)
	}
}

// constantScorer is a synthetic Scorer that iterates over a fixed doc set
// with a fixed score and maxScore.
type constantScorer struct {
	docs     []int
	pos      int
	score    float32
	maxScore float32
}

func newConstantScorer(docs []int, score, maxScore float32) *constantScorer {
	return &constantScorer{docs: docs, pos: -1, score: score, maxScore: maxScore}
}

func (s *constantScorer) DocID() int {
	if s.pos < 0 {
		return -1
	}
	if s.pos >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.pos]
}

func (s *constantScorer) NextDoc() (int, error) {
	s.pos++
	return s.DocID(), nil
}

func (s *constantScorer) Advance(target int) (int, error) {
	for s.pos++; s.pos < len(s.docs) && s.docs[s.pos] < target; s.pos++ {
	}
	return s.DocID(), nil
}

func (s *constantScorer) Cost() int64 { return int64(len(s.docs)) }

func (s *constantScorer) DocIDRunEnd() int { return s.DocID() + 1 }

func (s *constantScorer) Score() float32 { return s.score }

func (s *constantScorer) GetMaxScore(_ int) float32 { return s.maxScore }

func (s *constantScorer) AdvanceShallow(int) (int, error) { return search.NO_MORE_DOCS, nil }

// wandCollectAll iterates the scorer and returns collected (doc, score) pairs.
func wandCollectAll(t *testing.T, sc search.Scorer) ([]int, []float32) {
	t.Helper()
	var docs []int
	var scores []float32
	for {
		d, err := sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if d == search.NO_MORE_DOCS {
			break
		}
		docs = append(docs, d)
		scores = append(scores, sc.Score())
	}
	return docs, scores
}

// ─── TestWANDScorer_ScalingFactor ────────────────────────────────────────────

// TestWANDScorer_ScalingFactor mirrors TestWANDScorer.testScalingFactor.
func TestWANDScorer_ScalingFactor(t *testing.T) {
	doTestScalingFactor(t, 1)
	doTestScalingFactor(t, 2)
	doTestScalingFactor(t, float32(math.Nextafter(1, 0)))
	doTestScalingFactor(t, float32(math.Nextafter(1, 2)))
	doTestScalingFactor(t, math.SmallestNonzeroFloat32)
	doTestScalingFactor(t, float32(math.Nextafter(float64(math.SmallestNonzeroFloat32), 1)))
	doTestScalingFactor(t, math.MaxFloat32)
	doTestScalingFactor(t, float32(math.Nextafter(float64(math.MaxFloat32), 0)))

	// WANDScoreScalingFactor(0) == WANDScoreScalingFactor(MinValue)+1
	got0 := search.WANDScoreScalingFactor(0)
	wantMin := search.WANDScoreScalingFactor(math.SmallestNonzeroFloat32) + 1
	if got0 != wantMin {
		t.Errorf("scalingFactor(0)=%d, want %d", got0, wantMin)
	}

	// WANDScoreScalingFactor(+Inf) == WANDScoreScalingFactor(MaxFloat32)-1
	gotInf := search.WANDScoreScalingFactor(float32(math.Inf(1)))
	wantMax := search.WANDScoreScalingFactor(math.MaxFloat32) - 1
	if gotInf != wantMax {
		t.Errorf("scalingFactor(+Inf)=%d, want %d", gotInf, wantMax)
	}

	// Greater scores produce lower scaling factors.
	if !(search.WANDScoreScalingFactor(1) > search.WANDScoreScalingFactor(10)) {
		t.Error("scalingFactor(1) should be > scalingFactor(10)")
	}
	if !(search.WANDScoreScalingFactor(math.MaxFloat32) > search.WANDScoreScalingFactor(float32(math.Inf(1)))) {
		t.Error("scalingFactor(MaxFloat32) should be > scalingFactor(+Inf)")
	}
	if !(search.WANDScoreScalingFactor(0) > search.WANDScoreScalingFactor(math.SmallestNonzeroFloat32)) {
		t.Error("scalingFactor(0) should be > scalingFactor(MinValue)")
	}
}

// TestWANDScorer_ScalingFactorPanic verifies negative scores panic.
func TestWANDScorer_ScalingFactorPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for negative score, got none")
		}
	}()
	search.WANDScoreScalingFactor(-1)
}

// TestWANDScorer_ScaleMaxScore mirrors TestWANDScorer.testScaleMaxScore.
func TestWANDScorer_ScaleMaxScore(t *testing.T) {
	mantissaLo := int64(1) << (search.FloatMantissaBits - 1)
	if got := search.WANDScaleMaxScore(32, search.WANDScoreScalingFactor(32)); got != mantissaLo {
		t.Errorf("scaleMaxScore(32, sf(32))=%d, want %d", got, mantissaLo)
	}

	sf60 := search.WANDScoreScalingFactor(float32(math.Ldexp(1, 60)))
	if got := search.WANDScaleMaxScore(32, sf60); got != 1 {
		t.Errorf("scaleMaxScore(32, sf(2^60))=%d, want 1", got)
	}

	sfInf := search.WANDScoreScalingFactor(float32(math.Inf(1)))
	if got := search.WANDScaleMaxScore(32, sfInf); got != 1 {
		t.Errorf("scaleMaxScore(32, sf(+Inf))=%d, want 1", got)
	}
}

// TestWANDScorer_BasicConjunction tests basic WAND with minShouldMatch=0.
// Docs: scorer A covers {0,1,3}, scorer B covers {0,3,4,5}, scorer C covers {3,5}.
// Expected: all docs that appear in any scorer.
func TestWANDScorer_BasicConjunction(t *testing.T) {
	sA := newConstantScorer([]int{0, 1, 3}, 2, 2)
	sB := newConstantScorer([]int{0, 3, 4, 5}, 1, 1)
	sC := newConstantScorer([]int{3, 5}, 3, 3)

	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB, sC}, 0, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}

	docs, _ := wandCollectAll(t, ws)
	wantDocs := []int{0, 1, 3, 4, 5}
	if len(docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", docs, wantDocs)
	}
	for i, d := range wantDocs {
		if docs[i] != d {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], d)
		}
	}
}

// TestWANDScorer_MinShouldMatch2 tests WAND with minShouldMatch=2.
// Docs where ≥ 2 scorers match: {0,3,5}.
// Scorers: A={0,1,3}, B={0,3,4,5}, C={3,5}.
func TestWANDScorer_MinShouldMatch2(t *testing.T) {
	sA := newConstantScorer([]int{0, 1, 3}, 2, 2)
	sB := newConstantScorer([]int{0, 3, 4, 5}, 1, 1)
	sC := newConstantScorer([]int{3, 5}, 3, 3)

	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB, sC}, 2, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}

	docs, _ := wandCollectAll(t, ws)
	wantDocs := []int{0, 3, 5}
	if len(docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v (minShouldMatch=2)", docs, wantDocs)
	}
	for i, d := range wantDocs {
		if docs[i] != d {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], d)
		}
	}
}

// TestWANDScorer_MinShouldMatchTooHigh verifies the constructor error.
func TestWANDScorer_MinShouldMatchTooHigh(t *testing.T) {
	sA := newConstantScorer([]int{0, 1}, 1, 1)
	sB := newConstantScorer([]int{0, 1}, 1, 1)

	_, err := search.NewWANDScorer([]search.Scorer{sA, sB}, 2, search.COMPLETE, 100)
	if err == nil {
		t.Error("expected error for minShouldMatch >= len(scorers), got nil")
	}
}

// TestWANDScorer_Cost verifies Cost() reflects CostWithMinShouldMatch.
func TestWANDScorer_Cost(t *testing.T) {
	// Three scorers of cost 10, 20, 30; minShouldMatch=1 → keep 3-1+1=3 cheapest → sum = 60.
	sA := newConstantScorer(make([]int, 10), 1, 1)
	sB := newConstantScorer(make([]int, 20), 1, 1)
	sC := newConstantScorer(make([]int, 30), 1, 1)

	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB, sC}, 1, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}
	want := search.CostWithMinShouldMatch([]int64{10, 20, 30}, 3, 1)
	if ws.Cost() != want {
		t.Errorf("Cost()=%d, want %d", ws.Cost(), want)
	}
}

// TestWANDScorer_ImplementsScorer checks interface satisfaction.
func TestWANDScorer_ImplementsScorer(t *testing.T) {
	sA := newConstantScorer([]int{0}, 1, 1)
	sB := newConstantScorer([]int{0}, 1, 1)
	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB}, 0, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}
	var _ search.Scorer = ws
}

// TestWANDScorer_ScoreSum verifies scores are summed from all matching leads.
func TestWANDScorer_ScoreSum(t *testing.T) {
	// Both scorers match doc 0; expected score = 2+3=5.
	sA := newConstantScorer([]int{0}, 2, 2)
	sB := newConstantScorer([]int{0}, 3, 3)

	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB}, 0, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}
	doc, err := ws.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc()=(%d,%v), want (0, nil)", doc, err)
	}
	const eps = float32(1e-4)
	if got := ws.Score(); got < 5-eps || got > 5+eps {
		t.Errorf("Score()=%v, want 5", got)
	}
}

// TestWANDScorer_EmptyResult verifies no matches when no scorers match.
func TestWANDScorer_EmptyResult(t *testing.T) {
	sA := newConstantScorer([]int{}, 1, 1)
	sB := newConstantScorer([]int{}, 1, 1)

	ws, err := search.NewWANDScorer([]search.Scorer{sA, sB}, 0, search.COMPLETE, 100)
	if err != nil {
		t.Fatalf("NewWANDScorer: %v", err)
	}
	doc, err := ws.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("NextDoc()=%d, want NO_MORE_DOCS", doc)
	}
}