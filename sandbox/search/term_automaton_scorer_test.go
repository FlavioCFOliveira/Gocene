// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.TermAutomatonScorer tests.
// (No dedicated Java test peer located; tests verify core iteration and scoring.)
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// staticPostingsEnum is a PostingsEnum that returns predetermined positions
// for a single document.
type staticPostingsEnum struct {
	targetDocID int
	curDocID    int // -1 initially; updated to targetDocID or NO_MORE_DOCS
	positions   []int
	posIdx      int
	advanced    bool
}

func newStaticPostingsEnum(docID int, positions []int) *staticPostingsEnum {
	return &staticPostingsEnum{targetDocID: docID, curDocID: -1, positions: positions, posIdx: -1}
}

func (p *staticPostingsEnum) DocID() int { return p.curDocID }

func (p *staticPostingsEnum) NextDoc() (int, error) {
	if p.advanced {
		p.curDocID = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}
	p.advanced = true
	p.curDocID = p.targetDocID
	p.posIdx = -1
	return p.curDocID, nil
}

func (p *staticPostingsEnum) Advance(target int) (int, error) {
	if p.advanced || p.targetDocID < target {
		p.curDocID = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}
	return p.NextDoc()
}

func (p *staticPostingsEnum) Freq() (int, error)          { return len(p.positions), nil }
func (p *staticPostingsEnum) Cost() int64                 { return 1 }
func (p *staticPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (p *staticPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (p *staticPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

func (p *staticPostingsEnum) NextPosition() (int, error) {
	p.posIdx++
	if p.posIdx >= len(p.positions) {
		return -1, nil
	}
	return p.positions[p.posIdx], nil
}

var _ index.PostingsEnum = (*staticPostingsEnum)(nil)

// buildSingleTransitionAutomaton builds a 2-state automaton:
//
//	state0 --[termID]--> state1 (accept)
func buildSingleTransitionAutomaton(termID int) *automaton.Automaton {
	a := automaton.NewAutomaton()
	s0 := a.CreateState()
	s1 := a.CreateState()
	a.SetAccept(s1, true)
	a.AddTransitionSingle(s0, s1, termID)
	a.FinishState()
	return a
}

// unitSimScorer returns freq * 1.0 as the score.
type unitSimScorer struct{}

func (u *unitSimScorer) Score(_ int, freq float32, norm int64) float32 {
	_ = norm
	return freq
}

var _ search.SimScorer = (*unitSimScorer)(nil)

// TestTermAutomatonScorer_SingleTermMatch verifies that a scorer with a single
// sub-scorer matches the document and computes freq=1.
func TestTermAutomatonScorer_SingleTermMatch(t *testing.T) {
	// Automaton: state0 --[0]--> state1 (accept)
	a := buildSingleTransitionAutomaton(0)

	posEnum := newStaticPostingsEnum(5, []int{3})
	// Do NOT pre-advance; scorer's NextDoc will advance it.
	sub := &EnumAndScorer{TermID: 0, PosEnum: posEnum}

	weight := &TermAutomatonWeight{Automaton: a}
	scorer, err := NewTermAutomatonScorer(weight, []*EnumAndScorer{sub}, -1, &unitSimScorer{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 5 {
		t.Errorf("NextDoc() = %d; want 5", doc)
	}
	if got := scorer.Score(); got != 1.0 {
		t.Errorf("Score() = %v; want 1.0", got)
	}

	// Should be exhausted now.
	next, err := scorer.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if next != search.NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS, got %d", next)
	}
}

// TestTermAutomatonScorer_NoMatchWhenTermIDMismatch verifies that a document
// is skipped when the automaton doesn't accept the term sequence.
func TestTermAutomatonScorer_NoMatchWhenTermIDMismatch(t *testing.T) {
	// Automaton accepts termID=0, but we supply termID=1.
	a := buildSingleTransitionAutomaton(0)

	posEnum := newStaticPostingsEnum(3, []int{0})
	// sub has TermID=1 which the automaton doesn't accept via state0→state1.
	sub := &EnumAndScorer{TermID: 1, PosEnum: posEnum}

	weight := &TermAutomatonWeight{Automaton: a}
	scorer, err := NewTermAutomatonScorer(weight, []*EnumAndScorer{sub}, -1, &unitSimScorer{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// TermID=1 is not accepted by the automaton (only 0 is accepted),
	// so freq=0 and the document is skipped; exhausted = NO_MORE_DOCS.
	doc, err := scorer.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS (no match), got %d", doc)
	}
}

// TestTermAutomatonScorer_GetMaxScorePositive verifies GetMaxScore is positive.
func TestTermAutomatonScorer_GetMaxScorePositive(t *testing.T) {
	a := buildSingleTransitionAutomaton(0)
	posEnum := newStaticPostingsEnum(0, []int{0})
	sub := &EnumAndScorer{TermID: 0, PosEnum: posEnum}
	weight := &TermAutomatonWeight{Automaton: a}
	scorer, err := NewTermAutomatonScorer(weight, []*EnumAndScorer{sub}, -1, &unitSimScorer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := scorer.GetMaxScore(search.NO_MORE_DOCS); got <= 0 {
		t.Errorf("GetMaxScore() = %v; want > 0", got)
	}
}

// TestTermAutomatonScorer_Cost verifies Cost equals sum of sub-scorer costs.
func TestTermAutomatonScorer_Cost(t *testing.T) {
	a := buildSingleTransitionAutomaton(0)
	posEnum1 := newStaticPostingsEnum(0, []int{0})
	posEnum2 := newStaticPostingsEnum(1, []int{0})
	sub1 := &EnumAndScorer{TermID: 0, PosEnum: posEnum1}
	sub2 := &EnumAndScorer{TermID: 0, PosEnum: posEnum2}
	weight := &TermAutomatonWeight{Automaton: a}
	scorer, err := NewTermAutomatonScorer(weight, []*EnumAndScorer{sub1, sub2}, -1, &unitSimScorer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Each staticPostingsEnum has cost=1, so total = 2.
	if got := scorer.Cost(); got != 2 {
		t.Errorf("Cost() = %d; want 2", got)
	}
}
