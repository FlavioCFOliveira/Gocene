// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"fmt"
)

// LongBitSet is a bitset of fixed length (numBits), backed by accessible long[],
// accessed with a long index. Use it only if you intend to store more than 2.1B bits,
// otherwise you should use FixedBitSet.
type LongBitSet struct {
	bits     []uint64 // Array of uint64s holding the bits
	numBits  int64    // The number of bits in use
	numWords int      // The exact number of longs needed to hold numBits (<= len(bits))
}

// MaxNumBits is the maximum numBits supported
const MaxNumBits = 64 * (1 << 31) // 64 * max array length

// Bits2Words returns the number of 64 bit words it would take to hold numBits
func Bits2Words(numBits int64) int {
	if numBits < 0 || numBits > MaxNumBits {
		panic(fmt.Sprintf("numBits must be 0 .. %d; got: %d", MaxNumBits, numBits))
	}
	// Get the word-offset of the last bit and add one (make sure to use >> so 0 returns 0!)
	if numBits == 0 {
		return 0
	}
	return int((numBits-1)>>6) + 1
}

// NewLongBitSet creates a new LongBitSet. The internally allocated long array will be
// exactly the size needed to accommodate the numBits specified.
func NewLongBitSet(numBits int64) (*LongBitSet, error) {
	if numBits < 0 {
		return nil, errors.New("numBits must be non-negative")
	}
	numWords := Bits2Words(numBits)
	return &LongBitSet{
		bits:     make([]uint64, numWords),
		numBits:  numBits,
		numWords: numWords,
	}, nil
}

// NewLongBitSetFromBits creates a new LongBitSet using the provided uint64 array as backing store.
// The storedBits array must be large enough to accommodate the numBits specified, but may be larger.
// In that case the 'extra' or 'ghost' bits must be clear.
func NewLongBitSetFromBits(storedBits []uint64, numBits int64) (*LongBitSet, error) {
	numWords := Bits2Words(numBits)
	if numWords > len(storedBits) {
		return nil, fmt.Errorf("the given long array is too small to hold %d bits", numBits)
	}
	return &LongBitSet{
		bits:     storedBits,
		numBits:  numBits,
		numWords: numWords,
	}, nil
}

// EnsureCapacity returns a LongBitSet that can hold numBits+1 bits.
// If the given LongBitSet is large enough, returns the given bits,
// otherwise returns a new LongBitSet which can hold the requested number of bits.
// NOTE: the returned bitset reuses the underlying long[] of the given bits if possible.
func EnsureCapacity(bits *LongBitSet, numBits int64) *LongBitSet {
	if numBits < bits.numBits {
		return bits
	}
	// Depends on the ghost bits being clear!
	numWords := Bits2Words(numBits)
	arr := bits.bits
	if numWords >= len(arr) {
		// Grow the array
		newArr := make([]uint64, numWords+1)
		copy(newArr, arr)
		arr = newArr
	}
	return &LongBitSet{
		bits:     arr,
		numBits:  int64(len(arr)) << 6,
		numWords: len(arr),
	}
}

// Length returns the number of bits stored in this bitset
func (b *LongBitSet) Length() int64 {
	return b.numBits
}

// GetBits returns the backing bits array (expert use only)
func (b *LongBitSet) GetBits() []uint64 {
	return b.bits
}

// Cardinality returns the number of set bits.
// NOTE: this visits every long in the backing bits array, and the result is not internally cached!
func (b *LongBitSet) Cardinality() int64 {
	// Depends on the ghost bits being clear!
	var tot int64
	for i := 0; i < b.numWords; i++ {
		tot += int64(popcount(b.bits[i]))
	}
	return tot
}

// Get returns the value of the bit at the specified index
func (b *LongBitSet) Get(index int64) bool {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	i := index >> 6                        // div 64
	bitmask := uint64(1) << (index & 0x3f) // mod 64
	return (b.bits[i] & bitmask) != 0
}

// Set sets the bit at the specified index
func (b *LongBitSet) Set(index int64) {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	wordNum := index >> 6
	bitmask := uint64(1) << (index & 0x3f)
	b.bits[wordNum] |= bitmask
}

// GetAndSet sets the bit at the specified index and returns the previous value
func (b *LongBitSet) GetAndSet(index int64) bool {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	wordNum := index >> 6
	bitmask := uint64(1) << (index & 0x3f)
	val := (b.bits[wordNum] & bitmask) != 0
	b.bits[wordNum] |= bitmask
	return val
}

