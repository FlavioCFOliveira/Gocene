// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"math"
	"math/bits"
)

// SparseFixedBitSet is a bit set that only stores longs that have at least one bit set.
// The space of bits is divided into blocks of 4096 bits (64 longs). For each block:
// - a long[] which stores the non-zero longs for that block
// - a long so that bit i being set means that the i-th long of the block is non-null,
//   and its offset in the array of longs is the number of one bits on the right of the i-th bit
//
// This is the Go port of Lucene's org.apache.lucene.util.SparseFixedBitSet.
type SparseFixedBitSet struct {
	indices          []uint64
	bits             [][]uint64
	length           int
	nonZeroLongCount int
	ramBytesUsed     int64
}

const (
	// mask4096 is (1 << 12) - 1, used for 4096-bit blocks
	mask4096 = 4095
	// bitsPerBlock is 4096 bits per block
	bitsPerBlock = 4096
	// longsPerBlock is 64 longs per block (4096 / 64)
	longsPerBlock = 64
)

// blockCount returns the number of blocks needed for the given length
func blockCount(length int) int {
	bc := length >> 12
	if (bc << 12) < length {
		bc++
	}
	return bc
}

// NewSparseFixedBitSet creates a new SparseFixedBitSet that can contain bits between 0 (inclusive)
// and length (exclusive).
func NewSparseFixedBitSet(length int) (*SparseFixedBitSet, error) {
	if length < 1 {
		return nil, fmt.Errorf("length needs to be >= 1, got %d", length)
	}
	bc := blockCount(length)
	indices := make([]uint64, bc)
	bits := make([][]uint64, bc)

	// Calculate base RAM usage
	baseRam := int64(24) + int64(len(indices))*8 + int64(len(bits))*24 // approximate

	return &SparseFixedBitSet{
		indices:      indices,
		bits:         bits,
		length:       length,
		ramBytesUsed: baseRam,
	}, nil
}

// ClearAll clears all bits in the set.
func (sfs *SparseFixedBitSet) ClearAll() {
	for i := range sfs.bits {
		sfs.bits[i] = nil
	}
	for i := range sfs.indices {
		sfs.indices[i] = 0
	}
	sfs.nonZeroLongCount = 0

	// Recalculate base RAM
	bc := blockCount(sfs.length)
	sfs.ramBytesUsed = int64(24) + int64(bc)*8 + int64(bc)*24
}

// Length returns the number of bits in this bitset.
func (sfs *SparseFixedBitSet) Length() int {
	return sfs.length
}

// Cardinality returns the number of set bits.
func (sfs *SparseFixedBitSet) Cardinality() int {
	cardinality := 0
	for _, bitArray := range sfs.bits {
		if bitArray != nil {
			for _, b := range bitArray {
				cardinality += bits.OnesCount64(b)
			}
		}
	}
	return cardinality
}

// ApproximateCardinality returns an estimate of the cardinality using linear counting algorithm.
func (sfs *SparseFixedBitSet) ApproximateCardinality() int {
	// We are assuming that bits are uniformly set and use the linear counting
	// algorithm to estimate the number of bits that are set based on the number
	// of longs that are different from zero
	totalLongs := (sfs.length + 63) >> 6 // total number of longs in the space
	zeroLongs := totalLongs - sfs.nonZeroLongCount // number of longs that are zeros

	if zeroLongs == 0 {
		return sfs.length
	}

	// Linear counting algorithm
	estimate := math.Round(float64(totalLongs) * math.Log(float64(totalLongs)/float64(zeroLongs)))
	if estimate > float64(sfs.length) {
		return sfs.length
	}
	return int(estimate)
}

// Get returns true if the bit at index i is set.
func (sfs *SparseFixedBitSet) Get(i int) bool {
	if i < 0 || i >= sfs.length {
		panic(fmt.Sprintf("index out of bounds: %d (length: %d)", i, sfs.length))
	}
	i4096 := i >> 12
	index := sfs.indices[i4096]
	i64 := i >> 6
	i64bit := uint64(1) << uint(i64&63) // shifts are mod 64

	// First check the index, if the i64-th bit is not set, then i is not set
	if (index & i64bit) == 0 {
		return false
	}

	// If it is set, then we count the number of bits that are set on the right
	// of i64, and that gives us the index of the long that stores the bits we
	// are interested in
	bitArray := sfs.bits[i4096]
	if bitArray == nil {
		return false
	}
	idx := bits.OnesCount64(index & (i64bit - 1))
	if idx >= len(bitArray) {
		return false
	}
	b := bitArray[idx]
	return (b & (1 << uint(i&63))) != 0
}

