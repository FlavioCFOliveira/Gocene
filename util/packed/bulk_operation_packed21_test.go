// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked21; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked21 against the generic
// bulkOperationPacked(21) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked21_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xC0FFEE15, 0xBADF0015))
	blocks := make([]int64, iterations*21)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*64)
	want := make([]int64, iterations*64)

	newBulkOperationPacked21().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(21).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked21_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0xDEADBE15, 0xFEEDFA15))
	blocks := make([]byte, iterations*21)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*8)
	want := make([]int64, iterations*8)

	newBulkOperationPacked21().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(21).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked21_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xA5A5A515, 0x5A5A5A15))
	blocks := make([]int64, iterations*21)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*64)
	want := make([]int32, iterations*64)

	newBulkOperationPacked21().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(21).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked21_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0x12345615, 0x87654315))
	blocks := make([]byte, iterations*21)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*8)
	want := make([]int32, iterations*8)

	newBulkOperationPacked21().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(21).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked21_KnownPattern locks in the known bit layout
// for an all-ones and all-zero input.
func TestBulkOperationPacked21_KnownPattern(t *testing.T) {
	// All-ones pattern: every output value should be MaxValue(bpv).
	mask := MaxValue(21)
	ones := make([]byte, 21)
	for i := range ones {
		ones[i] = 0xFF
	}
	out := make([]int64, 8)
	newBulkOperationPacked21().DecodeBytes(ones, 0, out, 0, 1)
	for i, v := range out {
		if v != mask {
			t.Fatalf("ones pattern[%d]: got %d, want %d", i, v, mask)
		}
	}

	// All-zero pattern.
	zeros := make([]byte, 21)
	outZ := make([]int64, 8)
	newBulkOperationPacked21().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}
}
