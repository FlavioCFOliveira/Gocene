// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked1.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked1 is the hand-unrolled BulkOperation for
// bitsPerValue == 1. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked1, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=1.
type bulkOperationPacked1 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked1 returns the specialised BulkOperation for
// 1 bit per value.
func newBulkOperationPacked1() *bulkOperationPacked1 {
	return &bulkOperationPacked1{bulkOperationPacked: newBulkOperationPacked(1)}
}

// Compile-time guarantee that bulkOperationPacked1 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked1)(nil)

// DecodeLongs decodes 64 one-bit values from each 64-bit block into
// int64 slots. Literal port of
// BulkOperationPacked1.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked1) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 63; shift >= 0; shift -= 1 {
			values[valuesOffset] = int64((block >> uint(shift)) & 1)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes 8 one-bit values from each input byte into
// int64 slots. Literal port of
// BulkOperationPacked1.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked1) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((block >> 7) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 6) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 5) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 4) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 3) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 2) & 1)
		valuesOffset++
		values[valuesOffset] = int64((block >> 1) & 1)
		valuesOffset++
		values[valuesOffset] = int64(block & 1)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 one-bit values from each 64-bit block
// into int32 slots. Literal port of
// BulkOperationPacked1.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked1) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 63; shift >= 0; shift -= 1 {
			values[valuesOffset] = int32((block >> uint(shift)) & 1)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes 8 one-bit values from each input byte
// into int32 slots. Literal port of
// BulkOperationPacked1.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked1) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((block >> 7) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 6) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 5) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 4) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 3) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 2) & 1)
		valuesOffset++
		values[valuesOffset] = int32((block >> 1) & 1)
		valuesOffset++
		values[valuesOffset] = int32(block & 1)
		valuesOffset++
	}
}
