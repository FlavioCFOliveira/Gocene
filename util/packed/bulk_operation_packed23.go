// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked23.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked23 is the hand-unrolled BulkOperation for
// bitsPerValue == 23. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked23, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=23.
type bulkOperationPacked23 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked23 returns the specialised BulkOperation for
// 23 bits per value.
func newBulkOperationPacked23() *bulkOperationPacked23 {
	return &bulkOperationPacked23{bulkOperationPacked: newBulkOperationPacked(23)}
}

// Compile-time guarantee that bulkOperationPacked23 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked23)(nil)

// DecodeLongs decodes 64 twenty-three-bit values from each iteration
// of twenty-three 64-bit blocks into int64 slots. The bit-shift table
// is a literal port of BulkOperationPacked23.decode(long[], int,
// long[], int, int).
func (b *bulkOperationPacked23) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 41)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 18) & 8388607)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 262143) << 5) | (block1 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 36) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 13) & 8388607)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 8191) << 10) | (block2 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 31) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 8) & 8388607)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 255) << 15) | (block3 >> 49))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 26) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 3) & 8388607)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 7) << 20) | (block4 >> 44))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 21) & 8388607)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 2097151) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 39) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 16) & 8388607)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 65535) << 7) | (block6 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 34) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 11) & 8388607)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 2047) << 12) | (block7 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 29) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 6) & 8388607)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 63) << 17) | (block8 >> 47))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 24) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 1) & 8388607)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 1) << 22) | (block9 >> 42))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 19) & 8388607)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 524287) << 4) | (block10 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 37) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 14) & 8388607)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 16383) << 9) | (block11 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 32) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 9) & 8388607)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 511) << 14) | (block12 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 27) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 4) & 8388607)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block12 & 15) << 19) | (block13 >> 45))
		valuesOffset++
		values[valuesOffset] = int64((block13 >> 22) & 8388607)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block13 & 4194303) << 1) | (block14 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 40) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block14 >> 17) & 8388607)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block14 & 131071) << 6) | (block15 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 35) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block15 >> 12) & 8388607)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block15 & 4095) << 11) | (block16 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 30) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block16 >> 7) & 8388607)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block16 & 127) << 16) | (block17 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 25) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block17 >> 2) & 8388607)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block17 & 3) << 21) | (block18 >> 43))
		valuesOffset++
		values[valuesOffset] = int64((block18 >> 20) & 8388607)
		valuesOffset++
		block19 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block18 & 1048575) << 3) | (block19 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block19 >> 38) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block19 >> 15) & 8388607)
		valuesOffset++
		block20 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block19 & 32767) << 8) | (block20 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block20 >> 33) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block20 >> 10) & 8388607)
		valuesOffset++
		block21 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block20 & 1023) << 13) | (block21 >> 51))
		valuesOffset++
		values[valuesOffset] = int64((block21 >> 28) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64((block21 >> 5) & 8388607)
		valuesOffset++
		block22 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block21 & 31) << 18) | (block22 >> 46))
		valuesOffset++
		values[valuesOffset] = int64((block22 >> 23) & 8388607)
		valuesOffset++
		values[valuesOffset] = int64(block22 & 8388607)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 twenty-three-bit values from each iteration
// of twenty-three bytes into int64 slots. Literal port of
// BulkOperationPacked23.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked23) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 15) | (byte1 << 7) | (byte2 >> 1))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 1) << 22) | (byte3 << 14) | (byte4 << 6) | (byte5 >> 2))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 3) << 21) | (byte6 << 13) | (byte7 << 5) | (byte8 >> 3))
		valuesOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte8 & 7) << 20) | (byte9 << 12) | (byte10 << 4) | (byte11 >> 4))
		valuesOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte11 & 15) << 19) | (byte12 << 11) | (byte13 << 3) | (byte14 >> 5))
		valuesOffset++
		byte15 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte17 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte14 & 31) << 18) | (byte15 << 10) | (byte16 << 2) | (byte17 >> 6))
		valuesOffset++
		byte18 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte19 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte20 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte17 & 63) << 17) | (byte18 << 9) | (byte19 << 1) | (byte20 >> 7))
		valuesOffset++
		byte21 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte22 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte20 & 127) << 16) | (byte21 << 8) | byte22)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 twenty-three-bit values per iteration
