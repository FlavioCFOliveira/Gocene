// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

const (
	// bitsPerWord is the number of bits in a uint64
	bitsPerWord = 64
	// log2BitsPerWord is log2(bitsPerWord)
	log2BitsPerWord = 6
	// wordMask is the mask to get the bit index within a word
	wordMask = bitsPerWord - 1
)

// FixedBitSet is a fixed-size bitset implementation using []uint64.
// This is more memory-efficient than using []bool for large bitsets.
//
// This is the Go port of Lucene's org.apache.lucene.util.FixedBitSet.
type FixedBitSet struct {
	bits []uint64
	size int
}

// NewFixedBitSet creates a new FixedBitSet with the given number of bits.
func NewFixedBitSet(numBits int) (*FixedBitSet, error) {
	if numBits < 0 {
		return nil, fmt.Errorf("numBits must be non-negative, got %d", numBits)
	}
	numWords := wordsNeeded(numBits)
	return &FixedBitSet{
		bits: make([]uint64, numWords),
		size: numBits,
	}, nil
}

// wordsNeeded returns the number of uint64 words needed for the given number of bits.
func wordsNeeded(numBits int) int {
	if numBits <= 0 {
		return 0
	}
	return (numBits + bitsPerWord - 1) >> log2BitsPerWord
}

// wordIndex returns the index of the uint64 containing the given bit.
func wordIndex(bitIndex int) int {
	return bitIndex >> log2BitsPerWord
}

// bitIndex returns the position of the bit within its uint64 word.
func bitIndex(bitIndex int) uint {
	return uint(bitIndex & wordMask)
}

// Get returns true if the bit at the given index is set.
func (fs *FixedBitSet) Get(index int) bool {
	if index < 0 || index >= fs.size {
		panic(fmt.Sprintf("index out of bounds: %d (size: %d)", index, fs.size))
	}
	wordIdx := wordIndex(index)
	bitIdx := bitIndex(index)
	return (fs.bits[wordIdx] & (1 << bitIdx)) != 0
}

// Set sets the bit at the given index to true.
func (fs *FixedBitSet) Set(index int) {
	if index < 0 || index >= fs.size {
		panic(fmt.Sprintf("index out of bounds: %d (size: %d)", index, fs.size))
	}
	wordIdx := wordIndex(index)
	bitIdx := bitIndex(index)
	fs.bits[wordIdx] |= 1 << bitIdx
}

// Clear clears the bit at the given index (sets to false).
func (fs *FixedBitSet) Clear(index int) {
	if index < 0 || index >= fs.size {
		panic(fmt.Sprintf("index out of bounds: %d (size: %d)", index, fs.size))
	}
	wordIdx := wordIndex(index)
	bitIdx := bitIndex(index)
	fs.bits[wordIdx] &= ^(1 << bitIdx)
}

// GetAndClear atomically gets the bit at the given index and clears it.
// Returns true if the bit was set (1), false if it was clear (0).
// This is the critical operation for marking documents as deleted in live docs tracking.
//
// Port of Lucene's org.apache.lucene.util.FixedBitSet.getAndClear.
func (fs *FixedBitSet) GetAndClear(index int) bool {
	if index < 0 || index >= fs.size {
		panic(fmt.Sprintf("index out of bounds: %d (size: %d)", index, fs.size))
	}
	wordIdx := wordIndex(index)
	bitIdx := bitIndex(index)
	mask := uint64(1) << bitIdx
	old := (fs.bits[wordIdx] & mask) != 0
	fs.bits[wordIdx] &= ^mask
	return old
}

// ClearAll clears all bits in the bitset.
func (fs *FixedBitSet) ClearAll() {
	for i := range fs.bits {
		fs.bits[i] = 0
	}
}

// SetAll sets all bits in the bitset to true.
func (fs *FixedBitSet) SetAll() {
	for i := range fs.bits {
		fs.bits[i] = ^uint64(0)
	}
	// Clear extra bits beyond size
	fs.clearExtraBits()
}

// clearExtraBits clears any bits beyond the logical size.
func (fs *FixedBitSet) clearExtraBits() {
	if fs.size == 0 {
		return
	}
	lastWord := wordIndex(fs.size - 1)
	bitsInLastWord := fs.size - lastWord*bitsPerWord
	if bitsInLastWord < bitsPerWord {
		mask := uint64(1<<uint(bitsInLastWord)) - 1
		fs.bits[lastWord] &= mask
	}
}

// Length returns the number of bits in this bitset.
func (fs *FixedBitSet) Length() int {
	return fs.size
}

