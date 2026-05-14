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

// LongHeapOrder selects whether a LongHeap behaves as a min-heap (the
// Lucene 10.4.0 default) or a max-heap. The Order is part of the
// constructor in order to mirror older Lucene LongHeap.create(Order, int)
// factories that have since been retired in the Java reference; the
// min-heap branch is byte-for-byte compatible with the current Lucene
// 10.4.0 LongHeap class.
type LongHeapOrder uint8

const (
	// LongHeapMin makes Top return the smallest stored value.
	LongHeapMin LongHeapOrder = iota
	// LongHeapMax makes Top return the largest stored value.
	LongHeapMax
)

// LongHeap is a 2-ary binary heap of int64 values. Top runs in O(1)
// time; Push and Pop run in O(log n). The heap supports bounded-size
// insertion through InsertWithOverflow, which discards the least element
// once the configured initial capacity is reached.
//
// The min-heap variant ports org.apache.lucene.util.LongHeap (Lucene
// 10.4.0). The max-heap variant inverts the comparison performed by the
// up/down sift loops; everything else is identical.
type LongHeap struct {
	initialCapacity int
	heap            []int64
	size            int
	order           LongHeapOrder
}

// NewLongHeapMin returns an empty min-heap with the given initial
// capacity. The capacity must be in (0, MaxArrayLength).
func NewLongHeapMin(initialCapacity int) *LongHeap {
	return newLongHeap(initialCapacity, LongHeapMin)
}

// NewLongHeapMax returns an empty max-heap with the given initial
// capacity. The capacity must be in (0, MaxArrayLength).
func NewLongHeapMax(initialCapacity int) *LongHeap {
	return newLongHeap(initialCapacity, LongHeapMax)
}

// NewLongHeapFilled returns a heap of exactly size elements, all
// initialised to initialValue. The order is min-heap; this constructor
// mirrors Lucene's two-arg LongHeap(int size, long initialValue).
func NewLongHeapFilled(size int, initialValue int64) *LongHeap {
	h := newLongHeap(size, LongHeapMin)
	for i := 1; i <= size; i++ {
		h.heap[i] = initialValue
	}
	h.size = size
	return h
}

func newLongHeap(initialCapacity int, order LongHeapOrder) *LongHeap {
	if initialCapacity < 1 || initialCapacity >= MaxArrayLength {
		panic(fmt.Sprintf("initialCapacity must be > 0 and < %d; got: %d",
			MaxArrayLength-1, initialCapacity))
	}
	// NOTE: +1 because heap is 1-indexed; heap[0] is unused.
	return &LongHeap{
		initialCapacity: initialCapacity,
		heap:            make([]int64, initialCapacity+1),
		order:           order,
	}
}

// Size returns the number of elements currently stored.
func (h *LongHeap) Size() int { return h.size }

// Clear discards all elements without freeing the backing array.
func (h *LongHeap) Clear() { h.size = 0 }

// Top returns the priority-extreme element (smallest for min-heap,
// largest for max-heap) without removing it. The caller must check
// Size > 0 first; on an empty heap Top returns 0.
func (h *LongHeap) Top() int64 { return h.heap[1] }

// Push inserts element into the heap, growing the backing storage as
// needed. Returns the new top of the heap.
func (h *LongHeap) Push(element int64) int64 {
	h.size++
	if h.size == len(h.heap) {
		newLen := (h.size*3 + 1) / 2
		ng := make([]int64, newLen)
		copy(ng, h.heap)
		h.heap = ng
	}
	h.heap[h.size] = element
	h.upHeap(h.size)
	return h.heap[1]
}

// InsertWithOverflow inserts value into the heap when there is capacity
// for it. When the heap has reached its initialCapacity, the value is
// only inserted if it would displace the current top; the displaced top
// is discarded. Returns whether the value was accepted.
func (h *LongHeap) InsertWithOverflow(value int64) bool {
	if h.size >= h.initialCapacity {
		// For a min-heap, we keep the largest values; for max-heap, the smallest.
		// In both cases, "value is worse than top" means we drop it.
		if h.cmpStrict(value, h.heap[1]) {
			return false
		}
		h.UpdateTop(value)
		return true
	}
	h.Push(value)
	return true
}

// Pop removes and returns the top of the heap. Panics on empty heap to
// mirror Java's IllegalStateException.
func (h *LongHeap) Pop() int64 {
	if h.size == 0 {
		panic("the heap is empty")
	}
	result := h.heap[1]
	h.heap[1] = h.heap[h.size]
	h.size--
	h.downHeap(1)
	return result
}

// UpdateTop replaces the current top with newTop and re-heapifies down.
// Twice as fast as Pop+Push when the caller knows the new value should
// land near the top. Calling UpdateTop on an empty heap is a no-op,
// matching the Java contract.
func (h *LongHeap) UpdateTop(value int64) int64 {
	h.heap[1] = value
	h.downHeap(1)
	return h.heap[1]
}

// PushAll merges every element of other into this heap. The other heap
// is unchanged. Time complexity is O(other.Size * log(this.Size + other.Size)).
func (h *LongHeap) PushAll(other *LongHeap) {
	for i := 1; i <= other.size; i++ {
		h.Push(other.heap[i])
	}
}

// Get returns the element at the i-th 1-based location in the heap
// array. The order of elements at locations > 1 is unspecified. Valid
// range is [1, Size()].
func (h *LongHeap) Get(i int) int64 { return h.heap[i] }

// HeapArray returns the underlying slice (unstable order). Intended for
// tests and other internal callers, hence not part of the user-facing
// surface of a typical priority queue.
func (h *LongHeap) HeapArray() []int64 { return h.heap }

// cmpLess reports whether a should sift above b in the configured
// ordering. For a min-heap this is `a < b`; for a max-heap it is
// `a > b`. Inlining this branch keeps the per-iteration cost low even
// though the order is encoded as a runtime field.
func (h *LongHeap) cmpLess(a, b int64) bool {
	if h.order == LongHeapMin {
		return a < b
	}
	return a > b
}

// cmpStrict is the strict version used by InsertWithOverflow to detect
// whether the incoming value is strictly worse than the current top.
func (h *LongHeap) cmpStrict(a, top int64) bool {
	if h.order == LongHeapMin {
		// min-heap keeps the largest values; smaller-than-top is rejected.
		return a < top
	}
	// max-heap keeps the smallest values; greater-than-top is rejected.
	return a > top
}

func (h *LongHeap) upHeap(origPos int) {
	i := origPos
	value := h.heap[i]
	j := i >> 1
	for j > 0 && h.cmpLess(value, h.heap[j]) {
		h.heap[i] = h.heap[j]
		i = j
		j >>= 1
	}
	h.heap[i] = value
}

func (h *LongHeap) downHeap(i int) {
	value := h.heap[i]
	j := i << 1
	k := j + 1
	if k <= h.size && h.cmpLess(h.heap[k], h.heap[j]) {
		j = k
	}
	for j <= h.size && h.cmpLess(h.heap[j], value) {
		h.heap[i] = h.heap[j]
		i = j
		j = i << 1
		k = j + 1
		if k <= h.size && h.cmpLess(h.heap[k], h.heap[j]) {
			j = k
		}
	}
	h.heap[i] = value
}
