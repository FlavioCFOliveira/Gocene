// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"math/bits"
	"math/rand"
	"testing"
)

// bitTableReader yields the byte values stored in a presence
// bit-table, in writer-order. We back it by an OnHeapFSTStore so the
// same code paths used by FST traversal are exercised.
//
// Lucene's BitTableUtil sometimes reads one byte past the formal end
// of the bit-table (previousBitSet with bitIndex == numBits); in a
// real FST that byte is the next arc-flag byte and the masked result
// is harmless. To mirror that environment we pad the writer-order
// stream with two trailing zero bytes so any in-bounds Lucene
// access stays in-bounds in the standalone test.
func newBitTableReader(t *testing.T, table []byte) BytesReader {
	t.Helper()
	// Desired writer order: table[0], table[1], ..., table[n-1], 0, 0.
	// The reverse reader walks the underlying slice from the last
	// element to the first, so the slice is the writer stream
	// reversed: [0, 0, table[n-1], ..., table[0]].
	const trailingPad = 2
	storeLen := len(table) + trailingPad
	reversed := make([]byte, storeLen)
	for i, b := range table {
		reversed[storeLen-1-i] = b
	}
	// Bytes at writer positions [len(table), storeLen) are already
	// zero by make([]byte, ...).
	s := NewOnHeapFSTStoreFromBytes(reversed)
	return s.GetReverseBytesReader()
}

func TestBitTableIsBitSet(t *testing.T) {
	// Bits at indices 0, 1, 5, 8, 17 are set.
	// Byte 0: 0b00100011 = 0x23, Byte 1: 0b00000001 = 0x01, Byte 2: 0b00000010 = 0x02.
	table := []byte{0x23, 0x01, 0x02}
	cases := []struct {
		idx  int
		want bool
	}{
		{0, true}, {1, true}, {2, false}, {3, false}, {4, false},
		{5, true}, {6, false}, {7, false},
		{8, true}, {9, false}, {15, false},
		{16, false}, {17, true}, {18, false}, {23, false},
	}
	for _, c := range cases {
		r := newBitTableReader(t, table)
		got, err := bitTableIsBitSet(c.idx, r)
		if err != nil {
			t.Fatalf("bit %d: %v", c.idx, err)
		}
		if got != c.want {
			t.Fatalf("bit %d: got %v want %v", c.idx, got, c.want)
		}
	}
}

func TestBitTableCountBits(t *testing.T) {
	// Lengths around the 8-byte threshold to exercise both paths.
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 31} {
		table := make([]byte, n)
		want := 0
		for i := range table {
			table[i] = byte(i*31 + 1)
			want += bits.OnesCount8(table[i])
		}
		r := newBitTableReader(t, table)
		got, err := bitTableCountBits(n, r)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if got != want {
			t.Fatalf("n=%d: got %d want %d", n, got, want)
		}
	}
}

func TestBitTableCountBitsUpTo(t *testing.T) {
	// Build a 4-byte table = 0xFF 0x0F 0xF0 0xAA.
	// Bit-count up to each index:
	// idx 0  → 0
	// idx 1  → 1 (bit 0 set)
	// idx 8  → 8 (all of byte 0)
	// idx 9  → 9 (bit 8 set in 0x0F)
	// idx 12 → 12
	// idx 13 → 12 (bit 12 not set in 0x0F)
	// idx 16 → 12 (lower nibble of byte 1 is the four set bits)
	// idx 20 → 12 (low nibble of byte 2 is zero)
	// idx 24 → 16 (high nibble of byte 2 is 4 set)
	// idx 32 → 16 + popcount(0xAA) = 16 + 4 = 20
	table := []byte{0xFF, 0x0F, 0xF0, 0xAA}
	cases := []struct {
		idx, want int
	}{
		{0, 0}, {1, 1}, {8, 8}, {9, 9}, {12, 12}, {13, 12},
		{16, 12}, {20, 12}, {24, 16}, {32, 20},
	}
	for _, c := range cases {
		r := newBitTableReader(t, table)
		got, err := bitTableCountBitsUpTo(c.idx, r)
		if err != nil {
			t.Fatalf("idx %d: %v", c.idx, err)
		}
		if got != c.want {
			t.Fatalf("idx %d: got %d want %d", c.idx, got, c.want)
		}
	}
}

