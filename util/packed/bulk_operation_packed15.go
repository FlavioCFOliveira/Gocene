// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked15.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked15 is the hand-unrolled BulkOperation for
// bitsPerValue == 15. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked15, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=15.
type bulkOperationPacked15 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked15 returns the specialised BulkOperation for
// 15 bits per value.
func newBulkOperationPacked15() *bulkOperationPacked15 {
	return &bulkOperationPacked15{bulkOperationPacked: newBulkOperationPacked(15)}
}

// Compile-time guarantee that bulkOperationPacked15 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked15)(nil)

// DecodeLongs decodes 64 fifteen-bit values from each iteration of
// fifteen 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked15.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked15) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 49)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 34) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 19) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 32767)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 11) | (block1 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 23) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 32767)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 255) << 7) | (block2 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 42) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 27) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 32767)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 4095) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 46) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 31) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 16) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 1) & 32767)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 1) << 14) | (block4 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 35) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 20) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 5) & 32767)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 31) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 39) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 24) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 9) & 32767)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 511) << 6) | (block6 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 43) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 28) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 13) & 32767)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 8191) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 47) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 32) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 17) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 2) & 32767)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 3) << 13) | (block8 >> 51))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 36) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 21) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 6) & 32767)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 63) << 9) | (block9 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 40) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 25) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 10) & 32767)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 1023) << 5) | (block10 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 44) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 29) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 14) & 32767)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 16383) << 1) | (block11 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 48) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 33) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 18) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 3) & 32767)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 7) << 12) | (block12 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 37) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 22) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 7) & 32767)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block12 & 127) << 8) | (block13 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 41) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 26) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 11) & 32767)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block13 & 2047) << 4) | (block14 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 45) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 30) & 32767)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 15) & 32767)
		valuesOffset++
		values[valuesOffset] = int64(block14 & 32767)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 fifteen-bit values from each iteration of
// fifteen bytes into int64 slots. Literal port of
// BulkOperationPacked15.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked15) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 7) | (byte1 >> 1))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 1) << 14) | (byte2 << 6) | (byte3 >> 2))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 3) << 13) | (byte4 << 5) | (byte5 >> 3))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 7) << 12) | (byte6 << 4) | (byte7 >> 4))
		valuesOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte7 & 15) << 11) | (byte8 << 3) | (byte9 >> 5))
		valuesOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte9 & 31) << 10) | (byte10 << 2) | (byte11 >> 6))
		valuesOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte11 & 63) << 9) | (byte12 << 1) | (byte13 >> 7))
		valuesOffset++
		byte14 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte13 & 127) << 8) | byte14)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 fifteen-bit values per iteration from
// fifteen 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked15.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked15) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 49)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 34) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 19) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 32767)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 11) | (block1 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 23) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 32767)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 255) << 7) | (block2 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 42) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 27) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 32767)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 4095) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 46) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 31) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 16) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 1) & 32767)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 1) << 14) | (block4 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 35) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 20) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 5) & 32767)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 31) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 39) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 24) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 9) & 32767)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 511) << 6) | (block6 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 43) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 28) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 13) & 32767)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 8191) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 47) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 32) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 17) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 2) & 32767)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 3) << 13) | (block8 >> 51))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 36) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 21) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 6) & 32767)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 63) << 9) | (block9 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 40) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 25) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 10) & 32767)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 1023) << 5) | (block10 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 44) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 29) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 14) & 32767)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 16383) << 1) | (block11 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 48) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 33) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 18) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 3) & 32767)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 7) << 12) | (block12 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 37) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 22) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 7) & 32767)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block12 & 127) << 8) | (block13 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 41) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 26) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 11) & 32767)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block13 & 2047) << 4) | (block14 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 45) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 30) & 32767)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 15) & 32767)
		valuesOffset++
		values[valuesOffset] = int32(block14 & 32767)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 fifteen-bit values per iteration from
// fifteen bytes into int32 slots. Literal port of
// BulkOperationPacked15.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked15) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 7) | (byte1 >> 1))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 1) << 14) | (byte2 << 6) | (byte3 >> 2))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 3) << 13) | (byte4 << 5) | (byte5 >> 3))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 7) << 12) | (byte6 << 4) | (byte7 >> 4))
		valuesOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte7 & 15) << 11) | (byte8 << 3) | (byte9 >> 5))
		valuesOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte9 & 31) << 10) | (byte10 << 2) | (byte11 >> 6))
		valuesOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte11 & 63) << 9) | (byte12 << 1) | (byte13 >> 7))
		valuesOffset++
		byte14 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte13 & 127) << 8) | byte14)
		valuesOffset++
	}
}
