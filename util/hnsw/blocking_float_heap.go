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

import "sync"

// BlockingFloatHeap is a mutex-protected bounded min-heap of float32
// values. The top element is always the lowest value of the heap.
// Port of org.apache.lucene.util.hnsw.BlockingFloatHeap (Lucene
// 10.4.0).
//
// Notes on the port:
//
//   - Despite the name "Blocking", the Java class performs no
//     waiting; it merely uses a ReentrantLock to make a sibling of
//     [FloatHeap] safe for concurrent use. The Go port mirrors that:
//     a single sync.Mutex protects all state. There is no condition
//     variable, no wait, no signal.
//
//   - The public surface mirrors Java one-for-one: Offer (single and
//     bulk), Poll, Peek, Size. Unlike [FloatHeap] there is no Clear
//     or GetHeap in the Java reference, so they are intentionally
//     omitted here.
//
//   - Offer returns the float32 value that ends up at the top of the
//     heap after the operation. This differs from the non-blocking
//     [FloatHeap.Offer] (which returns bool for retained/discarded);
//     the Java contract is what callers rely on for blocking-heap
//     coordination, so we preserve it verbatim.
//
//   - Offer's full-heap retention rule is "value >= heap[1]" (not
//     strict greater-than). When the new value equals the current
//     top, Java performs updateTop anyway — observable behaviour is
//     identical to keeping the existing top, but we preserve the
//     code path for fidelity.
//
//   - Java's poll() reads size outside the lock; that is a benign
//     data race on the JVM thanks to ReentrantLock's happens-before
//     edges, but in Go the race detector would flag it (and torn
//     reads on 32-bit ARM are possible). We move the emptiness
//     check inside the critical section.
//
//   - The bulk Offer(values, len) expects values sorted in ascending
//     order. It walks from the largest (index len-1) down, pushing
//     while the heap is not full and switching to top-replacement
//     once full. The early-break on "values[i] < heap[1]" relies on
//     the ascending-sort precondition: any earlier element is
//     smaller still and would also be rejected.
//
// The zero value is NOT usable; construct via [NewBlockingFloatHeap].
type BlockingFloatHeap struct {
	mu      sync.Mutex
	heap    []float32
	maxSize int
	size    int
}

// NewBlockingFloatHeap returns a BlockingFloatHeap with the given
// fixed capacity. maxSize must be > 0; a non-positive value panics.
// The backing array is allocated immediately so subsequent Offer
// calls do not allocate.
func NewBlockingFloatHeap(maxSize int) *BlockingFloatHeap {
	if maxSize <= 0 {
		panic("hnsw: BlockingFloatHeap maxSize must be > 0")
	}
	return &BlockingFloatHeap{
		maxSize: maxSize,
		heap:    make([]float32, maxSize+1),
	}
}

// Offer inserts value into the heap. If the heap would exceed
// maxSize the least value is discarded. Returns the new top (least)
// element. Safe for concurrent use.
func (h *BlockingFloatHeap) Offer(value float32) float32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.size < h.maxSize {
		h.push(value)
		return h.heap[1]
	}
	if value >= h.heap[1] {
		h.updateTop(value)
	}
	return h.heap[1]
}

// OfferMany inserts the first len values from the slice. The values
// MUST be sorted in ascending order; behaviour is undefined
// otherwise. Returns the new top (least) element. Safe for
// concurrent use.
//
// This is the Go spelling of Java's overloaded
// "offer(float[] values, int len)". A separate name avoids the
// ambiguity Go would otherwise have between the single- and
// bulk-value forms.
func (h *BlockingFloatHeap) OfferMany(values []float32, length int) float32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := length - 1; i >= 0; i-- {
		if h.size < h.maxSize {
			h.push(values[i])
			continue
		}
		if values[i] >= h.heap[1] {
			h.updateTop(values[i])
			continue
		}
		// values is sorted ascending: every earlier element is
		// smaller still, so none would be retained. Stop.
		break
	}
	return h.heap[1]
}

// Poll removes and returns the head of the heap (the smallest
// value). Panics with [ErrEmptyHeap] when the heap is empty. Safe
// for concurrent use.
func (h *BlockingFloatHeap) Poll() float32 {
	h.mu.Lock()
	if h.size <= 0 {
		h.mu.Unlock()
		panic(ErrEmptyHeap)
	}
	result := h.heap[1]
	h.heap[1] = h.heap[h.size]
	h.size--
	h.downHeap(1)
	h.mu.Unlock()
	return result
}

// Peek returns the head of the heap without removing it. Like
// [FloatHeap.Peek], Peek does NOT check for emptiness: on an empty
// heap it returns the value previously stored at slot 1 (zero on a
// fresh heap). Safe for concurrent use.
func (h *BlockingFloatHeap) Peek() float32 {
	h.mu.Lock()
	v := h.heap[1]
	h.mu.Unlock()
	return v
}

// Size returns the number of live elements in the heap. Safe for
// concurrent use.
func (h *BlockingFloatHeap) Size() int {
	h.mu.Lock()
	s := h.size
	h.mu.Unlock()
	return s
}

// push appends value to the end of the heap and sifts it up. Caller
// must hold the lock.
func (h *BlockingFloatHeap) push(value float32) {
	h.size++
	h.heap[h.size] = value
	h.upHeap(h.size)
}

// updateTop replaces the root with value and re-heapifies down.
// Caller must hold the lock.
func (h *BlockingFloatHeap) updateTop(value float32) float32 {
	h.heap[1] = value
	h.downHeap(1)
	return h.heap[1]
}

// downHeap sifts the value at index i toward the leaves. Caller
// must hold the lock.
func (h *BlockingFloatHeap) downHeap(i int) {
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

// upHeap sifts the value at index origPos toward the root. Caller
// must hold the lock.
func (h *BlockingFloatHeap) upHeap(origPos int) {
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