// Cardinality returns the number of set bits.
func (fs *FixedBitSet) Cardinality() int {
	count := 0
	for _, word := range fs.bits {
		count += popcount(word)
	}
	return count
}

// popcount returns the number of set bits in a uint64.
func popcount(x uint64) int {
	// Using the SWAR (SIMD Within A Register) algorithm
	// This is efficient on modern processors
	const (
		m1  = 0x5555555555555555 // binary: 0101...
		m2  = 0x3333333333333333 // binary: 0011...
		m4  = 0x0f0f0f0f0f0f0f0f // binary: 00001111...
		h01 = 0x0101010101010101 // sum of 256^0 + 256^1 + ...
	)

	x -= (x >> 1) & m1
	x = (x & m2) + ((x >> 2) & m2)
	x = (x + (x >> 4)) & m4
	return int((x * h01) >> 56)
}

// IsEmpty returns true if no bits are set.
func (fs *FixedBitSet) IsEmpty() bool {
	for _, word := range fs.bits {
		if word != 0 {
			return false
		}
	}
	return true
}

// NextSetBit returns the index of the next set bit at or after the given index.
// Returns -1 if no more set bits exist.
func (fs *FixedBitSet) NextSetBit(fromIndex int) int {
	if fromIndex >= fs.size {
		return -1
	}
	if fromIndex < 0 {
		fromIndex = 0
	}

	wordIdx := wordIndex(fromIndex)
	bitIdx := bitIndex(fromIndex)

	// Mask out bits before fromIndex in the first word
	word := fs.bits[wordIdx] & (^uint64(0) << bitIdx)

	for wordIdx < len(fs.bits) {
		if word != 0 {
			// Found a set bit
			return wordIdx*bitsPerWord + trailingZeros(word)
		}
		wordIdx++
		if wordIdx < len(fs.bits) {
			word = fs.bits[wordIdx]
		}
	}

	return -1
}

// trailingZeros returns the number of trailing zeros in a uint64.
func trailingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	// Binary search for the first set bit
	n := 0
	if (x & 0xFFFFFFFF) == 0 {
		n += 32
		x >>= 32
	}
	if (x & 0xFFFF) == 0 {
		n += 16
		x >>= 16
	}
	if (x & 0xFF) == 0 {
		n += 8
		x >>= 8
	}
	if (x & 0xF) == 0 {
		n += 4
		x >>= 4
	}
	if (x & 0x3) == 0 {
		n += 2
		x >>= 2
	}
	if (x & 0x1) == 0 {
		n++
	}
	return n
}

// PrevSetBit returns the index of the previous set bit before the given index.
// Returns -1 if no previous set bits exist.
func (fs *FixedBitSet) PrevSetBit(fromIndex int) int {
	if fromIndex < 0 {
		return -1
	}
	if fromIndex >= fs.size {
		fromIndex = fs.size - 1
	}

	wordIdx := wordIndex(fromIndex)
	bitIdx := bitIndex(fromIndex)

	// Mask out bits after fromIndex in the first word
	var word uint64
	if bitIdx == 63 {
		word = fs.bits[wordIdx]
	} else {
		word = fs.bits[wordIdx] & ((1 << (bitIdx + 1)) - 1)
	}

	for wordIdx >= 0 {
		if word != 0 {
			// Found a set bit - return index of highest set bit
			return wordIdx*bitsPerWord + 63 - leadingZeros(word)
		}
		wordIdx--
		if wordIdx >= 0 {
			word = fs.bits[wordIdx]
		}
	}

	return -1
}

// leadingZeros returns the number of leading zeros in a uint64.
func leadingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x <= 0x00000000FFFFFFFF {
		n += 32
		x <<= 32
	}
	if x <= 0x0000FFFFFFFFFFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFFFFFFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFFFFFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFFFFFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFFFFFFFFFF {
		n++
	}
	return n
}

// And performs a bitwise AND with another FixedBitSet.
// Both bitsets must have the same size.
func (fs *FixedBitSet) And(other *FixedBitSet) error {
	if fs.size != other.size {
		return fmt.Errorf("bitset sizes do not match: %d vs %d", fs.size, other.size)
	}
	for i := range fs.bits {
		fs.bits[i] &= other.bits[i]
	}
	return nil
}

// Or performs a bitwise OR with another FixedBitSet.
// Both bitsets must have the same size.
func (fs *FixedBitSet) Or(other *FixedBitSet) error {
	if fs.size != other.size {
		return fmt.Errorf("bitset sizes do not match: %d vs %d", fs.size, other.size)
	}
	for i := range fs.bits {
		fs.bits[i] |= other.bits[i]
	}
	return nil
}

