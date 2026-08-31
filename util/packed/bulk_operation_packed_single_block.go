// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import "fmt"

// bulkOperationPackedSingleBlock handles the PACKED_SINGLE_BLOCK
// format where every value stays within a single 64-bit block.
type bulkOperationPackedSingleBlock struct {
	bitsPerValue int
	valueCount   int
	mask         uint64
}

func newBulkOperationPackedSingleBlock(bitsPerValue int) *bulkOperationPackedSingleBlock {
	return &bulkOperationPackedSingleBlock{
		bitsPerValue: bitsPerValue,
		valueCount:   64 / bitsPerValue,
		mask:         (uint64(1) << uint(bitsPerValue)) - 1,
	}
}

func (b *bulkOperationPackedSingleBlock) LongBlockCount() int { return 1 }
func (b *bulkOperationPackedSingleBlock) ByteBlockCount() int { return 8 }
func (b *bulkOperationPackedSingleBlock) LongValueCount() int { return b.valueCount }
func (b *bulkOperationPackedSingleBlock) ByteValueCount() int { return b.valueCount }

func (b *bulkOperationPackedSingleBlock) ComputeIterations(valueCount, ramBudget int) int {
	return computeIterations(b.ByteBlockCount(), b.ByteValueCount(), valueCount, ramBudget)
}

// readSingleBlockLong reads a 64-bit value in big-endian byte order
// from blocks[blocksOffset:].
func readSingleBlockLong(blocks []byte, blocksOffset int) uint64 {
	return uint64(blocks[blocksOffset])<<56 |
		uint64(blocks[blocksOffset+1])<<48 |
		uint64(blocks[blocksOffset+2])<<40 |
		uint64(blocks[blocksOffset+3])<<32 |
		uint64(blocks[blocksOffset+4])<<24 |
		uint64(blocks[blocksOffset+5])<<16 |
		uint64(blocks[blocksOffset+6])<<8 |
		uint64(blocks[blocksOffset+7])
}

// writeSingleBlockLong writes a 64-bit value in big-endian byte
// order into blocks[blocksOffset:].
func writeSingleBlockLong(block uint64, blocks []byte, blocksOffset int) {
	blocks[blocksOffset] = byte(block >> 56)
	blocks[blocksOffset+1] = byte(block >> 48)
	blocks[blocksOffset+2] = byte(block >> 40)
	blocks[blocksOffset+3] = byte(block >> 32)
	blocks[blocksOffset+4] = byte(block >> 24)
	blocks[blocksOffset+5] = byte(block >> 16)
	blocks[blocksOffset+6] = byte(block >> 8)
	blocks[blocksOffset+7] = byte(block)
}

func (b *bulkOperationPackedSingleBlock) decodeBlockLong(block uint64, values []int64, valuesOffset int) int {
	values[valuesOffset] = int64(block & b.mask)
	valuesOffset++
	for j := 1; j < b.valueCount; j++ {
		block >>= uint(b.bitsPerValue)
		values[valuesOffset] = int64(block & b.mask)
		valuesOffset++
	}
	return valuesOffset
}

func (b *bulkOperationPackedSingleBlock) decodeBlockInt(block uint64, values []int32, valuesOffset int) int {
	values[valuesOffset] = int32(block & b.mask)
	valuesOffset++
	for j := 1; j < b.valueCount; j++ {
		block >>= uint(b.bitsPerValue)
		values[valuesOffset] = int32(block & b.mask)
		valuesOffset++
	}
	return valuesOffset
}

func (b *bulkOperationPackedSingleBlock) encodeBlockLong(values []int64, valuesOffset int) uint64 {
	block := uint64(values[valuesOffset])
	for j := 1; j < b.valueCount; j++ {
		block |= uint64(values[valuesOffset+j]) << uint(j*b.bitsPerValue)
	}
	return block
}

func (b *bulkOperationPackedSingleBlock) encodeBlockInt(values []int32, valuesOffset int) uint64 {
	block := uint64(uint32(values[valuesOffset]))
	for j := 1; j < b.valueCount; j++ {
		block |= uint64(uint32(values[valuesOffset+j])) << uint(j*b.bitsPerValue)
	}
	return block
}

func (b *bulkOperationPackedSingleBlock) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		valuesOffset = b.decodeBlockLong(uint64(blocks[blocksOffset]), values, valuesOffset)
		blocksOffset++
	}
}

func (b *bulkOperationPackedSingleBlock) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := readSingleBlockLong(blocks, blocksOffset)
		blocksOffset += 8
		valuesOffset = b.decodeBlockLong(block, values, valuesOffset)
	}
}

func (b *bulkOperationPackedSingleBlock) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	if b.bitsPerValue > 32 {
		panic(fmt.Sprintf("packed: cannot decode %d-bits values into int32", b.bitsPerValue))
	}
	for i := 0; i < iterations; i++ {
		valuesOffset = b.decodeBlockInt(uint64(blocks[blocksOffset]), values, valuesOffset)
		blocksOffset++
	}
}

func (b *bulkOperationPackedSingleBlock) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	if b.bitsPerValue > 32 {
		panic(fmt.Sprintf("packed: cannot decode %d-bits values into int32", b.bitsPerValue))
	}
	for i := 0; i < iterations; i++ {
		block := readSingleBlockLong(blocks, blocksOffset)
		blocksOffset += 8
		valuesOffset = b.decodeBlockInt(block, values, valuesOffset)
	}
}

func (b *bulkOperationPackedSingleBlock) EncodeLongsToLongs(values []int64, valuesOffset int, blocks []int64, blocksOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		blocks[blocksOffset] = int64(b.encodeBlockLong(values, valuesOffset))
		blocksOffset++
		valuesOffset += b.valueCount
	}
}

func (b *bulkOperationPackedSingleBlock) EncodeIntsToLongs(values []int32, valuesOffset int, blocks []int64, blocksOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		blocks[blocksOffset] = int64(b.encodeBlockInt(values, valuesOffset))
		blocksOffset++
		valuesOffset += b.valueCount
	}
}

func (b *bulkOperationPackedSingleBlock) EncodeLongsToBytes(values []int64, valuesOffset int, blocks []byte, blocksOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := b.encodeBlockLong(values, valuesOffset)
		writeSingleBlockLong(block, blocks, blocksOffset)
		blocksOffset += 8
		valuesOffset += b.valueCount
	}
}

func (b *bulkOperationPackedSingleBlock) EncodeIntsToBytes(values []int32, valuesOffset int, blocks []byte, blocksOffset, iterations int) {
	for i := 0; i < iterations; i++ {
		block := b.encodeBlockInt(values, valuesOffset)
		writeSingleBlockLong(block, blocks, blocksOffset)
		blocksOffset += 8
		valuesOffset += b.valueCount
	}
}

// BulkOperationPackedSingleBlock is the exported alias of
// bulkOperationPackedSingleBlock preserved for external Lucene-aligned
// consumers. Behaviour and layout are identical to the unexported type.
type BulkOperationPackedSingleBlock = bulkOperationPackedSingleBlock

// NewBulkOperationPackedSingleBlock constructs a BulkOperationPackedSingleBlock
// for the given bits-per-value. It mirrors the unexported
// newBulkOperationPackedSingleBlock constructor.
func NewBulkOperationPackedSingleBlock(bitsPerValue int) *BulkOperationPackedSingleBlock {
	return newBulkOperationPackedSingleBlock(bitsPerValue)
}
