// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiLeafFieldComparator.java
//   No Java test peer exists — synthetic Go tests covering the contract.

package search

import "testing"

// stubLeafComparator is a minimal LeafFieldComparator for testing.
type stubLeafComparator struct {
	bottom    int
	cmpBottom int // value returned by CompareBottom
	cmpTop    int // value returned by CompareTop
	copies    [][2]int
	threshold bool
}

func (s *stubLeafComparator) SetBottom(slot int) error        { s.bottom = slot; return nil }
func (s *stubLeafComparator) CompareBottom(_ int) (int, error) { return s.cmpBottom, nil }
func (s *stubLeafComparator) CompareTop(_ int) (int, error)   { return s.cmpTop, nil }
func (s *stubLeafComparator) Copy(slot, doc int) error {
	s.copies = append(s.copies, [2]int{slot, doc})
	return nil
}
func (s *stubLeafComparator) SetScorer(_ Scorable) error { return nil }
func (s *stubLeafComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (s *stubLeafComparator) SetHitsThresholdReached()                       { s.threshold = true }

// TestMultiLeafFieldComparator_MismatchedLengthsReturnsError verifies the
// constructor rejects slice length mismatches.
func TestMultiLeafFieldComparator_MismatchedLengthsReturnsError(t *testing.T) {
	c := &stubLeafComparator{}
	_, err := newMultiLeafFieldComparator([]LeafFieldComparator{c}, []int{1, -1})
	if err == nil {
		t.Fatal("expected error for mismatched slice lengths")
	}
}

// TestMultiLeafFieldComparator_SetBottomForwardsToAll verifies SetBottom propagates.
func TestMultiLeafFieldComparator_SetBottomForwardsToAll(t *testing.T) {
	a, b := &stubLeafComparator{}, &stubLeafComparator{}
	m, err := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetBottom(7); err != nil {
		t.Fatal(err)
	}
	if a.bottom != 7 || b.bottom != 7 {
		t.Fatalf("expected both bottoms=7, got %d %d", a.bottom, b.bottom)
	}
}

// TestMultiLeafFieldComparator_CompareBottomFirstWins verifies short-circuit on first
// non-zero result.
func TestMultiLeafFieldComparator_CompareBottomFirstWins(t *testing.T) {
	a := &stubLeafComparator{cmpBottom: 1}
	b := &stubLeafComparator{cmpBottom: -1}
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	got, err := m.CompareBottom(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

// TestMultiLeafFieldComparator_CompareBottomFallsThrough verifies tiebreak via second comparator.
func TestMultiLeafFieldComparator_CompareBottomFallsThrough(t *testing.T) {
	a := &stubLeafComparator{cmpBottom: 0}
	b := &stubLeafComparator{cmpBottom: -1}
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	got, err := m.CompareBottom(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}

// TestMultiLeafFieldComparator_ReverseMul verifies direction multiplier application.
func TestMultiLeafFieldComparator_ReverseMul(t *testing.T) {
	a := &stubLeafComparator{cmpBottom: 0}
	b := &stubLeafComparator{cmpBottom: 1} // underlying returns 1, but reverseMul=-1 → -1
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, -1})
	got, err := m.CompareBottom(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}

// TestMultiLeafFieldComparator_CopyForwardsToAll verifies Copy calls both comparators.
func TestMultiLeafFieldComparator_CopyForwardsToAll(t *testing.T) {
	a, b := &stubLeafComparator{}, &stubLeafComparator{}
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	if err := m.Copy(3, 9); err != nil {
		t.Fatal(err)
	}
	if len(a.copies) == 0 || a.copies[0] != [2]int{3, 9} {
		t.Fatalf("a.copies: got %v", a.copies)
	}
	if len(b.copies) == 0 || b.copies[0] != [2]int{3, 9} {
		t.Fatalf("b.copies: got %v", b.copies)
	}
}

// TestMultiLeafFieldComparator_SetHitsThresholdReachedOnlyFirst verifies
// SetHitsThresholdReached propagates to first comparator only.
func TestMultiLeafFieldComparator_SetHitsThresholdReachedOnlyFirst(t *testing.T) {
	a, b := &stubLeafComparator{}, &stubLeafComparator{}
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	m.SetHitsThresholdReached()
	if !a.threshold {
		t.Fatal("expected first comparator threshold=true")
	}
	if b.threshold {
		t.Fatal("expected second comparator threshold=false")
	}
}

// TestMultiLeafFieldComparator_CompareTopFirstWins verifies CompareTop short-circuit.
func TestMultiLeafFieldComparator_CompareTopFirstWins(t *testing.T) {
	a := &stubLeafComparator{cmpTop: -1}
	b := &stubLeafComparator{cmpTop: 1}
	m, _ := newMultiLeafFieldComparator([]LeafFieldComparator{a, b}, []int{1, 1})
	got, err := m.CompareTop(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}
