// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"sort"
)

// BytesRefArray is an append-only random-access BytesRef array that stores
// full copies of the appended bytes. This is the Go port of Lucene's
// org.apache.lucene.util.BytesRefArray.
//
// Note: This class is not thread-safe.
type BytesRefArray struct {
	// blocks stores the bytes in chunks to avoid frequent reallocation
	blocks [][]byte

	// currentBlock is the block currently being filled
	currentBlock []byte

	// blockSize is the size of each block
	blockSize int

	// offsets stores the offset and length information for each element
	// Each entry is a pair: [offset, length]
	offsets [][2]int

	// currentOffset is the next free position in the current block
	currentOffset int

	// size is the number of elements in the array
	size int

	// bytesUsed tracks the total bytes used (for memory accounting)
	bytesUsed int64
}

// NewBytesRefArray creates a new BytesRefArray with the given block size.
// If blockSize is 0, a default of 1024 is used.
func NewBytesRefArray(blockSize int) *BytesRefArray {
	if blockSize <= 0 {
		blockSize = 1024
	}
	return &BytesRefArray{
		blockSize: blockSize,
		offsets:   make([][2]int, 0, 16),
	}
}

// Append appends a copy of the given BytesRef to this array.
// Returns the index of the appended element.
func (bra *BytesRefArray) Append(bytes *BytesRef) int {
	if bytes == nil || bytes.Length == 0 {
		// Store empty entry
		bra.offsets = append(bra.offsets, [2]int{-1, 0})
		idx := bra.size
		bra.size++
		return idx
	}

	data := bytes.ValidBytes()
	dataLen := len(data)

	// Check if we need a new block
	if bra.currentBlock == nil || bra.currentOffset+dataLen > len(bra.currentBlock) {
		// Allocate new block
		newBlockSize := bra.blockSize
		if dataLen > newBlockSize {
			newBlockSize = dataLen
		}
		bra.currentBlock = make([]byte, newBlockSize)
		bra.blocks = append(bra.blocks, bra.currentBlock)
		bra.currentOffset = 0
	}

	// Copy data to current block
	copy(bra.currentBlock[bra.currentOffset:], data)

	// Record offset and length
	blockIndex := len(bra.blocks) - 1
	globalOffset := blockIndex<<32 | bra.currentOffset
	bra.offsets = append(bra.offsets, [2]int{globalOffset, dataLen})

	// Update state
	bra.currentOffset += dataLen
	bra.size++
	bra.bytesUsed += int64(dataLen) + 8 // data + overhead for offset array

	return bra.size - 1
}

// AppendBytes appends a copy of the given byte slice.
// Returns the index of the appended element.
func (bra *BytesRefArray) AppendBytes(data []byte) int {
	if len(data) == 0 {
		return bra.Append(nil)
	}
	return bra.Append(&BytesRef{Bytes: data, Offset: 0, Length: len(data)})
}

// Get retrieves the n'th element and stores it in the provided BytesRef.
// Returns true if successful, false if index is out of bounds.
func (bra *BytesRefArray) Get(index int, spare *BytesRef) bool {
	if index < 0 || index >= bra.size {
		return false
	}

	offset := bra.offsets[index]
	if offset[0] == -1 {
		// Empty entry
		spare.Bytes = nil
		spare.Offset = 0
		spare.Length = 0
		return true
	}

	blockIndex := offset[0] >> 32
	blockOffset := offset[0] & 0xFFFFFFFF
	length := offset[1]

	// Ensure spare has enough capacity
	if spare.Bytes == nil || cap(spare.Bytes) < length {
		spare.Bytes = make([]byte, length)
	} else {
		spare.Bytes = spare.Bytes[:length]
	}

	copy(spare.Bytes, bra.blocks[blockIndex][blockOffset:blockOffset+length])
	spare.Offset = 0
	spare.Length = length

	return true
}

