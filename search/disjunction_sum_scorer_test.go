// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer exists for DisjunctionSumScorer or DisjunctionScorer
// directly (Lucene 10.4.0 tests them indirectly via BooleanQuery).
// These tests cover constructor panics, score summation, GetMaxScore,
// exhaustion, and DisiWrapper / DisiPriorityQueue contracts.

package search_test

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// dssFixedScorer is a Scorer over a fixed slice of (docID, score) pairs.
type dssFixedScorer struct {
	docs     []int
	scores   []float32
	idx      int
	maxScore float32
}

func newDssFixedScorer(docs []int, scores []float32) *dssFixedScorer {
	var mx float32
	for _, s := range scores {
		if s > mx {
			mx = s
		}
	}
	return &dssFixedScorer{docs: docs, scores: scores, idx: -1, maxScore: mx}
}

func (s *dssFixedScorer) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}

func (s *dssFixedScorer) NextDoc() (int, error) {
	s.idx++
	return s.DocID(), nil
}

func (s *dssFixedScorer) Advance(target int) (int, error) {
	if s.idx < 0 {
		s.idx = 0
	}
	for s.idx < len(s.docs) && s.docs[s.idx] < target {
		s.idx++
	}
	return s.DocID(), nil
}

func (s *dssFixedScorer) Cost() int64 { return int64(len(s.docs)) }

func (s *dssFixedScorer) DocIDRunEnd() (int, error) {
	doc := s.DocID()
	if doc == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}
	return doc + 1, nil
}

func (s *dssFixedScorer) Score() (float32, error) {
	if s.idx < 0 || s.idx >= len(s.scores) {
		return 0, nil
	}
	return s.scores[s.idx], nil
}

func (s *dssFixedScorer) GetMaxScore(_ int) (float32, error) { return s.maxScore, nil }

func (s *dssFixedScorer) AdvanceShallow(int) (int, error) { return search.NO_MORE_DOCS, nil }

// ─── DisjunctionSumScorer tests ───────────────────────────────────────────

func TestDisjunctionSumScorer_PanicsOnSingleScorer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic with < 2 subScorers")
		}
	}()
	s1 := newDssFixedScorer([]int{0}, []float32{1.0})
	search.NewDisjunctionSumScorer([]search.Scorer{s1}, search.COMPLETE, 100)
}

func TestDisjunctionSumScorer_ScoreSumsMatches(t *testing.T) {
	// Two scorers both match doc 5 with different scores.
	s1 := newDssFixedScorer([]int{3, 5}, []float32{1.0, 2.0})
	s2 := newDssFixedScorer([]int{5, 7}, []float32{0.5, 1.5})
	scorer := search.NewDisjunctionSumScorer(
		[]search.Scorer{s1, s2},
		search.COMPLETE,
		100,
	)

	// Advance to doc 5 — both sub-scorers match.
	doc, err := scorer.Advance(5)
	if err != nil {
		t.Fatalf("Advance(5) error: %v", err)
	}
	if doc != 5 {
		t.Fatalf("expected doc 5, got %d", doc)
	}
	got, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	// s1 contributes 2.0, s2 contributes 0.5 → sum = 2.5
	want := float32(2.5)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("Score() = %v, want %v", got, want)
	}
}

func TestDisjunctionSumScorer_ScoreSingleMatchPerDoc(t *testing.T) {
	// Non-overlapping doc ranges: s1 matches {1,2}, s2 matches {10,11}.
	s1 := newDssFixedScorer([]int{1, 2}, []float32{3.0, 4.0})
	s2 := newDssFixedScorer([]int{10, 11}, []float32{5.0, 6.0})
	scorer := search.NewDisjunctionSumScorer(
		[]search.Scorer{s1, s2},
		search.COMPLETE,
		100,
	)

	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc() error: %v", err)
	}
	if doc != 1 {
		t.Fatalf("expected doc 1, got %d", doc)
	}
	if got, err := scorer.Score(); err != nil || math.Abs(float64(got-3.0)) > 1e-6 {
		t.Errorf("Score() = %v, want 3.0 (err: %v)", got, err)
	}
}

