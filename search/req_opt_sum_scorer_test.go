// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   core/src/test/org/apache/lucene/search/TestReqOptSumScorer.java
//
// The Java test peers (testBasicsMust, testBasicsFilter, testMaxBlock,
// testMaxScoreSegment, testMustRandom*, testFilterRandom*) require
// RandomIndexWriter, CheckHits, BooleanQuery, TermQuery, and a full
// index stack — none of which are wired in Gocene yet.
// All Java test peers are ported as degraded stubs (t.Skip) below.
//
// The concrete unit tests verify the observable contract:
//   - DocID() tracks the required scorer
//   - Score() = req + opt when opt matches the same doc
//   - Score() = req when opt does not match
//   - GetMaxScore combines both scorers
//   - Exhaustion follows the required scorer

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── Java test peer stubs ─────────────────────────────────────────────────

func TestReqOptSumScorer_BasicsMust(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, BooleanQuery, TermQuery — not yet wired in Gocene")
}

func TestReqOptSumScorer_BasicsFilter(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, BooleanQuery, TermQuery — not yet wired in Gocene")
}

func TestReqOptSumScorer_MaxBlock(t *testing.T) {
	t.Fatal("requires IndexWriter, TermQuery, advanceShallow — not yet wired in Gocene")
}

func TestReqOptSumScorer_MaxScoreSegment(t *testing.T) {
	t.Fatal("requires IndexWriter, ConstantScoreQuery, TermQuery — not yet wired in Gocene")
}

func TestReqOptSumScorer_MustRandomFrequentOpt(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, CheckHits — not yet wired in Gocene")
}

func TestReqOptSumScorer_MustRandomRareOpt(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, CheckHits — not yet wired in Gocene")
}

func TestReqOptSumScorer_FilterRandomFrequentOpt(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, CheckHits — not yet wired in Gocene")
}

func TestReqOptSumScorer_FilterRandomRareOpt(t *testing.T) {
	t.Fatal("requires RandomIndexWriter, CheckHits — not yet wired in Gocene")
}

// ─── Unit tests ──────────────────────────────────────────────────────────

// rosFixedScorer is a Scorer over a fixed slice of (docID, score) pairs,
// shared between req and opt scorer helpers.
type rosFixedScorer struct {
	docs     []int
	scores   []float32
	idx      int
	maxScore float32
}

func newROSFixedScorer(docs []int, scores []float32) *rosFixedScorer {
	var mx float32
	for _, s := range scores {
		if s > mx {
			mx = s
		}
	}
	return &rosFixedScorer{docs: docs, scores: scores, idx: -1, maxScore: mx}
}

func (s *rosFixedScorer) DocID() int {
	if s.idx < 0 {
		return -1
	}
	if s.idx >= len(s.docs) {
		return search.NO_MORE_DOCS
	}
	return s.docs[s.idx]
}

func (s *rosFixedScorer) NextDoc() (int, error) {
	s.idx++
	return s.DocID(), nil
}

func (s *rosFixedScorer) Advance(target int) (int, error) {
	if s.idx < 0 {
		s.idx = 0
	}
	for s.idx < len(s.docs) && s.docs[s.idx] < target {
		s.idx++
	}
	return s.DocID(), nil
}

func (s *rosFixedScorer) Cost() int64 { return int64(len(s.docs)) }
func (s *rosFixedScorer) DocIDRunEnd() int {
	doc := s.DocID()
	if doc == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS
	}
	return doc + 1
}
func (s *rosFixedScorer) Score() float32            { return s.currentScore() }
func (s *rosFixedScorer) GetMaxScore(_ int) float32 { return s.maxScore }
func (s *rosFixedScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

func (s *rosFixedScorer) currentScore() float32 {
	if s.idx < 0 || s.idx >= len(s.scores) {
		return 0
	}
	return s.scores[s.idx]
}

// TestReqOptSumScorer_InitialDocID verifies DocID() starts at -1.
func TestReqOptSumScorer_InitialDocID(t *testing.T) {
	req := newROSFixedScorer([]int{1, 2}, []float32{1.0, 1.0})
	opt := newROSFixedScorer([]int{2}, []float32{0.5})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)
	if got := scorer.DocID(); got != -1 {
		t.Errorf("initial DocID() = %d, want -1", got)
	}
}

// TestReqOptSumScorer_ReqOnlyScoring verifies that when opt does not
// match the current doc, score equals req score only.
func TestReqOptSumScorer_ReqOnlyScoring(t *testing.T) {
	req := newROSFixedScorer([]int{1, 3}, []float32{2.0, 3.0})
	opt := newROSFixedScorer([]int{10}, []float32{5.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	doc, err := scorer.NextDoc()
	if err != nil || doc != 1 {
		t.Fatalf("NextDoc() = (%d, %v), want (1, nil)", doc, err)
	}
	got := scorer.Score()
	if math.Abs(float64(got-2.0)) > 1e-6 {
		t.Errorf("Score() = %v, want 2.0 (req only)", got)
	}
}

// TestReqOptSumScorer_SumWhenBothMatch verifies req+opt when both match.
func TestReqOptSumScorer_SumWhenBothMatch(t *testing.T) {
	req := newROSFixedScorer([]int{5}, []float32{3.0})
	opt := newROSFixedScorer([]int{5}, []float32{1.5})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	doc, err := scorer.NextDoc()
	if err != nil || doc != 5 {
		t.Fatalf("NextDoc() = (%d, %v), want (5, nil)", doc, err)
	}
	got := scorer.Score()
	want := float32(4.5)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("Score() = %v, want %v (req + opt)", got, want)
	}
}

// TestReqOptSumScorer_IterationFollowsReq verifies only required docs are visited.
func TestReqOptSumScorer_IterationFollowsReq(t *testing.T) {
	req := newROSFixedScorer([]int{2, 4, 6}, []float32{1.0, 1.0, 1.0})
	opt := newROSFixedScorer([]int{1, 3, 5}, []float32{1.0, 1.0, 1.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

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
	if len(docs) != 3 || docs[0] != 2 || docs[1] != 4 || docs[2] != 6 {
		t.Errorf("visited docs = %v, want [2 4 6]", docs)
	}
}

// TestReqOptSumScorer_Advance verifies Advance() delegates to the required scorer.
func TestReqOptSumScorer_Advance(t *testing.T) {
	req := newROSFixedScorer([]int{1, 5, 10}, []float32{1.0, 2.0, 3.0})
	opt := newROSFixedScorer([]int{5, 10}, []float32{0.5, 0.5})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	doc, err := scorer.Advance(5)
	if err != nil || doc != 5 {
		t.Fatalf("Advance(5) = (%d, %v), want (5, nil)", doc, err)
	}
	// Both match doc 5: req=2.0, opt=0.5
	got := scorer.Score()
	want := float32(2.5)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("Score() after Advance(5) = %v, want %v", got, want)
	}
}

// TestReqOptSumScorer_GetMaxScore verifies the max-score combination.
func TestReqOptSumScorer_GetMaxScore(t *testing.T) {
	req := newROSFixedScorer([]int{1}, []float32{3.0})
	opt := newROSFixedScorer([]int{1}, []float32{2.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	max := scorer.GetMaxScore(search.NO_MORE_DOCS)
	// Both scorers start at docID -1 which is ≤ NO_MORE_DOCS; expect 5.0.
	if math.Abs(float64(max-5.0)) > 1e-6 {
		t.Errorf("GetMaxScore() = %v, want 5.0", max)
	}
}
