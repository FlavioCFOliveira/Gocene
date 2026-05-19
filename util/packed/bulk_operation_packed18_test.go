// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked18; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked18 against the generic
// bulkOperationPacked(18) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked18_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 32
	const blocksPerIter = 9

	rng := rand.New(rand.NewPCG(0xC0FFEE18, 0xBADF0018))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked18().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(18).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked18_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 4
	const blocksPerIter = 9

	rng := rand.New(rand.NewPCG(0xDEADBE18, 0xFEEDFA18))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerIter)
	want := make([]int64, iterations*valuesPerIter)

	newBulkOperationPacked18().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(18).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked18_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerIter = 32
	const blocksPerIter = 9

	rng := rand.New(rand.NewPCG(0xA5A5A518, 0x5A5A5A18))
	blocks := make([]int64, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked18().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(18).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked18_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerIter = 4
	const blocksPerIter = 9

	rng := rand.New(rand.NewPCG(0x12345618, 0x87654318))
	blocks := make([]byte, iterations*blocksPerIter)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerIter)
	want := make([]int32, iterations*valuesPerIter)

	newBulkOperationPacked18().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(18).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked18_KnownPattern locks in the bit layout
// against the all-zero and all-ones inputs. All-ones across nine bytes
// yields four eighteen-bit values, each equal to 262143 (2^18 - 1).
func TestBulkOperationPacked18_KnownPattern(t *testing.T) {
	const valuesPerIter = 4
	const blocksPerIter = 9

	zeros := make([]byte, blocksPerIter)
	outZ := make([]int64, valuesPerIter)
	newBulkOperationPacked18().DecodeBytes(zeros, 0, outZ, 0, 1)
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
	newBulkOperationPacked18().DecodeBytes(ones, 0, outO, 0, 1)
	for i, v := range outO {
		if v != 262143 {
			t.Fatalf("ones pattern[%d]: got %d, want 262143", i, v)
		}
	}

	outOInts := make([]int32, valuesPerIter)
	newBulkOperationPacked18().DecodeBytesToInts(ones, 0, outOInts, 0, 1)
	for i, v := range outOInts {
		if v != 262143 {
			t.Fatalf("ones pattern ints[%d]: got %d, want 262143", i, v)
		}
	}
}