// GetBytes retrieves the n'th element as a byte slice.
// Returns nil if index is out of bounds.
func (bra *BytesRefArray) GetBytes(index int) []byte {
	if index < 0 || index >= bra.size {
		return nil
	}

	offset := bra.offsets[index]
	if offset[0] == -1 {
		return nil
	}

	blockIndex := offset[0] >> 32
	blockOffset := offset[0] & 0xFFFFFFFF
	length := offset[1]

	result := make([]byte, length)
	copy(result, bra.blocks[blockIndex][blockOffset:blockOffset+length])
	return result
}

// Size returns the number of elements in this array.
func (bra *BytesRefArray) Size() int {
	return bra.size
}

// Clear clears the array, removing all elements.
func (bra *BytesRefArray) Clear() {
	bra.blocks = nil
	bra.currentBlock = nil
	bra.offsets = bra.offsets[:0]
	bra.currentOffset = 0
	bra.size = 0
	bra.bytesUsed = 0
}

// BytesUsed returns the approximate memory usage in bytes.
func (bra *BytesRefArray) BytesUsed() int64 {
	return bra.bytesUsed + int64(len(bra.blocks)*bra.blockSize)
}

// Iterator returns an iterator over all elements in insertion order.
func (bra *BytesRefArray) Iterator() *BytesRefArrayIterator {
	return &BytesRefArrayIterator{
		array: bra,
		index: 0,
	}
}

// BytesRefArrayIterator is an iterator over BytesRefArray elements.
type BytesRefArrayIterator struct {
	array *BytesRefArray
	index int
	spare BytesRef
}

// Next returns the next element and true, or nil and false if done.
func (it *BytesRefArrayIterator) Next() (*BytesRef, bool) {
	if it.index >= it.array.size {
		return nil, false
	}
	it.array.Get(it.index, &it.spare)
	it.index++
	// Return a copy to avoid modification of iterator's internal state
	result := it.spare.Clone()
	return result, true
}

// HasNext returns true if there are more elements.
func (it *BytesRefArrayIterator) HasNext() bool {
	return it.index < it.array.size
}

// Reset resets the iterator to the beginning.
func (it *BytesRefArrayIterator) Reset() {
	it.index = 0
}

// Sort sorts the elements and returns a SortState for ordered iteration.
// The comparator should return -1, 0, or 1 for less, equal, greater.
func (bra *BytesRefArray) Sort(less func(a, b *BytesRef) bool) *BytesRefArraySortState {
	if bra.size == 0 {
		return &BytesRefArraySortState{indices: []int{}}
	}

	// Create indices array
	indices := make([]int, bra.size)
	for i := range indices {
		indices[i] = i
	}

	// Sort indices based on comparator
	var spareA, spareB BytesRef
	sort.Slice(indices, func(i, j int) bool {
		bra.Get(indices[i], &spareA)
		bra.Get(indices[j], &spareB)
		return less(&spareA, &spareB)
	})

	return &BytesRefArraySortState{
		array:   bra,
		indices: indices,
	}
}

// SortByBytes sorts the elements lexicographically by their byte content.
func (bra *BytesRefArray) SortByBytes() *BytesRefArraySortState {
	return bra.Sort(func(a, b *BytesRef) bool {
		return bytes.Compare(a.ValidBytes(), b.ValidBytes()) < 0
	})
}

// BytesRefArraySortState provides ordered iteration over sorted elements.
type BytesRefArraySortState struct {
	array   *BytesRefArray
	indices []int
	pos     int
}

// Next returns the next element in sorted order.
func (ss *BytesRefArraySortState) Next(spare *BytesRef) bool {
	if ss.pos >= len(ss.indices) {
		return false
	}
	ss.array.Get(ss.indices[ss.pos], spare)
	ss.pos++
	return true
}

// Reset resets the sort state to the beginning.
func (ss *BytesRefArraySortState) Reset() {
	ss.pos = 0
}

// Size returns the number of elements in the sort state.
func (ss *BytesRefArraySortState) Size() int {
	return len(ss.indices)
}
