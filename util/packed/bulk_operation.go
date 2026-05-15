// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import "fmt"

// BulkOperationOf returns the BulkOperation that handles the given
// (format, bitsPerValue) pair.
//
// For PACKED, every bitsPerValue in [1, 64] is handled by the
// non-specialized BulkOperationPacked. Lucene uses hand-unrolled
// BulkOperationPackedN classes for bitsPerValue ≤ 24 as a speed
// optimisation; the wire format produced by the generic
// implementation is identical, byte for byte.
//
// For PACKED_SINGLE_BLOCK the supported bitsPerValue values are
// {1,2,3,4,5,6,7,8,9,10,12,16,21,32}.
func BulkOperationOf(format Format, bitsPerValue int) (BulkOperation, error) {
	switch format {
	case FormatPacked:
		if bitsPerValue < 1 || bitsPerValue > 64 {
			return nil, fmt.Errorf("packed: bitsPerValue out of range for PACKED: %d", bitsPerValue)
		}
		return newBulkOperationPacked(bitsPerValue), nil
	case FormatPackedSingleBlock:
		if !packed64SingleBlockIsSupported(bitsPerValue) {
			return nil, fmt.Errorf("packed: bitsPerValue %d not supported by PACKED_SINGLE_BLOCK", bitsPerValue)
		}
		return newBulkOperationPackedSingleBlock(bitsPerValue), nil
	default:
		return nil, fmt.Errorf("packed: unknown format %d", format)
	}
}

// computeIterations returns the number of iterations a BulkOperation
// should perform given a value count and a RAM budget, mirroring
// Lucene's BulkOperation.computeIterations.
func computeIterations(byteBlockCount, byteValueCount, valueCount, ramBudget int) int {
	iterations := ramBudget / (byteBlockCount + 8*byteValueCount)
	if iterations == 0 {
		return 1
	}
	if (iterations-1)*byteValueCount >= valueCount {
		return (valueCount + byteValueCount - 1) / byteValueCount
	}
	return iterations
}

// bulkOperationPacked is the non-specialized BulkOperation that
// handles every bitsPerValue in [1, 64] for FormatPacked.
type bulkOperationPacked struct {
	bitsPerValue   int
	longBlockCount int
	longValueCount int
	byteBlockCount int
	byteValueCount int
	mask           uint64
	intMask        uint32
}

func newBulkOperationPacked(bitsPerValue int) *bulkOperationPacked {
	if bitsPerValue < 1 || bitsPerValue > 64 {
		panic(fmt.Sprintf("packed: bitsPerValue out of range: %d", bitsPerValue))
	}
	blocks := bitsPerValue
	for blocks&1 == 0 {
		blocks >>= 1
	}
	longBlockCount := blocks
	longValueCount := 64 * longBlockCount / bitsPerValue
	byteBlockCount := 8 * longBlockCount
	byteValueCount := longValueCount
	for byteBlockCount&1 == 0 && byteValueCount&1 == 0 {
		byteBlockCount >>= 1
		byteValueCount >>= 1
	}
	var mask uint64
	if bitsPerValue == 64 {
		mask = ^uint64(0)
	} else {
		mask = (uint64(1) << uint(bitsPerValue)) - 1
	}
	return &bulkOperationPacked{
		bitsPerValue:   bitsPerValue,
		longBlockCount: longBlockCount,
		longValueCount: longValueCount,
		byteBlockCount: byteBlockCount,
		byteValueCount: byteValueCount,
		mask:           mask,
		intMask:        uint32(mask),
	}
}

func (b *bulkOperationPacked) LongBlockCount() int { return b.longBlockCount }
func (b *bulkOperationPacked) LongValueCount() int { return b.longValueCount }
func (b *bulkOperationPacked) ByteBlockCount() int { return b.byteBlockCount }
func (b *bulkOperationPacked) ByteValueCount() int { return b.byteValueCount }

func (b *bulkOperationPacked) ComputeIterations(valueCount, ramBudget int) int {
	return computeIterations(b.byteBlockCount, b.byteValueCount, valueCount, ramBudget)
}

