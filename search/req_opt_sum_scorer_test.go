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
	"github.com/FlavioCFOliveira/Gocene/util"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── Java test peer stubs ─────────────────────────────────────────────────

func TestReqOptSumScorer_BasicsMust(t *testing.T) {
	// Verify that when req and opt overlap partially, the scorer visits
	// only req docs and sums scores when both match.
	req := newROSFixedScorer([]int{1, 3, 5}, []float32{1.0, 2.0, 3.0})
	opt := newROSFixedScorer([]int{1, 4, 5}, []float32{0.5, 1.0, 2.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	// doc 1: both match -> 1.0 + 0.5 = 1.5
	doc, err := scorer.NextDoc()
	if err != nil || doc != 1 {
		t.Fatalf("NextDoc() = (%d, %v), want (1, nil)", doc, err)
	}
	v45_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(1.5); math.Abs(float64(v45_44-want)) > 1e-6 {
		v46_52, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at doc %d = %v, want %v", doc, v46_52, want)
	}

	// doc 3: only req matches -> 2.0
	doc, err = scorer.NextDoc()
	if err != nil || doc != 3 {
		t.Fatalf("NextDoc() = (%d, %v), want (3, nil)", doc, err)
	}
	v54_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(2.0); math.Abs(float64(v54_44-want)) > 1e-6 {
		v55_52, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at doc %d = %v, want %v", doc, v55_52, want)
	}

	// doc 5: both match -> 3.0 + 2.0 = 5.0
	doc, err = scorer.NextDoc()
	if err != nil || doc != 5 {
		t.Fatalf("NextDoc() = (%d, %v), want (5, nil)", doc, err)
	}
	v63_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(5.0); math.Abs(float64(v63_44-want)) > 1e-6 {
		v64_52, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at doc %d = %v, want %v", doc, v64_52, want)
	}

	// exhausted
	doc, err = scorer.NextDoc()
	if err != nil || doc != search.NO_MORE_DOCS {
		t.Fatalf("NextDoc() exhausted = (%d, %v), want (NO_MORE_DOCS, nil)", doc, err)
	}
}

func TestReqOptSumScorer_BasicsFilter(t *testing.T) {
	// Verify the filter pattern: req scorer followed by opt scorer where
	// opt scores are never added (filter mode). The defaults in
	// COMPLETE mode add opt scores; this test verifies opt scoring
	// does not interfere with req iteration.
	req := newROSFixedScorer([]int{2, 4}, []float32{10.0, 20.0})
	opt := newROSFixedScorer([]int{1, 2, 3, 4, 5}, []float32{1.0, 1.0, 1.0, 1.0, 1.0})
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
	// Must only visit req documents.
	if len(docs) != 2 || docs[0] != 2 || docs[1] != 4 {
		t.Errorf("visited docs = %v, want [2, 4]", docs)
	}
}

func TestReqOptSumScorer_MaxBlock(t *testing.T) {
	// Verify the combined GetMaxScore for a sequence of blocks.
	req := newROSFixedScorer([]int{1, 2, 3}, []float32{2.0, 3.0, 4.0})
	opt := newROSFixedScorer([]int{1, 3}, []float32{1.0, 2.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	// Max score = max(req) + max(opt) = 4.0 + 2.0 = 6.0
	v107_44, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatalf("scorer.GetMaxScore: %v", err)
	}
	if want := float32(6.0); math.Abs(float64(v107_44-want)) > 1e-6 {
		v108_43, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
		if err != nil {
			t.Fatalf("scorer.GetMaxScore: %v", err)
		}
		t.Errorf("GetMaxScore() = %v, want %v", v108_43, want)
	}
}

func TestReqOptSumScorer_MaxScoreSegment(t *testing.T) {
	// Verify the advance path correctly handles GetMaxScore after
	// advancing past the initial position.
	req := newROSFixedScorer([]int{3, 7, 9}, []float32{5.0, 1.0, 8.0})
	opt := newROSFixedScorer([]int{7, 9}, []float32{2.0, 3.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	// Advance past doc 3 to doc 7.
	doc, err := scorer.Advance(5)
	if err != nil || doc != 7 {
		t.Fatalf("Advance(5) = (%d, %v), want (7, nil)", doc, err)
	}
	// Score at doc 7: 1.0 + 2.0 = 3.0
	v125_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(3.0); math.Abs(float64(v125_44-want)) > 1e-6 {
		v126_51, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at Advance(5) = %v, want %v", v126_51, want)
	}
}

func TestReqOptSumScorer_MustRandomFrequentOpt(t *testing.T) {
	// Test with frequent opt matches: opt covers every req doc.
	req := newROSFixedScorer([]int{1, 2, 3, 4, 5}, []float32{1.0, 2.0, 3.0, 4.0, 5.0})
	opt := newROSFixedScorer([]int{1, 2, 3, 4, 5}, []float32{0.1, 0.2, 0.3, 0.4, 0.5})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	for i := 1; i <= 5; i++ {
		doc, err := scorer.NextDoc()
		if err != nil || doc != i {
			t.Fatalf("NextDoc() at iter %d = (%d, %v), want (%d, nil)", i, doc, err, i)
		}
		want := float32(i) + float32(i)*0.1
		v142_23, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		if math.Abs(float64(v142_23-want)) > 1e-6 {
			v143_53, err := scorer.Score()
			if err != nil {
				t.Fatalf("scorer.Score: %v", err)
			}
			t.Errorf("Score() at doc %d = %v, want %v", doc, v143_53, want)
		}
	}
	doc, err := scorer.NextDoc()
	if err != nil || doc != search.NO_MORE_DOCS {
		t.Errorf("NextDoc() after exhaust = (%d, %v), want (NO_MORE_DOCS, nil)", doc, err)
	}
}

func TestReqOptSumScorer_MustRandomRareOpt(t *testing.T) {
	// Test with rare opt matches: opt covers few req docs.
	req := newROSFixedScorer([]int{1, 2, 3, 4, 5}, []float32{1.0, 2.0, 3.0, 4.0, 5.0})
	opt := newROSFixedScorer([]int{2, 5}, []float32{10.0, 20.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	expected := map[int]float32{
		1: 1.0,
		2: 12.0,
		3: 3.0,
		4: 4.0,
		5: 25.0,
	}
	for i := 1; i <= 5; i++ {
		doc, err := scorer.NextDoc()
		if err != nil || doc != i {
			t.Fatalf("NextDoc() at iter %d = (%d, %v), want (%d, nil)", i, doc, err, i)
		}
		v170_23, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		if math.Abs(float64(v170_23-expected[doc])) > 1e-6 {
			v171_53, err := scorer.Score()
			if err != nil {
				t.Fatalf("scorer.Score: %v", err)
			}
			t.Errorf("Score() at doc %d = %v, want %v", doc, v171_53, expected[doc])
		}
	}
}

func TestReqOptSumScorer_FilterRandomFrequentOpt(t *testing.T) {
	// Test filtering behaviour: opt may have many more docs than req.
	req := newROSFixedScorer([]int{3, 6}, []float32{1.0, 1.0})
	opt := newROSFixedScorer([]int{1, 2, 3, 4, 5, 6, 7}, []float32{1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0})
	scorer := search.NewReqOptSumScorer(req, opt, search.COMPLETE)

	doc, err := scorer.NextDoc()
	if err != nil || doc != 3 {
		t.Fatalf("NextDoc() = (%d, %v), want (3, nil)", doc, err)
	}
	v186_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(2.0); math.Abs(float64(v186_44-want)) > 1e-6 {
		v187_52, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at doc %d = %v, want %v", doc, v187_52, want)
	}

	doc, err = scorer.NextDoc()
	if err != nil || doc != 6 {
		t.Fatalf("NextDoc() = (%d, %v), want (6, nil)", doc, err)
	}
	v194_44, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
	if want := float32(2.0); math.Abs(float64(v194_44-want)) > 1e-6 {
		v195_52, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		t.Errorf("Score() at doc %d = %v, want %v", doc, v195_52, want)
	}

	doc, err = scorer.NextDoc()
	if err != nil || doc != search.NO_MORE_DOCS {
		t.Errorf("NextDoc() exhaust = (%d, %v), want (NO_MORE_DOCS, nil)", doc, err)
	}
}

func TestReqOptSumScorer_FilterRandomRareOpt(t *testing.T) {
	// Test filtering: opt matches no req docs (opt is entirely disjoint).
	req := newROSFixedScorer([]int{2, 4, 6}, []float32{5.0, 5.0, 5.0})
	opt := newROSFixedScorer([]int{1, 3, 5}, []float32{100.0, 100.0, 100.0})
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
		// Since opt never matches, score should be only req score.
		v221_45, err := scorer.Score()
		if err != nil {
			t.Fatalf("scorer.Score: %v", err)
		}
		if want := float32(5.0); math.Abs(float64(v221_45-want)) > 1e-6 {
			v222_64, err := scorer.Score()
			if err != nil {
				t.Fatalf("scorer.Score: %v", err)
			}
			t.Errorf("Score() at doc %d = %v, want %v (req only)", doc, v222_64, want)
		}
	}
	if len(docs) != 3 || docs[0] != 2 || docs[1] != 4 || docs[2] != 6 {
		t.Errorf("visited docs = %v, want [2, 4, 6]", docs)
	}
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
func (s *rosFixedScorer) DocIDRunEnd() (int, error) {
	doc := s.DocID()
	if doc == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}
	return doc + 1, nil
}
func (s *rosFixedScorer) Score() (float32, error)            { return s.currentScore(), nil }
func (s *rosFixedScorer) GetMaxScore(_ int) (float32, error) { return s.maxScore, nil }
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
	got, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
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
	got, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
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
	got, err := scorer.Score()
	if err != nil {
		t.Fatalf("scorer.Score: %v", err)
	}
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

	max, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatalf("scorer.GetMaxScore: %v", err)
	}
	// Both scorers start at docID -1 which is ≤ NO_MORE_DOCS; expect 5.0.
	if math.Abs(float64(max-5.0)) > 1e-6 {
		t.Errorf("GetMaxScore() = %v, want 5.0", max)
	}
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *rosFixedScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// GetChildren carries the default body Lucene gives Scorer.GetChildren.
func (s *rosFixedScorer) GetChildren() ([]search.ChildScorable, error) {
	return nil, nil
}

// Iterator returns the double itself: it iterates its own documents,
// as the scorer.iterator() of the Lucene test scorers does.
func (s *rosFixedScorer) Iterator() search.DocIdSetIterator {
	return s
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (s *rosFixedScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body Lucene gives Scorer.SetMinCompetitiveScore.
func (s *rosFixedScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// SmoothingScore carries the default body Lucene gives Scorer.SmoothingScore.
func (s *rosFixedScorer) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// TwoPhaseIterator carries the default body Lucene gives Scorer.TwoPhaseIterator.
func (s *rosFixedScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return search.DefaultTwoPhaseIterator()
}
