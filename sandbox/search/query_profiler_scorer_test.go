// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerScorer tests.
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
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

func (s *stubScorer) Score() float32            { return s.score }
func (s *stubScorer) GetMaxScore(_ int) float32 { return s.maxScore }
func (s *stubScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *stubScorer) NextDoc() (int, error)      { return s.nextDocVal, nil }
func (s *stubScorer) Advance(_ int) (int, error) { return s.advanceVal, nil }
func (s *stubScorer) DocIDRunEnd() int           { return s.BaseDocIdSetIterator.DocIDRunEnd() }

var _ search.Scorer = (*stubScorer)(nil)

// TestQueryProfilerScorer_ScoreTimerIncrements verifies that calling Score
// increments the score timer count.
func TestQueryProfilerScorer_ScoreTimerIncrements(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{score: 2.5}
	ps := newQueryProfilerScorer(stub, bd)

	for i := 0; i < 10; i++ {
		_ = ps.Score()
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
		if _, err := ps.NextDoc(); err != nil {
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
		if _, err := ps.Advance(10); err != nil {
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
		_ = ps.GetMaxScore(100)
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

	if got := ps.Score(); got != 3.14 {
		t.Errorf("Score() = %v; want 3.14", got)
	}
	if got := ps.GetMaxScore(100); got != 9.99 {
		t.Errorf("GetMaxScore() = %v; want 9.99", got)
	}
	if got, _ := ps.NextDoc(); got != 5 {
		t.Errorf("NextDoc() = %v; want 5", got)
	}
	if got, _ := ps.Advance(10); got != 20 {
		t.Errorf("Advance() = %v; want 20", got)
	}
}

// TestQueryProfilerScorer_CostDelegated verifies Cost is delegated.
func TestQueryProfilerScorer_CostDelegated(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	stub := &stubScorer{}
	ps := newQueryProfilerScorer(stub, bd)
	// BaseDocIdSetIterator.Cost() returns 0
	if got := ps.Cost(); got != 0 {
		t.Errorf("Cost() = %d; want 0", got)
	}
}
