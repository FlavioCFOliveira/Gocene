// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked13; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked13 against the generic
// bulkOperationPacked(13) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked13_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64
	const blocksPerIter = 13

	rng := rand.New(rand.NewPCG(0xC0FFEE13, 0xBADF0013))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked13().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(13).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked13_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8
	const blocksPerIter = 13

	rng := rand.New(rand.NewPCG(0xDEADBE13, 0xFEEDFA13))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked13().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(13).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked13_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 64
	const blocksPerIter = 13

	rng := rand.New(rand.NewPCG(0xA5A5A513, 0x5A5A5A13))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked13().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(13).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked13_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 8
	const blocksPerIter = 13

	rng := rand.New(rand.NewPCG(0x12345613, 0x87654313))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked13().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(13).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked13_KnownPattern locks in the bit layout
// against the all-zero and all-ones inputs. All-ones across thirteen
// bytes yields eight thirteen-bit values, each equal to 8191 (2^13 - 1).
func TestBulkOperationPacked13_KnownPattern(t *testing.T) {
	const valuesPerIter = 8
	const blocksPerIter = 13

	zeros := make([]byte, blocksPerIter)
	outZ := make([]int64, valuesPerIter)
	newBulkOperationPacked13().DecodeBytes(zeros, 0, outZ, 0, 1)
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
	newBulkOperationPacked13().DecodeBytes(ones, 0, outO, 0, 1)
	for i, v := range outO {
		if v != 8191 {
			t.Fatalf("ones pattern[%d]: got %d, want 8191", i, v)
		}
	}

	outOInts := make([]int32, valuesPerIter)
	newBulkOperationPacked13().DecodeBytesToInts(ones, 0, outOInts, 0, 1)
	for i, v := range outOInts {
		if v != 8191 {
			t.Fatalf("ones pattern ints[%d]: got %d, want 8191", i, v)
		}
	}
}
