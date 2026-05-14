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

import (
	"fmt"
	"sort"
)

// FixedLengthBytesRefArray is a variant of BytesRefArray where every
// entry has the exact same fixed length. Skipping the per-entry length
// prefix yields a smaller memory footprint and faster iteration than
// the general-purpose BytesRefArray.
//
// This is the Go port of org.apache.lucene.util.FixedLengthBytesRefArray.
// It is not safe for concurrent use.
//
// Storage layout: bytes are appended into a sequence of blocks. Each
// block holds maxValuesPerBlock values laid out contiguously, where
// maxValuesPerBlock = max(16, 1 << (24 - ceilLog2(valueLength))) so that
// an entire block fits comfortably below 16 MiB regardless of the value
// length. The blocks slice grows by one entry each time the current
// block fills.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/FixedLengthBytesRefArray.java
type FixedLengthBytesRefArray struct {
	valueLength       int
	maxValuesPerBlock int
	size              int
	blocks            [][]byte
}

// NewFixedLengthBytesRefArray creates an array whose entries all have
// the given fixed value length. valueLength must be strictly positive.
func NewFixedLengthBytesRefArray(valueLength int) (*FixedLengthBytesRefArray, error) {
	if valueLength <= 0 {
		return nil, fmt.Errorf("valueLength must be > 0, got %d", valueLength)
	}
	maxValues := computeMaxValuesPerBlock(valueLength)
	return &FixedLengthBytesRefArray{
		valueLength:       valueLength,
		maxValuesPerBlock: maxValues,
	}, nil
}

// computeMaxValuesPerBlock matches Lucene's heuristic:
// max(16, 1 << (24 - ceilLog2(valueLength))). The minimum of 16 ensures
// progress even for very large value lengths; the cap keeps each block
// at roughly 16 MiB.
func computeMaxValuesPerBlock(valueLength int) int {
	ceilLog2 := 0
	v := valueLength - 1
	for v > 0 {
		ceilLog2++
		v >>= 1
	}
	shift := 24 - ceilLog2
	max := 1
	if shift > 0 {
		max = 1 << shift
	}
	if max < 16 {
		max = 16
	}
	return max
}

// Append appends a copy of the given BytesRef to this array, returning
// the index of the new entry. The reference must have length equal to
// valueLength.
func (f *FixedLengthBytesRefArray) Append(b *BytesRef) (int, error) {
	if b == nil {
		return -1, fmt.Errorf("bytes ref is nil")
	}
	if b.Length != f.valueLength {
		return -1, fmt.Errorf("bytes ref length mismatch: expected %d, got %d",
			f.valueLength, b.Length)
	}

	indexInBlock := f.size % f.maxValuesPerBlock
	if indexInBlock == 0 {
		block := make([]byte, f.maxValuesPerBlock*f.valueLength)
		f.blocks = append(f.blocks, block)
	}
	dest := f.blocks[len(f.blocks)-1][indexInBlock*f.valueLength:]
	copy(dest, b.ValidBytes())
	id := f.size
	f.size++
	return id, nil
}

// Size returns the number of entries currently stored.
func (f *FixedLengthBytesRefArray) Size() int {
	return f.size
}

// Clear drops all stored entries and releases the block memory.
func (f *FixedLengthBytesRefArray) Clear() {
	f.size = 0
	f.blocks = nil
}

// ValueLength returns the per-entry length set at construction.
func (f *FixedLengthBytesRefArray) ValueLength() int {
	return f.valueLength
}

// Iterator returns a BytesRefIterator over the stored entries. When
// comparator is non-nil, entries are iterated in the order defined by
// the comparator; otherwise they are iterated in insertion order.
//
// The yielded BytesRef shares storage with the array's internal blocks;
// callers that need to outlive the next iterator call must copy the
// bytes.
func (f *FixedLengthBytesRefArray) Iterator(comparator BytesRefComparator) BytesRefIterator {
	if f.size == 0 {
		return EmptyBytesRefIterator
	}
	if comparator == nil {
		return &fixedLengthIter{arr: f}
	}
	// Sort an index permutation in-place and return a permuted iterator.
	order := make([]int, f.size)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		bi := f.bytesAt(order[i])
		bj := f.bytesAt(order[j])
		return comparator.Compare(bi, bj) < 0
	})
	return &fixedLengthSortedIter{arr: f, order: order}
}

// bytesAt returns a BytesRef view into the storage at logical index i.
// The returned BytesRef references the underlying block bytes directly
// and must not outlive subsequent mutations.
func (f *FixedLengthBytesRefArray) bytesAt(i int) *BytesRef {
	blockIdx := i / f.maxValuesPerBlock
	indexInBlock := i % f.maxValuesPerBlock
	offset := indexInBlock * f.valueLength
	return &BytesRef{
		Bytes:  f.blocks[blockIdx][offset : offset+f.valueLength],
		Offset: 0,
		Length: f.valueLength,
	}
}

// RamBytesUsed returns an estimate of the RAM footprint.
func (f *FixedLengthBytesRefArray) RamBytesUsed() int64 {
	if len(f.blocks) == 0 {
		return 0
	}
	per := int64(f.maxValuesPerBlock) * int64(f.valueLength)
	return int64(len(f.blocks)) * per
}

// fixedLengthIter walks the array in insertion order.
type fixedLengthIter struct {
	arr *FixedLengthBytesRefArray
	pos int
}

func (it *fixedLengthIter) Next() (*BytesRef, error) {
	if it.pos >= it.arr.size {
		return nil, nil
	}
	br := it.arr.bytesAt(it.pos)
	it.pos++
	return br, nil
}

// fixedLengthSortedIter walks the array in comparator order via a
// pre-computed permutation.
type fixedLengthSortedIter struct {
	arr   *FixedLengthBytesRefArray
	order []int
	pos   int
}

func (it *fixedLengthSortedIter) Next() (*BytesRef, error) {
	if it.pos >= len(it.order) {
		return nil, nil
	}
	br := it.arr.bytesAt(it.order[it.pos])
	it.pos++
	return br, nil
}

