// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked21.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked21 is the hand-unrolled BulkOperation for
// bitsPerValue == 21. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked21, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=21.
type bulkOperationPacked21 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked21 returns the specialised BulkOperation for
// 21 bits per value.
func newBulkOperationPacked21() *bulkOperationPacked21 {
	return &bulkOperationPacked21{bulkOperationPacked: newBulkOperationPacked(21)}
}

// Compile-time guarantee that bulkOperationPacked21 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked21)(nil)

// DecodeLongs decodes 64 twenty-one-bit values from each iteration of
// twenty-one 64-bit blocks into int64 slots. The bit-shift table is a
// literal port of BulkOperationPacked21.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked21) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 43)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 22) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 1) & 2097151)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1) << 20) | (block1 >> 44))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 23) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 2097151)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 19) | (block2 >> 45))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 3) & 2097151)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 7) << 18) | (block3 >> 46))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 25) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 4) & 2097151)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 15) << 17) | (block4 >> 47))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 26) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 5) & 2097151)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 31) << 16) | (block5 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 27) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 6) & 2097151)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 63) << 15) | (block6 >> 49))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 28) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 7) & 2097151)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 127) << 14) | (block7 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 29) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 8) & 2097151)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 255) << 13) | (block8 >> 51))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 30) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 9) & 2097151)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 511) << 12) | (block9 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 31) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 10) & 2097151)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 1023) << 11) | (block10 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 32) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 11) & 2097151)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 2047) << 10) | (block11 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 33) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 12) & 2097151)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 4095) << 9) | (block12 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 34) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 13) & 2097151)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block12 & 8191) << 8) | (block13 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 35) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 14) & 2097151)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block13 & 16383) << 7) | (block14 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 36) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 15) & 2097151)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block14 & 32767) << 6) | (block15 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 37) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 16) & 2097151)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block15 & 65535) << 5) | (block16 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 38) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 17) & 2097151)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block16 & 131071) << 4) | (block17 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 39) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 18) & 2097151)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block17 & 262143) << 3) | (block18 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block18 >> 40) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block18 >> 19) & 2097151)
		valuesOffset++
		block19 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block18 & 524287) << 2) | (block19 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block19 >> 41) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block19 >> 20) & 2097151)
		valuesOffset++
		block20 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block19 & 1048575) << 1) | (block20 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block20 >> 42) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64((block20 >> 21) & 2097151)
		valuesOffset++
		values[valuesOffset] = int64(block20 & 2097151)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 twenty-one-bit values from each iteration of
// twenty-one bytes into int64 slots. Literal port of
// BulkOperationPacked21.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked21) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 13) | (byte1 << 5) | (byte2 >> 3))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 7) << 18) | (byte3 << 10) | (byte4 << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 63) << 15) | (byte6 << 7) | (byte7 >> 1))
		valuesOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte7 & 1) << 20) | (byte8 << 12) | (byte9 << 4) | (byte10 >> 4))
		valuesOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte10 & 15) << 17) | (byte11 << 9) | (byte12 << 1) | (byte13 >> 7))
		valuesOffset++
		byte14 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte15 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte13 & 127) << 14) | (byte14 << 6) | (byte15 >> 2))
		valuesOffset++
		byte16 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte17 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte18 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte15 & 3) << 19) | (byte16 << 11) | (byte17 << 3) | (byte18 >> 5))
		valuesOffset++
		byte19 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte20 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte18 & 31) << 16) | (byte19 << 8) | byte20)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 twenty-one-bit values per iteration from
// twenty-one 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked21.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked21) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 43)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 22) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 1) & 2097151)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1) << 20) | (block1 >> 44))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 23) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 2097151)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 19) | (block2 >> 45))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 3) & 2097151)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 7) << 18) | (block3 >> 46))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 25) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 4) & 2097151)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 15) << 17) | (block4 >> 47))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 26) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 5) & 2097151)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 31) << 16) | (block5 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 27) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 6) & 2097151)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 63) << 15) | (block6 >> 49))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 28) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 7) & 2097151)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 127) << 14) | (block7 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 29) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 8) & 2097151)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 255) << 13) | (block8 >> 51))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 30) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 9) & 2097151)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 511) << 12) | (block9 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 31) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 10) & 2097151)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 1023) << 11) | (block10 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 32) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 11) & 2097151)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 2047) << 10) | (block11 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 33) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 12) & 2097151)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 4095) << 9) | (block12 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 34) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 13) & 2097151)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block12 & 8191) << 8) | (block13 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 35) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 14) & 2097151)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block13 & 16383) << 7) | (block14 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 36) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 15) & 2097151)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block14 & 32767) << 6) | (block15 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 37) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 16) & 2097151)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block15 & 65535) << 5) | (block16 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 38) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 17) & 2097151)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block16 & 131071) << 4) | (block17 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 39) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 18) & 2097151)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block17 & 262143) << 3) | (block18 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block18 >> 40) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block18 >> 19) & 2097151)
		valuesOffset++
		block19 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block18 & 524287) << 2) | (block19 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block19 >> 41) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block19 >> 20) & 2097151)
		valuesOffset++
		block20 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block19 & 1048575) << 1) | (block20 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block20 >> 42) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32((block20 >> 21) & 2097151)
		valuesOffset++
		values[valuesOffset] = int32(block20 & 2097151)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 twenty-one-bit values per iteration from
// twenty-one bytes into int32 slots. Literal port of
// BulkOperationPacked21.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked21) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 13) | (byte1 << 5) | (byte2 >> 3))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 7) << 18) | (byte3 << 10) | (byte4 << 2) | (byte5 >> 6))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 63) << 15) | (byte6 << 7) | (byte7 >> 1))
		valuesOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte7 & 1) << 20) | (byte8 << 12) | (byte9 << 4) | (byte10 >> 4))
		valuesOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte10 & 15) << 17) | (byte11 << 9) | (byte12 << 1) | (byte13 >> 7))
		valuesOffset++
		byte14 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte15 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte13 & 127) << 14) | (byte14 << 6) | (byte15 >> 2))
		valuesOffset++
		byte16 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte17 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte18 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte15 & 3) << 19) | (byte16 << 11) | (byte17 << 3) | (byte18 >> 5))
		valuesOffset++
		byte19 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte20 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte18 & 31) << 16) | (byte19 << 8) | byte20)
		valuesOffset++
	}
}
