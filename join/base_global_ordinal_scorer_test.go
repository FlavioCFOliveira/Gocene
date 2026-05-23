// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// emptyDISI is a DocIdSetIterator that returns NO_MORE_DOCS immediately.
type emptyDISI struct{}

func (emptyDISI) DocID() int                    { return search.NO_MORE_DOCS }
func (emptyDISI) NextDoc() (int, error)         { return search.NO_MORE_DOCS, nil }
func (emptyDISI) Advance(_ int) (int, error)    { return search.NO_MORE_DOCS, nil }
func (emptyDISI) Cost() int64                   { return 0 }
func (emptyDISI) DocIDRunEnd() int              { return search.NO_MORE_DOCS }

var _ search.DocIdSetIterator = emptyDISI{}

func TestBaseGlobalOrdinalScorer_Score(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 2.0, nil)
	s.score = 0.5
	if got := s.Score(); got != 1.0 {
		t.Errorf("Score() = %v, want 1.0 (0.5 * 2.0 boost)", got)
	}
}

func TestBaseGlobalOrdinalScorer_GetMaxScore(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, nil)
	if got := s.GetMaxScore(0); !math.IsInf(float64(got), 1) {
		t.Errorf("GetMaxScore() = %v, want +Inf", got)
	}
}

func TestBaseGlobalOrdinalScorer_DocID(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, nil)
	if got := s.DocID(); got != search.NO_MORE_DOCS {
		t.Errorf("DocID() = %d, want NO_MORE_DOCS", got)
	}
}

func TestBaseGlobalOrdinalScorer_NextDocNoTwoPhase(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, nil)
	doc, err := s.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("NextDoc() = %d, want NO_MORE_DOCS", doc)
	}
}

func TestBaseGlobalOrdinalScorer_AdvanceNoTwoPhase(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, nil)
	doc, err := s.Advance(5)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("Advance() = %d, want NO_MORE_DOCS", doc)
	}
}

func TestBaseGlobalOrdinalScorer_Cost(t *testing.T) {
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, nil)
	if s.Cost() != 0 {
		t.Errorf("Cost() = %d, want 0", s.Cost())
	}
}

func TestBaseGlobalOrdinalScorer_TwoPhaseIterator(t *testing.T) {
	called := false
	createFn := func(approx search.DocIdSetIterator) *search.TwoPhaseIterator {
		called = true
		return search.NewTwoPhaseIterator(approx, func() (bool, error) { return false, nil })
	}
	s := newBaseGlobalOrdinalScorer(nil, emptyDISI{}, 1.0, createFn)
	tpi := s.TwoPhaseIterator()
	if !called {
		t.Error("createTwoPhase factory was not called")
	}
	if tpi == nil {
		t.Fatal("TwoPhaseIterator() returned nil")
	}
	// Second call returns the cached instance.
	tpi2 := s.TwoPhaseIterator()
	if tpi2 != tpi {
		t.Error("TwoPhaseIterator() returned a different instance on second call")
	}
}
