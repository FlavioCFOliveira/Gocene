// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked14.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked14 is the hand-unrolled BulkOperation for
// bitsPerValue == 14. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked14, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=14.
type bulkOperationPacked14 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked14 returns the specialised BulkOperation for
// 14 bits per value.
func newBulkOperationPacked14() *bulkOperationPacked14 {
	return &bulkOperationPacked14{bulkOperationPacked: newBulkOperationPacked(14)}
}

// Compile-time guarantee that bulkOperationPacked14 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked14)(nil)

// DecodeLongs decodes 32 fourteen-bit values from each iteration of
// seven 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked14.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked14) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 50)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 36) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 22) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 8) & 16383)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 255) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 44) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 30) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 16) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 16383)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 12) | (block2 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 38) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 10) & 16383)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 1023) << 4) | (block3 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 46) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 32) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 18) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 4) & 16383)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 15) << 10) | (block4 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 40) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 26) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 12) & 16383)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 4095) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 48) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 34) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 20) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 6) & 16383)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 63) << 8) | (block6 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 42) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 28) & 16383)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 14) & 16383)
		valuesOffset++
		values[valuesOffset] = int64(block6 & 16383)
		valuesOffset++
	}
}

// DecodeBytes decodes 4 fourteen-bit values from each iteration of
// seven bytes into int64 slots. Literal port of
// BulkOperationPacked14.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked14) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 6) | (byte1 >> 2))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 3) << 12) | (byte2 << 4) | (byte3 >> 4))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 15) << 10) | (byte4 << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 63) << 8) | byte6)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 fourteen-bit values per iteration from
// seven 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked14.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked14) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 50)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 36) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 22) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 8) & 16383)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 255) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 44) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 30) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 16) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 16383)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 12) | (block2 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 38) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 10) & 16383)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 1023) << 4) | (block3 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 46) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 32) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 18) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 4) & 16383)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 15) << 10) | (block4 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 40) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 26) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 12) & 16383)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 4095) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 48) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 34) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 20) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 6) & 16383)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 63) << 8) | (block6 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 42) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 28) & 16383)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 14) & 16383)
		valuesOffset++
		values[valuesOffset] = int32(block6 & 16383)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 4 fourteen-bit values per iteration from
// seven bytes into int32 slots. Literal port of
// BulkOperationPacked14.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked14) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 6) | (byte1 >> 2))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 3) << 12) | (byte2 << 4) | (byte3 >> 4))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 15) << 10) | (byte4 << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 63) << 8) | byte6)
		valuesOffset++
	}
}
