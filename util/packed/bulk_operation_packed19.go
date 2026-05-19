// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked19.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked19 is the hand-unrolled BulkOperation for
// bitsPerValue == 19. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked19, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=19.
type bulkOperationPacked19 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked19 returns the specialised BulkOperation for
// 19 bits per value.
func newBulkOperationPacked19() *bulkOperationPacked19 {
	return &bulkOperationPacked19{bulkOperationPacked: newBulkOperationPacked(19)}
}

// Compile-time guarantee that bulkOperationPacked19 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked19)(nil)

// DecodeLongs decodes 64 nineteen-bit values from each iteration of
// nineteen 64-bit blocks into int64 slots. The bit-shift table is a
// literal port of BulkOperationPacked19.decode(long[], int, long[],
// int, int).
func (b *bulkOperationPacked19) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 45)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 26) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 7) & 524287)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 127) << 12) | (block1 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 33) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 14) & 524287)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 16383) << 5) | (block2 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 40) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 21) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 2) & 524287)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 3) << 17) | (block3 >> 47))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 28) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 9) & 524287)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 511) << 10) | (block4 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 35) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 16) & 524287)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 65535) << 3) | (block5 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 42) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 23) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 4) & 524287)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 15) << 15) | (block6 >> 49))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 30) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 11) & 524287)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 2047) << 8) | (block7 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 37) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 18) & 524287)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 262143) << 1) | (block8 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 44) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 25) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 6) & 524287)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 63) << 13) | (block9 >> 51))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 32) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 13) & 524287)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 8191) << 6) | (block10 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 39) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 20) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 1) & 524287)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 1) << 18) | (block11 >> 46))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 27) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 8) & 524287)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 255) << 11) | (block12 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 34) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 15) & 524287)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block12 & 32767) << 4) | (block13 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 41) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 22) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 3) & 524287)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block13 & 7) << 16) | (block14 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 29) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 10) & 524287)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block14 & 1023) << 9) | (block15 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 36) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 17) & 524287)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block15 & 131071) << 2) | (block16 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 43) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 24) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 5) & 524287)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block16 & 31) << 14) | (block17 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 31) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 12) & 524287)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block17 & 4095) << 7) | (block18 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block18 >> 38) & 524287)
		valuesOffset++
		values[valuesOffset] = int64((block18 >> 19) & 524287)
		valuesOffset++
		values[valuesOffset] = int64(block18 & 524287)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 nineteen-bit values from each iteration of
// nineteen bytes into int64 slots. Literal port of
// BulkOperationPacked19.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked19) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 11) | (byte1 << 3) | (byte2 >> 5))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 31) << 14) | (byte3 << 6) | (byte4 >> 2))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 3) << 17) | (byte5 << 9) | (byte6 << 1) | (byte7 >> 7))
		valuesOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte7 & 127) << 12) | (byte8 << 4) | (byte9 >> 4))
		valuesOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte9 & 15) << 15) | (byte10 << 7) | (byte11 >> 1))
		valuesOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte11 & 1) << 18) | (byte12 << 10) | (byte13 << 2) | (byte14 >> 6))
		valuesOffset++
		byte15 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte14 & 63) << 13) | (byte15 << 5) | (byte16 >> 3))
		valuesOffset++
		byte17 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte18 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte16 & 7) << 16) | (byte17 << 8) | byte18)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 nineteen-bit values per iteration
// from nineteen 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked19.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked19) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 45)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 26) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 7) & 524287)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 127) << 12) | (block1 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 33) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 14) & 524287)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 16383) << 5) | (block2 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 40) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 21) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 2) & 524287)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 3) << 17) | (block3 >> 47))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 28) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 9) & 524287)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 511) << 10) | (block4 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 35) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 16) & 524287)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 65535) << 3) | (block5 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 42) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 23) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 4) & 524287)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 15) << 15) | (block6 >> 49))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 30) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 11) & 524287)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 2047) << 8) | (block7 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 37) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 18) & 524287)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 262143) << 1) | (block8 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 44) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 25) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 6) & 524287)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 63) << 13) | (block9 >> 51))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 32) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 13) & 524287)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 8191) << 6) | (block10 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 39) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 20) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 1) & 524287)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 1) << 18) | (block11 >> 46))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 27) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 8) & 524287)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 255) << 11) | (block12 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 34) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 15) & 524287)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block12 & 32767) << 4) | (block13 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 41) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 22) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 3) & 524287)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block13 & 7) << 16) | (block14 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 29) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 10) & 524287)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block14 & 1023) << 9) | (block15 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 36) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 17) & 524287)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block15 & 131071) << 2) | (block16 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 43) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 24) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 5) & 524287)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block16 & 31) << 14) | (block17 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 31) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 12) & 524287)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block17 & 4095) << 7) | (block18 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block18 >> 38) & 524287)
		valuesOffset++
		values[valuesOffset] = int32((block18 >> 19) & 524287)
		valuesOffset++
		values[valuesOffset] = int32(block18 & 524287)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 nineteen-bit values per iteration from
// nineteen bytes into int32 slots. Literal port of
// BulkOperationPacked19.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked19) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 11) | (byte1 << 3) | (byte2 >> 5))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 31) << 14) | (byte3 << 6) | (byte4 >> 2))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 3) << 17) | (byte5 << 9) | (byte6 << 1) | (byte7 >> 7))
		valuesOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte7 & 127) << 12) | (byte8 << 4) | (byte9 >> 4))
		valuesOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte9 & 15) << 15) | (byte10 << 7) | (byte11 >> 1))
		valuesOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte11 & 1) << 18) | (byte12 << 10) | (byte13 << 2) | (byte14 >> 6))
		valuesOffset++
		byte15 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte14 & 63) << 13) | (byte15 << 5) | (byte16 >> 3))
		valuesOffset++
		byte17 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte18 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte16 & 7) << 16) | (byte17 << 8) | byte18)
		valuesOffset++
	}
}
