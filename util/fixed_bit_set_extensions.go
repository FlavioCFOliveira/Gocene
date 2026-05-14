// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import "fmt"

// This file closes API gaps in [FixedBitSet] relative to Lucene 10.4.0's
// org.apache.lucene.util.FixedBitSet public surface (flip, intersects,
// {intersection,union,andNot}Count, nextClearBit, range set/flip, raw
// word access, and OfBits constructor).

// Flip toggles the bit at the given index, mirroring
// {@code FixedBitSet#flip(int)}.
func (fs *FixedBitSet) Flip(index int) {
	if index < 0 || index >= fs.size {
		panic(fmt.Sprintf("index out of bounds: %d (size: %d)", index, fs.size))
	}
	wordIdx := wordIndex(index)
	bitIdx := bitIndex(index)
	fs.bits[wordIdx] ^= 1 << bitIdx
}

// FlipRange flips bits in the range [startIndex, endIndex), mirroring
// the two-arg overload of {@code FixedBitSet#flip}.
func (fs *FixedBitSet) FlipRange(startIndex, endIndex int) {
	if startIndex < 0 || startIndex > fs.size {
		panic(fmt.Sprintf("startIndex out of bounds: %d (size: %d)", startIndex, fs.size))
	}
	if endIndex < 0 || endIndex > fs.size {
		panic(fmt.Sprintf("endIndex out of bounds: %d (size: %d)", endIndex, fs.size))
	}
	if startIndex >= endIndex {
		return
	}

	startWord := wordIndex(startIndex)
	endWord := wordIndex(endIndex - 1)

	startMask := ^uint64(0) << bitIndex(startIndex)
	endMask := ^uint64(0) >> (63 - bitIndex(endIndex-1))

	if startWord == endWord {
		fs.bits[startWord] ^= startMask & endMask
		return
	}
	fs.bits[startWord] ^= startMask
	for i := startWord + 1; i < endWord; i++ {
		fs.bits[i] = ^fs.bits[i]
	}
	fs.bits[endWord] ^= endMask
}

// SetRange sets bits in the range [startIndex, endIndex), mirroring the
// two-arg overload of {@code FixedBitSet#set}.
func (fs *FixedBitSet) SetRange(startIndex, endIndex int) {
	if startIndex < 0 || startIndex > fs.size {
		panic(fmt.Sprintf("startIndex out of bounds: %d (size: %d)", startIndex, fs.size))
	}
	if endIndex < 0 || endIndex > fs.size {
		panic(fmt.Sprintf("endIndex out of bounds: %d (size: %d)", endIndex, fs.size))
	}
	if startIndex >= endIndex {
		return
	}

	startWord := wordIndex(startIndex)
	endWord := wordIndex(endIndex - 1)

	startMask := ^uint64(0) << bitIndex(startIndex)
	endMask := ^uint64(0) >> (63 - bitIndex(endIndex-1))

	if startWord == endWord {
		fs.bits[startWord] |= startMask & endMask
		return
	}
	fs.bits[startWord] |= startMask
	for i := startWord + 1; i < endWord; i++ {
		fs.bits[i] = ^uint64(0)
	}
	fs.bits[endWord] |= endMask
}

// Intersects reports whether this bitset has any bit in common with the
// other bitset, mirroring {@code FixedBitSet#intersects}.
func (fs *FixedBitSet) Intersects(other *FixedBitSet) bool {
	pos := len(fs.bits)
	if len(other.bits) < pos {
		pos = len(other.bits)
	}
	for i := 0; i < pos; i++ {
		if fs.bits[i]&other.bits[i] != 0 {
			return true
		}
	}
	return false
}

// NextClearBit returns the index of the next clear (unset) bit at or
// after the given index. Returns fs.Length() when no clear bit exists
// in [fromIndex, fs.Length()), mirroring {@code FixedBitSet#nextClearBit}.
func (fs *FixedBitSet) NextClearBit(fromIndex int) int {
	if fromIndex < 0 || fromIndex >= fs.size {
		panic(fmt.Sprintf("fromIndex out of bounds: %d (size: %d)", fromIndex, fs.size))
	}

	wordIdx := wordIndex(fromIndex)
	bitIdx := bitIndex(fromIndex)

	word := ^fs.bits[wordIdx] & (^uint64(0) << bitIdx)

	for {
		if word != 0 {
			result := wordIdx*bitsPerWord + trailingZeros(word)
			if result >= fs.size {
				return fs.size
			}
			return result
		}
		wordIdx++
		if wordIdx >= len(fs.bits) {
			return fs.size
		}
		word = ^fs.bits[wordIdx]
	}
}

// Bits returns the underlying uint64 word array. The returned slice is
// shared with the FixedBitSet; mutations through the slice are visible
// to subsequent operations. Mirrors {@code FixedBitSet#getBits}.
func (fs *FixedBitSet) Bits() []uint64 {
	return fs.bits
}

// NewFixedBitSetOfBits wraps an existing uint64 word array as a
// FixedBitSet of numBits bits. The slice must contain at least
// wordsNeeded(numBits) words. The slice is referenced — not copied — so
// the caller must not retain a reference unless aliasing is intentional.
// Mirrors the Java constructor FixedBitSet(long[], int).
func NewFixedBitSetOfBits(bits []uint64, numBits int) (*FixedBitSet, error) {
	if numBits < 0 {
		return nil, fmt.Errorf("numBits must be non-negative, got %d", numBits)
	}
	needed := wordsNeeded(numBits)
	if len(bits) < needed {
		return nil, fmt.Errorf("bits length %d is less than required %d for %d bits",
			len(bits), needed, numBits)
	}
	return &FixedBitSet{bits: bits[:needed], size: numBits}, nil
}

// IntersectionCount returns the number of bits that are set in both
// bitsets, mirroring {@code FixedBitSet#intersectionCount}.
func IntersectionCount(a, b *FixedBitSet) int {
	pos := len(a.bits)
	if len(b.bits) < pos {
		pos = len(b.bits)
	}
	count := 0
	for i := 0; i < pos; i++ {
		count += popcount(a.bits[i] & b.bits[i])
	}
	return count
}

// UnionCount returns the number of bits set in the union of the two
// bitsets, mirroring {@code FixedBitSet#unionCount}.
func UnionCount(a, b *FixedBitSet) int {
	common := len(a.bits)
	if len(b.bits) < common {
		common = len(b.bits)
	}
	count := 0
	for i := 0; i < common; i++ {
		count += popcount(a.bits[i] | b.bits[i])
	}
	for i := common; i < len(a.bits); i++ {
		count += popcount(a.bits[i])
	}
	for i := common; i < len(b.bits); i++ {
		count += popcount(b.bits[i])
	}
	return count
}

// AndNotCount returns the number of bits set in a that are not set in
// b, mirroring {@code FixedBitSet#andNotCount}.
func AndNotCount(a, b *FixedBitSet) int {
	common := len(a.bits)
	if len(b.bits) < common {
		common = len(b.bits)
	}
	count := 0
	for i := 0; i < common; i++ {
		count += popcount(a.bits[i] &^ b.bits[i])
	}
	for i := common; i < len(a.bits); i++ {
		count += popcount(a.bits[i])
	}
	return count
}