// OrIterator performs a bitwise OR with all documents from a DocIdSetIterator.
func (fs *FixedBitSet) OrIterator(iter DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		fs.Set(doc)
	}
	return nil
}

// Xor performs a bitwise XOR with another FixedBitSet.
// Both bitsets must have the same size.
func (fs *FixedBitSet) Xor(other *FixedBitSet) error {
	if fs.size != other.size {
		return fmt.Errorf("bitset sizes do not match: %d vs %d", fs.size, other.size)
	}
	for i := range fs.bits {
		fs.bits[i] ^= other.bits[i]
	}
	return nil
}

// Not inverts all bits in this bitset.
func (fs *FixedBitSet) Not() {
	for i := range fs.bits {
		fs.bits[i] = ^fs.bits[i]
	}
	fs.clearExtraBits()
}

// AndNot clears bits that are set in the other bitset (AND with NOT).
func (fs *FixedBitSet) AndNot(other *FixedBitSet) error {
	if fs.size != other.size {
		return fmt.Errorf("bitset sizes do not match: %d vs %d", fs.size, other.size)
	}
	for i := range fs.bits {
		fs.bits[i] &= ^other.bits[i]
	}
	return nil
}

// Clone creates a copy of this FixedBitSet.
func (fs *FixedBitSet) Clone() *FixedBitSet {
	newBits := make([]uint64, len(fs.bits))
	copy(newBits, fs.bits)
	return &FixedBitSet{
		bits: newBits,
		size: fs.size,
	}
}

// FixedBitSetCopy creates a copy of a Bits interface as a FixedBitSet.
// Used during copy-on-write for pending deletes.
func FixedBitSetCopy(bits Bits) *FixedBitSet {
	size := bits.Length()
	result, err := NewFixedBitSet(size)
	if err != nil {
		panic(err)
	}
	// Copy each bit from the source
	for i := 0; i < size; i++ {
		if bits.Get(i) {
			result.Set(i)
		}
	}
	return result
}

// Equals returns true if this bitset equals another.
func (fs *FixedBitSet) Equals(other *FixedBitSet) bool {
	if fs.size != other.size {
		return false
	}
	for i := range fs.bits {
		if fs.bits[i] != other.bits[i] {
			return false
		}
	}
	return true
}

// BitsIterator iterates over the set bits in a FixedBitSet.
type BitsIterator struct {
	bits      *FixedBitSet
	nextBit   int
	current   int
	exhausted bool
}

// NewBitsIterator creates a new iterator over the set bits.
func (fs *FixedBitSet) NewBitsIterator() *BitsIterator {
	return &BitsIterator{
		bits:      fs,
		nextBit:   fs.NextSetBit(0),
		exhausted: fs.NextSetBit(0) < 0,
	}
}

// HasNext returns true if there are more set bits.
func (bi *BitsIterator) HasNext() bool {
	return !bi.exhausted
}

// Next returns the next set bit index.
func (bi *BitsIterator) Next() int {
	if bi.exhausted {
		return -1
	}
	result := bi.nextBit
	bi.nextBit = bi.bits.NextSetBit(bi.nextBit + 1)
	bi.exhausted = bi.nextBit < 0
	return result
}

// GetBits returns the underlying uint64 word slice.
//
// The caller must not write to the returned slice outside of index bounds [0,
// NumWords()). Used by low-level codec writers that need to serialise
// individual words (e.g. the Lucene104 doc-block bit-set encoding).
func (fs *FixedBitSet) GetBits() []uint64 {
	return fs.bits
}

// NumWords returns the number of uint64 words backing this bitset.
func (fs *FixedBitSet) NumWords() int {
	return len(fs.bits)
}

// Ensure that FixedBitSet implements Bits
var _ Bits = (*FixedBitSet)(nil)

// bits2words returns the number of 64-bit words it would take to hold
// numBits. Mirrors Lucene's FixedBitSet.bits2words: get the
// word-offset of the last bit and add one (use >> so 0 returns 0!).
func bits2words(numBits int) int {
	return ((numBits - 1) >> log2BitsPerWord) + 1
}

// FixedBitSetEnsureCapacity returns a FixedBitSet able to store a
// value at the desiredBit index. If the current length is sufficient,
// bits is simply returned. Otherwise, a new, larger bitset is
// allocated, with contents of bits copied. Mirrors Lucene's static
// FixedBitSet.ensureCapacity. (Named FixedBitSetEnsureCapacity to
// avoid clashing with LongBitSet's EnsureCapacity.)
func FixedBitSetEnsureCapacity(bits *FixedBitSet, desiredBit int) *FixedBitSet {
	return ensureCapacityInternal(bits, desiredBit, true)
}