// GetAndSet sets the bit at index i and returns the previous value.
func (sfs *SparseFixedBitSet) GetAndSet(i int) bool {
	if i < 0 || i >= sfs.length {
		panic(fmt.Sprintf("index out of bounds: %d (length: %d)", i, sfs.length))
	}
	i4096 := i >> 12
	index := sfs.indices[i4096]
	i64 := i >> 6
	i64bit := uint64(1) << uint(i64&63)

	if (index & i64bit) != 0 {
		// The sub 64-bits block we are interested in already exists
		location := bits.OnesCount64(index & (i64bit - 1))
		bit := uint64(1) << uint(i&63)
		v := (sfs.bits[i4096][location] & bit) != 0
		sfs.bits[i4096][location] |= bit
		return v
	} else if index == 0 {
		// We just found a block of 4096 bits that has no bit set yet
		sfs.insertBlock(i4096, i64bit, i)
		return false
	} else {
		// We found a block with some values, but the sub-block of 64 bits has no value yet
		sfs.insertLong(i4096, i64bit, i, index)
		return false
	}
}

// sparseOversize returns a new size for growing arrays
func sparseOversize(s int) int {
	newSize := s + (s >> 1)
	if newSize > 50 {
		newSize = 64
	}
	return newSize
}

// Set sets the bit at index i.
func (sfs *SparseFixedBitSet) Set(i int) {
	if i < 0 || i >= sfs.length {
		panic(fmt.Sprintf("index out of bounds: %d (length: %d)", i, sfs.length))
	}
	i4096 := i >> 12
	index := sfs.indices[i4096]
	i64 := i >> 6
	i64bit := uint64(1) << uint(i64&63)

	if (index & i64bit) != 0 {
		// The sub 64-bits block we are interested in already exists
		location := bits.OnesCount64(index & (i64bit - 1))
		sfs.bits[i4096][location] |= 1 << uint(i&63)
	} else if index == 0 {
		// We just found a block of 4096 bits that has no bit set yet
		sfs.insertBlock(i4096, i64bit, i)
	} else {
		// We found a block with some values, but the sub-block of 64 bits has no value yet
		sfs.insertLong(i4096, i64bit, i, index)
	}
}

// insertBlock initializes a new block
func (sfs *SparseFixedBitSet) insertBlock(i4096 int, i64bit uint64, i int) {
	sfs.indices[i4096] = i64bit
	sfs.bits[i4096] = []uint64{1 << uint(i&63)}
	sfs.nonZeroLongCount++
	sfs.ramBytesUsed += 8 // one long
}

// insertLong inserts a new long into an existing block
func (sfs *SparseFixedBitSet) insertLong(i4096 int, i64bit uint64, i int, index uint64) {
	sfs.indices[i4096] |= i64bit
	// We count the number of bits that are set on the right of i64
	// This gives us the index at which to perform the insertion
	o := bits.OnesCount64(index & (i64bit - 1))
	bitArray := sfs.bits[i4096]
	if len(bitArray) > 0 && bitArray[len(bitArray)-1] == 0 {
		// Since we only store non-zero longs, if the last value is 0, it means
		// we already have extra space, make use of it
		copy(bitArray[o+1:], bitArray[o:len(bitArray)-1])
		bitArray[o] = 1 << uint(i&63)
	} else {
		// We don't have extra space so we need to resize to insert the new long
		newSize := sparseOversize(len(bitArray) + 1)
		newBitArray := make([]uint64, newSize)
		copy(newBitArray, bitArray[:o])
		newBitArray[o] = 1 << uint(i&63)
		copy(newBitArray[o+1:], bitArray[o:])
		sfs.bits[i4096] = newBitArray
		// We may slightly overestimate size here, but keep it cheap
		sfs.ramBytesUsed += int64(newSize-len(bitArray)) * 8
	}
	sfs.nonZeroLongCount++
}

