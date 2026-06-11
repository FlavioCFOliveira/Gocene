// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ConjunctionScorer.java
//
// No Java test peer found (class is package-private in Lucene).  These tests
// cover the exported Go contract: construction, DISI iteration semantics,
// score aggregation, and GetMaxScore.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestConjunctionScorer_ImplementsScorer checks the compile-time assertion.
func TestConjunctionScorer_ImplementsScorer(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 1, 2}, 1, 3)
	sc2 := newConstantScorer([]int{0, 1, 2}, 1, 3)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})
	var _ search.Scorer = cs
}

// TestConjunctionScorer_Intersection verifies AND semantics over two clauses.
func TestConjunctionScorer_Intersection(t *testing.T) {
	// clause A: 0,2,4,6; clause B: 2,4,8 → intersection: 2,4
	sc1 := newConstantScorer([]int{0, 2, 4, 6}, 1, 4)
	sc2 := newConstantScorer([]int{2, 4, 8}, 1, 3)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})

	docs := advanceAll(t, cs)
	want := []int{2, 4}
	if len(docs) != len(want) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], want[i])
		}
	}
}

// TestConjunctionScorer_EmptyIntersection verifies disjoint clauses yield no docs.
func TestConjunctionScorer_EmptyIntersection(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 2, 4}, 1, 3)
	sc2 := newConstantScorer([]int{1, 3, 5}, 1, 3)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})
	docs := advanceAll(t, cs)
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %v", docs)
	}
}

// TestConjunctionScorer_ThreeClauses verifies three-way AND.
func TestConjunctionScorer_ThreeClauses(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 2, 4, 6, 8, 10}, 1, 6)
	sc2 := newConstantScorer([]int{0, 3, 6, 9}, 1, 4)
	sc3 := newConstantScorer([]int{0, 5, 10}, 1, 3)
	cs := search.NewConjunctionScorer(
		[]search.Scorer{sc1, sc2, sc3},
		[]search.Scorer{sc1, sc2, sc3},
	)
	docs := advanceAll(t, cs)
	want := []int{0}
	if len(docs) != len(want) || (len(docs) > 0 && docs[0] != 0) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
}

// TestConjunctionScorer_ScoreSum verifies scores are summed across scoring clauses.
func TestConjunctionScorer_ScoreSum(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 1, 2}, 2.0, 3)
	sc2 := newConstantScorer([]int{0, 1, 2}, 3.0, 3)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})

	doc, err := cs.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc()=%d,%v, want 0,nil", doc, err)
	}
	score := cs.Score()
	if score != 5.0 {
		t.Errorf("Score()=%v, want 5.0 (sum of 2+3)", score)
	}
}

// TestConjunctionScorer_ScoringSubset verifies that only scoring-subset clauses
// contribute to the score (required-only clauses are filtered).
func TestConjunctionScorer_ScoringSubset(t *testing.T) {
	// scoring: sc1 (score=10), required-only: sc2 (score=99)
	sc1 := newConstantScorer([]int{0, 1, 2}, 10.0, 3)
	sc2 := newConstantScorer([]int{0, 1, 2}, 99.0, 3)
	cs := search.NewConjunctionScorer(
		[]search.Scorer{sc1, sc2},
		[]search.Scorer{sc1}, // only sc1 contributes to score
	)
	doc, err := cs.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("NextDoc()=%d,%v", doc, err)
	}
	score := cs.Score()
	if score != 10.0 {
		t.Errorf("Score()=%v, want 10.0 (only scoring subset)", score)
	}
}

// TestConjunctionScorer_Cost verifies Cost() ≤ min clause cost.
func TestConjunctionScorer_Cost(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 1}, 1, 2)
	sc2 := newConstantScorer([]int{0, 1, 2, 3, 4}, 1, 5)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})
	if cs.Cost() > 2 {
		t.Errorf("Cost()=%d, want ≤2 (cheapest clause)", cs.Cost())
	}
}

// TestConjunctionScorer_TwoPhaseIterator verifies TwoPhaseIterator() returns
// nil for plain DISI scorers (no two-phase support).
func TestConjunctionScorer_TwoPhaseIterator(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 1}, 1, 2)
	sc2 := newConstantScorer([]int{0, 1}, 1, 2)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})
	// constantScorer has no TwoPhaseIterator, so this should be nil.
	if tpi := cs.TwoPhaseIterator(); tpi != nil {
		t.Errorf("TwoPhaseIterator() = %v, want nil for plain scorers", tpi)
	}
}

// TestConjunctionScorer_GetMaxScore verifies GetMaxScore sums contributing scorers.
func TestConjunctionScorer_GetMaxScore(t *testing.T) {
	// score=3.0, maxScore=3.0 for sc1; score=4.0, maxScore=4.0 for sc2.
	sc1 := newConstantScorer([]int{0, 1}, 3.0, 3.0)
	sc2 := newConstantScorer([]int{0, 1}, 4.0, 4.0)
	cs := search.NewConjunctionScorer([]search.Scorer{sc1, sc2}, []search.Scorer{sc1, sc2})
	// Advance both clauses to doc 0.
	if _, err := cs.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	maxScore := cs.GetMaxScore(1)
	if maxScore != 7.0 {
		t.Errorf("GetMaxScore(1)=%v, want 7.0 (sum of 3+4)", maxScore)
	}

// advanceAll collects all doc IDs from a Scorer by calling NextDoc.
}
func advanceAll(t *testing.T, sc search.Scorer) []int {
	t.Helper()
	var docs []int
	for {
		d, err := sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == search.NO_MORE_DOCS {
			break
		}
		docs = append(docs, d)
	}
	return docs
}