// from twenty-three 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked23.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked23) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 41)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 18) & 8388607)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 262143) << 5) | (block1 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 36) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 13) & 8388607)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 8191) << 10) | (block2 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 31) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 8) & 8388607)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 255) << 15) | (block3 >> 49))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 26) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 3) & 8388607)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 7) << 20) | (block4 >> 44))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 21) & 8388607)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 2097151) << 2) | (block5 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 39) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 16) & 8388607)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 65535) << 7) | (block6 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 34) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 11) & 8388607)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 2047) << 12) | (block7 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 29) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 6) & 8388607)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 63) << 17) | (block8 >> 47))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 24) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 1) & 8388607)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 1) << 22) | (block9 >> 42))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 19) & 8388607)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 524287) << 4) | (block10 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 37) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 14) & 8388607)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 16383) << 9) | (block11 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 32) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 9) & 8388607)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 511) << 14) | (block12 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 27) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 4) & 8388607)
		valuesOffset++
		block13 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block12 & 15) << 19) | (block13 >> 45))
		valuesOffset++
		values[valuesOffset] = int32((block13 >> 22) & 8388607)
		valuesOffset++
		block14 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block13 & 4194303) << 1) | (block14 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 40) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block14 >> 17) & 8388607)
		valuesOffset++
		block15 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block14 & 131071) << 6) | (block15 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 35) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block15 >> 12) & 8388607)
		valuesOffset++
		block16 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block15 & 4095) << 11) | (block16 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 30) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block16 >> 7) & 8388607)
		valuesOffset++
		block17 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block16 & 127) << 16) | (block17 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 25) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block17 >> 2) & 8388607)
		valuesOffset++
		block18 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block17 & 3) << 21) | (block18 >> 43))
		valuesOffset++
		values[valuesOffset] = int32((block18 >> 20) & 8388607)
		valuesOffset++
		block19 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block18 & 1048575) << 3) | (block19 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block19 >> 38) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block19 >> 15) & 8388607)
		valuesOffset++
		block20 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block19 & 32767) << 8) | (block20 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block20 >> 33) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block20 >> 10) & 8388607)
		valuesOffset++
		block21 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block20 & 1023) << 13) | (block21 >> 51))
		valuesOffset++
		values[valuesOffset] = int32((block21 >> 28) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32((block21 >> 5) & 8388607)
		valuesOffset++
		block22 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block21 & 31) << 18) | (block22 >> 46))
		valuesOffset++
		values[valuesOffset] = int32((block22 >> 23) & 8388607)
		valuesOffset++
		values[valuesOffset] = int32(block22 & 8388607)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 twenty-three-bit values per iteration
// from twenty-three bytes into int32 slots. Literal port of
// BulkOperationPacked23.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked23) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 15) | (byte1 << 7) | (byte2 >> 1))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 1) << 22) | (byte3 << 14) | (byte4 << 6) | (byte5 >> 2))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 3) << 21) | (byte6 << 13) | (byte7 << 5) | (byte8 >> 3))
		valuesOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte8 & 7) << 20) | (byte9 << 12) | (byte10 << 4) | (byte11 >> 4))
		valuesOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte13 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte14 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte11 & 15) << 19) | (byte12 << 11) | (byte13 << 3) | (byte14 >> 5))
		valuesOffset++
		byte15 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte16 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte17 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte14 & 31) << 18) | (byte15 << 10) | (byte16 << 2) | (byte17 >> 6))
		valuesOffset++
		byte18 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte19 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte20 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte17 & 63) << 17) | (byte18 << 9) | (byte19 << 1) | (byte20 >> 7))
		valuesOffset++
		byte21 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte22 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte20 & 127) << 16) | (byte21 << 8) | byte22)
		valuesOffset++
	}
}
