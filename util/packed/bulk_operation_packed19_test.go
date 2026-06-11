// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked19; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked19 against the generic
// bulkOperationPacked(19) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked19_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xC0FFEE13, 0xBADF0013))
	blocks := make([]int64, iterations*19)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*64)
	want := make([]int64, iterations*64)

	newBulkOperationPacked19().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(19).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked19_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0xDEADBE13, 0xFEEDFA13))
	blocks := make([]byte, iterations*19)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*8)
	want := make([]int64, iterations*8)

	newBulkOperationPacked19().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(19).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked19_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xA5A5A513, 0x5A5A5A13))
	blocks := make([]int64, iterations*19)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*64)
	want := make([]int32, iterations*64)

	newBulkOperationPacked19().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(19).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked19_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0x12345613, 0x87654313))
	blocks := make([]byte, iterations*19)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*8)
	want := make([]int32, iterations*8)

	newBulkOperationPacked19().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(19).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked19_KnownPattern locks in the known bit layout
// for an all-ones and all-zero input.
func TestBulkOperationPacked19_KnownPattern(t *testing.T) {
	// All-ones pattern: every output value should be MaxValue(bpv).
	mask := MaxValue(19)
	ones := make([]byte, 19)
	for i := range ones {
		ones[i] = 0xFF
	}
	out := make([]int64, 8)
	newBulkOperationPacked19().DecodeBytes(ones, 0, out, 0, 1)
	for i, v := range out {
		if v != mask {
			t.Fatalf("ones pattern[%d]: got %d, want %d", i, v, mask)
		}
	}

	// All-zero pattern.
	zeros := make([]byte, 19)
	outZ := make([]int64, 8)
	newBulkOperationPacked19().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}
}