func TestDisjunctionSumScorer_Exhaustion(t *testing.T) {
	s1 := newDssFixedScorer([]int{1}, []float32{1.0})
	s2 := newDssFixedScorer([]int{2}, []float32{1.0})
	scorer := search.NewDisjunctionSumScorer(
		[]search.Scorer{s1, s2},
		search.COMPLETE_NO_SCORES,
		100,
	)

	var docs []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc() error: %v", err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		docs = append(docs, doc)
	}
	if len(docs) != 2 || docs[0] != 1 || docs[1] != 2 {
		t.Errorf("expected docs [1 2], got %v", docs)
	}
}

func TestDisjunctionSumScorer_GetMaxScore(t *testing.T) {
	s1 := newDssFixedScorer([]int{1, 2}, []float32{3.0, 4.0})
	s2 := newDssFixedScorer([]int{2, 3}, []float32{2.0, 1.0})
	scorer := search.NewDisjunctionSumScorer(
		[]search.Scorer{s1, s2},
		search.COMPLETE,
		100,
	)
	// Before any iteration: both scorers have docID -1 which is ≤ NO_MORE_DOCS.
	max, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatalf("scorer.GetMaxScore: %v", err)
	}
	if max <= 0 {
		t.Errorf("GetMaxScore() = %v, expected > 0", max)
	}
}

func TestDisjunctionSumScorer_InitialDocID(t *testing.T) {
	s1 := newDssFixedScorer([]int{5}, []float32{1.0})
	s2 := newDssFixedScorer([]int{6}, []float32{1.0})
	scorer := search.NewDisjunctionSumScorer(
		[]search.Scorer{s1, s2},
		search.COMPLETE_NO_SCORES,
		100,
	)
	if got := scorer.DocID(); got != -1 {
		t.Errorf("initial DocID() = %d, want -1", got)
	}

	// ─── DisiPriorityQueue tests ──────────────────────────────────────────────

}
func TestDisiPriorityQueue_AddTopPop(t *testing.T) {
	s1 := newDssFixedScorer([]int{10}, []float32{1.0})
	s2 := newDssFixedScorer([]int{5}, []float32{1.0})
	s3 := newDssFixedScorer([]int{8}, []float32{1.0})

	w1 := search.NewDisiWrapper(s1, false)
	w2 := search.NewDisiWrapper(s2, false)
	w3 := search.NewDisiWrapper(s3, false)
	// Manually set doc so the PQ can order them.
	w1.SetDoc(10)
	w2.SetDoc(5)
	w3.SetDoc(8)

	pq := search.OfMaxSize(3)
	pq.Add(w1)
	pq.Add(w2)
	pq.Add(w3)

	if top := pq.Top(); top.Doc() != 5 {
		t.Errorf("Top().doc = %d, want 5", top.Doc())
	}
	popped := pq.Pop()
	if popped.Doc() != 5 {
		t.Errorf("Pop() doc = %d, want 5", popped.Doc())
	}
	if top := pq.Top(); top.Doc() != 8 {
		t.Errorf("Top().doc = %d, want 8", top.Doc())
	}
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *dssFixedScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// GetChildren carries the default body Lucene gives Scorer.GetChildren.
func (s *dssFixedScorer) GetChildren() ([]search.ChildScorable, error) {
	return nil, nil
}

// Iterator returns the double itself: it iterates its own documents,
// as the scorer.iterator() of the Lucene test scorers does.
func (s *dssFixedScorer) Iterator() search.DocIdSetIterator {
	return s
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (s *dssFixedScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body Lucene gives Scorer.SetMinCompetitiveScore.
func (s *dssFixedScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// SmoothingScore carries the default body Lucene gives Scorer.SmoothingScore.
func (s *dssFixedScorer) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// TwoPhaseIterator carries the default body Lucene gives Scorer.TwoPhaseIterator.
func (s *dssFixedScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return search.DefaultTwoPhaseIterator()
}
