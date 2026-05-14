// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

func newTestBits(t *testing.T, numBits int, set []int) *FixedBitSet {
	t.Helper()
	bits, err := NewFixedBitSet(numBits)
	if err != nil {
		t.Fatalf("NewFixedBitSet(%d): %v", numBits, err)
	}
	for _, i := range set {
		bits.Set(i)
	}
	return bits
}

func TestDocBaseBitSetIterator_ZeroBase(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 128, []int{3, 17, 64, 100})
	it, err := NewDocBaseBitSetIterator(bits, 4, 0)
	if err != nil {
		t.Fatalf("NewDocBaseBitSetIterator: %v", err)
	}

	if got := it.DocID(); got != -1 {
		t.Errorf("initial DocID = %d, want -1", got)
	}
	wantSeq := []int{3, 17, 64, 100, NO_MORE_DOCS}
	for _, want := range wantSeq {
		got, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if got != want {
			t.Errorf("NextDoc = %d, want %d", got, want)
		}
	}
}

func TestDocBaseBitSetIterator_WithBase(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 128, []int{3, 17, 64})
	base := 64
	it, err := NewDocBaseBitSetIterator(bits, 3, base)
	if err != nil {
		t.Fatalf("NewDocBaseBitSetIterator: %v", err)
	}

	wantSeq := []int{base + 3, base + 17, base + 64, NO_MORE_DOCS}
	for _, want := range wantSeq {
		got, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if got != want {
			t.Errorf("NextDoc = %d, want %d", got, want)
		}
	}
}

func TestDocBaseBitSetIterator_Advance(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 128, []int{10, 20, 30, 40})
	base := 128
	it, err := NewDocBaseBitSetIterator(bits, 4, base)
	if err != nil {
		t.Fatalf("NewDocBaseBitSetIterator: %v", err)
	}

	// Advance to before the first bit -> should land on first set bit.
	got, _ := it.Advance(base + 5)
	if got != base+10 {
		t.Errorf("Advance(base+5) = %d, want %d", got, base+10)
	}
	// Advance to a doc id that is exactly the second set bit.
	got, _ = it.Advance(base + 20)
	if got != base+20 {
		t.Errorf("Advance(base+20) = %d, want %d", got, base+20)
	}
	// Advance past the last set bit -> NO_MORE_DOCS.
	got, _ = it.Advance(base + 99)
	if got != NO_MORE_DOCS {
		t.Errorf("Advance past last = %d, want NO_MORE_DOCS", got)
	}
}

func TestDocBaseBitSetIterator_Advance_TargetBeforeDocBase(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 128, []int{0})
	base := 64
	it, _ := NewDocBaseBitSetIterator(bits, 1, base)

	// Target below docBase clamps to bit-position 0.
	got, _ := it.Advance(10)
	if got != base+0 {
		t.Errorf("Advance(below base) = %d, want %d", got, base)
	}
}

func TestDocBaseBitSetIterator_RejectsBadDocBase(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 64, []int{0})
	_, err := NewDocBaseBitSetIterator(bits, 1, 7)
	if err == nil || !strings.Contains(err.Error(), "multiple of 64") {
		t.Errorf("expected docBase error, got %v", err)
	}
}

func TestDocBaseBitSetIterator_RejectsNegativeCost(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 64, nil)
	_, err := NewDocBaseBitSetIterator(bits, -1, 0)
	if err == nil || !strings.Contains(err.Error(), "cost must be >= 0") {
		t.Errorf("expected cost error, got %v", err)
	}
}

func TestDocBaseBitSetIterator_RejectsNilBits(t *testing.T) {
	t.Parallel()

	if _, err := NewDocBaseBitSetIterator(nil, 1, 0); err == nil {
		t.Errorf("expected nil-bits error")
	}
}

func TestDocBaseBitSetIterator_GettersExposeState(t *testing.T) {
	t.Parallel()

	bits := newTestBits(t, 64, []int{0})
	it, _ := NewDocBaseBitSetIterator(bits, 5, 128)
	if it.GetBitSet() != bits {
		t.Errorf("GetBitSet did not return wrapped bits")
	}
	if it.DocBase() != 128 {
		t.Errorf("DocBase = %d, want 128", it.DocBase())
	}
	if it.Cost() != 5 {
		t.Errorf("Cost = %d, want 5", it.Cost())
	}
}
