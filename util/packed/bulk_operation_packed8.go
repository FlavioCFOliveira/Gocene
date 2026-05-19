// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked8.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked8 is the hand-unrolled BulkOperation for
// bitsPerValue == 8. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked8, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=8.
type bulkOperationPacked8 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked8 returns the specialised BulkOperation for
// 8 bits per value.
func newBulkOperationPacked8() *bulkOperationPacked8 {
	return &bulkOperationPacked8{bulkOperationPacked: newBulkOperationPacked(8)}
}

// Compile-time guarantee that bulkOperationPacked8 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked8)(nil)

// DecodeLongs decodes 8 eight-bit values from every 64-bit block
// into int64 slots. Literal port of
// BulkOperationPacked8.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked8) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 56; shift >= 0; shift -= 8 {
			values[valuesOffset] = int64((block >> uint(shift)) & 255)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes one eight-bit value per input byte into
// int64 slots. Literal port of
// BulkOperationPacked8.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked8) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		values[valuesOffset] = int64(uint64(blocks[blocksOffset]) & 0xFF)
		valuesOffset++
		blocksOffset++
	}
}

// DecodeLongsToInts decodes 8 eight-bit values from every 64-bit
// block into int32 slots. Literal port of
// BulkOperationPacked8.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked8) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 56; shift >= 0; shift -= 8 {
			values[valuesOffset] = int32((block >> uint(shift)) & 255)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes one eight-bit value per input byte into
// int32 slots. Literal port of
// BulkOperationPacked8.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked8) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		values[valuesOffset] = int32(uint32(blocks[blocksOffset]) & 0xFF)
		valuesOffset++
		blocksOffset++
	}
}
