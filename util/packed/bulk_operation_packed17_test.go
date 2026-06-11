// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked17; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked17 against the generic
// bulkOperationPacked(17) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked17_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xC0FFEE11, 0xBADF0011))
	blocks := make([]int64, iterations*17)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*64)
	want := make([]int64, iterations*64)

	newBulkOperationPacked17().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(17).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked17_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0xDEADBE11, 0xFEEDFA11))
	blocks := make([]byte, iterations*17)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*8)
	want := make([]int64, iterations*8)

	newBulkOperationPacked17().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(17).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked17_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xA5A5A511, 0x5A5A5A11))
	blocks := make([]int64, iterations*17)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*64)
	want := make([]int32, iterations*64)

	newBulkOperationPacked17().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(17).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked17_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0x12345611, 0x87654311))
	blocks := make([]byte, iterations*17)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*8)
	want := make([]int32, iterations*8)

	newBulkOperationPacked17().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(17).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked17_KnownPattern locks in the known bit layout
// for an all-ones and all-zero input.
func TestBulkOperationPacked17_KnownPattern(t *testing.T) {
	// All-ones pattern: every output value should be MaxValue(bpv).
	mask := MaxValue(17)
	ones := make([]byte, 17)
	for i := range ones {
		ones[i] = 0xFF
	}
	out := make([]int64, 8)
	newBulkOperationPacked17().DecodeBytes(ones, 0, out, 0, 1)
	for i, v := range out {
		if v != mask {
			t.Fatalf("ones pattern[%d]: got %d, want %d", i, v, mask)
		}
	}

	// All-zero pattern.
	zeros := make([]byte, 17)
	outZ := make([]int64, 8)
	newBulkOperationPacked17().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}
}
