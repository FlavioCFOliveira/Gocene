// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked4.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked4 is the hand-unrolled BulkOperation for
// bitsPerValue == 4. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked4, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=4.
type bulkOperationPacked4 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked4 returns the specialised BulkOperation for
// 4 bits per value.
func newBulkOperationPacked4() *bulkOperationPacked4 {
	return &bulkOperationPacked4{bulkOperationPacked: newBulkOperationPacked(4)}
}

// Compile-time guarantee that bulkOperationPacked4 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked4)(nil)

// DecodeLongs decodes 16 four-bit values from each 64-bit block into
// int64 slots. Literal port of
// BulkOperationPacked4.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked4) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 60; shift >= 0; shift -= 4 {
			values[valuesOffset] = int64((block >> uint(shift)) & 15)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes 2 four-bit values from each input byte into
// int64 slots. Literal port of
// BulkOperationPacked4.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked4) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((block >> 4) & 15)
		valuesOffset++
		values[valuesOffset] = int64(block & 15)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 16 four-bit values from each 64-bit block
// into int32 slots. Literal port of
// BulkOperationPacked4.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked4) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 60; shift >= 0; shift -= 4 {
			values[valuesOffset] = int32((block >> uint(shift)) & 15)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes 2 four-bit values from each input byte
// into int32 slots. Literal port of
// BulkOperationPacked4.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked4) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		block := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((block >> 4) & 15)
		valuesOffset++
		values[valuesOffset] = int32(block & 15)
		valuesOffset++
	}
}
