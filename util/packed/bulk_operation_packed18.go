// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked18.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked18 is the hand-unrolled BulkOperation for
// bitsPerValue == 18. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked18, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=18.
type bulkOperationPacked18 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked18 returns the specialised BulkOperation for
// 18 bits per value.
func newBulkOperationPacked18() *bulkOperationPacked18 {
	return &bulkOperationPacked18{bulkOperationPacked: newBulkOperationPacked(18)}
}

// Compile-time guarantee that bulkOperationPacked18 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked18)(nil)

// DecodeLongs decodes 32 eighteen-bit values per iteration from nine
// 64-bit blocks into int64 slots. Literal port of
// BulkOperationPacked18.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked18) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 46)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 28) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 10) & 262143)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1023) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 38) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 20) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 2) & 262143)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 3) << 16) | (block2 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 30) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 12) & 262143)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 4095) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 40) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 22) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 4) & 262143)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 15) << 14) | (block4 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 32) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 14) & 262143)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 16383) << 4) | (block5 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 42) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 24) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 6) & 262143)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 63) << 12) | (block6 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 34) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 16) & 262143)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 65535) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 44) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 26) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 8) & 262143)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 255) << 10) | (block8 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 36) & 262143)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 18) & 262143)
		valuesOffset++
		values[valuesOffset] = int64(block8 & 262143)
		valuesOffset++
	}
}

// DecodeBytes decodes 4 eighteen-bit values per iteration from nine
// bytes into int64 slots. Literal port of
// BulkOperationPacked18.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked18) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 10) | (byte1 << 2) | (byte2 >> 6))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 63) << 12) | (byte3 << 4) | (byte4 >> 4))
		valuesOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte4 & 15) << 14) | (byte5 << 6) | (byte6 >> 2))
		valuesOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte6 & 3) << 16) | (byte7 << 8) | byte8)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 eighteen-bit values per iteration from
// nine 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked18.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked18) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 46)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 28) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 10) & 262143)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1023) << 8) | (block1 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 38) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 20) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 2) & 262143)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 3) << 16) | (block2 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 30) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 12) & 262143)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 4095) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 40) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 22) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 4) & 262143)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 15) << 14) | (block4 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 32) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 14) & 262143)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 16383) << 4) | (block5 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 42) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 24) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 6) & 262143)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 63) << 12) | (block6 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 34) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 16) & 262143)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 65535) << 2) | (block7 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 44) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 26) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 8) & 262143)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 255) << 10) | (block8 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 36) & 262143)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 18) & 262143)
		valuesOffset++
		values[valuesOffset] = int32(block8 & 262143)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 4 eighteen-bit values per iteration from
// nine bytes into int32 slots. Literal port of
// BulkOperationPacked18.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked18) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 10) | (byte1 << 2) | (byte2 >> 6))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 63) << 12) | (byte3 << 4) | (byte4 >> 4))
		valuesOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte4 & 15) << 14) | (byte5 << 6) | (byte6 >> 2))
		valuesOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte6 & 3) << 16) | (byte7 << 8) | byte8)
		valuesOffset++
	}
}
