// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// This file pins FixedBitSet.HashCode, which was missing from the port (the
// Lucene 10.5.0 member org.apache.lucene.util.FixedBitSet#hashCode()). The
// consistency loop renders TestFixedBitSet.testHashCodeEquals; the fixed
// values are the ones Java's algorithm yields.

import (
	"math/rand"
	"testing"
)

func TestFixedBitSet_HashCodeRegression(t *testing.T) {
	empty, _ := NewFixedBitSet(1)
	if got, want := empty.HashCode(), int(int32(-1737092556)); got != want { // (int) 0 + 0x98761234
		t.Fatalf("empty.HashCode() = %d, want %d", got, want)
	}
	one, _ := NewFixedBitSet(1)
	one.Set(0)
	// h = 1 rotated left by one = 2; (int)((2 >> 32) ^ 2) + 0x98761234
	if got, want := one.HashCode(), int(int32(-1737092554)); got != want {
		t.Fatalf("{0}.HashCode() = %d, want %d", got, want)
	}

	// TestFixedBitSet.testHashCodeEquals
	r := rand.New(rand.NewSource(rand.Int63()))
	numBits := r.Intn(2000) + 1
	b1, _ := NewFixedBitSet(numBits)
	b2, _ := NewFixedBitSet(numBits)
	if !b1.Equals(b2) || !b2.Equals(b1) {
		t.Fatal("empty sets must be equal")
	}
	for iter := 0; iter < 10; iter++ {
		idx := r.Intn(numBits)
		if !b1.Get(idx) {
			b1.Set(idx)
			if b1.Equals(b2) {
				t.Fatal("assertFalse(b1.equals(b2))")
			}
			if b1.HashCode() == b2.HashCode() {
				t.Fatal("assertFalse(b1.hashCode() == b2.hashCode())")
			}
			b2.Set(idx)
			if !b1.Equals(b2) {
				t.Fatal("assertEquals(b1, b2)")
			}
			if b1.HashCode() != b2.HashCode() {
				t.Fatal("assertEquals(b1.hashCode(), b2.hashCode())")
			}
		}
	}
}
