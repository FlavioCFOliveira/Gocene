// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked11.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked11 is the hand-unrolled BulkOperation for
// bitsPerValue == 11. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked11, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=11.
type bulkOperationPacked11 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked11 returns the specialised BulkOperation for
// 11 bits per value.
func newBulkOperationPacked11() *bulkOperationPacked11 {
	return &bulkOperationPacked11{bulkOperationPacked: newBulkOperationPacked(11)}
}

// Compile-time guarantee that bulkOperationPacked11 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked11)(nil)

// DecodeLongs decodes 64 eleven-bit values from each iteration of
// eleven 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked11.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked11) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 53)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 42) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 31) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 20) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 9) & 2047)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 511) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 51) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 40) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 29) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 18) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 7) & 2047)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 127) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 49) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 38) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 27) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 16) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 5) & 2047)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 31) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 47) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 36) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 25) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 14) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 3) & 2047)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 7) << 8) | (block4 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 45) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 34) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 23) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 12) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 1) & 2047)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 1) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 43) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 32) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 21) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 10) & 2047)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 1023) << 1) | (block6 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 52) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 41) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 30) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 19) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 8) & 2047)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 255) << 3) | (block7 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 50) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 39) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 28) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 17) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 6) & 2047)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 63) << 5) | (block8 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 48) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 37) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 26) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 15) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 4) & 2047)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 15) << 7) | (block9 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 46) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 35) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 24) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 13) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 2) & 2047)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 3) << 9) | (block10 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 44) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 33) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 22) & 2047)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 11) & 2047)
		valuesOffset++
		values[valuesOffset] = int64(block10 & 2047)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 eleven-bit values from each iteration of
// eleven bytes into int64 slots. Literal port of
// BulkOperationPacked11.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked11) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 3) | (byte1 >> 5))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 31) << 6) | (byte2 >> 2))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 3) << 9) | (byte3 << 1) | (byte4 >> 7))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 127) << 4) | (byte5 >> 4))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 15) << 7) | (byte6 >> 1))
		valuesOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte6 & 1) << 10) | (byte7 << 2) | (byte8 >> 6))
		valuesOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte8 & 63) << 5) | (byte9 >> 3))
		valuesOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte9 & 7) << 8) | byte10)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 eleven-bit values per iteration from
// eleven 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked11.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked11) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 53)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 42) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 31) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 20) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 9) & 2047)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 511) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 51) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 40) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 29) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 18) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 7) & 2047)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 127) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 49) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 38) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 27) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 16) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 5) & 2047)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 31) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 47) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 36) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 25) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 14) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 3) & 2047)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 7) << 8) | (block4 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 45) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 34) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 23) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 12) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 1) & 2047)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 1) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 43) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 32) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 21) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 10) & 2047)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 1023) << 1) | (block6 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 52) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 41) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 30) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 19) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 8) & 2047)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 255) << 3) | (block7 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 50) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 39) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 28) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 17) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 6) & 2047)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 63) << 5) | (block8 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 48) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 37) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 26) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 15) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 4) & 2047)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 15) << 7) | (block9 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 46) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 35) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 24) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 13) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 2) & 2047)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 3) << 9) | (block10 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 44) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 33) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 22) & 2047)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 11) & 2047)
		valuesOffset++
		values[valuesOffset] = int32(block10 & 2047)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 eleven-bit values per iteration from
// eleven bytes into int32 slots. Literal port of
// BulkOperationPacked11.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked11) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 3) | (byte1 >> 5))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 31) << 6) | (byte2 >> 2))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 3) << 9) | (byte3 << 1) | (byte4 >> 7))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 127) << 4) | (byte5 >> 4))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 15) << 7) | (byte6 >> 1))
		valuesOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte6 & 1) << 10) | (byte7 << 2) | (byte8 >> 6))
		valuesOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte8 & 63) << 5) | (byte9 >> 3))
		valuesOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte9 & 7) << 8) | byte10)
		valuesOffset++
	}
}