func TestBitTableNextBitSet(t *testing.T) {
	// Same set as the doc example: bits 100011 (bits 0, 1, 5 set in
	// byte 0). One extra byte 0x80 to push a bit far away.
	table := []byte{0b00100011, 0x80} // bit 15 set
	cases := []struct {
		from, want int
	}{
		{-1, 0}, {0, 1}, {1, 5}, {5, 15}, {15, -1},
	}
	for _, c := range cases {
		r := newBitTableReader(t, table)
		got, err := bitTableNextBitSet(c.from, len(table), r)
		if err != nil {
			t.Fatalf("from %d: %v", c.from, err)
		}
		if got != c.want {
			t.Fatalf("from %d: got %d want %d", c.from, got, c.want)
		}
	}
}

func TestBitTablePreviousBitSet(t *testing.T) {
	// 100011 then 0x80 → bits at 0, 1, 5, 15.
	table := []byte{0b00100011, 0x80}
	cases := []struct {
		from, want int
	}{
		{0, -1}, {1, 0}, {5, 1}, {6, 5}, {15, 5}, {16, 15},
	}
	for _, c := range cases {
		r := newBitTableReader(t, table)
		got, err := bitTablePreviousBitSet(c.from, r)
		if err != nil {
			t.Fatalf("from %d: %v", c.from, err)
		}
		if got != c.want {
			t.Fatalf("from %d: got %d want %d", c.from, got, c.want)
		}
	}
}

// TestBitTableRandomized cross-checks the four scan operations
// against a naive bit-by-bit implementation over random tables.
func TestBitTableRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0FFEE))
	for trial := 0; trial < 64; trial++ {
		n := 1 + rng.Intn(17)
		table := make([]byte, n)
		for i := range table {
			table[i] = byte(rng.Intn(256))
		}
		// countBits
		want := 0
		for _, b := range table {
			want += bits.OnesCount8(b)
		}
		r := newBitTableReader(t, table)
		got, err := bitTableCountBits(n, r)
		if err != nil {
			t.Fatalf("countBits trial %d: %v", trial, err)
		}
		if got != want {
			t.Fatalf("countBits trial %d: got %d want %d (table=%x)", trial, got, want, table)
		}

		// isBitSet, countBitsUpTo
		for bit := 0; bit < n*8; bit++ {
			byteI := bit >> 3
			bitInByte := uint(bit & 7)
			wantSet := (table[byteI]>>bitInByte)&1 == 1

			r = newBitTableReader(t, table)
			gotSet, err := bitTableIsBitSet(bit, r)
			if err != nil {
				t.Fatalf("isBitSet trial %d bit %d: %v", trial, bit, err)
			}
			if gotSet != wantSet {
				t.Fatalf("isBitSet trial %d bit %d: got %v want %v (table=%x)", trial, bit, gotSet, wantSet, table)
			}

			// countBitsUpTo(bit): bits strictly before bit.
			wantUpTo := 0
			for i := 0; i < bit; i++ {
				if (table[i>>3]>>uint(i&7))&1 == 1 {
					wantUpTo++
				}
			}
			r = newBitTableReader(t, table)
			gotUpTo, err := bitTableCountBitsUpTo(bit, r)
			if err != nil {
				t.Fatalf("countBitsUpTo trial %d bit %d: %v", trial, bit, err)
			}
			if gotUpTo != wantUpTo {
				t.Fatalf("countBitsUpTo trial %d bit %d: got %d want %d (table=%x)", trial, bit, gotUpTo, wantUpTo, table)
			}
		}

		// nextBitSet / previousBitSet
		for bit := -1; bit < n*8; bit++ {
			wantNext := -1
			for i := bit + 1; i < n*8; i++ {
				if (table[i>>3]>>uint(i&7))&1 == 1 {
					wantNext = i
					break
				}
			}
			r := newBitTableReader(t, table)
			gotNext, err := bitTableNextBitSet(bit, n, r)
			if err != nil {
				t.Fatalf("nextBitSet trial %d bit %d: %v", trial, bit, err)
			}
			if gotNext != wantNext {
				t.Fatalf("nextBitSet trial %d bit %d: got %d want %d (table=%x)", trial, bit, gotNext, wantNext, table)
			}
		}
		for bit := 0; bit <= n*8; bit++ {
			wantPrev := -1
			for i := bit - 1; i >= 0; i-- {
				if (table[i>>3]>>uint(i&7))&1 == 1 {
					wantPrev = i
					break
				}
			}
			r := newBitTableReader(t, table)
			gotPrev, err := bitTablePreviousBitSet(bit, r)
			if err != nil {
				t.Fatalf("previousBitSet trial %d bit %d: %v", trial, bit, err)
			}
			if gotPrev != wantPrev {
				t.Fatalf("previousBitSet trial %d bit %d: got %d want %d (table=%x)", trial, bit, gotPrev, wantPrev, table)
			}
		}
	}
}
