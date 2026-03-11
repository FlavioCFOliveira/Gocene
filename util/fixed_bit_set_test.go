// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestNewFixedBitSet(t *testing.T) {
	// Test normal creation
	fs, err := NewFixedBitSet(100)
	if err != nil {
		t.Fatalf("Failed to create FixedBitSet: %v", err)
	}
	if fs == nil {
		t.Fatal("Expected non-nil FixedBitSet")
	}
	if fs.Length() != 100 {
		t.Errorf("Expected length 100, got %d", fs.Length())
	}

	// Test empty bitset
	fs2, err := NewFixedBitSet(0)
	if err != nil {
		t.Fatalf("Failed to create empty FixedBitSet: %v", err)
	}
	if fs2.Length() != 0 {
		t.Errorf("Expected length 0, got %d", fs2.Length())
	}

	// Test negative size
	_, err = NewFixedBitSet(-1)
	if err == nil {
		t.Error("Expected error for negative size")
	}
}

func TestFixedBitSet_SetAndGet(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	// Initially all bits should be 0
	if fs.Get(50) {
		t.Error("Expected bit 50 to be unset initially")
	}

	// Set a bit
	fs.Set(50)
	if !fs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}

	// Other bits should still be 0
	if fs.Get(51) {
		t.Error("Expected bit 51 to be unset")
	}

	// Test boundary
	fs.Set(0)
	if !fs.Get(0) {
		t.Error("Expected bit 0 to be set")
	}

	fs.Set(99)
	if !fs.Get(99) {
		t.Error("Expected bit 99 to be set")
	}
}

func TestFixedBitSet_Clear(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	fs.Set(50)
	if !fs.Get(50) {
		t.Fatal("Bit should be set before clearing")
	}

	fs.Clear(50)
	if fs.Get(50) {
		t.Error("Expected bit 50 to be cleared")
	}
}

func TestFixedBitSet_ClearAll(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	// Set multiple bits
	fs.Set(10)
	fs.Set(50)
	fs.Set(90)

	fs.ClearAll()

	if !fs.IsEmpty() {
		t.Error("Expected bitset to be empty after ClearAll")
	}
}

func TestFixedBitSet_SetAll(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	fs.SetAll()

	// Check all bits are set
	for i := 0; i < 100; i++ {
		if !fs.Get(i) {
			t.Errorf("Expected bit %d to be set", i)
		}
	}

	// Cardinality should be 100
	if fs.Cardinality() != 100 {
		t.Errorf("Expected cardinality 100, got %d", fs.Cardinality())
	}
}

func TestFixedBitSet_Cardinality(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	if fs.Cardinality() != 0 {
		t.Errorf("Expected initial cardinality 0, got %d", fs.Cardinality())
	}

	fs.Set(10)
	fs.Set(50)
	fs.Set(90)

	if fs.Cardinality() != 3 {
		t.Errorf("Expected cardinality 3, got %d", fs.Cardinality())
	}
}

func TestFixedBitSet_IsEmpty(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	if !fs.IsEmpty() {
		t.Error("Expected empty bitset initially")
	}

	fs.Set(50)
	if fs.IsEmpty() {
		t.Error("Expected non-empty after setting bit")
	}

	fs.Clear(50)
	if !fs.IsEmpty() {
		t.Error("Expected empty after clearing bit")
	}
}

func TestFixedBitSet_NextSetBit(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	fs.Set(10)
	fs.Set(50)
	fs.Set(90)

	// Find first set bit
	next := fs.NextSetBit(0)
	if next != 10 {
		t.Errorf("Expected next set bit from 0 to be 10, got %d", next)
	}

	// Find next set bit
	next = fs.NextSetBit(11)
	if next != 50 {
		t.Errorf("Expected next set bit from 11 to be 50, got %d", next)
	}

	// Find last set bit
	next = fs.NextSetBit(51)
	if next != 90 {
		t.Errorf("Expected next set bit from 51 to be 90, got %d", next)
	}

	// No more set bits
	next = fs.NextSetBit(91)
	if next != -1 {
		t.Errorf("Expected -1 after last set bit, got %d", next)
	}
}

