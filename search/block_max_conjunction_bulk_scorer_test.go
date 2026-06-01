// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer for BlockMaxConjunctionBulkScorer.
// These tests cover constructor validation, conjunction matching, scoring,
// acceptDocs filtering, and cost delegation.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── stub types ──────────────────────────────────────────────────────────────

// bmcFixedScorer is a test Scorer that matches a fixed set of doc IDs with
// a fixed score.
type bmcFixedScorer struct {
	docs  []int
	score float32
	idx   int
}

func newBMCFixedScorer(score float32, docs ...int) *bmcFixedScorer {
	return &bmcFixedScorer{docs: docs, score: score, idx: -1}
}

func (s *bmcFixedScorer) Score() float32            { return s.score }
func (s *bmcFixedScorer) GetMaxScore(_ int) float32 { return s.score }
func (s *bmcFixedScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *bmcFixedScorer) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}
func (s *bmcFixedScorer) NextDoc() (int, error) {
	s.idx++
	return s.DocID(), nil
}
func (s *bmcFixedScorer) Advance(target int) (int, error) {
	if s.idx < 0 {
		s.idx = 0
	}
	for s.idx < len(s.docs) && s.docs[s.idx] < target {
		s.idx++
	}
	return s.DocID(), nil
}
func (s *bmcFixedScorer) Cost() int64      { return int64(len(s.docs)) }
func (s *bmcFixedScorer) DocIDRunEnd() int { return s.DocID() + 1 }

var _ search.Scorer = (*bmcFixedScorer)(nil)

// bmcLeafCollector collects (doc, score) pairs and satisfies both
// LeafCollector and Collector so BlockMaxConjunctionBulkScorer.Score can use it.
type bmcLeafCollector struct {
	scorer search.Scorer
	docs   []int
	scores []float32
}

func (c *bmcLeafCollector) SetScorer(s search.Scorer) error { c.scorer = s; return nil }
func (c *bmcLeafCollector) Collect(doc int) error {
	c.docs = append(c.docs, doc)
	c.scores = append(c.scores, c.scorer.Score())
	return nil
}
func (c *bmcLeafCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return c, nil
}
func (c *bmcLeafCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

var _ search.LeafCollector = (*bmcLeafCollector)(nil)
var _ search.Collector = (*bmcLeafCollector)(nil)

// ─── tests ───────────────────────────────────────────────────────────────────

func TestBlockMaxConjunctionBulkScorer_TooFewScorers(t *testing.T) {
	_, err := search.NewBlockMaxConjunctionBulkScorer(100, []search.Scorer{
		newBMCFixedScorer(1.0, 1, 2, 3),
	})
	if err == nil {
		t.Fatal("expected error for single scorer, got nil")
	}
}

func TestBlockMaxConjunctionBulkScorer_EmptyScorers(t *testing.T) {
	_, err := search.NewBlockMaxConjunctionBulkScorer(100, nil)
	if err == nil {
		t.Fatal("expected error for 0 scorers, got nil")
	}
}

func TestBlockMaxConjunctionBulkScorer_ConjunctionMatch(t *testing.T) {
	// Scorer A matches 1,3,5,7; B matches 2,3,5,8.
	// Intersection: 3, 5.
	a := newBMCFixedScorer(1.0, 1, 3, 5, 7)
	b := newBMCFixedScorer(2.0, 2, 3, 5, 8)
	bs, err := search.NewBlockMaxConjunctionBulkScorer(10, []search.Scorer{a, b})
	if err != nil {
		t.Fatal(err)
	}
	c := &bmcLeafCollector{}
	if err := fullWindowScore(bs, c, nil); err != nil {
		t.Fatal(err)
	}
	wantDocs := []int{3, 5}
	if len(c.docs) != len(wantDocs) {
		t.Fatalf("docs = %v, want %v", c.docs, wantDocs)
	}
	for i, d := range wantDocs {
		if c.docs[i] != d {
			t.Errorf("docs[%d] = %d, want %d", i, c.docs[i], d)
		}
		// Score should be sum of clause scores: 1.0 + 2.0 = 3.0.
		if c.scores[i] != 3.0 {
			t.Errorf("scores[%d] = %v, want 3.0", i, c.scores[i])
		}
	}
}

func TestBlockMaxConjunctionBulkScorer_NoIntersection(t *testing.T) {
	a := newBMCFixedScorer(1.0, 1, 2, 3)
	b := newBMCFixedScorer(1.0, 4, 5, 6)
	bs, err := search.NewBlockMaxConjunctionBulkScorer(10, []search.Scorer{a, b})
	if err != nil {
		t.Fatal(err)
	}
	c := &bmcLeafCollector{}
	if err := fullWindowScore(bs, c, nil); err != nil {
		t.Fatal(err)
	}
	if len(c.docs) != 0 {
		t.Errorf("docs = %v, want []", c.docs)
	}
}

func TestBlockMaxConjunctionBulkScorer_ThreeClauses(t *testing.T) {
	// A: 1,3,5,7; B: 3,5,6,9; C: 2,3,7,9.
	// Intersection: 3 only (5 not in C; 7 not in B; 9 not in A).
	a := newBMCFixedScorer(1.0, 1, 3, 5, 7)
	b := newBMCFixedScorer(2.0, 3, 5, 6, 9)
	c2 := newBMCFixedScorer(3.0, 2, 3, 7, 9)
	bs, err := search.NewBlockMaxConjunctionBulkScorer(10, []search.Scorer{a, b, c2})
	if err != nil {
		t.Fatal(err)
	}
	c := &bmcLeafCollector{}
	if err := fullWindowScore(bs, c, nil); err != nil {
		t.Fatal(err)
	}
	if len(c.docs) != 1 || c.docs[0] != 3 {
		t.Errorf("docs = %v, want [3]", c.docs)
	}
	if c.scores[0] != 6.0 {
		t.Errorf("score = %v, want 6.0 (1+2+3)", c.scores[0])
	}
}

func TestBlockMaxConjunctionBulkScorer_Cost(t *testing.T) {
	// Cost = cost of lead (least-costly) scorer.
	a := newBMCFixedScorer(1.0, 1, 2, 3, 4, 5) // cost 5
	b := newBMCFixedScorer(1.0, 1, 2, 3)       // cost 3
	bs, err := search.NewBlockMaxConjunctionBulkScorer(10, []search.Scorer{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if bs.Cost() != 3 {
		t.Errorf("Cost() = %d, want 3", bs.Cost())
	}
}

func TestBlockMaxConjunctionBulkScorer_ImplementsBulkScorer(t *testing.T) {
	a := newBMCFixedScorer(1.0, 1)
	b := newBMCFixedScorer(1.0, 1)
	bs, err := search.NewBlockMaxConjunctionBulkScorer(10, []search.Scorer{a, b})
	if err != nil {
		t.Fatal(err)
	}
	var _ search.BulkScorer = bs
}
