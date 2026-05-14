// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestFixedBitSet_Flip exercises the single-bit and range variants of
// FixedBitSet.Flip / FlipRange, validating both intra-word and cross-word
// boundaries.
func TestFixedBitSet_Flip(t *testing.T) {
	fs, _ := NewFixedBitSet(200)
	fs.Flip(0)
	fs.Flip(63)
	fs.Flip(64)
	fs.Flip(199)
	for _, i := range []int{0, 63, 64, 199} {
		if !fs.Get(i) {
			t.Fatalf("bit %d should be set after Flip", i)
		}
	}
	fs.Flip(0)
	if fs.Get(0) {
		t.Fatalf("Flip(0) twice should leave bit 0 clear")
	}
}

func TestFixedBitSet_FlipRange(t *testing.T) {
	t.Run("intra word", func(t *testing.T) {
		fs, _ := NewFixedBitSet(64)
		fs.FlipRange(2, 10)
		for i := 0; i < 64; i++ {
			expected := i >= 2 && i < 10
			if fs.Get(i) != expected {
				t.Fatalf("bit %d: got %v want %v", i, fs.Get(i), expected)
			}
		}
		fs.FlipRange(2, 10)
		if !fs.IsEmpty() {
			t.Fatalf("double flip should restore empty bitset")
		}
	})

	t.Run("cross word", func(t *testing.T) {
		fs, _ := NewFixedBitSet(200)
		fs.FlipRange(60, 130)
		for i := 0; i < 200; i++ {
			expected := i >= 60 && i < 130
			if fs.Get(i) != expected {
				t.Fatalf("bit %d: got %v want %v", i, fs.Get(i), expected)
			}
		}
	})

	t.Run("empty range", func(t *testing.T) {
		fs, _ := NewFixedBitSet(64)
		fs.Set(5)
		fs.FlipRange(5, 5)
		if !fs.Get(5) {
			t.Fatalf("empty range must not affect bits")
		}
	})
}

func TestFixedBitSet_SetRange(t *testing.T) {
	t.Run("intra word", func(t *testing.T) {
		fs, _ := NewFixedBitSet(64)
		fs.SetRange(10, 20)
		for i := 0; i < 64; i++ {
			expected := i >= 10 && i < 20
			if fs.Get(i) != expected {
				t.Fatalf("bit %d: got %v want %v", i, fs.Get(i), expected)
			}
		}
	})

	t.Run("cross word", func(t *testing.T) {
		fs, _ := NewFixedBitSet(300)
		fs.SetRange(50, 250)
		if fs.Cardinality() != 200 {
			t.Fatalf("cardinality after SetRange(50,250) got %d want 200", fs.Cardinality())
		}
		for i := 0; i < 300; i++ {
			expected := i >= 50 && i < 250
			if fs.Get(i) != expected {
				t.Fatalf("bit %d: got %v want %v", i, fs.Get(i), expected)
			}
		}
	})
}

func TestFixedBitSet_Intersects(t *testing.T) {
	a, _ := NewFixedBitSet(100)
	b, _ := NewFixedBitSet(100)
	if a.Intersects(b) {
		t.Fatalf("empty bitsets should not intersect")
	}
	a.Set(50)
	b.Set(60)
	if a.Intersects(b) {
		t.Fatalf("disjoint sets should not intersect")
	}
	b.Set(50)
	if !a.Intersects(b) {
		t.Fatalf("sets sharing a bit must intersect")
	}
}

func TestFixedBitSet_NextClearBit(t *testing.T) {
	fs, _ := NewFixedBitSet(200)
	fs.SetRange(0, 100)
	if got := fs.NextClearBit(0); got != 100 {
		t.Fatalf("NextClearBit(0) got %d want 100", got)
	}
	if got := fs.NextClearBit(50); got != 100 {
		t.Fatalf("NextClearBit(50) got %d want 100", got)
	}
	fs.SetAll()
	if got := fs.NextClearBit(0); got != 200 {
		t.Fatalf("NextClearBit on all-set returns length, got %d want 200", got)
	}
}

func TestFixedBitSet_BitsAccessor(t *testing.T) {
	fs, _ := NewFixedBitSet(128)
	fs.Set(0)
	fs.Set(64)
	bits := fs.Bits()
	if len(bits) != 2 {
		t.Fatalf("words len: got %d want 2", len(bits))
	}
	if bits[0]&1 == 0 || bits[1]&1 == 0 {
		t.Fatalf("Bits() must reflect underlying state")
	}
}

func TestNewFixedBitSetOfBits(t *testing.T) {
	bits := []uint64{1, 1 << 63}
	fs, err := NewFixedBitSetOfBits(bits, 128)
	if err != nil {
		t.Fatalf("OfBits: %v", err)
	}
	if !fs.Get(0) || !fs.Get(127) {
		t.Fatalf("OfBits did not respect input")
	}
	if fs.Cardinality() != 2 {
		t.Fatalf("cardinality got %d want 2", fs.Cardinality())
	}
	if _, err := NewFixedBitSetOfBits(nil, -1); err == nil {
		t.Fatalf("negative numBits must error")
	}
	if _, err := NewFixedBitSetOfBits([]uint64{0}, 128); err == nil {
		t.Fatalf("undersized backing slice must error")
	}
}

func TestFixedBitSet_IntersectionUnionAndNotCount(t *testing.T) {
	a, _ := NewFixedBitSet(200)
	b, _ := NewFixedBitSet(200)
	a.Set(10)
	a.Set(50)
	a.Set(100)
	b.Set(50)
	b.Set(100)
	b.Set(150)

	if got := IntersectionCount(a, b); got != 2 {
		t.Fatalf("IntersectionCount got %d want 2", got)
	}
	if got := UnionCount(a, b); got != 4 {
		t.Fatalf("UnionCount got %d want 4", got)
	}
	if got := AndNotCount(a, b); got != 1 {
		t.Fatalf("AndNotCount got %d want 1", got)
	}
	if got := AndNotCount(b, a); got != 1 {
		t.Fatalf("AndNotCount(b,a) got %d want 1", got)
	}
}
