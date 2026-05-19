// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked12.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked12 is the hand-unrolled BulkOperation for
// bitsPerValue == 12. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked12, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=12.
type bulkOperationPacked12 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked12 returns the specialised BulkOperation for
// 12 bits per value.
func newBulkOperationPacked12() *bulkOperationPacked12 {
	return &bulkOperationPacked12{bulkOperationPacked: newBulkOperationPacked(12)}
}

// Compile-time guarantee that bulkOperationPacked12 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked12)(nil)

// DecodeLongs decodes 16 twelve-bit values from each iteration of
// three 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked12.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked12) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 52)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 40) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 28) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 16) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 4095)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 44) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 32) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 20) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 4095)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 255) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 48) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 36) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 4095)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 4095)
		valuesOffset++
		values[valuesOffset] = int64(block2 & 4095)
		valuesOffset++
	}
}

// DecodeBytes decodes 2 twelve-bit values from each iteration of
// three bytes into int64 slots. Literal port of
// BulkOperationPacked12.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked12) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 4) | (byte1 >> 4))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 15) << 8) | byte2)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 16 twelve-bit values per iteration from
// three 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked12.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked12) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 52)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 40) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 28) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 16) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 4095)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 44) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 32) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 20) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 4095)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 255) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 48) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 36) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 4095)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 4095)
		valuesOffset++
		values[valuesOffset] = int32(block2 & 4095)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 2 twelve-bit values per iteration from
// three bytes into int32 slots. Literal port of
// BulkOperationPacked12.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked12) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 4) | (byte1 >> 4))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 15) << 8) | byte2)
		valuesOffset++
	}
}
