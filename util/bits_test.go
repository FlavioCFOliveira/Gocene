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
