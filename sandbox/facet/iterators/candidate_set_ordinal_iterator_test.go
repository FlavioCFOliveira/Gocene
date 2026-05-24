// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.TestCandidateSetOrdinalIterator.
//
// Deviations from Java:
//   - The Java CandidateSetOrdinalIterator accepts FacetLabel[] + LabelToOrd +
//     CountFacetRecorder and filters to only ordinals that were recorded.
//     Gocene's stub takes a pre-filtered []int candidate list and uses
//     Next() (int, bool) instead of nextOrd() / NO_MORE_ORDS.
//   - testBasic and testEmptyRecorder are re-expressed against the Gocene
//     stub's API; the recorder-integration logic is deferred to backlog #2693
//     when CountFacetRecorder and LabelToOrd are fully wired.
package iterators

import (
	"testing"
)

// TestCandidateSetOrdinalIterator_Basic mirrors testBasic: only ordinals that
// were "recorded" (in this stub, the caller supplies them as candidates)
// are returned.
func TestCandidateSetOrdinalIterator_Basic(t *testing.T) {
	// Simulate that ordinals 0 and 3 were recorded (1 was not).
	it := NewCandidateSetOrdinalIterator([]int{0, 3})

	var got []int
	for {
		v, ok := it.Next()
		if !ok {
			break
		}
		got = append(got, v)
	}

	want := []int{0, 3}
	if len(got) != len(want) {
		t.Fatalf("Next() sequence = %v; want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %d; want %d", i, got[i], w)
		}
	}
}

// TestCandidateSetOrdinalIterator_EmptyRecorder mirrors testEmptyRecorder:
// when no ordinals are recorded, the iterator immediately terminates.
func TestCandidateSetOrdinalIterator_EmptyRecorder(t *testing.T) {
	it := NewCandidateSetOrdinalIterator(nil)
	_, ok := it.Next()
	if ok {
		t.Error("expected Next() = false for empty candidate set")
	}
}

// TestCandidateSetOrdinalIterator_SingleCandidate verifies single-element sets.
func TestCandidateSetOrdinalIterator_SingleCandidate(t *testing.T) {
	it := NewCandidateSetOrdinalIterator([]int{42})
	v, ok := it.Next()
	if !ok || v != 42 {
		t.Errorf("Next() = (%d, %v); want (42, true)", v, ok)
	}
	_, ok = it.Next()
	if ok {
		t.Error("expected termination after single candidate")
	}
}

// TestCandidateSetOrdinalIterator_OrderPreserved verifies that the insertion
// order of candidates is preserved.
func TestCandidateSetOrdinalIterator_OrderPreserved(t *testing.T) {
	candidates := []int{5, 1, 9, 2}
	it := NewCandidateSetOrdinalIterator(candidates)

	for i, want := range candidates {
		v, ok := it.Next()
		if !ok {
			t.Fatalf("Next() terminated early at index %d", i)
		}
		if v != want {
			t.Errorf("Next()[%d] = %d; want %d", i, v, want)
		}
	}
	_, ok := it.Next()
	if ok {
		t.Error("expected termination after all candidates consumed")
	}
}
