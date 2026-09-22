// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerScorer tests.
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubScorer is a minimal search.Scorer that records calls and returns
// predetermined values.
type stubScorer struct {
	search.BaseDocIdSetIterator
	score      float32
	maxScore   float32
	nextDocVal int
	advanceVal int
}

func (s *stubScorer) Score() (float32, error)            { return s.score, nil }
func (s *stubScorer) GetMaxScore(_ int) (float32, error) { return s.maxScore, nil }
func (s *stubScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *stubScorer) NextDoc() (int, error)      { return s.nextDocVal, nil }
func (s *stubScorer) Advance(_ int) (int, error) { return s.advanceVal, nil }
func (s *stubScorer) DocIDRunEnd() (int, error)  { return s.DocID() + 1, nil }

var _ search.Scorer = (*stubScorer)(nil)

// TestQueryProfilerScorer_ScoreTimerIncrements verifies that calling Score
// increments the score timer count.
func TestQueryProfilerScorer_ScoreTimerIncrements(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{score: 2.5}
	ps := newQueryProfilerScorer(stub, bd)

	for i := 0; i < 10; i++ {
		v44, err := ps.Score()
		if err != nil {
			t.Fatalf("ps.Score: %v", err)
		}
		_ = v44
	}

	timer := bd.GetTimer(TimingTypeScore)
	if timer.GetCount() != 10 {
		t.Errorf("expected score count 10, got %d", timer.GetCount())
	}
}

// TestQueryProfilerScorer_NextDocTimerIncrements verifies that calling NextDoc
// increments the nextDoc timer count.
func TestQueryProfilerScorer_NextDocTimerIncrements(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{nextDocVal: 1}
	ps := newQueryProfilerScorer(stub, bd)

	for i := 0; i < 5; i++ {
		if _, err := ps.Iterator().NextDoc(); err != nil {
			t.Fatal(err)
		}
	}

	timer := bd.GetTimer(TimingTypeNextDoc)
	if timer.GetCount() != 5 {
		t.Errorf("expected nextDoc count 5, got %d", timer.GetCount())
	}
}

// TestQueryProfilerScorer_AdvanceTimerIncrements verifies that calling Advance
// increments the advance timer count.
func TestQueryProfilerScorer_AdvanceTimerIncrements(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{advanceVal: 42}
	ps := newQueryProfilerScorer(stub, bd)

	for i := 0; i < 3; i++ {
		if _, err := ps.Iterator().Advance(10); err != nil {
			t.Fatal(err)
		}
	}

	timer := bd.GetTimer(TimingTypeAdvance)
	if timer.GetCount() != 3 {
		t.Errorf("expected advance count 3, got %d", timer.GetCount())
	}
}

// TestQueryProfilerScorer_ComputeMaxScoreTimerIncrements verifies that calling
// GetMaxScore increments the computeMaxScore timer count.
func TestQueryProfilerScorer_ComputeMaxScoreTimerIncrements(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{maxScore: 1.0}
	ps := newQueryProfilerScorer(stub, bd)

	for i := 0; i < 7; i++ {
		v99, err := ps.GetMaxScore(100)
		if err != nil {
			t.Fatalf("ps.GetMaxScore: %v", err)
		}
		_ = v99
	}

	timer := bd.GetTimer(TimingTypeComputeMaxScore)
	if timer.GetCount() != 7 {
		t.Errorf("expected computeMaxScore count 7, got %d", timer.GetCount())
	}
}

// TestQueryProfilerScorer_DelegatesValues verifies that the profiler scorer
// correctly delegates Score, NextDoc, Advance, and GetMaxScore return values.
func TestQueryProfilerScorer_DelegatesValues(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{score: 3.14, maxScore: 9.99, nextDocVal: 5, advanceVal: 20}
	ps := newQueryProfilerScorer(stub, bd)

	if got, err := ps.Score(); err != nil || got != 3.14 {
		t.Errorf("Score() = %v; want 3.14 (err: %v)", got, err)
	}
	if got, err := ps.GetMaxScore(100); err != nil || got != 9.99 {
		t.Errorf("GetMaxScore() = %v; want 9.99 (err: %v)", got, err)
	}
	if got, _ := ps.Iterator().NextDoc(); got != 5 {
		t.Errorf("NextDoc() = %v; want 5", got)
	}
	if got, _ := ps.Iterator().Advance(10); got != 20 {
		t.Errorf("Advance() = %v; want 20", got)
	}
}

// TestQueryProfilerScorer_CostDelegated verifies Cost is delegated.
func TestQueryProfilerScorer_CostDelegated(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{}
	ps := newQueryProfilerScorer(stub, bd)
	// BaseDocIdSetIterator.Cost() returns 0
	if got := ps.Iterator().Cost(); got != 0 {
		t.Errorf("Cost() = %d; want 0", got)
	}
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *stubScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// Cost is abstract in Lucene's DocIdSetIterator; this double does not support it.
func (s *stubScorer) Cost() int64 {
	panic("stubScorer.Cost: unsupported operation")
}

// DocID is abstract in Lucene's DocIdSetIterator; this double does not support it.
func (s *stubScorer) DocID() int {
	panic("stubScorer.DocID: unsupported operation")
}

// GetChildren carries the default body Lucene gives Scorer.GetChildren.
func (s *stubScorer) GetChildren() ([]search.ChildScorable, error) {
	return nil, nil
}

// Iterator returns the double itself: it iterates its own documents,
// as the scorer.iterator() of the Lucene test scorers does.
func (s *stubScorer) Iterator() search.DocIdSetIterator {
	return s
}

// NextDocsAndScores carries the default body Lucene gives Scorer.NextDocsAndScores.
func (s *stubScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// SetMinCompetitiveScore carries the default body Lucene gives Scorer.SetMinCompetitiveScore.
func (s *stubScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// SmoothingScore carries the default body Lucene gives Scorer.SmoothingScore.
func (s *stubScorer) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// TwoPhaseIterator carries the default body Lucene gives Scorer.TwoPhaseIterator.
func (s *stubScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return search.DefaultTwoPhaseIterator()
}
