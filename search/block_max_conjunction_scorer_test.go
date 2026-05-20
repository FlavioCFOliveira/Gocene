// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBlockMaxConjunctionScorer.java
//   No Java test peer exists — synthetic Go tests covering the contract.

package search

import "testing"

// TestBlockMaxConjunctionScorer_SortsByCost verifies that the lead scorer is cheapest.
func TestBlockMaxConjunctionScorer_SortsByCost(t *testing.T) {
	cheap := newFakeScorer([]int{1, 2, 3}, 0.5)
	expensive := newFakeScorer([]int{1, 2, 3}, 1.5)
	expensive.cost = 100
	cheap.cost = 1
	s := NewBlockMaxConjunctionScorer([]Scorer{expensive, cheap})
	if s.scorers[0].Cost() != 1 {
		t.Fatalf("expected cheapest scorer first; got cost %d", s.scorers[0].Cost())
	}
}

// TestBlockMaxConjunctionScorer_ScoreSumsAllClauses verifies score accumulation.
func TestBlockMaxConjunctionScorer_ScoreSumsAllClauses(t *testing.T) {
	a := newFakeScorer([]int{1, 2}, 1.0)
	b := newFakeScorer([]int{1, 2}, 2.0)
	s := NewBlockMaxConjunctionScorer([]Scorer{a, b})
	doc, _ := s.NextDoc()
	if doc != 1 {
		t.Fatalf("expected doc 1, got %d", doc)
	}
	if got := s.Score(); got != 3.0 {
		t.Fatalf("expected score 3.0, got %v", got)
	}
}

// TestBlockMaxConjunctionScorer_Intersection verifies conjunction semantics.
func TestBlockMaxConjunctionScorer_Intersection(t *testing.T) {
	a := newFakeScorer([]int{1, 3, 5}, 1.0)
	b := newFakeScorer([]int{2, 3, 4}, 1.0)
	s := NewBlockMaxConjunctionScorer([]Scorer{a, b})
	doc, _ := s.NextDoc()
	if doc != 3 {
		t.Fatalf("expected first match at doc 3, got %d", doc)
	}
	doc, _ = s.NextDoc()
	if doc != NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
	}
}

// TestBlockMaxConjunctionScorer_GetMaxScore verifies sum of clause max scores.
func TestBlockMaxConjunctionScorer_GetMaxScore(t *testing.T) {
	a := newFakeScorer([]int{1, 2}, 3.0)
	b := newFakeScorer([]int{1, 2}, 4.0)
	s := NewBlockMaxConjunctionScorer([]Scorer{a, b})
	got := s.GetMaxScore(NO_MORE_DOCS)
	if got != 7.0 {
		t.Fatalf("expected 7.0, got %v", got)
	}
}

// TestBlockMaxConjunctionScorer_GetChildren returns all scorers.
func TestBlockMaxConjunctionScorer_GetChildren(t *testing.T) {
	a := newFakeScorer([]int{1}, 1.0)
	b := newFakeScorer([]int{1}, 2.0)
	s := NewBlockMaxConjunctionScorer([]Scorer{a, b})
	children := s.GetChildren()
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
}
