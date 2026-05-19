// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked10.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked10 is the hand-unrolled BulkOperation for
// bitsPerValue == 10. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked10, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=10.
type bulkOperationPacked10 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked10 returns the specialised BulkOperation for
// 10 bits per value.
func newBulkOperationPacked10() *bulkOperationPacked10 {
	return &bulkOperationPacked10{bulkOperationPacked: newBulkOperationPacked(10)}
}

// Compile-time guarantee that bulkOperationPacked10 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked10)(nil)

// DecodeLongs decodes 32 ten-bit values from each iteration of five
// 64-bit blocks into int64 slots. The bit-shift table is a literal
// port of BulkOperationPacked10.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked10) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 54)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 44) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 34) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 24) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 14) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 1023)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 48) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 28) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 18) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 1023)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 255) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 52) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 42) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 32) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 22) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 2) & 1023)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 3) << 8) | (block3 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 46) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 36) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 26) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 16) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 6) & 1023)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 63) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 50) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 40) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 30) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 20) & 1023)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 10) & 1023)
		valuesOffset++
		values[valuesOffset] = int64(block4 & 1023)
		valuesOffset++
	}
}

// DecodeBytes decodes 4 ten-bit values from each iteration of five
// bytes into int64 slots. Literal port of BulkOperationPacked10.
// decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked10) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 2) | (byte1 >> 6))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 63) << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 15) << 6) | (byte3 >> 2))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 3) << 8) | byte4)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 ten-bit values per iteration from
// five 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked10.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked10) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 54)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 44) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 34) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 24) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 14) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 1023)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 48) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 28) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 18) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 1023)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 255) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 52) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 42) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 32) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 22) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 2) & 1023)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 3) << 8) | (block3 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 46) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 36) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 26) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 16) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 6) & 1023)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 63) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 50) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 40) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 30) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 20) & 1023)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 10) & 1023)
		valuesOffset++
		values[valuesOffset] = int32(block4 & 1023)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 4 ten-bit values per iteration from
// five bytes into int32 slots. Literal port of
// BulkOperationPacked10.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked10) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 2) | (byte1 >> 6))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 63) << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 15) << 6) | (byte3 >> 2))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 3) << 8) | byte4)
		valuesOffset++
	}
}
