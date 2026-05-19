// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// This file has been hand-ported from Lucene's auto-generated
// BulkOperationPacked16.java, preserving every constant and bit-shift
// for byte-level wire compatibility. DO NOT EDIT BY HAND beyond
// keeping it in sync with the upstream generator.

package packed

// bulkOperationPacked16 is the hand-unrolled BulkOperation for
// bitsPerValue == 16. It mirrors org.apache.lucene.util.packed.
// BulkOperationPacked16, overriding only the four decode variants;
// every other method is inherited from the embedded
// bulkOperationPacked, which itself is constructed with bpv=16.
type bulkOperationPacked16 struct {
	*bulkOperationPacked
}

// newBulkOperationPacked16 returns the specialised BulkOperation for
// 16 bits per value.
func newBulkOperationPacked16() *bulkOperationPacked16 {
	return &bulkOperationPacked16{bulkOperationPacked: newBulkOperationPacked(16)}
}

// Compile-time guarantee that bulkOperationPacked16 satisfies the
// BulkOperation contract through the embedded bulkOperationPacked.
var _ BulkOperation = (*bulkOperationPacked16)(nil)

// DecodeLongs decodes 4 sixteen-bit values from each 64-bit block into
// int64 slots. Literal port of
// BulkOperationPacked16.decode(long[], int, long[], int, int).
func (b *bulkOperationPacked16) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 48; shift >= 0; shift -= 16 {
			values[valuesOffset] = int64((block >> uint(shift)) & 65535)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes 1 sixteen-bit value from every 2 input bytes
// into int64 slots. Literal port of
// BulkOperationPacked16.decode(byte[], int, long[], int, int).
func (b *bulkOperationPacked16) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		hi := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		lo := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int64((hi << 8) | lo)
		valuesOffset++
	}
}

// DecodeLongsToInts decodes 4 sixteen-bit values from each 64-bit
// block into int32 slots. Literal port of
// BulkOperationPacked16.decode(long[], int, int[], int, int).
func (b *bulkOperationPacked16) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := uint64(blocks[blocksOffset])
		blocksOffset++
		for shift := 48; shift >= 0; shift -= 16 {
			values[valuesOffset] = int32((block >> uint(shift)) & 65535)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes 1 sixteen-bit value from every 2 input
// bytes into int32 slots. Literal port of
// BulkOperationPacked16.decode(byte[], int, int[], int, int).
func (b *bulkOperationPacked16) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	for j := 0; j < iterations; j++ {
		hi := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		lo := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		values[valuesOffset] = int32((hi << 8) | lo)
		valuesOffset++
	}
}
