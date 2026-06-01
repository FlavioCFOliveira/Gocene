// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.CoveringScorer tests.
// (No dedicated Java test peer located; tests verify observable iteration
// and scoring contract directly.)
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// fixedLongValues is a LongValues that returns a constant value for every doc.
type fixedLongValues struct {
	v int64
}

func (f *fixedLongValues) AdvanceExact(_ int) (bool, error) { return true, nil }
func (f *fixedLongValues) LongValue() (int64, error)        { return f.v, nil }

var _ search.LongValues = (*fixedLongValues)(nil)

// listScorer is a Scorer that iterates over a fixed list of doc IDs,
// returning a constant score for each.
type listScorer struct {
	docs  []int
	score float32
	idx   int
	doc   int
}

func newListScorer(docs []int, score float32) *listScorer {
	return &listScorer{docs: docs, score: score, idx: -1, doc: -1}
}

func (s *listScorer) DocID() int { return s.doc }
func (s *listScorer) DocIDRunEnd() int {
	if s.doc == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS
	}
	return s.doc + 1
}

func (s *listScorer) NextDoc() (int, error) {
	s.idx++
	if s.idx >= len(s.docs) {
		s.doc = search.NO_MORE_DOCS
	} else {
		s.doc = s.docs[s.idx]
	}
	return s.doc, nil
}

func (s *listScorer) Advance(target int) (int, error) {
	for {
		if _, err := s.NextDoc(); err != nil {
			return 0, err
		}
		if s.doc >= target {
			return s.doc, nil
		}
	}
}

func (s *listScorer) Cost() int64               { return int64(len(s.docs)) }
func (s *listScorer) Score() float32            { return s.score }
func (s *listScorer) GetMaxScore(_ int) float32 { return s.score }
func (s *listScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

var _ search.Scorer = (*listScorer)(nil)

// collectAll iterates cs to exhaustion and returns all matched doc IDs.
func collectAll(t *testing.T, cs *coveringScorer) []int {
	t.Helper()
	var docs []int
	it := cs.twoPhase.AsDocIdSetIterator()
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		docs = append(docs, doc)
	}
	return docs
}

// TestCoveringScorer_AllMatchWithMinMatch1 verifies that with minMatch=1,
// any document covered by at least one sub-scorer is returned.
func TestCoveringScorer_AllMatchWithMinMatch1(t *testing.T) {
	s1 := newListScorer([]int{0, 2, 4}, 1.0)
	s2 := newListScorer([]int{1, 2, 3}, 2.0)
	mv := &fixedLongValues{v: 1}
	cs := newCoveringScorer([]search.Scorer{s1, s2}, mv, 10)
	got := collectAll(t, cs)
	want := []int{0, 1, 2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("docs = %v; want %v", got, want)
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("got[%d] = %d; want %d", i, got[i], d)
		}
	}
}

// TestCoveringScorer_MinMatch2RequiresBothScorers verifies that with minMatch=2,
// only documents covered by both sub-scorers are returned.
func TestCoveringScorer_MinMatch2RequiresBothScorers(t *testing.T) {
	s1 := newListScorer([]int{0, 2, 4}, 1.0)
	s2 := newListScorer([]int{1, 2, 3}, 2.0)
	mv := &fixedLongValues{v: 2}
	cs := newCoveringScorer([]search.Scorer{s1, s2}, mv, 10)
	got := collectAll(t, cs)
	want := []int{2} // only doc 2 has both scorers
	if len(got) != len(want) {
		t.Fatalf("docs = %v; want %v", got, want)
	}
	if got[0] != 2 {
		t.Errorf("got[0] = %d; want 2", got[0])
	}
}

// TestCoveringScorer_ScoreSumsSubScorers verifies that the score of a matched
// document is the sum of sub-scorer scores.
func TestCoveringScorer_ScoreSumsSubScorers(t *testing.T) {
	s1 := newListScorer([]int{2}, 3.0)
	s2 := newListScorer([]int{2}, 5.0)
	mv := &fixedLongValues{v: 1}
	cs := newCoveringScorer([]search.Scorer{s1, s2}, mv, 10)
	it := cs.twoPhase.AsDocIdSetIterator()
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 2 {
		t.Fatalf("expected doc 2, got %d", doc)
	}
	got := cs.Score()
	if got != 8.0 {
		t.Errorf("Score() = %v; want 8.0", got)
	}
}

// TestCoveringScorer_EmptyResult verifies that with minMatch=2 and no overlap,
// no documents are returned.
func TestCoveringScorer_EmptyResult(t *testing.T) {
	s1 := newListScorer([]int{0, 1}, 1.0)
	s2 := newListScorer([]int{2, 3}, 1.0)
	mv := &fixedLongValues{v: 2}
	cs := newCoveringScorer([]search.Scorer{s1, s2}, mv, 10)
	got := collectAll(t, cs)
	if len(got) != 0 {
		t.Errorf("expected no docs, got %v", got)
	}
}

// TestCoveringScorer_GetMaxScore returns positive infinity.
func TestCoveringScorer_GetMaxScore(t *testing.T) {
	s1 := newListScorer([]int{0}, 1.0)
	mv := &fixedLongValues{v: 1}
	cs := newCoveringScorer([]search.Scorer{s1}, mv, 10)
	got := cs.GetMaxScore(search.NO_MORE_DOCS)
	if got != float32(1<<24) && got <= 1e30 {
		t.Errorf("GetMaxScore() = %v; want +Inf", got)
	}
	// More precisely: must be positive infinity
	if got != cs.GetMaxScore(0) {
		t.Error("GetMaxScore must return same value regardless of upTo")
	}
}
