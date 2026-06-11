// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ConjunctionBulkScorer.java
//
// No dedicated Java test peer found.  These tests cover the Go public
// contract of the conjunction bulk-scorer.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// newConjBulkScorer is a convenience wrapper.
func newConjBulkScorer(t *testing.T, scoring []search.Scorer, filtering []search.Scorer) *search.ConjunctionBulkScorer {
	t.Helper()
	bs, err := search.NewConjunctionBulkScorer(scoring, filtering)
	if err != nil {
		t.Fatalf("NewConjunctionBulkScorer: %v", err)
	}
	return bs
}

// collectConj runs scorer and returns collected doc IDs.
func collectConj(t *testing.T, bs *search.ConjunctionBulkScorer) []int {
	t.Helper()
	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, nil); err != nil {
		t.Fatalf("Score: %v", err)
	}
	return lc.docs
}

// TestConjunctionBulkScorer_TooFewClauses verifies ≤1 clause yields an error.
func TestConjunctionBulkScorer_TooFewClauses(t *testing.T) {
	sc := newConstantScorer([]int{0}, 1, 1)
	_, err := search.NewConjunctionBulkScorer([]search.Scorer{sc}, nil)
	if err == nil {
		t.Fatal("expected error for 1 clause, got nil")
	}
	_, err = search.NewConjunctionBulkScorer(nil, nil)
	if err == nil {
		t.Fatal("expected error for 0 clauses, got nil")
	}
}

// TestConjunctionBulkScorer_ImplementsBulkScorer checks interface.
func TestConjunctionBulkScorer_ImplementsBulkScorer(t *testing.T) {
	sc1 := newConstantScorer([]int{0}, 1, 1)
	sc2 := newConstantScorer([]int{0}, 1, 1)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2}, nil)
	var _ search.BulkScorer = bs
}

// TestConjunctionBulkScorer_Intersection verifies AND semantics.
func TestConjunctionBulkScorer_Intersection(t *testing.T) {
	// clause A: 0,2,4,6; clause B: 2,4,8 → intersection: 2,4
	sc1 := newConstantScorer([]int{0, 2, 4, 6}, 1, 4)
	sc2 := newConstantScorer([]int{2, 4, 8}, 1, 3)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2}, nil)
	docs := collectConj(t, bs)
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

// TestConjunctionBulkScorer_EmptyIntersection verifies disjoint clauses yield 0 docs.
func TestConjunctionBulkScorer_EmptyIntersection(t *testing.T) {
	sc1 := newConstantScorer([]int{0, 2, 4}, 1, 3)
	sc2 := newConstantScorer([]int{1, 3, 5}, 1, 3)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2}, nil)
	docs := collectConj(t, bs)
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %v", docs)
	}
}

// TestConjunctionBulkScorer_ThreeClauses verifies three-way AND.
func TestConjunctionBulkScorer_ThreeClauses(t *testing.T) {
	// multiples of 2: 0,2,4,6,8,10
	// multiples of 3: 0,3,6,9
	// multiples of 5: 0,5,10
	// AND: multiples of lcm(2,3,5)=30 within [0,10] → only 0
	sc1 := newConstantScorer([]int{0, 2, 4, 6, 8, 10}, 1, 6)
	sc2 := newConstantScorer([]int{0, 3, 6, 9}, 1, 4)
	sc3 := newConstantScorer([]int{0, 5, 10}, 1, 3)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2, sc3}, nil)
	docs := collectConj(t, bs)
	want := []int{0}
	if len(docs) != len(want) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
	if docs[0] != 0 {
		t.Errorf("docs[0]=%d, want 0", docs[0])
	}
}

// TestConjunctionBulkScorer_FilteringClause verifies requiredNoScoring clauses
// filter correctly without contributing to score.
func TestConjunctionBulkScorer_FilteringClause(t *testing.T) {
	// scoring: {0,2,4,6,8}; filtering: {2,4,6} → result: {2,4,6}
	scoring := newConstantScorer([]int{0, 2, 4, 6, 8}, 3, 5)
	filtering := newConstantScorer([]int{2, 4, 6}, 1, 3)
	bs := newConjBulkScorer(t, []search.Scorer{scoring}, []search.Scorer{filtering})
	docs := collectConj(t, bs)
	want := []int{2, 4, 6}
	if len(docs) != len(want) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], want[i])
		}
	}
}

// TestConjunctionBulkScorer_AcceptDocs verifies acceptDocs filtering.
func TestConjunctionBulkScorer_AcceptDocs(t *testing.T) {
	maxDoc := 20
	sc1 := newConstantScorer([]int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18}, 1, 10)
	sc2 := newConstantScorer([]int{0, 4, 8, 12, 16}, 1, 5)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2}, nil)

	// acceptDocs: every 4th doc starting at 4 — {4,8,12,16}
	everyFour, _ := util.NewFixedBitSet(maxDoc)
	for i := 4; i < maxDoc; i += 4 {
		everyFour.Set(i)
	}

	lc := &batchLeafCollector{}
	if err := fullWindowScore(bs, lc, everyFour); err != nil {
		t.Fatalf("Score: %v", err)
	}
	want := []int{4, 8, 12, 16}
	if len(lc.docs) != len(want) {
		t.Fatalf("docs=%v, want %v", lc.docs, want)
	}
	for i := range want {
		if lc.docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, lc.docs[i], want[i])
		}
	}

// TestConjunctionBulkScorer_Cost verifies Cost equals lead1 cost (cheapest).
func TestConjunctionBulkScorer_Cost(t *testing.T) {
	// clause1 has cost 2, clause2 has cost 5
	sc1 := newConstantScorer([]int{0, 1}, 1, 2)
	sc2 := newConstantScorer([]int{0, 1, 2, 3, 4}, 1, 5)
	bs := newConjBulkScorer(t, []search.Scorer{sc1, sc2}, nil)
	// Lead1 should be the cheaper one (cost=2).
	if bs.Cost() > 2 {
		t.Errorf("Cost()=%d, want ≤2 (cheapest clause)", bs.Cost())
	}
}