// DecodeLongs decodes packed values stored as 64-bit blocks.
func (b *bulkOperationPacked) DecodeLongs(blocks []int64, blocksOffset int, values []int64, valuesOffset, iterations int) {
	bpv := b.bitsPerValue
	bitsLeft := 64
	for i := 0; i < b.longValueCount*iterations; i++ {
		bitsLeft -= bpv
		if bitsLeft < 0 {
			high := uint64(blocks[blocksOffset]) & ((uint64(1) << uint(bpv+bitsLeft)) - 1)
			blocksOffset++
			low := uint64(blocks[blocksOffset]) >> uint(64+bitsLeft)
			values[valuesOffset] = int64((high << uint(-bitsLeft)) | low)
			valuesOffset++
			bitsLeft += 64
		} else {
			values[valuesOffset] = int64((uint64(blocks[blocksOffset]) >> uint(bitsLeft)) & b.mask)
			valuesOffset++
		}
	}
}

// DecodeBytes decodes packed values stored as a byte sequence
// (big-endian inside each 64-bit block).
func (b *bulkOperationPacked) DecodeBytes(blocks []byte, blocksOffset int, values []int64, valuesOffset, iterations int) {
	bpv := b.bitsPerValue
	var nextValue uint64
	bitsLeft := bpv
	for i := 0; i < iterations*b.byteBlockCount; i++ {
		bb := uint64(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		if bitsLeft > 8 {
			bitsLeft -= 8
			nextValue |= bb << uint(bitsLeft)
		} else {
			bitsRem := 8 - bitsLeft
			values[valuesOffset] = int64(nextValue | (bb >> uint(bitsRem)))
			valuesOffset++
			for bitsRem >= bpv {
				bitsRem -= bpv
				values[valuesOffset] = int64((bb >> uint(bitsRem)) & b.mask)
				valuesOffset++
			}
			bitsLeft = bpv - bitsRem
			nextValue = (bb & ((uint64(1) << uint(bitsRem)) - 1)) << uint(bitsLeft)
		}
	}
}

// DecodeLongsToInts decodes packed values into int32 slots; panics
// for bitsPerValue > 32.
func (b *bulkOperationPacked) DecodeLongsToInts(blocks []int64, blocksOffset int, values []int32, valuesOffset, iterations int) {
	if b.bitsPerValue > 32 {
		panic(fmt.Sprintf("packed: cannot decode %d-bits values into int32", b.bitsPerValue))
	}
	bpv := b.bitsPerValue
	bitsLeft := 64
	for i := 0; i < b.longValueCount*iterations; i++ {
		bitsLeft -= bpv
		if bitsLeft < 0 {
			high := uint64(blocks[blocksOffset]) & ((uint64(1) << uint(bpv+bitsLeft)) - 1)
			blocksOffset++
			low := uint64(blocks[blocksOffset]) >> uint(64+bitsLeft)
			values[valuesOffset] = int32((high << uint(-bitsLeft)) | low)
			valuesOffset++
			bitsLeft += 64
		} else {
			values[valuesOffset] = int32((uint64(blocks[blocksOffset]) >> uint(bitsLeft)) & b.mask)
			valuesOffset++
		}
	}
}

// DecodeBytesToInts decodes packed bytes into int32 slots.
func (b *bulkOperationPacked) DecodeBytesToInts(blocks []byte, blocksOffset int, values []int32, valuesOffset, iterations int) {
	if b.bitsPerValue > 32 {
		panic(fmt.Sprintf("packed: cannot decode %d-bits values into int32", b.bitsPerValue))
	}
	bpv := b.bitsPerValue
	var nextValue uint32
	bitsLeft := bpv
	for i := 0; i < iterations*b.byteBlockCount; i++ {
		bb := uint32(blocks[blocksOffset]) & 0xFF
		blocksOffset++
		if bitsLeft > 8 {
			bitsLeft -= 8
			nextValue |= bb << uint(bitsLeft)
		} else {
			bitsRem := 8 - bitsLeft
			values[valuesOffset] = int32(nextValue | (bb >> uint(bitsRem)))
			valuesOffset++
			for bitsRem >= bpv {
				bitsRem -= bpv
				values[valuesOffset] = int32((bb >> uint(bitsRem)) & b.intMask)
				valuesOffset++
			}
			bitsLeft = bpv - bitsRem
			nextValue = (bb & ((uint32(1) << uint(bitsRem)) - 1)) << uint(bitsLeft)
		}
	}
}

// EncodeLongsToLongs encodes values into 64-bit blocks.
func (b *bulkOperationPacked) EncodeLongsToLongs(values []int64, valuesOffset int, blocks []int64, blocksOffset, iterations int) {
	bpv := b.bitsPerValue
	var nextBlock uint64
	bitsLeft := 64
	for i := 0; i < b.longValueCount*iterations; i++ {
		bitsLeft -= bpv
		v := uint64(values[valuesOffset])
		switch {
		case bitsLeft > 0:
			nextBlock |= v << uint(bitsLeft)
			valuesOffset++
		case bitsLeft == 0:
			nextBlock |= v
			blocks[blocksOffset] = int64(nextBlock)
			blocksOffset++
			valuesOffset++
			nextBlock = 0
			bitsLeft = 64
		default: // bitsLeft < 0
			nextBlock |= v >> uint(-bitsLeft)
			blocks[blocksOffset] = int64(nextBlock)
			blocksOffset++
			nextBlock = (v & ((uint64(1) << uint(-bitsLeft)) - 1)) << uint(64+bitsLeft)
			valuesOffset++
			bitsLeft += 64
		}
	}
}

// EncodeIntsToLongs encodes int32 values into 64-bit blocks.
func (b *bulkOperationPacked) EncodeIntsToLongs(values []int32, valuesOffset int, blocks []int64, blocksOffset, iterations int) {
	bpv := b.bitsPerValue
	var nextBlock uint64
	bitsLeft := 64
	for i := 0; i < b.longValueCount*iterations; i++ {
		bitsLeft -= bpv
		v := uint64(uint32(values[valuesOffset]))
		switch {
		case bitsLeft > 0:
			nextBlock |= v << uint(bitsLeft)
			valuesOffset++
		case bitsLeft == 0:
			nextBlock |= v
			blocks[blocksOffset] = int64(nextBlock)
			blocksOffset++
			valuesOffset++
			nextBlock = 0
			bitsLeft = 64
		default:
			nextBlock |= v >> uint(-bitsLeft)
			blocks[blocksOffset] = int64(nextBlock)
			blocksOffset++
			nextBlock = (v & ((uint64(1) << uint(-bitsLeft)) - 1)) << uint(64+bitsLeft)
			valuesOffset++
			bitsLeft += 64
		}
	}
}

// EncodeLongsToBytes encodes values into a byte stream (big-endian
// inside each 64-bit conceptual block).
func (b *bulkOperationPacked) EncodeLongsToBytes(values []int64, valuesOffset int, blocks []byte, blocksOffset, iterations int) {
	bpv := b.bitsPerValue
	var nextBlock uint32
	bitsLeft := 8
	for i := 0; i < b.byteValueCount*iterations; i++ {
		v := uint64(values[valuesOffset])
		valuesOffset++
		if bpv < bitsLeft {
			nextBlock |= uint32(v << uint(bitsLeft-bpv))
			bitsLeft -= bpv
		} else {
			bitsRem := bpv - bitsLeft
			blocks[blocksOffset] = byte(nextBlock | uint32(v>>uint(bitsRem)))
			blocksOffset++
			for bitsRem >= 8 {
				bitsRem -= 8
				blocks[blocksOffset] = byte(v >> uint(bitsRem))
				blocksOffset++
			}
			bitsLeft = 8 - bitsRem
			nextBlock = uint32((v & ((uint64(1) << uint(bitsRem)) - 1)) << uint(bitsLeft))
		}
	}
}

// EncodeIntsToBytes encodes int32 values into a byte stream.
func (b *bulkOperationPacked) EncodeIntsToBytes(values []int32, valuesOffset int, blocks []byte, blocksOffset, iterations int) {
	bpv := b.bitsPerValue
	var nextBlock uint32
	bitsLeft := 8
	for i := 0; i < b.byteValueCount*iterations; i++ {
		v := uint32(values[valuesOffset])
		valuesOffset++
		if bpv < bitsLeft {
			nextBlock |= v << uint(bitsLeft-bpv)
			bitsLeft -= bpv
		} else {
			bitsRem := bpv - bitsLeft
			blocks[blocksOffset] = byte(nextBlock | (v >> uint(bitsRem)))
			blocksOffset++
			for bitsRem >= 8 {
				bitsRem -= 8
				blocks[blocksOffset] = byte(v >> uint(bitsRem))
				blocksOffset++
			}
			bitsLeft = 8 - bitsRem
			nextBlock = (v & ((uint32(1) << uint(bitsRem)) - 1)) << uint(bitsLeft)
		}
	}
}

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