func TestFixedBitSet_PrevSetBit(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	fs.Set(10)
	fs.Set(50)
	fs.Set(90)

	// Find previous set bit
	prev := fs.PrevSetBit(89)
	if prev != 50 {
		t.Errorf("Expected prev set bit from 89 to be 50, got %d", prev)
	}

	// Find first set bit
	prev = fs.PrevSetBit(49)
	if prev != 10 {
		t.Errorf("Expected prev set bit from 49 to be 10, got %d", prev)
	}

	// No previous set bits
	prev = fs.PrevSetBit(9)
	if prev != -1 {
		t.Errorf("Expected -1 before first set bit, got %d", prev)
	}
}

func TestFixedBitSet_And(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs2, _ := NewFixedBitSet(100)

	fs1.Set(10)
	fs1.Set(50)

	fs2.Set(50)
	fs2.Set(90)

	if err := fs1.And(fs2); err != nil {
		t.Fatalf("And failed: %v", err)
	}

	if fs1.Get(10) {
		t.Error("Expected bit 10 to be cleared")
	}
	if !fs1.Get(50) {
		t.Error("Expected bit 50 to remain set")
	}
	if fs1.Get(90) {
		t.Error("Expected bit 90 to be cleared")
	}
}

func TestFixedBitSet_Or(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs2, _ := NewFixedBitSet(100)

	fs1.Set(10)

	fs2.Set(50)

	if err := fs1.Or(fs2); err != nil {
		t.Fatalf("Or failed: %v", err)
	}

	if !fs1.Get(10) {
		t.Error("Expected bit 10 to remain set")
	}
	if !fs1.Get(50) {
		t.Error("Expected bit 50 to be set")
	}
}

func TestFixedBitSet_Xor(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs2, _ := NewFixedBitSet(100)

	fs1.Set(10)
	fs1.Set(50)

	fs2.Set(50)
	fs2.Set(90)

	if err := fs1.Xor(fs2); err != nil {
		t.Fatalf("Xor failed: %v", err)
	}

	if !fs1.Get(10) {
		t.Error("Expected bit 10 to be set (only in fs1)")
	}
	if fs1.Get(50) {
		t.Error("Expected bit 50 to be cleared (in both)")
	}
	if !fs1.Get(90) {
		t.Error("Expected bit 90 to be set (only in fs2)")
	}
}

func TestFixedBitSet_Not(t *testing.T) {
	fs, _ := NewFixedBitSet(10)

	fs.Set(5)
	fs.Not()

	for i := 0; i < 10; i++ {
		if i == 5 {
			if fs.Get(i) {
				t.Errorf("Expected bit %d to be cleared after Not", i)
			}
		} else {
			if !fs.Get(i) {
				t.Errorf("Expected bit %d to be set after Not", i)
			}
		}
	}
}

func TestFixedBitSet_AndNot(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs2, _ := NewFixedBitSet(100)

	fs1.Set(10)
	fs1.Set(50)

	fs2.Set(50)

	if err := fs1.AndNot(fs2); err != nil {
		t.Fatalf("AndNot failed: %v", err)
	}

	if !fs1.Get(10) {
		t.Error("Expected bit 10 to remain set")
	}
	if fs1.Get(50) {
		t.Error("Expected bit 50 to be cleared")
	}
}

func TestFixedBitSet_Clone(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs1.Set(10)
	fs1.Set(50)

	fs2 := fs1.Clone()

	if !fs2.Get(10) || !fs2.Get(50) {
		t.Error("Clone should have same bits set")
	}

	// Modify original
	fs1.Set(90)
	if fs2.Get(90) {
		t.Error("Clone should be independent of original")
	}
}

func TestFixedBitSet_Equals(t *testing.T) {
	fs1, _ := NewFixedBitSet(100)
	fs2, _ := NewFixedBitSet(100)

	if !fs1.Equals(fs2) {
		t.Error("Expected empty bitsets to be equal")
	}

	fs1.Set(50)
	if fs1.Equals(fs2) {
		t.Error("Expected different bitsets to not be equal")
	}

	fs2.Set(50)
	if !fs1.Equals(fs2) {
		t.Error("Expected identical bitsets to be equal")
	}

	// Different sizes
	fs3, _ := NewFixedBitSet(50)
	if fs1.Equals(fs3) {
		t.Error("Expected different sizes to not be equal")
	}
}