// EnsureCapacityAndClear clears the given bits and ensures it can
// store a value at the desiredBit index. If the current length is
// sufficient, bits is simply cleared and returned. Otherwise, a new,
// larger bitset is allocated. Mirrors Lucene's static
// FixedBitSet.ensureCapacityAndClear.
func EnsureCapacityAndClear(bits *FixedBitSet, desiredBit int) *FixedBitSet {
	return ensureCapacityInternal(bits, desiredBit, false)
}

func ensureCapacityInternal(bits *FixedBitSet, desiredBit int, preserveData bool) *FixedBitSet {
	if desiredBit < bits.size {
		if !preserveData {
			bits.ClearAll()
		}
		return bits
	}
	// Depends on the ghost bits being clear!
	// (Otherwise, they may become visible in the new instance)
	numWords := bits2words(desiredBit)
	arr := bits.bits
	if numWords >= len(arr) {
		if preserveData {
			arr = GrowExact(arr, numWords+1)
		} else {
			arr = make([]uint64, Oversize(numWords+1, 8))
		}
	}
	return &FixedBitSet{bits: arr, size: len(arr) << log2BitsPerWord}
}

// readNBits reads numBits (0 < numBits < 64) bits starting at bit
// index from from the given word array. Mirrors Lucene's private
// FixedBitSet.readNBits.
func readNBits(bitSet []uint64, from int, numBits int) uint64 {
	bits := bitSet[from>>log2BitsPerWord] >> (from & wordMask)
	numBitsSoFar := bitsPerWord - (from & wordMask)
	if numBitsSoFar < numBits {
		// Java's `<< -from` masks the shift count to 63 bits; Go does
		// not, so compute the equivalent positive shift explicitly.
		bits |= bitSet[(from>>log2BitsPerWord)+1] << (bitsPerWord - (from & wordMask))
	}
	return bits & ((uint64(1) << numBits) - 1)
}

// FixedBitSetOrRange ors `length` bits starting at sourceFrom from
// source into dest starting at destFrom. Mirrors Lucene's static
// FixedBitSet.orRange. It panics on an invalid index, mirroring the
// IndexOutOfBoundsException thrown by Lucene's
// Objects.checkFromIndexSize.
func FixedBitSetOrRange(source *FixedBitSet, sourceFrom int, dest *FixedBitSet, destFrom int, length int) {
	if length < 0 || sourceFrom < 0 || destFrom < 0 ||
		sourceFrom > source.Length()-length || destFrom > dest.Length()-length {
		panic(fmt.Sprintf("FixedBitSet.orRange: sourceFrom=%d destFrom=%d length=%d out of bounds",
			sourceFrom, destFrom, length))
	}

	if length == 0 {
		return
	}

	sourceBits := source.bits
	destBits := dest.bits

	// First, align `destFrom` with a word start, ie. a multiple of uint64 (64)
	if (destFrom & wordMask) != 0 {
		numBitsNeeded := min(bitsPerWord-(destFrom&wordMask), length)
		// Java's `<< destFrom` masks the shift count to 63 bits.
		bits := readNBits(sourceBits, sourceFrom, numBitsNeeded) << (destFrom & wordMask)
		destBits[destFrom>>log2BitsPerWord] |= bits

		sourceFrom += numBitsNeeded
		destFrom += numBitsNeeded
		length -= numBitsNeeded
	}

	if length == 0 {
		return
	}

	// Now OR at the word level
	numFullWords := length >> log2BitsPerWord
	sourceWordFrom := sourceFrom >> log2BitsPerWord
	destWordFrom := destFrom >> log2BitsPerWord

	// Note: these two for loops auto-vectorize
	if (sourceFrom & wordMask) == 0 {
		// sourceFrom and destFrom are both aligned with a uint64
		for i := 0; i < numFullWords; i++ {
			destBits[destWordFrom+i] |= sourceBits[sourceWordFrom+i]
		}
	} else {
		for i := 0; i < numFullWords; i++ {
			// Java's `<< -sourceFrom` masks the shift count to 63 bits.
			destBits[destWordFrom+i] |=
				(sourceBits[sourceWordFrom+i] >> (sourceFrom & wordMask)) |
					(sourceBits[sourceWordFrom+i+1] << (bitsPerWord - (sourceFrom & wordMask)))
		}
	}

	sourceFrom += numFullWords << log2BitsPerWord
	destFrom += numFullWords << log2BitsPerWord
	length -= numFullWords << log2BitsPerWord

	// Finally handle tail bits
	if length > 0 {
		bits := readNBits(sourceBits, sourceFrom, length)
		destBits[destFrom>>log2BitsPerWord] |= bits
	}
}
