// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked6; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked6 against the generic
// bulkOperationPacked(6) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked6_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 32

	rng := rand.New(rand.NewPCG(0xC0FFEE6, 0xBADF0066))
	blocks := make([]int64, iterations*3)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked6().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(6).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked6_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 4

	rng := rand.New(rand.NewPCG(0xDEADBEE6, 0xFEEDFAC6))
	blocks := make([]byte, iterations*3)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked6().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(6).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked6_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 32

	rng := rand.New(rand.NewPCG(0xA5A5A506, 0x5A5A5A06))
	blocks := make([]int64, iterations*3)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked6().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(6).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked6_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 4

	rng := rand.New(rand.NewPCG(0x12345606, 0x87654306))
	blocks := make([]byte, iterations*3)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked6().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(6).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked6_KnownPattern locks in the bit layout
// against a fixed 3-byte input. The 24 bits decode MSB-first into four
// six-bit values: 0xFC=0b11111100, 0x0F=0b00001111, 0xC0=0b11000000.
//
//	bits:  111111 000000 111111 000000
//	value:     63      0     63      0
func TestBulkOperationPacked6_KnownPattern(t *testing.T) {
	in := []byte{0xFC, 0x0F, 0xC0}
	expected := []int64{63, 0, 63, 0}

	out := make([]int64, len(expected))
	newBulkOperationPacked6().DecodeBytes(in, 0, out, 0, 1)
	for i := range expected {
		if out[i] != expected[i] {
			t.Fatalf("KnownPattern bytes->longs[%d]: got %d, want %d", i, out[i], expected[i])
		}
	}

	outInts := make([]int32, len(expected))
	newBulkOperationPacked6().DecodeBytesToInts(in, 0, outInts, 0, 1)
	for i := range expected {
		if int64(outInts[i]) != expected[i] {
			t.Fatalf("KnownPattern bytes->ints[%d]: got %d, want %d", i, outInts[i], expected[i])
		}
	}
}
