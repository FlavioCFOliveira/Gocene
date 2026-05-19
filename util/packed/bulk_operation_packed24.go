// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked24.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked24 is the hand-unrolled BulkOperation for
// bitsPerValue == 24. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked24, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=24.
type bulkOperationPacked24 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked24 returns the specialised BulkOperation for
// 24 bits per value.
func newBulkOperationPacked24() *bulkOperationPacked24 {
	return &bulkOperationPacked24{bulkOperationPacked: newBulkOperationPacked(24)}
}

// Compile-time guarantee that bulkOperationPacked24 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked24)(nil)

// DecodeLongs decodes 8 twenty-four-bit values from each iteration of
// three 64-bit blocks into int64 slots. The bit-shift table is a
// literal port of BulkOperationPacked24.decode(long[], int, long[],
// int, int).
func (b *bulkOperationPacked24) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 40)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 16) & 16777215)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 65535) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 32) & 16777215)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 16777215)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 255) << 16) | (block2 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 16777215)
		valuesOffset++
		values[valuesOffset] = int64(block2 & 16777215)
		valuesOffset++
	}
}

// DecodeBytes decodes 1 twenty-four-bit value per iteration from three
// bytes into int64 slots. Literal port of
// BulkOperationPacked24.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked24) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 16) | (byte1 << 8) | byte2)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 8 twenty-four-bit values per iteration
// from three 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked24.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked24) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 40)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 16) & 16777215)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 65535) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 32) & 16777215)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 16777215)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 255) << 16) | (block2 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 16777215)
		valuesOffset++
		values[valuesOffset] = int32(block2 & 16777215)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 1 twenty-four-bit value per iteration
// from three bytes into int32 slots. Literal port of
// BulkOperationPacked24.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked24) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 16) | (byte1 << 8) | byte2)
		valuesOffset++
	}
}
