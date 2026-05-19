// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestFixedBits_GetLength validates the read-only delegations to the
// wrapped FixedBitSet.
func TestFixedBits_GetLength(t *testing.T) {
	bs, err := NewFixedBitSet(70)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for _, i := range []int{0, 1, 5, 63, 64, 69} {
		bs.Set(i)
	}

	view := bs.AsReadOnlyBits()

	if got := view.Length(); got != 70 {
		t.Fatalf("Length: got %d want 70", got)
	}

	want := map[int]bool{0: true, 1: true, 5: true, 63: true, 64: true, 69: true}
	for i := 0; i < 70; i++ {
		if got := view.Get(i); got != want[i] {
			t.Errorf("Get(%d): got %v want %v", i, got, want[i])
		}
	}
}

// TestFixedBits_NewFromWords exercises the wire-style constructor that
// mirrors Java's FixedBits(long[], int).
func TestFixedBits_NewFromWords(t *testing.T) {
	words := []uint64{0b1011, 0}
	fb, err := NewFixedBits(words, 4)
	if err != nil {
		t.Fatalf("NewFixedBits: %v", err)
	}
	want := []bool{true, true, false, true}
	for i, w := range want {
		if got := fb.Get(i); got != w {
			t.Errorf("Get(%d): got %v want %v", i, got, w)
		}
	}
}

// TestFixedBits_NewFromWords_Errors validates input rejection.
func TestFixedBits_NewFromWords_Errors(t *testing.T) {
	if _, err := NewFixedBits(nil, -1); err == nil {
		t.Error("expected error for negative numBits")
	}
	if _, err := NewFixedBits([]uint64{0}, 65); err == nil {
		t.Error("expected error for slice shorter than required words")
	}
}

// TestFixedBits_LiveView confirms that mutations on the underlying
// FixedBitSet remain visible through the read-only view, matching
// Lucene's documented behaviour.
func TestFixedBits_LiveView(t *testing.T) {
	bs, err := NewFixedBitSet(8)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	view := bs.AsReadOnlyBits()
	if view.Get(3) {
		t.Fatal("bit 3 unexpectedly set before mutation")
	}
	bs.Set(3)
	if !view.Get(3) {
		t.Fatal("bit 3 not visible through view after Set")
	}
	bs.Clear(3)
	if view.Get(3) {
		t.Fatal("bit 3 still visible through view after Clear")
	}
}

// TestFixedBits_ApplyMaskDispatch verifies that FixedBits implements
// the BitsMaskApplier optimisation hook and that the package-level
// ApplyMask dispatches through it. We assert behaviour: bits in dest
// not set in the mask must be cleared.
func TestFixedBits_ApplyMaskDispatch(t *testing.T) {
	mask, err := NewFixedBitSet(8)
	if err != nil {
		t.Fatalf("NewFixedBitSet mask: %v", err)
	}
	for _, i := range []int{0, 2, 4, 6} {
		mask.Set(i)
	}

	dest, err := NewFixedBitSet(8)
	if err != nil {
		t.Fatalf("NewFixedBitSet dest: %v", err)
	}
	for i := 0; i < 8; i++ {
		dest.Set(i)
	}

	view := mask.AsReadOnlyBits()
	if _, ok := view.(BitsMaskApplier); !ok {
		t.Fatal("FixedBits must satisfy BitsMaskApplier")
	}

	ApplyMask(view, dest, 0)

	want := []bool{true, false, true, false, true, false, true, false}
	for i, w := range want {
		if got := dest.Get(i); got != w {
			t.Errorf("dest.Get(%d): got %v want %v", i, got, w)
		}
	}
}

// TestFixedBits_BitSet exposes the underlying bitset for codec use.
func TestFixedBits_BitSet(t *testing.T) {
	bs, err := NewFixedBitSet(4)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	fb, err := NewFixedBits(bs.bits, 4)
	if err != nil {
		t.Fatalf("NewFixedBits: %v", err)
	}
	if fb.BitSet() == nil {
		t.Fatal("BitSet returned nil")
	}
	if fb.BitSet().Length() != 4 {
		t.Fatalf("BitSet().Length: got %d want 4", fb.BitSet().Length())
	}
}
