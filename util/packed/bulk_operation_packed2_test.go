// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked2; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked2 against the generic
// bulkOperationPacked(2) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked2_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xC0FFEE02, 0xBADF0002))
	blocks := make([]int64, iterations*1)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*32)
	want := make([]int64, iterations*32)

	newBulkOperationPacked2().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(2).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked2_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0xDEADBE02, 0xFEEDFA02))
	blocks := make([]byte, iterations*1)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*4)
	want := make([]int64, iterations*4)

	newBulkOperationPacked2().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(2).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked2_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xA5A5A502, 0x5A5A5A02))
	blocks := make([]int64, iterations*1)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*32)
	want := make([]int32, iterations*32)

	newBulkOperationPacked2().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(2).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked2_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0x12345602, 0x87654302))
	blocks := make([]byte, iterations*1)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*4)
	want := make([]int32, iterations*4)

	newBulkOperationPacked2().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(2).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked2_KnownPattern locks in the known bit layout
// for an all-ones and all-zero input.
func TestBulkOperationPacked2_KnownPattern(t *testing.T) {
	// All-ones pattern: every output value should be MaxValue(bpv).
	mask := MaxValue(2)
	ones := make([]byte, 1)
	for i := range ones {
		ones[i] = 0xFF
	}
	out := make([]int64, 4)
	newBulkOperationPacked2().DecodeBytes(ones, 0, out, 0, 1)
	for i, v := range out {
		if v != mask {
			t.Fatalf("ones pattern[%d]: got %d, want %d", i, v, mask)
		}
	}

	// All-zero pattern.
	zeros := make([]byte, 1)
	outZ := make([]int64, 4)
	newBulkOperationPacked2().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}
}
