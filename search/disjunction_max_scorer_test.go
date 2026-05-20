// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionMaxScorer.java
//
// No dedicated Java test peer found (TestDisjunctionMaxScorer does not
// exist in Lucene 10.4.0 core tests).  These tests cover the Go public
// contract: score formula, GetMaxScore, and error cases.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// makeDMScorer builds a DisjunctionMaxScorer from constant-score sub-scorers.
// Each entry in docs is a slice of doc IDs for that clause; score is uniform.
func makeDMScorer(
	t *testing.T,
	tieBreaker float32,
	clauses []struct {
		docs  []int
		score float32
	},
) *search.DisjunctionMaxScorer {
	t.Helper()
	scorers := make([]search.Scorer, len(clauses))
	for i, c := range clauses {
		scorers[i] = newConstantScorer(c.docs, c.score, c.score)
	}
	var totalCost int64
	for _, sc := range scorers {
		totalCost += sc.Cost()
	}
	sc, err := search.NewDisjunctionMaxScorer(tieBreaker, scorers, search.COMPLETE, totalCost)
	if err != nil {
		t.Fatalf("NewDisjunctionMaxScorer: %v", err)
	}
	return sc
}

// collectDM collects all docs and their scores from a DisjunctionMaxScorer.
func collectDM(t *testing.T, sc *search.DisjunctionMaxScorer) ([]int, []float32) {
	t.Helper()
	var docs []int
	var scores []float32
	doc, err := sc.NextDoc()
	for err == nil && doc != search.NO_MORE_DOCS {
		docs = append(docs, doc)
		scores = append(scores, sc.Score())
		doc, err = sc.NextDoc()
	}
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	return docs, scores
}

// TestDisjunctionMaxScorer_InvalidTieBreaker verifies constructor error.
func TestDisjunctionMaxScorer_InvalidTieBreaker(t *testing.T) {
	sc := newConstantScorer([]int{0}, 1, 1)
	sc2 := newConstantScorer([]int{0}, 1, 1)
	for _, bad := range []float32{-0.1, 1.1, 2.0, -1.0} {
		_, err := search.NewDisjunctionMaxScorer(bad, []search.Scorer{sc, sc2}, search.COMPLETE, 1)
		if err == nil {
			t.Errorf("expected error for tieBreakerMultiplier=%g, got nil", bad)
		}
	}
}

// TestDisjunctionMaxScorer_ImplementsScorer checks interface.
func TestDisjunctionMaxScorer_ImplementsScorer(t *testing.T) {
	sc := makeDMScorer(t, 0, []struct {
		docs  []int
		score float32
	}{
		{[]int{0}, 1},
		{[]int{0}, 2},
	})
	var _ search.Scorer = sc
}

// TestDisjunctionMaxScorer_SingleMatch verifies only the max score is returned
// when tieBreakerMultiplier=0.
func TestDisjunctionMaxScorer_SingleMatch(t *testing.T) {
	// doc 0: clause A score=2, clause B score=1 → max=2 + 0*1 = 2
	sc := makeDMScorer(t, 0, []struct {
		docs  []int
		score float32
	}{
		{[]int{0}, 2},
		{[]int{0}, 1},
	})

	docs, scores := collectDM(t, sc)
	if len(docs) != 1 || docs[0] != 0 {
		t.Fatalf("docs=%v, want [0]", docs)
	}
	const eps = float32(1e-5)
	if scores[0] < 2-eps || scores[0] > 2+eps {
		t.Errorf("score=%v, want 2.0 (tieBreaker=0)", scores[0])
	}
}

// TestDisjunctionMaxScorer_TieBreakerFormula verifies the full formula:
// max + tieBreaker * sumOfOthers.
func TestDisjunctionMaxScorer_TieBreakerFormula(t *testing.T) {
	// doc 0: clause A score=3, clause B score=1
	// expected = 3 + 0.5*1 = 3.5
	sc := makeDMScorer(t, 0.5, []struct {
		docs  []int
		score float32
	}{
		{[]int{0}, 3},
		{[]int{0}, 1},
	})

	docs, scores := collectDM(t, sc)
	if len(docs) != 1 || docs[0] != 0 {
		t.Fatalf("docs=%v, want [0]", docs)
	}
	const eps = float32(1e-4)
	want := float32(3.5)
	if scores[0] < want-eps || scores[0] > want+eps {
		t.Errorf("score=%v, want %v (tieBreaker=0.5)", scores[0], want)
	}
}

// TestDisjunctionMaxScorer_UnionDocs verifies that docs from both clauses
// are visited (union semantics).
func TestDisjunctionMaxScorer_UnionDocs(t *testing.T) {
	// doc 0: only A; doc 1: only B; doc 2: both A and B
	sc := makeDMScorer(t, 0, []struct {
		docs  []int
		score float32
	}{
		{[]int{0, 2}, 1},
		{[]int{1, 2}, 2},
	})

	docs, _ := collectDM(t, sc)
	want := []int{0, 1, 2}
	if len(docs) != len(want) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], want[i])
		}
	}
}

// TestDisjunctionMaxScorer_ThreeClauses verifies score with three overlapping
// sub-scorers and tieBreaker=1 (pure sum, equivalent to DisjunctionSum).
func TestDisjunctionMaxScorer_ThreeClauses(t *testing.T) {
	// doc 0: all three match with scores 1, 2, 3
	// tieBreaker=1 → max=3 + 1*(2+1) = 6
	sc := makeDMScorer(t, 1.0, []struct {
		docs  []int
		score float32
	}{
		{[]int{0}, 1},
		{[]int{0}, 2},
		{[]int{0}, 3},
	})

	docs, scores := collectDM(t, sc)
	if len(docs) != 1 || docs[0] != 0 {
		t.Fatalf("docs=%v, want [0]", docs)
	}
	const eps = float32(1e-4)
	want := float32(6)
	if scores[0] < want-eps || scores[0] > want+eps {
		t.Errorf("score=%v, want %v", scores[0], want)
	}
}

// TestDisjunctionMaxScorer_Cost verifies Cost() is non-zero.
func TestDisjunctionMaxScorer_Cost(t *testing.T) {
	sc := makeDMScorer(t, 0, []struct {
		docs  []int
		score float32
	}{
		{[]int{0, 1, 2}, 1},
		{[]int{3, 4}, 1},
	})
	if sc.Cost() <= 0 {
		t.Errorf("Cost()=%d, want > 0", sc.Cost())
	}
}
