// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked17.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked17 is the hand-unrolled BulkOperation for
// bitsPerValue == 17. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked17, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=17.
type bulkOperationPacked17 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked17 returns the specialised BulkOperation for
// 17 bits per value.
func newBulkOperationPacked17() *bulkOperationPacked17 {
	return &bulkOperationPacked17{bulkOperationPacked: newBulkOperationPacked(17)}
}

// Compile-time guarantee that bulkOperationPacked17 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked17)(nil)

// DecodeLongs decodes 64 seventeen-bit values per iteration from
// seventeen 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked17.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked17) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 47)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 30) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 13) & 131071)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 8191) << 4) | (block1 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 43) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 26) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 9) & 131071)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 511) << 8) | (block2 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 39) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 22) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 5) & 131071)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 31) << 12) | (block3 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 35) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 18) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 1) & 131071)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 1) << 16) | (block4 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 31) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 14) & 131071)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 16383) << 3) | (block5 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 44) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 27) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 10) & 131071)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 1023) << 7) | (block6 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 40) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 23) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 6) & 131071)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 63) << 11) | (block7 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 36) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 19) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 2) & 131071)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 3) << 15) | (block8 >> 49))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 32) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 15) & 131071)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 32767) << 2) | (block9 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 45) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 28) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 11) & 131071)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 2047) << 6) | (block10 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 41) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 24) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 7) & 131071)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 127) << 10) | (block11 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 37) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 20) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 3) & 131071)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 7) << 14) | (block12 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 33) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 16) & 131071)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block12 & 65535) << 1) | (block13 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 46) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 29) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 12) & 131071)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block13 & 4095) << 5) | (block14 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 42) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 25) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 8) & 131071)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block14 & 255) << 9) | (block15 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 38) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 21) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 4) & 131071)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block15 & 15) << 13) | (block16 >> 51))
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 34) & 131071)
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 17) & 131071)
		valuesOffset++
		values[valuesOffset] = int64(block16 & 131071)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 seventeen-bit values per iteration from
// seventeen bytes into int64 slots. Literal port of
// BulkOperationPacked17.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked17) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 9) | (byte1 << 1) | (byte2 >> 7))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 127) << 10) | (byte3 << 2) | (byte4 >> 6))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 63) << 11) | (byte5 << 3) | (byte6 >> 5))
		valuesOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte6 & 31) << 12) | (byte7 << 4) | (byte8 >> 4))
		valuesOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte8 & 15) << 13) | (byte9 << 5) | (byte10 >> 3))
		valuesOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte10 & 7) << 14) | (byte11 << 6) | (byte12 >> 2))
		valuesOffset++
		byte13 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte12 & 3) << 15) | (byte13 << 7) | (byte14 >> 1))
		valuesOffset++
		byte15 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte14 & 1) << 16) | (byte15 << 8) | byte16)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 seventeen-bit values per iteration
// from seventeen 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked17.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked17) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 47)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 30) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 13) & 131071)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 8191) << 4) | (block1 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 43) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 26) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 9) & 131071)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 511) << 8) | (block2 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 39) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 22) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 5) & 131071)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 31) << 12) | (block3 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 35) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 18) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 1) & 131071)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 1) << 16) | (block4 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 31) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 14) & 131071)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 16383) << 3) | (block5 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 44) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 27) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 10) & 131071)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 1023) << 7) | (block6 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 40) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 23) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 6) & 131071)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 63) << 11) | (block7 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 36) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 19) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 2) & 131071)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 3) << 15) | (block8 >> 49))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 32) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 15) & 131071)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 32767) << 2) | (block9 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 45) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 28) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 11) & 131071)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 2047) << 6) | (block10 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 41) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 24) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 7) & 131071)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 127) << 10) | (block11 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 37) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 20) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 3) & 131071)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 7) << 14) | (block12 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 33) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 16) & 131071)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block12 & 65535) << 1) | (block13 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 46) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 29) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 12) & 131071)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block13 & 4095) << 5) | (block14 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 42) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 25) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 8) & 131071)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block14 & 255) << 9) | (block15 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 38) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 21) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 4) & 131071)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block15 & 15) << 13) | (block16 >> 51))
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 34) & 131071)
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 17) & 131071)
		valuesOffset++
		values[valuesOffset] = int32(block16 & 131071)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 seventeen-bit values per iteration from
// seventeen bytes into int32 slots. Literal port of
// BulkOperationPacked17.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked17) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 9) | (byte1 << 1) | (byte2 >> 7))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 127) << 10) | (byte3 << 2) | (byte4 >> 6))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 63) << 11) | (byte5 << 3) | (byte6 >> 5))
		valuesOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte6 & 31) << 12) | (byte7 << 4) | (byte8 >> 4))
		valuesOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte8 & 15) << 13) | (byte9 << 5) | (byte10 >> 3))
		valuesOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte10 & 7) << 14) | (byte11 << 6) | (byte12 >> 2))
		valuesOffset++
		byte13 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte12 & 3) << 15) | (byte13 << 7) | (byte14 >> 1))
		valuesOffset++
		byte15 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte14 & 1) << 16) | (byte15 << 8) | byte16)
		valuesOffset++
	}
}
