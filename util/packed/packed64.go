// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	packed64BlockSize = 64 // bits per block (long)
	packed64BlockBits = 6  // log2(blockSize)
	packed64ModMask   = packed64BlockSize - 1
)

// Packed64 is the dense, contiguous packed Mutable used by
// FormatPacked. Values are stored back to back in a long[] backing
// array, possibly spanning two adjacent longs.
type Packed64 struct {
	valueCount        int
	bitsPerValue      int
	blocks            []int64
	maskRight         uint64
	bpvMinusBlockSize int
}

// newPacked64 returns a zero-initialised Packed64 of valueCount
// values, each bitsPerValue bits wide.
func newPacked64(valueCount, bitsPerValue int) *Packed64 {
	if bitsPerValue < 1 || bitsPerValue > 64 {
		panic(fmt.Sprintf("packed: bitsPerValue out of range: %d", bitsPerValue))
	}
	longCount := FormatPacked.LongCount(VersionCurrent, valueCount, bitsPerValue)
	mask := (^uint64(0) << uint(packed64BlockSize-bitsPerValue)) >> uint(packed64BlockSize-bitsPerValue)
	return &Packed64{
		valueCount:        valueCount,
		bitsPerValue:      bitsPerValue,
		blocks:            make([]int64, longCount),
		maskRight:         mask,
		bpvMinusBlockSize: bitsPerValue - packed64BlockSize,
	}
}

// Size returns the number of values stored.
func (p *Packed64) Size() int { return p.valueCount }

// GetBitsPerValue returns the bits-per-value of the array.
func (p *Packed64) GetBitsPerValue() int { return p.bitsPerValue }

// GetFormat returns FormatPacked.
func (p *Packed64) GetFormat() Format { return FormatPacked }

// Get returns the value at the given index.
func (p *Packed64) Get(index int) int64 {
	majorBitPos := int64(index) * int64(p.bitsPerValue)
	elementPos := int(uint64(majorBitPos) >> uint(packed64BlockBits))
	endBits := int(majorBitPos&packed64ModMask) + p.bpvMinusBlockSize
	if endBits <= 0 {
		return int64((uint64(p.blocks[elementPos]) >> uint(-endBits)) & p.maskRight)
	}
	high := uint64(p.blocks[elementPos]) << uint(endBits)
	low := uint64(p.blocks[elementPos+1]) >> uint(packed64BlockSize-endBits)
	return int64((high | low) & p.maskRight)
}

// GetBulk reads multiple values starting at index.
func (p *Packed64) GetBulk(index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	originalIndex := index
	decoder := newBulkOperationPacked(p.bitsPerValue)

	offsetInBlocks := index % decoder.LongValueCount()
	if offsetInBlocks != 0 {
		for i := offsetInBlocks; i < decoder.LongValueCount() && length > 0; i++ {
			arr[off] = p.Get(index)
			off++
			index++
			length--
		}
		if length == 0 {
			return index - originalIndex
		}
	}

	blockIndex := int((int64(index) * int64(p.bitsPerValue)) >> uint(packed64BlockBits))
	iterations := length / decoder.LongValueCount()
	decoder.DecodeLongs(p.blocks, blockIndex, arr, off, iterations)
	gotValues := iterations * decoder.LongValueCount()
	index += gotValues
	length -= gotValues

	if index > originalIndex {
		return index - originalIndex
	}
	// no progress => fall back to sequential
	return readerBulkGet(p, index, arr, off, length)
}

