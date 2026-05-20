// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No Java test peer for PhrasePositions (TestPhrasePositions does not
// exist in Lucene 10.4.0). These tests cover constructor state,
// FirstPosition, NextPosition, exhaustion, and String().

package search_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── stub PostingsEnum ────────────────────────────────────────────────────

// ppStubPostings is a minimal PostingsEnum for testing PhrasePositions.
type ppStubPostings struct {
	freq      int
	positions []int
	posIdx    int
}

func newPPStubPostings(freq int, positions []int) *ppStubPostings {
	return &ppStubPostings{freq: freq, positions: positions, posIdx: -1}
}

func (p *ppStubPostings) DocID() int                 { return 0 }
func (p *ppStubPostings) NextDoc() (int, error)      { return 0, nil }
func (p *ppStubPostings) Advance(_ int) (int, error) { return 0, nil }
func (p *ppStubPostings) Cost() int64                { return 1 }
func (p *ppStubPostings) DocIDRunEnd() int           { return 1 }
func (p *ppStubPostings) Freq() (int, error)         { return p.freq, nil }
func (p *ppStubPostings) NextPosition() (int, error) {
	p.posIdx++
	if p.posIdx >= len(p.positions) {
		return index.NO_MORE_POSITIONS, nil
	}
	return p.positions[p.posIdx], nil
}
func (p *ppStubPostings) StartOffset() (int, error)   { return 0, nil }
func (p *ppStubPostings) EndOffset() (int, error)     { return 0, nil }
func (p *ppStubPostings) GetPayload() ([]byte, error) { return nil, nil }

// ─── tests ───────────────────────────────────────────────────────────────

func TestPhrasePositions_Constructor(t *testing.T) {
	pe := newPPStubPostings(2, []int{5, 10})
	term := index.NewTerm("f", "foo")
	pp := search.NewPhrasePositions(pe, 1, 42, []*index.Term{term})

	if pp.Offset != 1 {
		t.Errorf("Offset = %d, want 1", pp.Offset)
	}
	if pp.Ord != 42 {
		t.Errorf("Ord = %d, want 42", pp.Ord)
	}
	if pp.RptGroup != -1 {
		t.Errorf("RptGroup = %d, want -1", pp.RptGroup)
	}
	if pp.Count != 0 {
		t.Errorf("initial Count = %d, want 0", pp.Count)
	}
}

func TestPhrasePositions_FirstPositionAndNext(t *testing.T) {
	// Three positions: 3, 7, 12. Offset = 1.
	// Expected adjusted positions: 2, 6, 11.
	pe := newPPStubPostings(3, []int{3, 7, 12})
	pp := search.NewPhrasePositions(pe, 1, 0, nil)

	if err := pp.FirstPosition(); err != nil {
		t.Fatalf("FirstPosition() error: %v", err)
	}
	// After FirstPosition: first position consumed (count=2, position=2).
	if pp.Count != 2 {
		t.Errorf("Count after FirstPosition = %d, want 2", pp.Count)
	}
	if pp.Position != 2 {
		t.Errorf("Position after FirstPosition = %d, want 2", pp.Position)
	}

	ok, err := pp.NextPosition()
	if err != nil || !ok {
		t.Fatalf("NextPosition() = (%v, %v), want (true, nil)", ok, err)
	}
	if pp.Position != 6 {
		t.Errorf("Position = %d, want 6", pp.Position)
	}
	if pp.Count != 1 {
		t.Errorf("Count = %d, want 1", pp.Count)
	}

	ok, err = pp.NextPosition()
	if err != nil || !ok {
		t.Fatalf("NextPosition() = (%v, %v), want (true, nil)", ok, err)
	}
	if pp.Position != 11 {
		t.Errorf("Position = %d, want 11", pp.Position)
	}
	if pp.Count != 0 {
		t.Errorf("Count = %d, want 0", pp.Count)
	}
}

func TestPhrasePositions_Exhaustion(t *testing.T) {
	pe := newPPStubPostings(1, []int{5})
	pp := search.NewPhrasePositions(pe, 0, 0, nil)
	if err := pp.FirstPosition(); err != nil {
		t.Fatalf("FirstPosition() error: %v", err)
	}
	// Count is now 0 — next call must return false.
	ok, err := pp.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition() error: %v", err)
	}
	if ok {
		t.Errorf("NextPosition() = true after exhaustion, want false")
	}
}

func TestPhrasePositions_String(t *testing.T) {
	pe := newPPStubPostings(1, []int{5})
	pp := search.NewPhrasePositions(pe, 2, 0, nil)
	if err := pp.FirstPosition(); err != nil {
		t.Fatalf("FirstPosition() error: %v", err)
	}
	s := pp.String()
	if !strings.Contains(s, "o:2") {
		t.Errorf("String() = %q, missing offset info", s)
	}
	// With rptGroup == -1, should not contain "rpt:".
	if strings.Contains(s, "rpt:") {
		t.Errorf("String() = %q, unexpected rpt info when rptGroup == -1", s)
	}
}

func TestPhrasePositions_StringWithRptGroup(t *testing.T) {
	pe := newPPStubPostings(1, []int{5})
	pp := search.NewPhrasePositions(pe, 0, 0, nil)
	pp.RptGroup = 3
	pp.RptInd = 7
	if err := pp.FirstPosition(); err != nil {
		t.Fatalf("FirstPosition() error: %v", err)
	}
	s := pp.String()
	if !strings.Contains(s, "rpt:3,i7") {
		t.Errorf("String() = %q, missing rpt info", s)
	}
}
