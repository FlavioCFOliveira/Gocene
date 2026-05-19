// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked5; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked5 against the generic
// bulkOperationPacked(5) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked5_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64

	rng := rand.New(rand.NewPCG(0xC0FFEE5, 0xBADF0055))
	blocks := make([]int64, iterations*5)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked5().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(5).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked5_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8

	rng := rand.New(rand.NewPCG(0xDEADBEE5, 0xFEEDFAC5))
	blocks := make([]byte, iterations*5)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked5().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(5).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked5_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64

	rng := rand.New(rand.NewPCG(0xA5A5A505, 0x5A5A5A05))
	blocks := make([]int64, iterations*5)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked5().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(5).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked5_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8

	rng := rand.New(rand.NewPCG(0x12345605, 0x87654305))
	blocks := make([]byte, iterations*5)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked5().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(5).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked5_KnownPattern locks in the bit layout
// against a fixed 5-byte input. The 40 bits decode MSB-first into
// eight five-bit values alternating 31/0:
//
//	bytes:  0xF8 0x3E 0x0F 0x83 0xE0
//	bits:   11111000 00111110 00001111 10000011 11100000
//	groups: 11111 00000 11111 00000 11111 00000 11111 00000
//	values:    31     0    31     0    31     0    31     0
func TestBulkOperationPacked5_KnownPattern(t *testing.T) {
	in := []byte{0xF8, 0x3E, 0x0F, 0x83, 0xE0}
	expected := []int64{31, 0, 31, 0, 31, 0, 31, 0}

	out := make([]int64, len(expected))
	newBulkOperationPacked5().DecodeBytes(in, 0, out, 0, 1)
	for i := range expected {
		if out[i] != expected[i] {
			t.Fatalf("KnownPattern bytes->longs[%d]: got %d, want %d", i, out[i], expected[i])
		}
	}

	outInts := make([]int32, len(expected))
	newBulkOperationPacked5().DecodeBytesToInts(in, 0, outInts, 0, 1)
	for i := range expected {
		if int64(outInts[i]) != expected[i] {
			t.Fatalf("KnownPattern bytes->ints[%d]: got %d, want %d", i, outInts[i], expected[i])
		}
	}
}
