// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"
)

// TestConstantScoreScorer_Score_ConstantAcrossAllDocs locks the
// invariant that Score and GetMaxScore always return the score the
// scorer was built with, regardless of the iterator's position or
// the upTo argument.
func TestConstantScoreScorer_Score_ConstantAcrossAllDocs(t *testing.T) {
	t.Parallel()
	iter := NewRangeDocIdSetIterator(0, 5)
	score := float32(0.42)
	s := NewConstantScoreScorer(score, COMPLETE, iter)

	for {
		doc, err := s.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if got := s.Score(); got != score {
			t.Fatalf("Score at doc %d: got %v, want %v", doc, got, score)
		}
		if got := s.GetMaxScore(doc + 1); got != score {
			t.Fatalf("GetMaxScore at doc %d: got %v, want %v", doc, got, score)
		}
	}
}

// TestConstantScoreScorer_Cost_ForwardsIterator confirms Cost is a
// pass-through to the underlying iterator's Cost.
func TestConstantScoreScorer_Cost_ForwardsIterator(t *testing.T) {
	t.Parallel()
	iter := NewRangeDocIdSetIterator(10, 30)
	s := NewConstantScoreScorer(1.0, COMPLETE_NO_SCORES, iter)
	if got, want := s.Cost(), int64(20); got != want {
		t.Fatalf("Cost: got %d, want %d", got, want)
	}
}

// TestConstantScoreScorer_DocID_ForwardsIterator confirms DocID,
// Advance and DocIDRunEnd forward to the underlying iterator.
func TestConstantScoreScorer_DocID_ForwardsIterator(t *testing.T) {
	t.Parallel()
	iter := NewRangeDocIdSetIterator(0, 10)
	s := NewConstantScoreScorer(1.0, COMPLETE_NO_SCORES, iter)
	if got := s.DocID(); got != -1 {
		t.Fatalf("DocID before iteration: got %d, want -1", got)
	}
	if doc, err := s.Advance(5); err != nil || doc != 5 {
		t.Fatalf("Advance(5): doc=%d err=%v, want doc=5 err=nil", doc, err)
	}
	if got := s.DocIDRunEnd(); got != 10 {
		t.Fatalf("DocIDRunEnd: got %d, want 10", got)
	}
}

// TestConstantScoreScorer_GettersExposeFields exercises the
// Gocene-specific getters that surface internal state (ScoreMode
// and approximation iterator).
func TestConstantScoreScorer_GettersExposeFields(t *testing.T) {
	t.Parallel()
	iter := NewEmptyDocIdSetIterator()
	s := NewConstantScoreScorer(0.5, TOP_SCORES, iter)
	if got, want := s.GetScoreMode(), TOP_SCORES; got != want {
		t.Fatalf("GetScoreMode: got %v, want %v", got, want)
	}
	if got := s.GetApproximation(); got == nil {
		t.Fatalf("GetApproximation: got nil, want non-nil")
	}
}

// TestConstantScoreScorer_SatisfiesInterface confirms the type
// asserts as Scorer at compile time (the var assertion in the
// production file is duplicated here so a future refactor that
// breaks the interface surfaces here as well).
func TestConstantScoreScorer_SatisfiesInterface(t *testing.T) {
	t.Parallel()
	var _ Scorer = NewConstantScoreScorer(1.0, COMPLETE, NewEmptyDocIdSetIterator())
}

// TestConstantScoreScorer_NextDocPropagatesError fails the test if
// the iterator's NextDoc error is swallowed.
func TestConstantScoreScorer_NextDocPropagatesError(t *testing.T) {
	t.Parallel()
	target := errors.New("disk failure")
	s := NewConstantScoreScorer(1.0, COMPLETE, &errIterator{err: target})
	if _, err := s.NextDoc(); !errors.Is(err, target) {
		t.Fatalf("NextDoc: got err %v, want wrapped %v", err, target)
	}
	if _, err := s.Advance(0); !errors.Is(err, target) {
		t.Fatalf("Advance: got err %v, want wrapped %v", err, target)
	}

// errIterator is a test helper: every method returns target as the
// error. Used to confirm error propagation through the scorer.
type errIterator struct {
	err error
}

}
func (e *errIterator) DocID() int                 { return -1 }
func (e *errIterator) NextDoc() (int, error)      { return -1, e.err }
func (e *errIterator) Advance(_ int) (int, error) { return -1, e.err }
func (e *errIterator) Cost() int64                { return 0 }
func (e *errIterator) DocIDRunEnd() int           { return -1 }