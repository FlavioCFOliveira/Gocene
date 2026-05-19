// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked5.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked5 is the hand-unrolled BulkOperation for
// bitsPerValue == 5. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked5, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=5.
type bulkOperationPacked5 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked5 returns the specialised BulkOperation for
// 5 bits per value.
func newBulkOperationPacked5() *bulkOperationPacked5 {
	return &bulkOperationPacked5{bulkOperationPacked: newBulkOperationPacked(5)}
}

// Compile-time guarantee that bulkOperationPacked5 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked5)(nil)

// DecodeLongs decodes 64 five-bit values from each group of five
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked5.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked5) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 59)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 54) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 49) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 44) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 39) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 34) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 29) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 24) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 19) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 14) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 9) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 31)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 1) | (block1 >> 63))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 58) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 53) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 48) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 43) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 33) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 28) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 23) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 18) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 13) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 3) & 31)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 7) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 57) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 52) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 47) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 42) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 37) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 32) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 27) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 22) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 17) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 7) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 2) & 31)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 3) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 56) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 51) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 46) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 41) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 36) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 31) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 26) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 21) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 16) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 11) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 6) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 1) & 31)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 1) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 55) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 50) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 45) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 40) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 35) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 30) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 25) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 20) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 15) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 10) & 31)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 5) & 31)
		valuesOffset++
		values[valuesOffset] = int64(block4 & 31)
		valuesOffset++
	}
}

// DecodeBytes decodes 8 five-bit values from each group of five
// input bytes into int64 slots. Literal port of
// BulkOperationPacked5.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked5) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(byte0 >> 3)
		valuesOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte0 & 7) << 2) | (byte1 >> 6))
		valuesOffset++
		values[valuesOffset] = int64((byte1 >> 1) & 31)
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 1) << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 15) << 1) | (byte3 >> 7))
		valuesOffset++
		values[valuesOffset] = int64((byte3 >> 2) & 31)
		valuesOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte3 & 3) << 3) | (byte4 >> 5))
		valuesOffset++
		values[valuesOffset] = int64(byte4 & 31)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 64 five-bit values from each group of
// five 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked5.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked5) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 59)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 54) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 49) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 44) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 39) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 34) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 29) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 24) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 19) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 14) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 9) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 31)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 1) | (block1 >> 63))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 58) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 53) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 48) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 43) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 33) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 28) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 23) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 18) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 13) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 3) & 31)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 7) << 2) | (block2 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 57) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 52) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 47) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 42) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 37) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 32) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 27) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 22) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 17) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 7) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 2) & 31)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 3) << 3) | (block3 >> 61))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 56) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 51) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 46) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 41) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 36) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 31) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 26) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 21) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 16) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 11) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 6) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 1) & 31)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 1) << 4) | (block4 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 55) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 50) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 45) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 40) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 35) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 30) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 25) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 20) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 15) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 10) & 31)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 5) & 31)
		valuesOffset++
		values[valuesOffset] = int32(block4 & 31)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 8 five-bit values from each group of
// five input bytes into int32 slots. Literal port of
// BulkOperationPacked5.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked5) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(byte0 >> 3)
		valuesOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte0 & 7) << 2) | (byte1 >> 6))
		valuesOffset++
		values[valuesOffset] = int32((byte1 >> 1) & 31)
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 1) << 4) | (byte2 >> 4))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 15) << 1) | (byte3 >> 7))
		valuesOffset++
		values[valuesOffset] = int32((byte3 >> 2) & 31)
		valuesOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte3 & 3) << 3) | (byte4 >> 5))
		valuesOffset++
		values[valuesOffset] = int32(byte4 & 31)
		valuesOffset++
	}
}
