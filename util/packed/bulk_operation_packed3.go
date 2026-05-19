// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked3.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked3 is the hand-unrolled BulkOperation for
// bitsPerValue == 3. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked3, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=3.
type bulkOperationPacked3 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked3 returns the specialised BulkOperation for
// 3 bits per value.
func newBulkOperationPacked3() *bulkOperationPacked3 {
	return &bulkOperationPacked3{bulkOperationPacked: newBulkOperationPacked(3)}
}

// Compile-time guarantee that bulkOperationPacked3 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked3)(nil)

// DecodeLongs decodes 64 three-bit values from each window of three
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked3.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked3) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 61)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 58) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 55) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 52) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 49) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 46) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 43) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 40) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 37) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 34) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 31) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 28) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 25) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 22) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 19) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 16) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 13) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 10) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 7) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 1) & 7)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 59) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 56) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 53) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 50) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 47) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 44) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 41) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 35) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 32) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 29) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 26) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 23) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 20) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 17) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 14) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 11) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 5) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 7)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 1) | (block2 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 60) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 57) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 54) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 51) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 48) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 45) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 42) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 39) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 36) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 33) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 30) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 27) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 21) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 18) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 15) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 9) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 6) & 7)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 3) & 7)
		valuesOffset++
		values[valuesOffset] = int64(block2 & 7)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 three-bit values from each window of three
// input bytes into int64 slots. Literal port of
// BulkOperationPacked3.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked3) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(byte0 >> 5)
		valuesOffset++
		values[valuesOffset] = int64((byte0 >> 2) & 7)
		valuesOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte0 & 3) << 1) | (byte1 >> 7))
		valuesOffset++
		values[valuesOffset] = int64((byte1 >> 4) & 7)
		valuesOffset++
		values[valuesOffset] = int64((byte1 >> 1) & 7)
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 1) << 2) | (byte2 >> 6))
		valuesOffset++
		values[valuesOffset] = int64((byte2 >> 3) & 7)
		valuesOffset++
		values[valuesOffset] = int64(byte2 & 7)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 three-bit values from each window of
// three 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked3.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked3) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 61)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 58) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 55) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 52) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 49) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 46) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 43) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 40) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 37) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 34) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 31) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 28) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 25) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 22) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 19) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 16) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 13) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 10) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 7) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 1) & 7)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 59) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 56) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 53) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 50) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 47) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 44) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 41) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 35) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 32) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 29) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 26) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 23) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 20) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 17) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 14) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 11) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 5) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 7)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 1) | (block2 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 60) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 57) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 54) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 51) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 48) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 45) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 42) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 39) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 36) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 33) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 30) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 27) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 21) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 18) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 15) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 9) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 6) & 7)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 3) & 7)
		valuesOffset++
		values[valuesOffset] = int32(block2 & 7)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 three-bit values from each window of
// three input bytes into int32 slots. Literal port of
// BulkOperationPacked3.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked3) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(byte0 >> 5)
		valuesOffset++
		values[valuesOffset] = int32((byte0 >> 2) & 7)
		valuesOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte0 & 3) << 1) | (byte1 >> 7))
		valuesOffset++
		values[valuesOffset] = int32((byte1 >> 4) & 7)
		valuesOffset++
		values[valuesOffset] = int32((byte1 >> 1) & 7)
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 1) << 2) | (byte2 >> 6))
		valuesOffset++
		values[valuesOffset] = int32((byte2 >> 3) & 7)
		valuesOffset++
		values[valuesOffset] = int32(byte2 & 7)
		valuesOffset++
	}
}