// Clear clears the bit at index i.
func (sfs *SparseFixedBitSet) Clear(i int) {
	if i < 0 || i >= sfs.length {
		panic(fmt.Sprintf("index out of bounds: %d (length: %d)", i, sfs.length))
	}
	i4096 := i >> 12
	i64 := i >> 6
	sfs.and(i4096, i64, ^(uint64(1) << uint(i&63)))
}

// and performs a bitwise AND with a mask
func (sfs *SparseFixedBitSet) and(i4096, i64 int, mask uint64) {
	index := sfs.indices[i4096]
	i64bit := uint64(1) << uint(i64&63)
	if (index & i64bit) != 0 {
		// Offset of the long bits we are interested in in the array
		o := bits.OnesCount64(index & ((i64bit) - 1))
		if o < len(sfs.bits[i4096]) {
			newBits := sfs.bits[i4096][o] & mask
			if newBits == 0 {
				sfs.removeLong(i4096, i64, index, o)
			} else {
				sfs.bits[i4096][o] = newBits
			}
		}
	}
}

// removeLong removes a long from a block
func (sfs *SparseFixedBitSet) removeLong(i4096, i64 int, index uint64, o int) {
	index &= ^(uint64(1) << uint(i64&63))
	sfs.indices[i4096] = index
	if index == 0 {
		// Release memory, there is nothing in this block anymore
		sfs.bits[i4096] = nil
	} else {
		length := bits.OnesCount64(index)
		bitArray := sfs.bits[i4096]
		if o+1 < len(bitArray) {
			copy(bitArray[o:], bitArray[o+1:])
		}
		if length < len(bitArray) {
			bitArray[length] = 0
		}
	}
	sfs.nonZeroLongCount--
}

// ClearRange clears bits from start (inclusive) to end (exclusive).
func (sfs *SparseFixedBitSet) ClearRange(from, to int) {
	if from >= to {
		return
	}
	if from < 0 {
		from = 0
	}
	if to > sfs.length {
		to = sfs.length
	}

	firstBlock := from >> 12
	lastBlock := (to - 1) >> 12
	if firstBlock == lastBlock {
		sfs.clearWithinBlock(firstBlock, from&mask4096, (to-1)&mask4096)
	} else {
		sfs.clearWithinBlock(firstBlock, from&mask4096, mask4096)
		for i := firstBlock + 1; i < lastBlock; i++ {
			sfs.nonZeroLongCount -= bits.OnesCount64(sfs.indices[i])
			sfs.indices[i] = 0
			sfs.bits[i] = nil
		}
		sfs.clearWithinBlock(lastBlock, 0, (to-1)&mask4096)
	}
}

// maskForRange creates a long that has bits set to one between from and to
func maskForRange(from, to int) uint64 {
	return ((uint64(1) << uint(to-from+1)) - 1) << uint(from)
}

// clearWithinBlock clears bits within a single block
func (sfs *SparseFixedBitSet) clearWithinBlock(i4096, from, to int) {
	firstLong := from >> 6
	lastLong := to >> 6

	if firstLong == lastLong {
		sfs.and(i4096, firstLong, ^maskForRange(from&63, to&63))
	} else {
		sfs.and(i4096, lastLong, ^maskForRange(0, to&63))
		for i := lastLong - 1; i >= firstLong+1; i-- {
			sfs.and(i4096, i, 0)
		}
		sfs.and(i4096, firstLong, ^maskForRange(from&63, 63))
	}
}

// NextSetBit returns the index of the next set bit at or after the given index.
// If upperBound is provided, it returns the next set bit before the upper bound.
// Returns -1 if no more set bits exist.
func (sfs *SparseFixedBitSet) NextSetBit(start int, upperBound ...int) int {
	ub := sfs.length
	if len(upperBound) > 0 && upperBound[0] < ub {
		ub = upperBound[0]
	}
	if start >= sfs.length {
		return -1
	}
	if start < 0 {
		start = 0
	}
	res := sfs.nextSetBitInRange(start, ub)
	if res < ub && res >= 0 {
		return res
	}
	return -1
}

