// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked6.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked6 is the hand-unrolled BulkOperation for
// bitsPerValue == 6. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked6, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=6.
type bulkOperationPacked6 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked6 returns the specialised BulkOperation for
// 6 bits per value.
func newBulkOperationPacked6() *bulkOperationPacked6 {
	return &bulkOperationPacked6{bulkOperationPacked: newBulkOperationPacked(6)}
}

// Compile-time guarantee that bulkOperationPacked6 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked6)(nil)

// DecodeLongs decodes 32 six-bit values from each group of three
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked6.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked6) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 58)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 52) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 46) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 40) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 34) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 28) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 22) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 16) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 10) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 4) & 63)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 15) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 56) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 50) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 44) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 32) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 26) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 20) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 14) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 8) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 63)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 54) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 48) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 42) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 36) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 30) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 24) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 18) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 63)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 6) & 63)
		valuesOffset++
		values[valuesOffset] = int64(block2 & 63)
		valuesOffset++
	}
}

// DecodeBytes decodes 4 six-bit values from each group of three
// input bytes into int64 slots. Literal port of
// BulkOperationPacked6.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked6) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(byte0 >> 2)
		valuesOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte0 & 3) << 4) | (byte1 >> 4))
		valuesOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte1 & 15) << 2) | (byte2 >> 6))
		valuesOffset++
		values[valuesOffset] = int64(byte2 & 63)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 six-bit values from each group of
// three 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked6.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked6) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 58)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 52) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 46) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 40) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 34) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 28) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 22) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 16) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 10) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 4) & 63)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 15) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 56) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 50) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 44) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 32) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 26) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 20) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 14) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 8) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 63)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 54) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 48) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 42) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 36) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 30) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 24) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 18) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 63)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 6) & 63)
		valuesOffset++
		values[valuesOffset] = int32(block2 & 63)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 4 six-bit values from each group of
// three input bytes into int32 slots. Literal port of
// BulkOperationPacked6.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked6) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(byte0 >> 2)
		valuesOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte0 & 3) << 4) | (byte1 >> 4))
		valuesOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte1 & 15) << 2) | (byte2 >> 6))
		valuesOffset++
		values[valuesOffset] = int32(byte2 & 63)
		valuesOffset++
	}
}
