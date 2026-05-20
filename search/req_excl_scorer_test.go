// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer for ReqExclScorer (TestReqExclScorer does not exist in
// Lucene 10.4.0). These tests cover basic exclusion, TwoPhaseIterator
// propagation, score delegation, and cost delegation.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// reqExclFixedScorer is a minimal Scorer over a fixed doc set with a
// fixed score. No two-phase support.
type reqExclFixedScorer struct {
	docs  []int
	score float32
	idx   int
}

func newREFixedScorer(score float32, docs ...int) *reqExclFixedScorer {
	return &reqExclFixedScorer{docs: docs, score: score, idx: -1}
}

func (s *reqExclFixedScorer) Score() float32            { return s.score }
func (s *reqExclFixedScorer) GetMaxScore(_ int) float32 { return s.score }
func (s *reqExclFixedScorer) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}
func (s *reqExclFixedScorer) NextDoc() (int, error) {
	s.idx++
	return s.DocID(), nil
}
func (s *reqExclFixedScorer) Advance(target int) (int, error) {
	if s.idx < 0 {
		s.idx = 0
	}
	for s.idx < len(s.docs) && s.docs[s.idx] < target {
		s.idx++
	}
	return s.DocID(), nil
}
func (s *reqExclFixedScorer) Cost() int64      { return int64(len(s.docs)) }
func (s *reqExclFixedScorer) DocIDRunEnd() int { return s.DocID() + 1 }

var _ search.Scorer = (*reqExclFixedScorer)(nil)

// collectAll drains a Scorer and returns all matching doc IDs.
func reqExclCollectAll(t *testing.T, sc search.Scorer) []int {
	t.Helper()
	var docs []int
	doc, err := sc.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	for doc != search.NO_MORE_DOCS {
		docs = append(docs, doc)
		doc, err = sc.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
	}
	return docs
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestReqExclScorer_BasicExclusion(t *testing.T) {
	// req: 1,3,5,7; excl: 3,5. Result: 1, 7.
	req := newREFixedScorer(2.0, 1, 3, 5, 7)
	excl := newREFixedScorer(0.0, 3, 5)
	s := search.NewReqExclScorer(req, excl)
	got := reqExclCollectAll(t, s)
	want := []int{1, 7}
	if len(got) != len(want) {
		t.Fatalf("docs = %v, want %v", got, want)
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("docs[%d] = %d, want %d", i, got[i], d)
		}
	}
}

func TestReqExclScorer_NoExclusion(t *testing.T) {
	// excl matches nothing in req's set.
	req := newREFixedScorer(1.0, 2, 4, 6)
	excl := newREFixedScorer(0.0, 1, 3, 5)
	s := search.NewReqExclScorer(req, excl)
	got := reqExclCollectAll(t, s)
	want := []int{2, 4, 6}
	if len(got) != len(want) {
		t.Fatalf("docs = %v, want %v", got, want)
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("docs[%d] = %d, want %d", i, got[i], d)
		}
	}
}

func TestReqExclScorer_AllExcluded(t *testing.T) {
	req := newREFixedScorer(1.0, 1, 2, 3)
	excl := newREFixedScorer(0.0, 1, 2, 3)
	s := search.NewReqExclScorer(req, excl)
	got := reqExclCollectAll(t, s)
	if len(got) != 0 {
		t.Errorf("docs = %v, want []", got)
	}
}

func TestReqExclScorer_ScoreDelegation(t *testing.T) {
	req := newREFixedScorer(3.5, 1, 2, 3)
	excl := newREFixedScorer(0.0, 2)
	s := search.NewReqExclScorer(req, excl)
	doc, err := s.NextDoc()
	if err != nil || doc != 1 {
		t.Fatalf("NextDoc() = (%d, %v), want (1, nil)", doc, err)
	}
	if s.Score() != 3.5 {
		t.Errorf("Score() = %v, want 3.5", s.Score())
	}
}

func TestReqExclScorer_GetMaxScoreDelegation(t *testing.T) {
	req := newREFixedScorer(5.0, 1, 2)
	excl := newREFixedScorer(0.0, 999)
	s := search.NewReqExclScorer(req, excl)
	if s.GetMaxScore(100) != 5.0 {
		t.Errorf("GetMaxScore() = %v, want 5.0", s.GetMaxScore(100))
	}
}

func TestReqExclScorer_CostIsReqCost(t *testing.T) {
	req := newREFixedScorer(1.0, 1, 2, 3, 4, 5) // cost 5
	excl := newREFixedScorer(0.0, 1, 2)         // cost 2
	s := search.NewReqExclScorer(req, excl)
	// Cost comes from tpDisi which is backed by reqApproximation (cost 5).
	if s.Cost() != 5 {
		t.Errorf("Cost() = %d, want 5", s.Cost())
	}
}

func TestReqExclScorer_Advance(t *testing.T) {
	req := newREFixedScorer(1.0, 1, 3, 5, 7, 9)
	excl := newREFixedScorer(0.0, 3, 7)
	s := search.NewReqExclScorer(req, excl)

	doc, err := s.Advance(4)
	if err != nil {
		t.Fatalf("Advance(4) error: %v", err)
	}
	// First non-excluded doc ≥ 4 is 5.
	if doc != 5 {
		t.Errorf("Advance(4) = %d, want 5", doc)
	}
}

func TestReqExclScorer_ImplementsScorer(t *testing.T) {
	var _ search.Scorer = search.NewReqExclScorer(
		newREFixedScorer(1.0, 1),
		newREFixedScorer(0.0, 2),
	)
}
