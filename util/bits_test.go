// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestMatchAllBits(t *testing.T) {
	m := NewMatchAllBits(100)

	if m.Length() != 100 {
		t.Errorf("Expected length 100, got %d", m.Length())
	}

	// All bits should be set
	for i := 0; i < 100; i++ {
		if !m.Get(i) {
			t.Errorf("Expected bit %d to be set", i)
		}
	}

	// Out of bounds should return false
	if m.Get(-1) {
		t.Error("Expected Get(-1) to return false")
	}
	if m.Get(100) {
		t.Error("Expected Get(100) to return false")
	}
}

func TestMatchNoBits(t *testing.T) {
	m := NewMatchNoBits(100)

	if m.Length() != 100 {
		t.Errorf("Expected length 100, got %d", m.Length())
	}

	// No bits should be set
	for i := 0; i < 100; i++ {
		if m.Get(i) {
			t.Errorf("Expected bit %d to be unset", i)
		}
	}

	// Out of bounds should still return false
	if m.Get(-1) {
		t.Error("Expected Get(-1) to return false")
	}
	if m.Get(100) {
		t.Error("Expected Get(100) to return false")
	}
}

func TestMatchAllBitsInterface(t *testing.T) {
	var bits Bits = NewMatchAllBits(10)

	if bits.Length() != 10 {
		t.Errorf("Expected length 10, got %d", bits.Length())
	}

	// Test BitsMatchAll
	if !BitsMatchAll(bits) {
		t.Error("Expected BitsMatchAll to return true for MatchAllBits")
	}

	// Test BitsMatchNone
	if BitsMatchNone(bits) {
		t.Error("Expected BitsMatchNone to return false for MatchAllBits")
	}
}

func TestMatchNoBitsInterface(t *testing.T) {
	var bits Bits = NewMatchNoBits(10)

	if bits.Length() != 10 {
		t.Errorf("Expected length 10, got %d", bits.Length())
	}

	// Test BitsMatchAll
	if BitsMatchAll(bits) {
		t.Error("Expected BitsMatchAll to return false for MatchNoBits")
	}

	// Test BitsMatchNone
	if !BitsMatchNone(bits) {
		t.Error("Expected BitsMatchNone to return true for MatchNoBits")
	}
}

func TestMatchAllBitsZeroLength(t *testing.T) {
	m := NewMatchAllBits(0)

	if m.Length() != 0 {
		t.Errorf("Expected length 0, got %d", m.Length())
	}

	if m.Get(0) {
		t.Error("Expected Get(0) to return false for zero-length")
	}
}

func TestMatchNoBitsZeroLength(t *testing.T) {
	m := NewMatchNoBits(0)

	if m.Length() != 0 {
		t.Errorf("Expected length 0, got %d", m.Length())
	}

	if m.Get(0) {
		t.Error("Expected Get(0) to return false for zero-length")
	}
}

// TestApplyMask_MatchAllPreservesBits verifies the Lucene default
// applyMask: a Bits whose Get always returns true never clears any
// FixedBitSet bit.
func TestApplyMask_MatchAllPreservesBits(t *testing.T) {
	fbs, err := NewFixedBitSet(16)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	fbs.Set(1)
	fbs.Set(5)
	fbs.Set(15)

	ApplyMask(NewMatchAllBits(32), fbs, 0)

	for _, i := range []int{1, 5, 15} {
		if !fbs.Get(i) {
			t.Errorf("bit %d should remain set after MatchAllBits mask", i)
		}
	}
}

// TestApplyMask_MatchNoneClearsAll verifies the Lucene default
// applyMask: a Bits whose Get always returns false clears every set
// bit in the FixedBitSet.
func TestApplyMask_MatchNoneClearsAll(t *testing.T) {
	fbs, err := NewFixedBitSet(16)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	fbs.Set(1)
	fbs.Set(5)
	fbs.Set(15)

	ApplyMask(NewMatchNoBits(32), fbs, 0)

	if fbs.Cardinality() != 0 {
		t.Errorf("after MatchNoBits mask cardinality = %d, want 0", fbs.Cardinality())
	}
}

// TestApplyMask_RespectsOffset verifies the offset semantics: the mask
// is consulted at (offset + i), not i.
func TestApplyMask_RespectsOffset(t *testing.T) {
	fbs, err := NewFixedBitSet(8)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < 8; i++ {
		fbs.Set(i)
	}

	// Mask: bits 8-15 are set (false elsewhere). With offset=8, all
	// fbs bits should be preserved.
	mask := NewMatchAllBits(16)
	ApplyMask(mask, fbs, 8)
	if fbs.Cardinality() != 8 {
		t.Errorf("offset shift broke MatchAllBits semantics: cardinality = %d", fbs.Cardinality())
	}
}

// TestEmptyBitsArrayIsEmpty ensures the package-level constant is the
// documented zero-length slice.
func TestEmptyBitsArrayIsEmpty(t *testing.T) {
	if len(EmptyBitsArray) != 0 {
		t.Errorf("EmptyBitsArray should be empty, got len %d", len(EmptyBitsArray))
	}
}
