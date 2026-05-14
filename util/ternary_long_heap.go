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
//	http://www.apache.org/licenses/LICENSE-2.0

package util

import "fmt"

// TernaryLongHeap is a 3-ary min-heap of int64 values. Top runs in O(1);
// Push and Pop run in O(log_3 n). Compared with the 2-ary [LongHeap], the
// shallower tree reduces the number of sift levels at the cost of more
// comparisons per level — the trade-off Lucene picked for KNN search.
//
// This is a port of org.apache.lucene.util.TernaryLongHeap (Lucene 10.4.0).
// The heap is 1-indexed: slot 0 is unused so that the parent/child arithmetic
// matches Lucene's UpHeap/DownHeap helpers exactly.
type TernaryLongHeap struct {
	initialCapacity int
	heap            []int64
	size            int
}

// ternaryArity is the number of children per node (3-ary heap).
const ternaryArity = 3

// NewTernaryLongHeap creates an empty 3-ary min-heap with the given initial
// capacity. The capacity must be in (0, MaxArrayLength).
func NewTernaryLongHeap(initialCapacity int) *TernaryLongHeap {
	if initialCapacity < 1 || initialCapacity >= MaxArrayLength {
		panic(fmt.Sprintf("initialCapacity must be > 0 and < %d; got: %d",
			MaxArrayLength-1, initialCapacity))
	}
	// NOTE: +1 because the heap is 1-indexed; heap[0] is unused.
	return &TernaryLongHeap{
		initialCapacity: initialCapacity,
		heap:            make([]int64, initialCapacity+1),
	}
}

// NewTernaryLongHeapFilled returns a heap of exactly size elements, all
// initialised to initialValue. Mirrors Lucene's TernaryLongHeap(int, long).
func NewTernaryLongHeapFilled(size int, initialValue int64) *TernaryLongHeap {
	capacity := size
	if capacity <= 0 {
		capacity = 1
	}
	h := NewTernaryLongHeap(capacity)
	for i := 1; i <= size; i++ {
		h.heap[i] = initialValue
	}
	h.size = size
	return h
}

// Push adds a value in O(log_3 n). The heap grows unbounded as needed; the
// new top is returned.
func (h *TernaryLongHeap) Push(element int64) int64 {
	h.size++
	if h.size == len(h.heap) {
		// Match Lucene's growth policy: (size * 3 + 1) / 2.
		h.heap = GrowExactInt64(h.heap, (h.size*3+1)/2)
	}
	h.heap[h.size] = element
	ternaryUpHeap(h.heap, h.size)
	return h.heap[1]
}

// InsertWithOverflow adds a value, discarding the current minimum if the
// heap is at initialCapacity. Returns true if the value was accepted into
// the heap (either as a new entry or by replacing the top), false when the
// heap is full and the candidate would not change the top.
func (h *TernaryLongHeap) InsertWithOverflow(value int64) bool {
	if h.size >= h.initialCapacity {
		if value < h.heap[1] {
			return false
		}
		h.UpdateTop(value)
		return true
	}
	h.Push(value)
	return true
}

// Top returns the least element in O(1). It is the caller's responsibility
// to verify that the heap is not empty; no checking is done, and if no
// elements have been added, 0 is returned (matching Lucene).
func (h *TernaryLongHeap) Top() int64 {
	return h.heap[1]
}

// Pop removes and returns the least element in O(log_3 n).
// Panics if the heap is empty (mirroring Lucene's IllegalStateException).
func (h *TernaryLongHeap) Pop() int64 {
	if h.size <= 0 {
		panic("The heap is empty")
	}
	result := h.heap[1]
	h.heap[1] = h.heap[h.size]
	h.size--
	ternaryDownHeap(h.heap, 1, h.size)
	return result
}

// UpdateTop replaces the top of the heap with newTop and sifts down.
// Faster than a Pop followed by a Push when the new value is known to be
// less than the current top.
func (h *TernaryLongHeap) UpdateTop(value int64) int64 {
	h.heap[1] = value
	ternaryDownHeap(h.heap, 1, h.size)
	return h.heap[1]
}

// Size returns the number of elements currently stored in the heap.
func (h *TernaryLongHeap) Size() int {
	return h.size
}

// Clear removes all entries. The backing array is preserved for reuse.
func (h *TernaryLongHeap) Clear() {
	h.size = 0
}

// PushAll inserts every element of other into this heap in O(n log_3 n).
func (h *TernaryLongHeap) PushAll(other *TernaryLongHeap) {
	for i := 1; i <= other.size; i++ {
		h.Push(other.heap[i])
	}
}

// Get returns the element stored at index i in the underlying array. Valid
// indices range from 1 to Size() inclusive; the order is the internal heap
// order, not sorted order. Useful for iteration when ordering is irrelevant.
func (h *TernaryLongHeap) Get(i int) int64 {
	return h.heap[i]
}

// HeapArray returns the internal 1-indexed heap array. Exposed for parity
// with Lucene's package-private getHeapArray() used by tests; consumers must
// not mutate the returned slice.
func (h *TernaryLongHeap) HeapArray() []int64 {
	return h.heap
}

// ternaryUpHeap moves the element at index i upward until the heap invariant
// is restored. 1-based indexing; works for any heap where the parent of node
// i is ((i-2)/arity)+1.
func ternaryUpHeap(heap []int64, i int) {
	value := heap[i]
	for i > 1 {
		parent := ((i - 2) / ternaryArity) + 1
		parentVal := heap[parent]
		if value >= parentVal {
			break
		}
		heap[i] = parentVal
		i = parent
	}
	heap[i] = value
}

// ternaryDownHeap moves the element at index i downward through its three
// children until the heap invariant is restored. 1-based indexing; the first
// child of node i is arity*(i-1)+2.
func ternaryDownHeap(heap []int64, i, size int) {
	value := heap[i]
	for {
		firstChild := ternaryArity*(i-1) + 2
		if firstChild > size {
			break
		}
		lastChild := firstChild + ternaryArity - 1
		if lastChild > size {
			lastChild = size
		}
		best := firstChild
		bestVal := heap[firstChild]
		for c := firstChild + 1; c <= lastChild; c++ {
			v := heap[c]
			if v < bestVal {
				bestVal = v
				best = c
			}
		}
		if bestVal >= value {
			break
		}
		heap[i] = bestVal
		i = best
	}
	heap[i] = value
}
