// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Packed64SingleBlockMaxSupportedBitsPerValue is the largest
// bitsPerValue accepted by Packed64SingleBlock.
const Packed64SingleBlockMaxSupportedBitsPerValue = 32

// packed64SingleBlockSupported lists the bitsPerValue values
// supported by the PACKED_SINGLE_BLOCK format.
var packed64SingleBlockSupported = [...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 21, 32}

func packed64SingleBlockIsSupported(bitsPerValue int) bool {
	for _, b := range packed64SingleBlockSupported {
		if b == bitsPerValue {
			return true
		}
	}
	return false
}

// Packed64SingleBlock is the Mutable used by FormatPackedSingleBlock.
// Values never cross 64-bit block boundaries, trading space for
// faster decode.
type Packed64SingleBlock struct {
	valueCount     int
	bitsPerValue   int
	blocks         []int64
	valuesPerBlock int
	mask           uint64
}

func newPacked64SingleBlock(valueCount, bitsPerValue int) *Packed64SingleBlock {
	if !packed64SingleBlockIsSupported(bitsPerValue) {
		panic(fmt.Sprintf("packed: bitsPerValue %d not supported by PACKED_SINGLE_BLOCK", bitsPerValue))
	}
	vpb := 64 / bitsPerValue
	required := valueCount/vpb + boolToInt(valueCount%vpb != 0)
	return &Packed64SingleBlock{
		valueCount:     valueCount,
		bitsPerValue:   bitsPerValue,
		blocks:         make([]int64, required),
		valuesPerBlock: vpb,
		mask:           (uint64(1) << uint(bitsPerValue)) - 1,
	}
}

// Size returns the number of values stored.
func (p *Packed64SingleBlock) Size() int { return p.valueCount }

// GetBitsPerValue returns the per-value bit width.
func (p *Packed64SingleBlock) GetBitsPerValue() int { return p.bitsPerValue }

// GetFormat returns FormatPackedSingleBlock.
func (p *Packed64SingleBlock) GetFormat() Format { return FormatPackedSingleBlock }

// Get returns the value at the given index.
func (p *Packed64SingleBlock) Get(index int) int64 {
	o := index / p.valuesPerBlock
	b := index % p.valuesPerBlock
	shift := uint(b * p.bitsPerValue)
	return int64((uint64(p.blocks[o]) >> shift) & p.mask)
}

// Set assigns the value at the given index.
func (p *Packed64SingleBlock) Set(index int, value int64) {
	o := index / p.valuesPerBlock
	b := index % p.valuesPerBlock
	shift := uint(b * p.bitsPerValue)
	v := uint64(value)
	p.blocks[o] = int64((uint64(p.blocks[o]) & ^(p.mask << shift)) | (v << shift))
}

// GetBulk reads up to length values into arr.
func (p *Packed64SingleBlock) GetBulk(index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	originalIndex := index

	offsetInBlock := index % p.valuesPerBlock
	if offsetInBlock != 0 {
		for i := offsetInBlock; i < p.valuesPerBlock && length > 0; i++ {
			arr[off] = p.Get(index)
			off++
			index++
			length--
		}
		if length == 0 {
			return index - originalIndex
		}
	}

	decoder := newBulkOperationPackedSingleBlock(p.bitsPerValue)
	blockIndex := index / p.valuesPerBlock
	nblocks := (index+length)/p.valuesPerBlock - blockIndex
	decoder.DecodeLongs(p.blocks, blockIndex, arr, off, nblocks)
	diff := nblocks * p.valuesPerBlock
	index += diff
	length -= diff

	if index > originalIndex {
		return index - originalIndex
	}
	return readerBulkGet(p, index, arr, off, length)
}

// SetBulk writes up to length values from arr.
func (p *Packed64SingleBlock) SetBulk(index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	originalIndex := index

	offsetInBlock := index % p.valuesPerBlock
	if offsetInBlock != 0 {
		for i := offsetInBlock; i < p.valuesPerBlock && length > 0; i++ {
			p.Set(index, arr[off])
			off++
			index++
			length--
		}
		if length == 0 {
			return index - originalIndex
		}
	}

	encoder := newBulkOperationPackedSingleBlock(p.bitsPerValue)
	blockIndex := index / p.valuesPerBlock
	nblocks := (index+length)/p.valuesPerBlock - blockIndex
	encoder.EncodeLongsToLongs(arr, off, p.blocks, blockIndex, nblocks)
	diff := nblocks * p.valuesPerBlock
	index += diff
	length -= diff

	if index > originalIndex {
		return index - originalIndex
	}
	return mutableBulkSet(p, index, arr, off, length)
}

// Fill assigns val to every index in [fromIndex, toIndex).
func (p *Packed64SingleBlock) Fill(fromIndex, toIndex int, val int64) {
	if toIndex-fromIndex <= p.valuesPerBlock<<1 {
		mutableFill(p, fromIndex, toIndex, val)
		return
	}

	fromOffsetInBlock := fromIndex % p.valuesPerBlock
	if fromOffsetInBlock != 0 {
		for i := fromOffsetInBlock; i < p.valuesPerBlock; i++ {
			p.Set(fromIndex, val)
			fromIndex++
		}
	}

	fromBlock := fromIndex / p.valuesPerBlock
	toBlock := toIndex / p.valuesPerBlock

	var blockValue uint64
	for i := 0; i < p.valuesPerBlock; i++ {
		blockValue |= uint64(val) << uint(i*p.bitsPerValue)
	}
	for i := fromBlock; i < toBlock; i++ {
		p.blocks[i] = int64(blockValue)
	}

	for i := p.valuesPerBlock * toBlock; i < toIndex; i++ {
		p.Set(i, val)
	}
}

// Clear zeroes the underlying blocks.
func (p *Packed64SingleBlock) Clear() {
	for i := range p.blocks {
		p.blocks[i] = 0
	}
}

// RamBytesUsed mirrors the Java accounting.
func (p *Packed64SingleBlock) RamBytesUsed() int64 {
	return util.AlignObjectSize(int64(2*4+util.NumBytesObjectRef)) + util.SizeOfInt64Slice(p.blocks)
}

// String returns a debug-friendly representation.
func (p *Packed64SingleBlock) String() string {
	return fmt.Sprintf("Packed64SingleBlock(bitsPerValue=%d,size=%d,blocks=%d)", p.bitsPerValue, p.valueCount, len(p.blocks))
}
