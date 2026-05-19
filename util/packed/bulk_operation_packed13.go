// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked13.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked13 is the hand-unrolled BulkOperation for
// bitsPerValue == 13. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked13, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=13.
type bulkOperationPacked13 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked13 returns the specialised BulkOperation for
// 13 bits per value.
func newBulkOperationPacked13() *bulkOperationPacked13 {
	return &bulkOperationPacked13{bulkOperationPacked: newBulkOperationPacked(13)}
}

// Compile-time guarantee that bulkOperationPacked13 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked13)(nil)

// DecodeLongs decodes 64 thirteen-bit values from each iteration of
// thirteen 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked13.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked13) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 51)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 38) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 25) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 12) & 8191)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 4095) << 1) | (block1 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 50) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 37) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 24) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 11) & 8191)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 2047) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 49) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 36) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 23) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 10) & 8191)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 1023) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 48) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 35) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 22) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 9) & 8191)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 511) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 47) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 34) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 21) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 8) & 8191)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 255) << 5) | (block5 >> 59))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 46) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 33) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 20) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 7) & 8191)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 127) << 6) | (block6 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 45) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 32) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 19) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 6) & 8191)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 63) << 7) | (block7 >> 57))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 44) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 31) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 18) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 5) & 8191)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 31) << 8) | (block8 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 43) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 30) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 17) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 4) & 8191)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 15) << 9) | (block9 >> 55))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 42) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 29) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 16) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 3) & 8191)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 7) << 10) | (block10 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 41) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 28) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 15) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 2) & 8191)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block10 & 3) << 11) | (block11 >> 53))
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 40) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 27) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 14) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block11 >> 1) & 8191)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block11 & 1) << 12) | (block12 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 39) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 26) & 8191)
		valuesOffset++
		values[valuesOffset] = int64((block12 >> 13) & 8191)
		valuesOffset++
		values[valuesOffset] = int64(block12 & 8191)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 thirteen-bit values from each iteration of
// thirteen bytes into int64 slots. Literal port of
// BulkOperationPacked13.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked13) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 5) | (byte1 >> 3))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 7) << 10) | (byte2 << 2) | (byte3 >> 6))
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 63) << 7) | (byte4 >> 1))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 1) << 12) | (byte5 << 4) | (byte6 >> 4))
		valuesOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte6 & 15) << 9) | (byte7 << 1) | (byte8 >> 7))
		valuesOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte8 & 127) << 6) | (byte9 >> 2))
		valuesOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte9 & 3) << 11) | (byte10 << 3) | (byte11 >> 5))
		valuesOffset++
		byte12 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte11 & 31) << 8) | byte12)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 thirteen-bit values per iteration from
// thirteen 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked13.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked13) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 51)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 38) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 25) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 12) & 8191)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 4095) << 1) | (block1 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 50) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 37) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 24) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 11) & 8191)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 2047) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 49) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 36) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 23) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 10) & 8191)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 1023) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 48) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 35) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 22) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 9) & 8191)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 511) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 47) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 34) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 21) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 8) & 8191)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 255) << 5) | (block5 >> 59))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 46) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 33) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 20) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 7) & 8191)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 127) << 6) | (block6 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 45) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 32) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 19) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 6) & 8191)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 63) << 7) | (block7 >> 57))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 44) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 31) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 18) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 5) & 8191)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 31) << 8) | (block8 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 43) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 30) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 17) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 4) & 8191)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 15) << 9) | (block9 >> 55))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 42) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 29) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 16) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 3) & 8191)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 7) << 10) | (block10 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 41) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 28) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 15) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 2) & 8191)
		valuesOffset++
		block11 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block10 & 3) << 11) | (block11 >> 53))
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 40) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 27) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 14) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block11 >> 1) & 8191)
		valuesOffset++
		block12 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block11 & 1) << 12) | (block12 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 39) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 26) & 8191)
		valuesOffset++
		values[valuesOffset] = int32((block12 >> 13) & 8191)
		valuesOffset++
		values[valuesOffset] = int32(block12 & 8191)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 thirteen-bit values per iteration from
// thirteen bytes into int32 slots. Literal port of
// BulkOperationPacked13.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked13) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 5) | (byte1 >> 3))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 7) << 10) | (byte2 << 2) | (byte3 >> 6))
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 63) << 7) | (byte4 >> 1))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 1) << 12) | (byte5 << 4) | (byte6 >> 4))
		valuesOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte6 & 15) << 9) | (byte7 << 1) | (byte8 >> 7))
		valuesOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte8 & 127) << 6) | (byte9 >> 2))
		valuesOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte11 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte9 & 3) << 11) | (byte10 << 3) | (byte11 >> 5))
		valuesOffset++
		byte12 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte11 & 31) << 8) | byte12)
		valuesOffset++
	}
}
