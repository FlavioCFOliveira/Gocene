// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"math/rand/v2"
	"testing"
)

// Mirrors lucene/core/src/test/org/apache/lucene/util/bkd/TestBKDUtil.java
// from Apache Lucene 10.4.0. Each Java test peer is ported one-for-one
// with the same fixture-generation logic. Where the Java test relies
// on Lucene's TestUtil.nextInt(random(), low, high), we use math/rand's
// helpers seeded per-subtest so failures are reproducible.

func randIntInRange(r *rand.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + r.IntN(hi-lo+1)
}

// TestEquals4 mirrors TestBKDUtil.testEquals4.
func TestEquals4(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))

	aOffset := randIntInRange(r, 0, 3)
	a := make([]byte, 4+aOffset)
	bOffset := randIntInRange(r, 0, 3)
	b := make([]byte, 4+bOffset)

	for i := 0; i < 4; i++ {
		a[aOffset+i] = byte(r.IntN(1 << 8))
	}
	copy(b[bOffset:bOffset+4], a[aOffset:aOffset+4])

	if !Equals4(a, aOffset, b, bOffset) {
		t.Fatalf("expected Equals4 true for identical bytes")
	}

	for i := 0; i < 4; i++ {
		for {
			b[bOffset+i] = byte(r.IntN(1 << 8))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
		if Equals4(a, aOffset, b, bOffset) {
			t.Fatalf("expected Equals4 false at differing byte %d", i)
		}
		b[bOffset+i] = a[aOffset+i]
	}
}

// TestEquals8 mirrors TestBKDUtil.testEquals8.
func TestEquals8(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))

	aOffset := randIntInRange(r, 0, 7)
	a := make([]byte, 8+aOffset)
	bOffset := randIntInRange(r, 0, 7)
	b := make([]byte, 8+bOffset)

	for i := 0; i < 8; i++ {
		a[aOffset+i] = byte(r.IntN(1 << 8))
	}
	copy(b[bOffset:bOffset+8], a[aOffset:aOffset+8])

	if !Equals8(a, aOffset, b, bOffset) {
		t.Fatalf("expected Equals8 true for identical bytes")
	}

	for i := 0; i < 8; i++ {
		for {
			b[bOffset+i] = byte(r.IntN(1 << 8))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
		if Equals8(a, aOffset, b, bOffset) {
			t.Fatalf("expected Equals8 false at differing byte %d", i)
		}
		b[bOffset+i] = a[aOffset+i]
	}
}

// TestCommonPrefixLength4 mirrors TestBKDUtil.testCommonPrefixLength4.
func TestCommonPrefixLength4(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))

	aOffset := randIntInRange(r, 0, 3)
	a := make([]byte, 4+aOffset)
	bOffset := randIntInRange(r, 0, 3)
	b := make([]byte, 4+bOffset)

	for i := 0; i < 4; i++ {
		a[aOffset+i] = byte(r.IntN(1 << 8))
		for {
			b[bOffset+i] = byte(r.IntN(1 << 8))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
	}

	for i := 0; i < 4; i++ {
		got := CommonPrefixLength4(a, aOffset, b, bOffset)
		if got != i {
			t.Fatalf("CommonPrefixLength4 at step %d: got %d want %d", i, got, i)
		}
		b[bOffset+i] = a[aOffset+i]
	}

	if got := CommonPrefixLength4(a, aOffset, b, bOffset); got != 4 {
		t.Fatalf("CommonPrefixLength4 fully equal: got %d want 4", got)
	}
}

// TestCommonPrefixLength8 mirrors TestBKDUtil.testCommonPrefixLength8.
func TestCommonPrefixLength8(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))

	aOffset := randIntInRange(r, 0, 7)
	a := make([]byte, 8+aOffset)
	bOffset := randIntInRange(r, 0, 7)
	b := make([]byte, 8+bOffset)

	for i := 0; i < 8; i++ {
		a[aOffset+i] = byte(r.IntN(1 << 8))
		for {
			b[bOffset+i] = byte(r.IntN(1 << 8))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
	}

	for i := 0; i < 8; i++ {
		got := CommonPrefixLength8(a, aOffset, b, bOffset)
		if got != i {
			t.Fatalf("CommonPrefixLength8 at step %d: got %d want %d", i, got, i)
		}
		b[bOffset+i] = a[aOffset+i]
	}

	if got := CommonPrefixLength8(a, aOffset, b, bOffset); got != 8 {
		t.Fatalf("CommonPrefixLength8 fully equal: got %d want 8", got)
	}
}

// TestCommonPrefixLengthN mirrors TestBKDUtil.testCommonPrefixLengthN.
func TestCommonPrefixLengthN(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))

	numBytes := randIntInRange(r, 2, 16)
	aOffset := randIntInRange(r, 0, numBytes-1)
	a := make([]byte, numBytes+aOffset)
	bOffset := randIntInRange(r, 0, numBytes-1)
	b := make([]byte, numBytes+bOffset)

	for i := 0; i < numBytes; i++ {
		a[aOffset+i] = byte(r.IntN(1 << 8))
		for {
			b[bOffset+i] = byte(r.IntN(1 << 8))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
	}

	for i := 0; i < numBytes; i++ {
		got := CommonPrefixLengthN(a, aOffset, b, bOffset, numBytes)
		if got != i {
			t.Fatalf("CommonPrefixLengthN[%d] step %d: got %d want %d", numBytes, i, got, i)
		}
		b[bOffset+i] = a[aOffset+i]
	}

	if got := CommonPrefixLengthN(a, aOffset, b, bOffset, numBytes); got != numBytes {
		t.Fatalf("CommonPrefixLengthN[%d] fully equal: got %d want %d", numBytes, got, numBytes)
	}
}