// Set assigns the value at the given index.
func (p *Packed64) Set(index int, value int64) {
	majorBitPos := int64(index) * int64(p.bitsPerValue)
	elementPos := int(uint64(majorBitPos) >> uint(packed64BlockBits))
	endBits := int(majorBitPos&packed64ModMask) + p.bpvMinusBlockSize
	v := uint64(value)
	if endBits <= 0 {
		shift := uint(-endBits)
		p.blocks[elementPos] = int64((uint64(p.blocks[elementPos]) & ^(p.maskRight << shift)) | (v << shift))
		return
	}
	shiftA := uint(endBits)
	shiftB := uint(packed64BlockSize - endBits)
	p.blocks[elementPos] = int64((uint64(p.blocks[elementPos]) & ^(p.maskRight >> shiftA)) | (v >> shiftA))
	p.blocks[elementPos+1] = int64((uint64(p.blocks[elementPos+1]) & (^uint64(0) >> shiftA)) | (v << shiftB))
}

// SetBulk writes multiple values starting at index.
func (p *Packed64) SetBulk(index int, arr []int64, off, length int) int {
	if length <= 0 {
		panic("packed: length must be > 0")
	}
	if remaining := p.valueCount - index; remaining < length {
		length = remaining
	}
	originalIndex := index
	encoder := newBulkOperationPacked(p.bitsPerValue)

	offsetInBlocks := index % encoder.LongValueCount()
	if offsetInBlocks != 0 {
		for i := offsetInBlocks; i < encoder.LongValueCount() && length > 0; i++ {
			p.Set(index, arr[off])
			off++
			index++
			length--
		}
		if length == 0 {
			return index - originalIndex
		}
	}

	blockIndex := int((int64(index) * int64(p.bitsPerValue)) >> uint(packed64BlockBits))
	iterations := length / encoder.LongValueCount()
	encoder.EncodeLongsToLongs(arr, off, p.blocks, blockIndex, iterations)
	setValues := iterations * encoder.LongValueCount()
	index += setValues
	length -= setValues

	if index > originalIndex {
		return index - originalIndex
	}
	return mutableBulkSet(p, index, arr, off, length)
}

// Fill assigns val to every index in [fromIndex, toIndex).
func (p *Packed64) Fill(fromIndex, toIndex int, val int64) {
	if UnsignedBitsRequired(uint64(val)) > p.bitsPerValue {
		panic("packed: value exceeds bitsPerValue range")
	}
	nAlignedValues := 64 / gcd(64, p.bitsPerValue)
	span := toIndex - fromIndex
	if span <= 3*nAlignedValues {
		mutableFill(p, fromIndex, toIndex, val)
		return
	}

	fromMod := fromIndex % nAlignedValues
	if fromMod != 0 {
		for i := fromMod; i < nAlignedValues; i++ {
			p.Set(fromIndex, val)
			fromIndex++
		}
	}

	nAlignedBlocks := (nAlignedValues * p.bitsPerValue) >> 6
	tmp := newPacked64(nAlignedValues, p.bitsPerValue)
	for i := 0; i < nAlignedValues; i++ {
		tmp.Set(i, val)
	}
	startBlock := int((int64(fromIndex) * int64(p.bitsPerValue)) >> 6)
	endBlock := int((int64(toIndex) * int64(p.bitsPerValue)) >> 6)
	for block := startBlock; block < endBlock; block++ {
		p.blocks[block] = tmp.blocks[block%nAlignedBlocks]
	}

	for i := int((int64(endBlock) << 6) / int64(p.bitsPerValue)); i < toIndex; i++ {
		p.Set(i, val)
	}
}

// Clear zeroes the underlying blocks.
func (p *Packed64) Clear() {
	for i := range p.blocks {
		p.blocks[i] = 0
	}
}

// RamBytesUsed mirrors the Java accounting.
func (p *Packed64) RamBytesUsed() int64 {
	return util.AlignObjectSize(int64(3*4+8+util.NumBytesObjectRef)) + util.SizeOfInt64Slice(p.blocks)
}

// String returns a debug-friendly representation.
func (p *Packed64) String() string {
	return fmt.Sprintf("Packed64(bitsPerValue=%d,size=%d,blocks=%d)", p.bitsPerValue, p.valueCount, len(p.blocks))
}

func gcd(a, b int) int {
	if a < b {
		return gcd(b, a)
	}
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}
