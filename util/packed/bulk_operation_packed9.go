// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked9.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked9 is the hand-unrolled BulkOperation for
// bitsPerValue == 9. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked9, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=9.
type bulkOperationPacked9 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked9 returns the specialised BulkOperation for
// 9 bits per value.
func newBulkOperationPacked9() *bulkOperationPacked9 {
	return &bulkOperationPacked9{bulkOperationPacked: newBulkOperationPacked(9)}
}

// Compile-time guarantee that bulkOperationPacked9 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked9)(nil)

// DecodeLongs decodes 64 nine-bit values from each group of nine
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked9.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked9) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 55)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 46) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 37) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 28) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 19) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 10) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 1) & 511)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 47) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 29) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 20) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 11) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 511)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 7) | (block2 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 48) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 39) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 30) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 21) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 3) & 511)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 7) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 49) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 40) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 31) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 22) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 13) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 4) & 511)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 15) << 5) | (block4 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 50) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 41) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 32) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 23) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 14) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 5) & 511)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 31) << 4) | (block5 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 51) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 42) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 33) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 24) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 15) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 6) & 511)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 63) << 3) | (block6 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 52) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 43) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 34) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 25) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 16) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 7) & 511)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 127) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 53) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 44) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 35) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 26) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 17) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 8) & 511)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 255) << 1) | (block8 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 54) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 45) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 36) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 27) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 18) & 511)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 9) & 511)
		valuesOffset++
		values[valuesOffset] = int64(block8 & 511)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 nine-bit values from each group of nine
// input bytes into int64 slots. Literal port of
// BulkOperationPacked9.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked9) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 1) | (byte1 >> 7))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 127) << 2) | (byte2 >> 6))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 63) << 3) | (byte3 >> 5))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 31) << 4) | (byte4 >> 4))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 15) << 5) | (byte5 >> 3))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 7) << 6) | (byte6 >> 2))
		valuesOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte6 & 3) << 7) | (byte7 >> 1))
		valuesOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte7 & 1) << 8) | byte8)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 nine-bit values from each group of
// nine 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked9.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked9) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 55)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 46) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 37) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 28) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 19) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 10) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 1) & 511)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 47) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 29) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 20) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 11) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 511)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 7) | (block2 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 48) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 39) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 30) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 21) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 3) & 511)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 7) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 49) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 40) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 31) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 22) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 13) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 4) & 511)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 15) << 5) | (block4 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 50) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 41) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 32) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 23) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 14) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 5) & 511)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 31) << 4) | (block5 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 51) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 42) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 33) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 24) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 15) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 6) & 511)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 63) << 3) | (block6 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 52) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 43) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 34) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 25) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 16) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 7) & 511)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 127) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 53) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 44) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 35) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 26) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 17) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 8) & 511)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 255) << 1) | (block8 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 54) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 45) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 36) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 27) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 18) & 511)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 9) & 511)
		valuesOffset++
		values[valuesOffset] = int32(block8 & 511)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 nine-bit values from each group of
// nine input bytes into int32 slots. Literal port of
// BulkOperationPacked9.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked9) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 1) | (byte1 >> 7))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 127) << 2) | (byte2 >> 6))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 63) << 3) | (byte3 >> 5))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 31) << 4) | (byte4 >> 4))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 15) << 5) | (byte5 >> 3))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 7) << 6) | (byte6 >> 2))
		valuesOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte6 & 3) << 7) | (byte7 >> 1))
		valuesOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte7 & 1) << 8) | byte8)
		valuesOffset++
	}
}
