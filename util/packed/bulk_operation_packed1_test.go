// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand/v2"
	"testing"
)

// Lucene 10.4.0 ships no dedicated TestBulkOperationPacked1; the
// generated specialisations are exercised through TestPackedInts and
// the generic BulkOperationPacked path. We cross-validate the
// hand-unrolled bulkOperationPacked1 against the generic
// bulkOperationPacked(1) implementation to prove byte-for-byte
// equivalence across every decode variant.

func TestBulkOperationPacked1_DecodeLongsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerBlock = 64

	rng := rand.New(rand.NewPCG(0xC0FFEE, 0xBADF00D))
	blocks := make([]int64, iterations)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int64, iterations*valuesPerBlock)
	want := make([]int64, iterations*valuesPerBlock)

	newBulkOperationPacked1().DecodeLongs(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(1).DecodeLongs(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongs[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked1_DecodeBytesMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerByte = 8

	rng := rand.New(rand.NewPCG(0xDEADBEEF, 0xFEEDFACE))
	blocks := make([]byte, iterations)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int64, iterations*valuesPerByte)
	want := make([]int64, iterations*valuesPerByte)

	newBulkOperationPacked1().DecodeBytes(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(1).DecodeBytes(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytes[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked1_DecodeLongsToIntsMatchesGeneric(t *testing.T) {
	const iterations = 4
	const valuesPerBlock = 64

	rng := rand.New(rand.NewPCG(0xA5A5A5A5, 0x5A5A5A5A))
	blocks := make([]int64, iterations)
	for i := range blocks {
		blocks[i] = int64(rng.Uint64())
	}

	got := make([]int32, iterations*valuesPerBlock)
	want := make([]int32, iterations*valuesPerBlock)

	newBulkOperationPacked1().DecodeLongsToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(1).DecodeLongsToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeLongsToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBulkOperationPacked1_DecodeBytesToIntsMatchesGeneric(t *testing.T) {
	const iterations = 32
	const valuesPerByte = 8

	rng := rand.New(rand.NewPCG(0x12345678, 0x87654321))
	blocks := make([]byte, iterations)
	for i := range blocks {
		blocks[i] = byte(rng.Uint32())
	}

	got := make([]int32, iterations*valuesPerByte)
	want := make([]int32, iterations*valuesPerByte)

	newBulkOperationPacked1().DecodeBytesToInts(blocks, 0, got, 0, iterations)
	newBulkOperationPacked(1).DecodeBytesToInts(blocks, 0, want, 0, iterations)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DecodeBytesToInts[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

// TestBulkOperationPacked1_KnownPattern locks in the bit layout against
// a fixed input that matches the Java reference's shift sequence: the
// most significant bit is decoded first.
func TestBulkOperationPacked1_KnownPattern(t *testing.T) {
	// 0xAA = 0b10101010 -> 1,0,1,0,1,0,1,0
	// 0x55 = 0b01010101 -> 0,1,0,1,0,1,0,1
	in := []byte{0xAA, 0x55}
	expected := []int64{1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 1, 0, 1}

	out := make([]int64, len(expected))
	newBulkOperationPacked1().DecodeBytes(in, 0, out, 0, len(in))

	for i := range expected {
		if out[i] != expected[i] {
			t.Fatalf("KnownPattern bytes->longs[%d]: got %d, want %d", i, out[i], expected[i])
		}
	}

	outInts := make([]int32, len(expected))
	newBulkOperationPacked1().DecodeBytesToInts(in, 0, outInts, 0, len(in))
	for i := range expected {
		if int64(outInts[i]) != expected[i] {
			t.Fatalf("KnownPattern bytes->ints[%d]: got %d, want %d", i, outInts[i], expected[i])
		}
	}

	// Build a single 64-bit block with the same alternating pattern so the
	// long-decoder path is exercised against a known shift sequence.
	blockBits := uint64(0xAAAAAAAAAAAAAAAA)
	block := int64(blockBits)
	blockExpected := make([]int64, 64)
	for i := 0; i < 64; i += 2 {
		blockExpected[i] = 1
	}
	outBlock := make([]int64, 64)
	newBulkOperationPacked1().DecodeLongs([]int64{block}, 0, outBlock, 0, 1)
	for i := range blockExpected {
		if outBlock[i] != blockExpected[i] {
			t.Fatalf("KnownPattern longs[%d]: got %d, want %d", i, outBlock[i], blockExpected[i])
		}
	}
}
