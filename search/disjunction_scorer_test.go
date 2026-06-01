// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionScorer.java
//
// No Java test peer found (class is package-private in Lucene). Tests are
// written as internal package tests (package search) to access the
// unexported newDisjunctionScorer constructor, matching the Java visibility.
// They cover: OR semantics, score aggregation via a concrete shadow of
// scoreTopList, two-clause union, three-clause union, empty result, Cost.

package search

import (
	"testing"
)

// sumScoreDisjunctionScorer is a concrete subclass of DisjunctionScorer for
// tests that sums the scores of all matching sub-scorers.  It mirrors the
// behaviour of DisjunctionSumScorer.
type sumScoreDisjunctionScorer struct {
	*DisjunctionScorer
}

func newSumScoreDisjunctionScorer(subScorers []Scorer) *sumScoreDisjunctionScorer {
	return &sumScoreDisjunctionScorer{
		DisjunctionScorer: newDisjunctionScorer(subScorers, COMPLETE, 0),
	}
}

// Score overrides the default to return the sum of sub-match scores.
func (s *sumScoreDisjunctionScorer) Score() float32 {
	topList, err := s.getSubMatches()
	if err != nil {
		return 0
	}
	var sum float64
	for w := topList; w != nil; w = w.next {
		sum += float64(w.scorer.Score())
	}
	return float32(sum)
}

// GetMaxScore returns +Inf (no max for tests).
func (s *sumScoreDisjunctionScorer) GetMaxScore(_ int) float32 { return maxFloat32 }

func (s *sumScoreDisjunctionScorer) AdvanceShallow(int) (int, error) { return NO_MORE_DOCS, nil }

// collectAllDisj iterates the scorer and returns (docs, scores).
func collectAllDisj(t *testing.T, sc *sumScoreDisjunctionScorer) ([]int, []float32) {
	t.Helper()
	var docs []int
	var scores []float32
	for {
		d, err := sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			break
		}
		docs = append(docs, d)
		scores = append(scores, sc.Score())
	}
	return docs, scores
}

// fakeScorer is a simple Scorer for disjunction tests (internal package use).
type fakeScorer struct {
	BaseDocIdSetIterator
	docs  []int
	pos   int
	score float32
	cost  int64
}

func newFakeScorer(docs []int, score float32) *fakeScorer {
	return &fakeScorer{docs: docs, pos: -1, score: score, cost: int64(len(docs))}
}

func (s *fakeScorer) DocID() int {
	if s.pos < 0 {
		return -1
	}
	if s.pos >= len(s.docs) {
		return NO_MORE_DOCS
	}
	return s.docs[s.pos]
}

func (s *fakeScorer) NextDoc() (int, error) {
	s.pos++
	return s.DocID(), nil
}

func (s *fakeScorer) Advance(target int) (int, error) {
	for s.pos++; s.pos < len(s.docs) && s.docs[s.pos] < target; s.pos++ {
	}
	return s.DocID(), nil
}

func (s *fakeScorer) Cost() int64               { return s.cost }
func (s *fakeScorer) Score() float32            { return s.score }
func (s *fakeScorer) GetMaxScore(_ int) float32 { return s.score }
func (s *fakeScorer) AdvanceShallow(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestDisjunctionScorer_TooFewSubScorers verifies that < 2 sub-scorers panics.
func TestDisjunctionScorer_TooFewSubScorers(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for 1 sub-scorer, got none")
		}
	}()
	sc := newFakeScorer([]int{0}, 1)
	newDisjunctionScorer([]Scorer{sc}, COMPLETE, 0)
}

// TestDisjunctionScorer_Union verifies OR semantics for two disjoint sets.
func TestDisjunctionScorer_Union(t *testing.T) {
	// A: 0,2,4  B: 1,3,5 → union: 0,1,2,3,4,5
	sc1 := newFakeScorer([]int{0, 2, 4}, 1.0)
	sc2 := newFakeScorer([]int{1, 3, 5}, 1.0)
	ds := newSumScoreDisjunctionScorer([]Scorer{sc1, sc2})
	docs, _ := collectAllDisj(t, ds)
	want := []int{0, 1, 2, 3, 4, 5}
	if len(docs) != len(want) {
		t.Fatalf("docs=%v, want %v", docs, want)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], want[i])
		}
	}
}

