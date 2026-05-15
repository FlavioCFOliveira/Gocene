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

package hnsw

// FloatHeap is a bounded min-heap of float32 values. The top element
// is always the lowest value of the heap. Port of
// org.apache.lucene.util.hnsw.FloatHeap (Lucene 10.4.0).
//
// Implementation notes (preserved from the Java reference for
// byte-for-byte semantic compatibility):
//
//   - Storage is 1-indexed: slot 0 is unused, slots 1..size hold
//     live values. The backing slice has length maxSize+1 and is
//     allocated once at construction.
//   - Comparisons use float32 strict less-than. NaN handling
//     therefore mirrors Java's IEEE-754 semantics: NaN < x is false
//     for every x, so a NaN that enters the heap is treated as
//     "not less than" anything and behaves like +Inf when sifted.
//   - Clear() only resets size; previously occupied slots are NOT
//     zeroed, mirroring Java which simply rewinds the cursor. The
//     test peer (TestFloatHeap.testClear) relies on this: peek()
//     after clear returns the prior top value.
//   - Poll() on an empty heap panics with the same wording Java
//     would surface via IllegalStateException; the package-level
//     ErrEmptyHeap value is exported so callers can use
//     recover/errors.Is-style sentinel matching if desired.
//
// FloatHeap is not safe for concurrent use.
type FloatHeap struct {
	maxSize int
	heap    []float32
	size    int
}

// NewFloatHeap returns a FloatHeap with the given fixed capacity.
// maxSize must be > 0; a non-positive value panics. The backing
// array is allocated immediately so subsequent Offer calls do not
// allocate.
func NewFloatHeap(maxSize int) *FloatHeap {
	if maxSize <= 0 {
		panic("hnsw: FloatHeap maxSize must be > 0")
	}
	return &FloatHeap{
		maxSize: maxSize,
		heap:    make([]float32, maxSize+1),
	}
}

// Offer inserts value into the heap. If the heap is full, the new
// value replaces the current minimum iff value > min; otherwise it
// is discarded. Returns true when the value was retained, false
// when discarded.
func (h *FloatHeap) Offer(value float32) bool {
	if h.size >= h.maxSize {
		if value < h.heap[1] {
			return false
		}
		h.updateTop(value)
		return true
	}
	h.push(value)
	return true
}

// GetHeap returns a freshly allocated copy of the live portion of
// the heap (length == Size()). The returned values are in internal
// heap order, NOT sorted; callers that need a sorted slice must
// drain via Poll.
func (h *FloatHeap) GetHeap() []float32 {
	out := make([]float32, h.size)
	copy(out, h.heap[1:1+h.size])
	return out
}

// Poll removes and returns the head of the heap (the smallest
// value). Panics with ErrEmptyHeap when the heap is empty.
func (h *FloatHeap) Poll() float32 {
	if h.size <= 0 {
		panic(ErrEmptyHeap)
	}
	result := h.heap[1]
	h.heap[1] = h.heap[h.size]
	h.size--
	h.downHeap(1)
	return result
}

// Peek returns the head of the heap without removing it. Unlike
// Poll, Peek does NOT check for emptiness: on an empty heap it
// returns the value previously stored at slot 1 (zero on a fresh
// heap, the prior top after Clear). This mirrors Java's
// FloatHeap.peek() and is relied on by TestFloatHeap.testClear.
func (h *FloatHeap) Peek() float32 { return h.heap[1] }

// Size returns the number of live elements in the heap.
func (h *FloatHeap) Size() int { return h.size }

// Clear empties the heap by rewinding the size cursor. Underlying
// slots are intentionally NOT zeroed, matching the Java reference.
func (h *FloatHeap) Clear() { h.size = 0 }

// push appends value to the end of the heap and sifts it up.
func (h *FloatHeap) push(value float32) {
	h.size++
	h.heap[h.size] = value
	h.upHeap(h.size)
}

// updateTop replaces the root with value and re-heapifies down.
func (h *FloatHeap) updateTop(value float32) float32 {
	h.heap[1] = value
	h.downHeap(1)
	return h.heap[1]
}

// downHeap sifts the value at index i toward the leaves.
func (h *FloatHeap) downHeap(i int) {
	value := h.heap[i]
	j := i << 1
	k := j + 1
	if k <= h.size && h.heap[k] < h.heap[j] {
		j = k
	}
	for j <= h.size && h.heap[j] < value {
		h.heap[i] = h.heap[j]
		i = j
		j = i << 1
		k = j + 1
		if k <= h.size && h.heap[k] < h.heap[j] {
			j = k
		}
	}
	h.heap[i] = value
}

// upHeap sifts the value at index origPos toward the root.
func (h *FloatHeap) upHeap(origPos int) {
	i := origPos
	value := h.heap[i]
	j := i >> 1
	for j > 0 && value < h.heap[j] {
		h.heap[i] = h.heap[j]
		i = j
		j = j >> 1
	}
	h.heap[i] = value
}

// ErrEmptyHeap is the value passed to panic when Poll is called on
// an empty heap. Callers that want to recover can compare via
// errors.Is or a direct equality check against this sentinel.
var ErrEmptyHeap = emptyHeapError("hnsw: FloatHeap is empty")

type emptyHeapError string

func (e emptyHeapError) Error() string { return string(e) }
