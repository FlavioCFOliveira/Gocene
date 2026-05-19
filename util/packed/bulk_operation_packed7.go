// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked7.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked7 is the hand-unrolled BulkOperation for
// bitsPerValue == 7. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked7, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=7.
type bulkOperationPacked7 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked7 returns the specialised BulkOperation for
// 7 bits per value.
func newBulkOperationPacked7() *bulkOperationPacked7 {
	return &bulkOperationPacked7{bulkOperationPacked: newBulkOperationPacked(7)}
}

// Compile-time guarantee that bulkOperationPacked7 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked7)(nil)

// DecodeLongs decodes 64 seven-bit values from each group of seven
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked7.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked7) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 57)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 50) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 43) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 36) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 29) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 22) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 15) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 8) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 1) & 127)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 51) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 44) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 37) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 30) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 23) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 16) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 9) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 127)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 5) | (block2 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 52) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 45) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 38) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 31) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 17) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 10) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 3) & 127)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 7) << 4) | (block3 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 53) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 46) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 39) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 32) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 25) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 18) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 11) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 4) & 127)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 15) << 3) | (block4 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 54) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 47) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 40) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 33) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 26) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 19) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 12) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 5) & 127)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 31) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 55) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 48) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 41) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 34) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 27) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 20) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 13) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 6) & 127)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 63) << 1) | (block6 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 56) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 49) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 42) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 35) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 28) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 21) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 14) & 127)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 7) & 127)
		valuesOffset++
		values[valuesOffset] = int64(block6 & 127)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 seven-bit values from each group of seven
// input bytes into int64 slots. Literal port of
// BulkOperationPacked7.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked7) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(byte0 >> 1)
		valuesOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte0 & 1) << 6) | (byte1 >> 2))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 3) << 5) | (byte2 >> 3))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 7) << 4) | (byte3 >> 4))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 15) << 3) | (byte4 >> 5))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 31) << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 63) << 1) | (byte6 >> 7))
		valuesOffset++
		values[valuesOffset] = int64(byte6 & 127)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 seven-bit values from each group of
// seven 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked7.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked7) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 57)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 50) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 43) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 36) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 29) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 22) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 15) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 8) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 1) & 127)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1) << 6) | (block1 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 51) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 44) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 37) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 30) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 23) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 16) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 9) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 127)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 5) | (block2 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 52) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 45) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 38) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 31) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 17) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 10) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 3) & 127)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 7) << 4) | (block3 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 53) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 46) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 39) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 32) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 25) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 18) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 11) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 4) & 127)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 15) << 3) | (block4 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 54) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 47) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 40) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 33) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 26) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 19) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 12) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 5) & 127)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 31) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 55) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 48) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 41) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 34) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 27) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 20) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 13) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 6) & 127)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 63) << 1) | (block6 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 56) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 49) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 42) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 35) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 28) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 21) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 14) & 127)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 7) & 127)
		valuesOffset++
		values[valuesOffset] = int32(block6 & 127)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 seven-bit values from each group of
// seven input bytes into int32 slots. Literal port of
// BulkOperationPacked7.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked7) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(byte0 >> 1)
		valuesOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte0 & 1) << 6) | (byte1 >> 2))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 3) << 5) | (byte2 >> 3))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 7) << 4) | (byte3 >> 4))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 15) << 3) | (byte4 >> 5))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 31) << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 63) << 1) | (byte6 >> 7))
		valuesOffset++
		values[valuesOffset] = int32(byte6 & 127)
		valuesOffset++
	}
}
