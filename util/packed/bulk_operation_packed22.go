// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked22.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked22 is the hand-unrolled BulkOperation for
// bitsPerValue == 22. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked22, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=22.
type bulkOperationPacked22 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked22 returns the specialised BulkOperation for
// 22 bits per value.
func newBulkOperationPacked22() *bulkOperationPacked22 {
	return &bulkOperationPacked22{bulkOperationPacked: newBulkOperationPacked(22)}
}

// Compile-time guarantee that bulkOperationPacked22 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked22)(nil)

// DecodeLongs decodes 32 twenty-two-bit values from each iteration
// of eleven 64-bit blocks into int64 slots. The bit-shift table is a
// literal port of BulkOperationPacked22.decode(long[], int, long[],
// int, int).
func (b *bulkOperationPacked22) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(block0 >> 42)
		valuesOffset++
		values[valuesOffset] = int64((block0 >> 20) & 4194303)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block0 & 1048575) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 40) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block1 >> 18) & 4194303)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block1 & 262143) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 38) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block2 >> 16) & 4194303)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block2 & 65535) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 36) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block3 >> 14) & 4194303)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block3 & 16383) << 8) | (block4 >> 56))
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 34) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block4 >> 12) & 4194303)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block4 & 4095) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 32) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block5 >> 10) & 4194303)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block5 & 1023) << 12) | (block6 >> 52))
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 30) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block6 >> 8) & 4194303)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block6 & 255) << 14) | (block7 >> 50))
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 28) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block7 >> 6) & 4194303)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block7 & 63) << 16) | (block8 >> 48))
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 26) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block8 >> 4) & 4194303)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block8 & 15) << 18) | (block9 >> 46))
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 24) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64((block9 >> 2) & 4194303)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int64(((block9 & 3) << 20) | (block10 >> 44))
		valuesOffset++
		values[valuesOffset] = int64((block10 >> 22) & 4194303)
		valuesOffset++
		values[valuesOffset] = int64(block10 & 4194303)
		valuesOffset++
	}
}

// DecodeBytes decodes 4 twenty-two-bit values from each iteration of
// eleven bytes into int64 slots. Literal port of
// BulkOperationPacked22.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked22) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((byte0 << 14) | (byte1 << 6) | (byte2 >> 2))
		valuesOffset++
		byte3 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte2 & 3) << 20) | (byte3 << 12) | (byte4 << 4) | (byte5 >> 4))
		valuesOffset++
		byte6 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte5 & 15) << 18) | (byte6 << 10) | (byte7 << 2) | (byte8 >> 6))
		valuesOffset++
		byte9 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64(((byte8 & 63) << 16) | (byte9 << 8) | byte10)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 32 twenty-two-bit values per iteration
// from eleven 64-bit blocks into int32 slots. Literal port of
// BulkOperationPacked22.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked22) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block0 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(block0 >> 42)
		valuesOffset++
		values[valuesOffset] = int32((block0 >> 20) & 4194303)
		valuesOffset++
		block1 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block0 & 1048575) << 2) | (block1 >> 62))
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 40) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block1 >> 18) & 4194303)
		valuesOffset++
		block2 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block1 & 262143) << 4) | (block2 >> 60))
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 38) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block2 >> 16) & 4194303)
		valuesOffset++
		block3 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block2 & 65535) << 6) | (block3 >> 58))
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 36) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block3 >> 14) & 4194303)
		valuesOffset++
		block4 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block3 & 16383) << 8) | (block4 >> 56))
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 34) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block4 >> 12) & 4194303)
		valuesOffset++
		block5 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block4 & 4095) << 10) | (block5 >> 54))
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 32) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block5 >> 10) & 4194303)
		valuesOffset++
		block6 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block5 & 1023) << 12) | (block6 >> 52))
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 30) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block6 >> 8) & 4194303)
		valuesOffset++
		block7 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block6 & 255) << 14) | (block7 >> 50))
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 28) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block7 >> 6) & 4194303)
		valuesOffset++
		block8 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block7 & 63) << 16) | (block8 >> 48))
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 26) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block8 >> 4) & 4194303)
		valuesOffset++
		block9 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block8 & 15) << 18) | (block9 >> 46))
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 24) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32((block9 >> 2) & 4194303)
		valuesOffset++
		block10 := uint64(blocks[blocksOffset])
		blocksOffset++
		values[valuesOffset] = int32(((block9 & 3) << 20) | (block10 >> 44))
		valuesOffset++
		values[valuesOffset] = int32((block10 >> 22) & 4194303)
		valuesOffset++
		values[valuesOffset] = int32(block10 & 4194303)
		valuesOffset++
	}
}

// DecodeBytesToInts decodes 4 twenty-two-bit values per iteration
// from eleven bytes into int32 slots. Literal port of
// BulkOperationPacked22.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked22) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		byte0 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte1 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte2 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((byte0 << 14) | (byte1 << 6) | (byte2 >> 2))
		valuesOffset++
		byte3 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte4 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte5 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte2 & 3) << 20) | (byte3 << 12) | (byte4 << 4) | (byte5 >> 4))
		valuesOffset++
		byte6 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte7 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte8 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte5 & 15) << 18) | (byte6 << 10) | (byte7 << 2) | (byte8 >> 6))
		valuesOffset++
		byte9 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		byte10 := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32(((byte8 & 63) << 16) | (byte9 << 8) | byte10)
		valuesOffset++
	}
}