// TestCommonPrefixLength4_Identical is a fast-path regression: identical
// 4-byte runs must return 4 (Java's algorithm relies on
// numberOfLeadingZeros of zero returning 64; the Go port uses
// TrailingZeros and short-circuits on XOR == 0).
func TestCommonPrefixLength4_Identical(t *testing.T) {
	a := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	b := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if got := CommonPrefixLength4(a, 0, b, 0); got != 4 {
		t.Fatalf("identical: got %d want 4", got)
	}
}

// TestCommonPrefixLength8_Identical mirrors the above for 8 bytes.
func TestCommonPrefixLength8_Identical(t *testing.T) {
	a := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	b := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	if got := CommonPrefixLength8(a, 0, b, 0); got != 8 {
		t.Fatalf("identical: got %d want 8", got)
	}
}

// TestGetPrefixLengthComparator validates the dispatch table for the
// three branches (4, 8, generic).
func TestGetPrefixLengthComparator(t *testing.T) {
	type tcase struct {
		name     string
		numBytes int
		a, b     []byte
		want     int
	}
	tests := []tcase{
		{"4 prefix 0", 4, []byte{0xFF, 0, 0, 0}, []byte{0xFE, 0, 0, 0}, 0},
		{"4 prefix 1", 4, []byte{0xFF, 0xAA, 0, 0}, []byte{0xFF, 0xAB, 0, 0}, 1},
		{"4 prefix 3", 4, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 5}, 3},
		{"4 full match", 4, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4}, 4},
		{"8 prefix 0", 8, []byte{0xFF, 0, 0, 0, 0, 0, 0, 0}, []byte{0xFE, 0, 0, 0, 0, 0, 0, 0}, 0},
		{"8 prefix 7", 8, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6, 7, 9}, 7},
		{"8 full match", 8, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6, 7, 8}, 8},
		{"N=3 prefix 0", 3, []byte{0xAA, 0xBB, 0xCC}, []byte{0xAB, 0xBB, 0xCC}, 0},
		{"N=3 prefix 2", 3, []byte{0xAA, 0xBB, 0xCC}, []byte{0xAA, 0xBB, 0xCD}, 2},
		{"N=3 full match", 3, []byte{1, 2, 3}, []byte{1, 2, 3}, 3},
		{"N=10 prefix 5", 10, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []byte{1, 2, 3, 4, 5, 0, 0, 0, 0, 0}, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmp := GetPrefixLengthComparator(tc.numBytes)
			got := cmp(tc.a, 0, tc.b, 0)
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// TestGetEqualsPredicate validates the dispatch table for the three
// branches (4, 8, generic).
func TestGetEqualsPredicate(t *testing.T) {
	type tcase struct {
		name     string
		numBytes int
		a, b     []byte
		want     bool
	}
	tests := []tcase{
		{"4 equal", 4, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4}, true},
		{"4 diff", 4, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 5}, false},
		{"8 equal", 8, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6, 7, 8}, true},
		{"8 diff", 8, []byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{1, 2, 3, 4, 5, 6, 7, 9}, false},
		{"N=3 equal", 3, []byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{"N=3 diff at 1", 3, []byte{1, 2, 3}, []byte{1, 0, 3}, false},
		{"N=5 equal", 5, []byte{1, 2, 3, 4, 5}, []byte{1, 2, 3, 4, 5}, true},
		{"N=5 diff at 4", 5, []byte{1, 2, 3, 4, 5}, []byte{1, 2, 3, 4, 6}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eq := GetEqualsPredicate(tc.numBytes)
			if got := eq(tc.a, 0, tc.b, 0); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCommonPrefixLength_OffsetSafety verifies that aOffset/bOffset
// are honoured: lengths past offset+numBytes must not be inspected
// even if they would differ.
func TestCommonPrefixLength_OffsetSafety(t *testing.T) {
	// 4-byte case
	a := []byte{0x99, 0x99, 1, 2, 3, 4, 0x55, 0x55}
	b := []byte{0xAA, 0xAA, 1, 2, 3, 4, 0x66, 0x66}
	if got := CommonPrefixLength4(a, 2, b, 2); got != 4 {
		t.Fatalf("offset 4: got %d want 4", got)
	}
	// 8-byte case
	a8 := []byte{0x99, 1, 2, 3, 4, 5, 6, 7, 8, 0x55}
	b8 := []byte{0xAA, 1, 2, 3, 4, 5, 6, 7, 8, 0x66}
	if got := CommonPrefixLength8(a8, 1, b8, 1); got != 8 {
		t.Fatalf("offset 8: got %d want 8", got)
	}
	// Generic case
	aN := []byte{0x99, 1, 2, 3, 0x55}
	bN := []byte{0xAA, 1, 2, 3, 0x66}
	if got := CommonPrefixLengthN(aN, 1, bN, 1, 3); got != 3 {
		t.Fatalf("offset N: got %d want 3", got)
	}
}
