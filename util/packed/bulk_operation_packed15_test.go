// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked15; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked15 against the generic
// bulkOperationPacked(15) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked15_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64
	const blocksPerIter = 15

	rng := rand.New(rand.NewPCG(0xC0FFEE15, 0xBADF0015))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked15().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(15).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked15_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8
	const blocksPerIter = 15

	rng := rand.New(rand.NewPCG(0xDEADBE15, 0xFEEDFA15))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked15().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(15).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked15_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64
	const blocksPerIter = 15

	rng := rand.New(rand.NewPCG(0xA5A5A515, 0x5A5A5A15))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked15().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(15).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked15_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8
	const blocksPerIter = 15

	rng := rand.New(rand.NewPCG(0x12345615, 0x87654315))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked15().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(15).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked15_KnownPattern locks in the bit layout
// against a fixed 15-byte input. The 120 bits decode MSB-first into
// eight 15-bit values, each set to all-ones (32767) by alternating
// bytes 0xFF / 0xFE / 0xFC / 0xF8 / 0xF0 / 0xE0 / 0xC0 / 0x80 ...
// patterns. We use the simplest invariant: every byte=0xFF yields
// values that fill 15 bits except the highest bit of every group of 8
// (i.e. an alternating mask). To keep the assertion mechanical, we
// instead verify the all-zero input maps to all zeros and the
// all-ones input maps to all 32767.
func TestBulkOperationPacked15_KnownPattern(t *testing.T) {
	const valuesPerIter = 8
	const blocksPerIter = 15

	zeros := make([]byte, blocksPerIter)
	outZ := make([]int64, valuesPerIter)
	newBulkOperationPacked15().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}

	ones := make([]byte, blocksPerIter)
	for i := range ones {
		ones[i] = 0xFF
	}
	outO := make([]int64, valuesPerIter)
	newBulkOperationPacked15().DecodeBytes(ones, 0, outO, 0, 1)
	for i, v := range outO {
		if v != 32767 {
			t.Fatalf("ones pattern[%d]: got %d, want 32767", i, v)
		}
	}

	outOInts := make([]int32, valuesPerIter)
	newBulkOperationPacked15().DecodeBytesToInts(ones, 0, outOInts, 0, 1)
	for i, v := range outOInts {
		if v != 32767 {
			t.Fatalf("ones pattern ints[%d]: got %d, want 32767", i, v)
		}
	}
}
