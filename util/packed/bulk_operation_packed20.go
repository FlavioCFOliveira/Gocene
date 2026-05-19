// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked20.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked20 is the hand-unrolled BulkOperation for
// bitsPerValue == 20. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked20, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=20.
type bulkOperationPacked20 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked20 returns the specialised BulkOperation for
// 20 bits per value.
func newBulkOperationPacked20() *bulkOperationPacked20 {
	return &bulkOperationPacked20{bulkOperationPacked: newBulkOperationPacked(20)}
}

// Compile-time guarantee that bulkOperationPacked20 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked20)(nil)

// DecodeLongs decodes 16 twenty-bit values from each iteration of five
// 64-bit blocks into int64 slots. The bit-shift table is a literal
// port of BulkOperationPacked20.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked20) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 44)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 24) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 1048575)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 16) | (block1 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 28) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 1048575)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 255) << 12) | (block2 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 32) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 1048575)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 4095) << 8) | (block3 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 36) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 16) & 1048575)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 65535) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 40) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 20) & 1048575)
		valuesOffset++
		values[valuesOffset] = int64(block4 & 1048575)
		valuesOffset++
	}
}

// DecodeBytes decodes 2 twenty-bit values from each iteration of five
// bytes into int64 slots. Literal port of
// BulkOperationPacked20.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked20) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 12) | (byte1 << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 15) << 16) | (byte3 << 8) | byte4)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 16 twenty-bit values per iteration from
// five 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked20.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked20) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 44)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 24) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 1048575)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 16) | (block1 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 28) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 1048575)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 255) << 12) | (block2 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 32) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 1048575)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 4095) << 8) | (block3 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 36) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 16) & 1048575)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 65535) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 40) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 20) & 1048575)
		valuesOffset++
		values[valuesOffset] = int32(block4 & 1048575)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 2 twenty-bit values per iteration from
// five bytes into int32 slots. Literal port of
// BulkOperationPacked20.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked20) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 12) | (byte1 << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 15) << 16) | (byte3 << 8) | byte4)
		valuesOffset++
	}
}