// nextSetBitInRange returns the next set bit in the specified range.
func (sfs *SparseFixedBitSet) nextSetBitInRange(start, upperBound int) int {
	if start >= sfs.length {
		return -1
	}
	i4096 := start >> 12
	if i4096 >= len(sfs.indices) {
		return -1
	}
	index := sfs.indices[i4096]
	i64 := start >> 6
	i64bit := uint64(1) << uint(i64&63)
	o := bits.OnesCount64(index & (i64bit - 1))

	if (index & i64bit) != 0 {
		// There is at least one bit that is set in the current long
		bitArray := sfs.bits[i4096]
		if o < len(bitArray) {
			b := bitArray[o] >> uint(start&63)
			if b != 0 {
				return start + bits.TrailingZeros64(b)
			}
		}
		o++
	}

	indexBits := index >> uint(i64&63) >> 1
	if indexBits == 0 {
		// No more bits are set in the current block of 4096 bits, go to the next one
		i4096upper := blockCount(upperBound)
		if upperBound == sfs.length {
			i4096upper = len(sfs.indices)
		}
		return sfs.firstDoc(i4096+1, i4096upper)
	}

	// There are still set bits
	i64 += 1 + bits.TrailingZeros64(indexBits)
	bitArray := sfs.bits[i4096]
	if o < len(bitArray) {
		b := bitArray[o]
		return (i64 << 6) + bits.TrailingZeros64(b)
	}
	return -1
}

// firstDoc returns the first document that occurs on or after the provided block index.
func (sfs *SparseFixedBitSet) firstDoc(i4096, i4096upper int) int {
	for i4096 < i4096upper && i4096 < len(sfs.indices) {
		index := sfs.indices[i4096]
		if index != 0 {
			i64 := bits.TrailingZeros64(index)
			if sfs.bits[i4096] != nil && len(sfs.bits[i4096]) > 0 {
				return (i4096 << 12) | (i64 << 6) | bits.TrailingZeros64(sfs.bits[i4096][0])
			}
		}
		i4096++
	}
	return -1
}

// PrevSetBit returns the index of the previous set bit before the given index.
// Returns -1 if no previous set bits exist.
func (sfs *SparseFixedBitSet) PrevSetBit(i int) int {
	if i < 0 {
		return -1
	}
	if i >= sfs.length {
		i = sfs.length - 1
	}

	i4096 := i >> 12
	if i4096 >= len(sfs.indices) {
		return sfs.lastDoc(len(sfs.indices) - 1)
	}
	index := sfs.indices[i4096]
	i64 := i >> 6
	indexBits := index & ((uint64(1) << uint(i64&63)) - 1)
	o := bits.OnesCount64(indexBits)

	i64bit := uint64(1) << uint(i64&63)
	if (index & i64bit) != 0 {
		// There is at least one bit that is set in the same long
		bitArray := sfs.bits[i4096]
		if o < len(bitArray) {
			b := bitArray[o] & ((uint64(1) << uint((i&63)+1)) - 1)
			if b != 0 {
				return (i64 << 6) + (63 - bits.LeadingZeros64(b))
			}
		}
	}

	if indexBits == 0 {
		// No more bits are set in this block, go find the last bit in the previous block
		return sfs.lastDoc(i4096 - 1)
	}

	// Go to the previous long
	i64 = 63 - bits.LeadingZeros64(indexBits)
	if o > 0 {
		bitArray := sfs.bits[i4096]
		if o-1 < len(bitArray) {
			b := bitArray[o-1]
			return (i4096 << 12) | (i64 << 6) | (63 - bits.LeadingZeros64(b))
		}
	}
	return -1
}

// lastDoc returns the last document that occurs on or before the provided block index.
func (sfs *SparseFixedBitSet) lastDoc(i4096 int) int {
	for i4096 >= 0 {
		index := sfs.indices[i4096]
		if index != 0 {
			i64 := 63 - bits.LeadingZeros64(index)
			bitArray := sfs.bits[i4096]
			if bitArray != nil && len(bitArray) > 0 {
				idx := bits.OnesCount64(index) - 1
				if idx >= 0 && idx < len(bitArray) {
					b := bitArray[idx]
					return (i4096 << 12) | (i64 << 6) | (63 - bits.LeadingZeros64(b))
				}
			}
		}
		i4096--
	}
	return -1
}

// IsEmpty returns true if no bits are set.
func (sfs *SparseFixedBitSet) IsEmpty() bool {
	return sfs.nonZeroLongCount == 0
}