// Clear clears the bit at the specified index
func (b *LongBitSet) Clear(index int64) {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	wordNum := index >> 6
	bitmask := uint64(1) << (index & 0x3f)
	b.bits[wordNum] &^= bitmask
}

// GetAndClear clears the bit at the specified index and returns the previous value
func (b *LongBitSet) GetAndClear(index int64) bool {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	wordNum := index >> 6
	bitmask := uint64(1) << (index & 0x3f)
	val := (b.bits[wordNum] & bitmask) != 0
	b.bits[wordNum] &^= bitmask
	return val
}

// NextSetBit returns the index of the first set bit starting at the index specified.
// -1 is returned if there are no more set bits.
func (b *LongBitSet) NextSetBit(index int64) int64 {
	if index < 0 {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	if index >= b.numBits {
		return -1
	}
	i := int(index >> 6)
	word := b.bits[i] >> (index & 0x3f) // skip all the bits to the right of index

	if word != 0 {
		return index + int64(trailingZeros(word))
	}

	for i++; i < b.numWords; i++ {
		word = b.bits[i]
		if word != 0 {
			return int64(i<<6) + int64(trailingZeros(word))
		}
	}

	return -1
}

// PrevSetBit returns the index of the last set bit before or on the index specified.
// -1 is returned if there are no more set bits.
func (b *LongBitSet) PrevSetBit(index int64) int64 {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	i := int(index >> 6)
	subIndex := int(index & 0x3f)        // index within the word
	word := b.bits[i] << (63 - subIndex) // skip all the bits to the left of index

	if word != 0 {
		return int64(i<<6) + int64(subIndex) - int64(leadingZeros(word))
	}

	for i--; i >= 0; i-- {
		word = b.bits[i]
		if word != 0 {
			return int64(i<<6) + 63 - int64(leadingZeros(word))
		}
	}

	return -1
}

// Or performs this = this OR other
func (b *LongBitSet) Or(other *LongBitSet) {
	if other.numWords > b.numWords {
		panic(fmt.Sprintf("numWords=%d, other.numWords=%d", b.numWords, other.numWords))
	}
	pos := min(b.numWords, other.numWords)
	for i := 0; i < pos; i++ {
		b.bits[i] |= other.bits[i]
	}
}

// Xor performs this = this XOR other
func (b *LongBitSet) Xor(other *LongBitSet) {
	if other.numWords > b.numWords {
		panic(fmt.Sprintf("numWords=%d, other.numWords=%d", b.numWords, other.numWords))
	}
	pos := min(b.numWords, other.numWords)
	for i := 0; i < pos; i++ {
		b.bits[i] ^= other.bits[i]
	}
}

// Intersects returns true if the sets have any elements in common
func (b *LongBitSet) Intersects(other *LongBitSet) bool {
	// Depends on the ghost bits being clear!
	pos := min(b.numWords, other.numWords)
	for i := 0; i < pos; i++ {
		if (b.bits[i] & other.bits[i]) != 0 {
			return true
		}
	}
	return false
}

// And performs this = this AND other
func (b *LongBitSet) And(other *LongBitSet) {
	pos := min(b.numWords, other.numWords)
	for i := 0; i < pos; i++ {
		b.bits[i] &= other.bits[i]
	}
	if b.numWords > other.numWords {
		for i := other.numWords; i < b.numWords; i++ {
			b.bits[i] = 0
		}
	}
}

// AndNot performs this = this AND NOT other
func (b *LongBitSet) AndNot(other *LongBitSet) {
	pos := min(b.numWords, other.numWords)
	for i := 0; i < pos; i++ {
		b.bits[i] &^= other.bits[i]
	}
}

// ScanIsEmpty scans the backing store to check if all bits are clear.
// The method is deliberately not called "IsEmpty" to emphasize it is not low cost.
func (b *LongBitSet) ScanIsEmpty() bool {
	// Depends on the ghost bits being clear!
	for i := 0; i < b.numWords; i++ {
		if b.bits[i] != 0 {
			return false
		}
	}
	return true
}

// Flip flips a range of bits [startIndex, endIndex)
func (b *LongBitSet) Flip(startIndex, endIndex int64) {
	if startIndex < 0 || startIndex >= b.numBits {
		panic(fmt.Sprintf("startIndex=%d, numBits=%d", startIndex, b.numBits))
	}
	if endIndex < 0 || endIndex > b.numBits {
		panic(fmt.Sprintf("endIndex=%d, numBits=%d", endIndex, b.numBits))
	}
	if endIndex <= startIndex {
		return
	}

	startWord := int(startIndex >> 6)
	endWord := int((endIndex - 1) >> 6)

	startmask := ^uint64(0) << (startIndex & 0x3f)
	endmask := ^uint64(0) >> (64 - (endIndex & 0x3f))
	if endIndex&0x3f == 0 {
		endmask = ^uint64(0)
	}

	if startWord == endWord {
		b.bits[startWord] ^= (startmask & endmask)
		return
	}

	b.bits[startWord] ^= startmask

	for i := startWord + 1; i < endWord; i++ {
		b.bits[i] = ^b.bits[i]
	}

	b.bits[endWord] ^= endmask
}

// FlipSingle flips the bit at the provided index
func (b *LongBitSet) FlipSingle(index int64) {
	if index < 0 || index >= b.numBits {
		panic(fmt.Sprintf("index=%d, numBits=%d", index, b.numBits))
	}
	wordNum := index >> 6
	bitmask := uint64(1) << (index & 0x3f)
	b.bits[wordNum] ^= bitmask
}

// SetRange sets a range of bits [startIndex, endIndex)
func (b *LongBitSet) SetRange(startIndex, endIndex int64) {
	if startIndex < 0 || startIndex >= b.numBits {
		panic(fmt.Sprintf("startIndex=%d, numBits=%d", startIndex, b.numBits))
	}
	if endIndex < 0 || endIndex > b.numBits {
		panic(fmt.Sprintf("endIndex=%d, numBits=%d", endIndex, b.numBits))
	}
	if endIndex <= startIndex {
		return
	}

	startWord := int(startIndex >> 6)
	endWord := int((endIndex - 1) >> 6)

	startmask := ^uint64(0) << (startIndex & 0x3f)
	endmask := ^uint64(0) >> (64 - (endIndex & 0x3f))
	if endIndex&0x3f == 0 {
		endmask = ^uint64(0)
	}

	if startWord == endWord {
		b.bits[startWord] |= (startmask & endmask)
		return
	}

	b.bits[startWord] |= startmask
	for i := startWord + 1; i < endWord; i++ {
		b.bits[i] = ^uint64(0)
	}
	b.bits[endWord] |= endmask
}

// ClearRange clears a range of bits [startIndex, endIndex)
func (b *LongBitSet) ClearRange(startIndex, endIndex int64) {
	if startIndex < 0 || startIndex >= b.numBits {
		panic(fmt.Sprintf("startIndex=%d, numBits=%d", startIndex, b.numBits))
	}
	if endIndex < 0 || endIndex > b.numBits {
		panic(fmt.Sprintf("endIndex=%d, numBits=%d", endIndex, b.numBits))
	}
	if endIndex <= startIndex {
		return
	}

	startWord := int(startIndex >> 6)
	endWord := int((endIndex - 1) >> 6)

	startmask := ^uint64(0) << (startIndex & 0x3f)
	endmask := ^uint64(0) >> (64 - (endIndex & 0x3f))
	if endIndex&0x3f == 0 {
		endmask = ^uint64(0)
	}

	// invert masks since we are clearing
	startmask = ^startmask
	endmask = ^endmask

	if startWord == endWord {
		b.bits[startWord] &= (startmask | endmask)
		return
	}

	b.bits[startWord] &= startmask
	for i := startWord + 1; i < endWord; i++ {
		b.bits[i] = 0
	}
	b.bits[endWord] &= endmask
}

// Clone returns a copy of this bitset
func (b *LongBitSet) Clone() *LongBitSet {
	newBits := make([]uint64, len(b.bits))
	copy(newBits, b.bits)
	return &LongBitSet{
		bits:     newBits,
		numBits:  b.numBits,
		numWords: b.numWords,
	}
}

// Equals returns true if both sets have the same bits set
func (b *LongBitSet) Equals(other interface{}) bool {
	if b == other {
		return true
	}
	otherSet, ok := other.(*LongBitSet)
	if !ok {
		return false
	}
	if b.numBits != otherSet.numBits {
		return false
	}
	// Depends on the ghost bits being clear!
	for i := 0; i < len(b.bits); i++ {
		if b.bits[i] != otherSet.bits[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this bitset
func (b *LongBitSet) HashCode() int {
	// Depends on the ghost bits being clear!
	var h uint64
	for i := b.numWords - 1; i >= 0; i-- {
		h ^= b.bits[i]
		h = (h << 1) | (h >> 63) // rotate left
	}
	// fold leftmost bits into right and add a constant to prevent
	// empty sets from returning 0, which is too common.
	return int(((h >> 32) ^ h) + 0x98761234)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