// TestDisjunctionScorer_OverlappingUnion verifies OR with overlap.
func TestDisjunctionScorer_OverlappingUnion(t *testing.T) {
	// A: 0,2,4  B: 2,4,6 → union: 0,2,4,6
	sc1 := newFakeScorer([]int{0, 2, 4}, 1.0)
	sc2 := newFakeScorer([]int{2, 4, 6}, 2.0)
	ds := newSumScoreDisjunctionScorer([]Scorer{sc1, sc2})
	docs, scores := collectAllDisj(t, ds)
	wantDocs := []int{0, 2, 4, 6}
	if len(docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", docs, wantDocs)
	}
	for i := range wantDocs {
		if docs[i] != wantDocs[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], wantDocs[i])
		}
	}
	// doc 0: only sc1 → score 1.0
	if scores[0] != 1.0 {
		t.Errorf("scores[0]=%v, want 1.0", scores[0])
	}
	// doc 2: both → score 1+2=3.0
	if scores[1] != 3.0 {
		t.Errorf("scores[1]=%v, want 3.0 (both match)", scores[1])
	}
	// doc 6: only sc2 → score 2.0
	if scores[3] != 2.0 {
		t.Errorf("scores[3]=%v, want 2.0", scores[3])
	}
}

// TestDisjunctionScorer_ThreeClauses verifies three-way union.
func TestDisjunctionScorer_ThreeClauses(t *testing.T) {
	sc1 := newFakeScorer([]int{0, 3}, 1.0)
	sc2 := newFakeScorer([]int{1, 3}, 1.0)
	sc3 := newFakeScorer([]int{2, 3}, 1.0)
	ds := newSumScoreDisjunctionScorer([]Scorer{sc1, sc2, sc3})
	docs, scores := collectAllDisj(t, ds)
	wantDocs := []int{0, 1, 2, 3}
	if len(docs) != len(wantDocs) {
		t.Fatalf("docs=%v, want %v", docs, wantDocs)
	}
	// doc 3: all three match → score 3.0
	if scores[3] != 3.0 {
		t.Errorf("scores[3]=%v, want 3.0 (three matches)", scores[3])
	}
}

// TestDisjunctionScorer_EmptyResult verifies all-empty sub-scorers.
func TestDisjunctionScorer_EmptyResult(t *testing.T) {
	sc1 := newFakeScorer(nil, 1.0)
	sc2 := newFakeScorer(nil, 1.0)
	ds := newSumScoreDisjunctionScorer([]Scorer{sc1, sc2})
	docs, _ := collectAllDisj(t, ds)
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %v", docs)
	}
}

// TestDisjunctionScorer_Cost verifies Cost() approximates union size.
func TestDisjunctionScorer_Cost(t *testing.T) {
	sc1 := newFakeScorer([]int{0, 1, 2}, 1.0)
	sc2 := newFakeScorer([]int{3, 4}, 1.0)
	ds := newSumScoreDisjunctionScorer([]Scorer{sc1, sc2})
	cost := ds.Cost()
	if cost < 2 {
		t.Errorf("Cost()=%d, want ≥2", cost)
	}
}

// TestDisjunctionScorer_TwoPhaseNil verifies TwoPhaseIterator() is nil when
// no sub-scorer uses two-phase iteration.
func TestDisjunctionScorer_TwoPhaseNil(t *testing.T) {
	sc1 := newFakeScorer([]int{0}, 1.0)
	sc2 := newFakeScorer([]int{0}, 1.0)
	ds := newDisjunctionScorer([]Scorer{sc1, sc2}, COMPLETE, 0)
	if tpi := ds.TwoPhaseIterator(); tpi != nil {
		t.Errorf("TwoPhaseIterator() = %v, want nil for plain scorers", tpi)
	}
}

// TestDisjunctionScorer_ImplementsScorer is a compile-time interface check.
func TestDisjunctionScorer_ImplementsScorer(t *testing.T) {
	sc1 := newFakeScorer([]int{0}, 1.0)
	sc2 := newFakeScorer([]int{0}, 1.0)
	var _ Scorer = newDisjunctionScorer([]Scorer{sc1, sc2}, COMPLETE, 0)
}