// Or performs a bitwise OR with another SparseFixedBitSet.
func (sfs *SparseFixedBitSet) Or(other *SparseFixedBitSet) error {
	if sfs.length != other.length {
		return fmt.Errorf("bitset lengths do not match: %d vs %d", sfs.length, other.length)
	}

	for i := 0; i < len(other.indices); i++ {
		index := other.indices[i]
		if index != 0 {
			sfs.orBlock(i, index, other.bits[i], bits.OnesCount64(index))
		}
	}
	return nil
}

// orBlock performs OR on a single block
func (sfs *SparseFixedBitSet) orBlock(i4096 int, index uint64, otherBits []uint64, nonZeroLongCount int) {
	currentIndex := sfs.indices[i4096]
	if currentIndex == 0 {
		// Fast path: if we currently have nothing in the block, just copy the data
		sfs.indices[i4096] = index
		newBits := make([]uint64, nonZeroLongCount)
		copy(newBits, otherBits[:nonZeroLongCount])
		sfs.bits[i4096] = newBits
		sfs.ramBytesUsed += int64(len(newBits)) * 8
		sfs.nonZeroLongCount += nonZeroLongCount
		return
	}

	currentBits := sfs.bits[i4096]
	newIndex := currentIndex | index
	requiredCapacity := bits.OnesCount64(newIndex)

	var newBits []uint64
	if len(currentBits) >= requiredCapacity {
		newBits = currentBits
	} else {
		newBits = make([]uint64, sparseOversize(requiredCapacity))
		sfs.ramBytesUsed += int64(len(newBits)-len(currentBits)) * 8
	}

	// Iterate through all bit positions in the new index
	// and compute the OR for each position
	newO := 0
	for i := 0; i < 64; i++ {
		if (newIndex & (uint64(1) << uint(i))) != 0 {
			newBits[newO] = sfs.longBits(currentIndex, currentBits, i) | sfs.longBits(index, otherBits, i)
			newO++
		}
	}

	sfs.indices[i4096] = newIndex
	sfs.bits[i4096] = newBits
	sfs.nonZeroLongCount += nonZeroLongCount - bits.OnesCount64(currentIndex&index)
}

// longBits returns the long bits at the given i64 index
func (sfs *SparseFixedBitSet) longBits(index uint64, bitArray []uint64, i64 int) uint64 {
	if (index & (uint64(1) << uint(i64&63))) == 0 {
		return 0
	}
	idx := bits.OnesCount64(index & ((uint64(1) << uint(i64&63)) - 1))
	if idx < len(bitArray) {
		return bitArray[idx]
	}
	return 0
}

// RamBytesUsed returns the RAM usage in bytes.
func (sfs *SparseFixedBitSet) RamBytesUsed() int64 {
	return sfs.ramBytesUsed
}

// Clone creates a copy of this SparseFixedBitSet.
func (sfs *SparseFixedBitSet) Clone() *SparseFixedBitSet {
	newIndices := make([]uint64, len(sfs.indices))
	copy(newIndices, sfs.indices)

	newBits := make([][]uint64, len(sfs.bits))
	for i, bitArray := range sfs.bits {
		if bitArray != nil {
			newBits[i] = make([]uint64, len(bitArray))
			copy(newBits[i], bitArray)
		}
	}

	return &SparseFixedBitSet{
		indices:          newIndices,
		bits:             newBits,
		length:           sfs.length,
		nonZeroLongCount: sfs.nonZeroLongCount,
		ramBytesUsed:     sfs.ramBytesUsed,
	}
}

// Equals returns true if this bitset equals another.
func (sfs *SparseFixedBitSet) Equals(other *SparseFixedBitSet) bool {
	if sfs.length != other.length {
		return false
	}
	if sfs.nonZeroLongCount != other.nonZeroLongCount {
		return false
	}
	for i := range sfs.indices {
		if sfs.indices[i] != other.indices[i] {
			return false
		}
		if len(sfs.bits[i]) != len(other.bits[i]) {
			return false
		}
		for j := range sfs.bits[i] {
			if sfs.bits[i][j] != other.bits[i][j] {
				return false
			}
		}
	}
	return true
}

// Ensure SparseFixedBitSet implements Bits interface
var _ Bits = (*SparseFixedBitSet)(nil)
