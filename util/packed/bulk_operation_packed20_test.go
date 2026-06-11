// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked20; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked20 against the generic
// bulkOperationPacked(20) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked20_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xC0FFEE14, 0xBADF0014))
	blocks := make([]int64, iterations*5)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*16)
	want := make([]int64, iterations*16)

	newBulkOperationPacked20().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(20).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked20_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0xDEADBE14, 0xFEEDFA14))
	blocks := make([]byte, iterations*5)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*2)
	want := make([]int64, iterations*2)

	newBulkOperationPacked20().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(20).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked20_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4

	rng := rand.New(rand.NewPCG(0xA5A5A514, 0x5A5A5A14))
	blocks := make([]int64, iterations*5)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*16)
	want := make([]int32, iterations*16)

	newBulkOperationPacked20().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(20).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked20_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32

	rng := rand.New(rand.NewPCG(0x12345614, 0x87654314))
	blocks := make([]byte, iterations*5)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*2)
	want := make([]int32, iterations*2)

	newBulkOperationPacked20().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(20).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked20_KnownPattern locks in the known bit layout
// for an all-ones and all-zero input.
func TestBulkOperationPacked20_KnownPattern(t *testing.T) {
	// All-ones pattern: every output value should be MaxValue(bpv).
	mask := MaxValue(20)
	ones := make([]byte, 5)
	for i := range ones {
		ones[i] = 0xFF
	}
	out := make([]int64, 2)
	newBulkOperationPacked20().DecodeBytes(ones, 0, out, 0, 1)
	for i, v := range out {
		if v != mask {
			t.Fatalf("ones pattern[%d]: got %d, want %d", i, v, mask)
		}
	}

	// All-zero pattern.
	zeros := make([]byte, 5)
	outZ := make([]int64, 2)
	newBulkOperationPacked20().DecodeBytes(zeros, 0, outZ, 0, 1)
	for i, v := range outZ {
		if v != 0 {
			t.Fatalf("zero pattern[%d]: got %d, want 0", i, v)
		}
	}
}
