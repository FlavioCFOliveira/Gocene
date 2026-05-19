// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked2.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked2 is the hand-unrolled BulkOperation for
// bitsPerValue == 2. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked2, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=2.
type bulkOperationPacked2 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked2 returns the specialised BulkOperation for
// 2 bits per value.
func newBulkOperationPacked2() *bulkOperationPacked2 {
	return &bulkOperationPacked2{bulkOperationPacked: newBulkOperationPacked(2)}
}

// Compile-time guarantee that bulkOperationPacked2 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked2)(nil)

// DecodeLongs decodes 32 two-bit values from each 64-bit block into
// int64 slots. Literal port of
// BulkOperationPacked2.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked2) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 62; shift >= 0; shift -= 2 {
			values[valuesOffset] = int64((block >> uint(shift)) & 3)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes 4 two-bit values from each input byte into
// int64 slots. Literal port of
// BulkOperationPacked2.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked2) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((block >> 6) & 3)
		valuesOffset++
		values[valuesOffset] = int64((block >> 4) & 3)
		valuesOffset++
		values[valuesOffset] = int64((block >> 2) & 3)
		valuesOffset++
		values[valuesOffset] = int64(block & 3)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 two-bit values from each 64-bit block
// into int32 slots. Literal port of
// BulkOperationPacked2.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked2) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 62; shift >= 0; shift -= 2 {
			values[valuesOffset] = int32((block >> uint(shift)) & 3)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes 4 two-bit values from each input byte
// into int32 slots. Literal port of
// BulkOperationPacked2.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked2) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((block >> 6) & 3)
		valuesOffset++
		values[valuesOffset] = int32((block >> 4) & 3)
		valuesOffset++
		values[valuesOffset] = int32((block >> 2) & 3)
		valuesOffset++
		values[valuesOffset] = int32(block & 3)
		valuesOffset++
	}
}