func TestFixedBitSet_BitsIterator(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	fs.Set(10)
	fs.Set(50)
	fs.Set(90)

	iter := fs.NewBitsIterator()

	expected := []int{10, 50, 90}
	idx := 0
	for iter.HasNext() {
		bit := iter.Next()
		if idx >= len(expected) {
			t.Fatalf("Iterator returned more bits than expected")
		}
		if bit != expected[idx] {
			t.Errorf("Expected bit %d, got %d", expected[idx], bit)
		}
		idx++
	}

	if idx != len(expected) {
		t.Errorf("Expected %d bits, got %d", len(expected), idx)
	}
}

func TestFixedBitSet_LargeBitset(t *testing.T) {
	// Test with a large bitset that spans multiple uint64 words
	fs, _ := NewFixedBitSet(10000)

	// Set bits in different words
	fs.Set(0)
	fs.Set(63)
	fs.Set(64)
	fs.Set(127)
	fs.Set(128)
	fs.Set(9999)

	if !fs.Get(0) {
		t.Error("Expected bit 0 to be set")
	}
	if !fs.Get(63) {
		t.Error("Expected bit 63 to be set")
	}
	if !fs.Get(64) {
		t.Error("Expected bit 64 to be set")
	}
	if !fs.Get(9999) {
		t.Error("Expected bit 9999 to be set")
	}

	if fs.Cardinality() != 6 {
		t.Errorf("Expected cardinality 6, got %d", fs.Cardinality())
	}
}

func TestBitsLength(t *testing.T) {
	fs, _ := NewFixedBitSet(100)

	if BitsLength(fs) != 100 {
		t.Errorf("Expected BitsLength 100, got %d", BitsLength(fs))
	}

	if BitsLength(nil) != 0 {
		t.Errorf("Expected BitsLength 0 for nil, got %d", BitsLength(nil))
	}
}

func TestBitsMatchAll(t *testing.T) {
	// Empty bitset (size 0) should match all
	fs, _ := NewFixedBitSet(0)
	if !BitsMatchAll(fs) {
		t.Error("Expected MatchAll for empty bitset (size 0)")
	}

	if !BitsMatchAll(nil) {
		t.Error("Expected MatchAll for nil")
	}

	// Non-empty bitset that is all set
	fs2, _ := NewFixedBitSet(10)
	fs2.SetAll()
	if !BitsMatchAll(fs2) {
		t.Error("Expected MatchAll for all-set bitset")
	}

	fs2.Clear(5)
	if BitsMatchAll(fs2) {
		t.Error("Expected not MatchAll when one bit is cleared")
	}
}

func TestBitsMatchNone(t *testing.T) {
	fs, _ := NewFixedBitSet(10)

	if !BitsMatchNone(fs) {
		t.Error("Expected MatchNone for empty bitset")
	}

	if !BitsMatchNone(nil) {
		t.Error("Expected MatchNone for nil")
	}

	fs.Set(5)
	if BitsMatchNone(fs) {
		t.Error("Expected not MatchNone when one bit is set")
	}
}

func TestPopcount(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
	}{
		{0, 0},
		{1, 1},
		{0xFF, 8},
		{0xFFFF, 16},
		{^uint64(0), 64},
	}

	for _, tc := range tests {
		result := popcount(tc.input)
		if result != tc.expected {
			t.Errorf("popcount(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestTrailingZeros(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
	}{
		{0, 64},
		{1, 0},
		{2, 1},
		{4, 2},
		{0x10, 4},
		{0x100, 8},
	}

	for _, tc := range tests {
		result := trailingZeros(tc.input)
		if result != tc.expected {
			t.Errorf("trailingZeros(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestLeadingZeros(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
	}{
		{0, 64},
		{1 << 63, 0},
		{1 << 62, 1},
		{1 << 60, 3},
		{0x80000000, 32},
	}

	for _, tc := range tests {
		result := leadingZeros(tc.input)
		if result != tc.expected {
			t.Errorf("leadingZeros(%d) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}
