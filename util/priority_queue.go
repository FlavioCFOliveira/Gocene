// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// PriorityQueue is a generic priority queue implementation based on a binary heap.
// Elements are ordered according to their natural ordering (Less method) or
// a custom comparator.
//
// This is the Go port of Lucene's org.apache.lucene.util.PriorityQueue.
type PriorityQueue[T any] struct {
	heap     []T
	size     int
	maxSize  int
	lessFunc func(a, b T) bool
}

// NewPriorityQueue creates a new PriorityQueue with the given maximum size.
// The less function defines the ordering: if lessFunc(a, b) is true, then a
// has higher priority than b (will be popped first).
func NewPriorityQueue[T any](maxSize int, lessFunc func(a, b T) bool) (*PriorityQueue[T], error) {
	if maxSize < 0 {
		return nil, fmt.Errorf("maxSize must be non-negative, got %d", maxSize)
	}
	if lessFunc == nil {
		return nil, fmt.Errorf("lessFunc cannot be nil")
	}

	// Add 1 to account for 1-based indexing in the heap
	heapSize := maxSize + 1
	if maxSize == 0 {
		heapSize = 1
	}

	return &PriorityQueue[T]{
		heap:     make([]T, 0, heapSize),
		size:     0,
		maxSize:  maxSize,
		lessFunc: lessFunc,
	}, nil
}

// Add adds an element to the queue.
// Returns true if the element was added, false if the queue is full
// and the element has lower priority than the current top.
func (pq *PriorityQueue[T]) Add(element T) bool {
	if pq.size < pq.maxSize {
		// Room to add element
		pq.heap = append(pq.heap, element)
		pq.size++
		pq.upHeap(pq.size - 1)
		return true
	}

	// Queue is full, check if element is better than top
	if pq.lessFunc(element, pq.Top()) {
		// Element has higher priority, replace top
		pq.heap[0] = element
		pq.downHeap(0)
		return true
	}

	return false
}

// Pop removes and returns the highest priority element.
// Returns zero value if the queue is empty.
func (pq *PriorityQueue[T]) Pop() T {
	if pq.size == 0 {
		var zero T
		return zero
	}

	result := pq.heap[0]
	pq.size--

	if pq.size > 0 {
		// Move last element to root and heapify down
		pq.heap[0] = pq.heap[pq.size]
		pq.heap = pq.heap[:pq.size]
		pq.downHeap(0)
	} else {
		pq.heap = pq.heap[:0]
	}

	return result
}

// Top returns the highest priority element without removing it.
// Returns zero value if the queue is empty.
func (pq *PriorityQueue[T]) Top() T {
	if pq.size == 0 {
		var zero T
		return zero
	}
	return pq.heap[0]
}

// UpdateTop updates the top element and re-heapifies.
// This is useful when the top element's priority changes.
func (pq *PriorityQueue[T]) UpdateTop() {
	if pq.size > 0 {
		pq.downHeap(0)
	}
}

// Size returns the current number of elements in the queue.
func (pq *PriorityQueue[T]) Size() int {
	return pq.size
}

// MaxSize returns the maximum size of the queue.
func (pq *PriorityQueue[T]) MaxSize() int {
	return pq.maxSize
}

// IsEmpty returns true if the queue is empty.
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.size == 0
}

// Clear removes all elements from the queue.
func (pq *PriorityQueue[T]) Clear() {
	pq.heap = pq.heap[:0]
	pq.size = 0
}

// Get returns the element at the given index.
// Index 0 is the top element.
func (pq *PriorityQueue[T]) Get(index int) (T, error) {
	if index < 0 || index >= pq.size {
		var zero T
		return zero, fmt.Errorf("index out of bounds: %d (size: %d)", index, pq.size)
	}
	return pq.heap[index], nil
}

// Set updates the element at the given index.
func (pq *PriorityQueue[T]) Set(index int, element T) error {
	if index < 0 || index >= pq.size {
		return fmt.Errorf("index out of bounds: %d (size: %d)", index, pq.size)
	}
	pq.heap[index] = element
	return nil
}

// upHeap moves the element at the given index up to its correct position.
func (pq *PriorityQueue[T]) upHeap(index int) {
	if index == 0 {
		return
	}

	parent := (index - 1) / 2
	if pq.lessFunc(pq.heap[index], pq.heap[parent]) {
		pq.heap[index], pq.heap[parent] = pq.heap[parent], pq.heap[index]
		pq.upHeap(parent)
	}
}

// downHeap moves the element at the given index down to its correct position.
func (pq *PriorityQueue[T]) downHeap(index int) {
	leftChild := 2*index + 1
	rightChild := 2*index + 2
	smallest := index

	if leftChild < pq.size && pq.lessFunc(pq.heap[leftChild], pq.heap[smallest]) {
		smallest = leftChild
	}
	if rightChild < pq.size && pq.lessFunc(pq.heap[rightChild], pq.heap[smallest]) {
		smallest = rightChild
	}

	if smallest != index {
		pq.heap[index], pq.heap[smallest] = pq.heap[smallest], pq.heap[index]
		pq.downHeap(smallest)
	}
}

// ToSlice returns a copy of the heap as a slice.
// The order is the heap order (not sorted).
func (pq *PriorityQueue[T]) ToSlice() []T {
	result := make([]T, pq.size)
	copy(result, pq.heap[:pq.size])
	return result
}

// AddAll bulk-adds elements using the O(n) Floyd build-heap, which is
// faster than O(n log n) individual Adds when all elements are known
// up-front. Returns an error when the addition would exceed maxSize,
// matching the Java ArrayIndexOutOfBoundsException semantics.
func (pq *PriorityQueue[T]) AddAll(elements []T) error {
	if pq.size+len(elements) > pq.maxSize {
		return fmt.Errorf("cannot add %d elements to a queue with remaining capacity %d",
			len(elements), pq.maxSize-pq.size)
	}
	for _, e := range elements {
		pq.heap = append(pq.heap, e)
		pq.size++
	}
	// Floyd build-heap: sift down every non-leaf, starting from the
	// last parent.
	for i := pq.size/2 - 1; i >= 0; i-- {
		pq.downHeap(i)
	}
	return nil
}

// InsertWithOverflow inserts element and returns:
//   - (zero, false) when the queue had room and the element was added;
//   - (overflowed, true) when the queue was full and element replaced
//     a strictly-smaller top, with overflowed being the displaced
//     value;
//   - (element, true) when the queue was full and element was rejected
//     (its priority did not beat the top).
//
// Mirrors the Java insertWithOverflow contract.
func (pq *PriorityQueue[T]) InsertWithOverflow(element T) (T, bool) {
	var zero T
	if pq.size < pq.maxSize {
		pq.Add(element)
		return zero, false
	}
	if pq.size > 0 && pq.lessFunc(pq.heap[0], element) {
		ret := pq.heap[0]
		pq.heap[0] = element
		pq.downHeap(0)
		return ret, true
	}
	return element, true
}

// UpdateTopWith replaces the top with newTop and re-heapifies,
// matching Java's updateTop(T newTop). The previous top is discarded.
func (pq *PriorityQueue[T]) UpdateTopWith(newTop T) {
	if pq.size == 0 {
		return
	}
	pq.heap[0] = newTop
	pq.downHeap(0)
}

// Remove removes the first element for which eq returns true. Returns
// true on success. Cost is O(n). Mirrors Java's remove(T element)
// where Java relies on == (object identity); the Go port accepts a
// caller-provided equality predicate so users can choose identity- or
// value-based semantics.
func (pq *PriorityQueue[T]) Remove(eq func(T) bool) bool {
	if eq == nil {
		return false
	}
	for i := 0; i < pq.size; i++ {
		if eq(pq.heap[i]) {
			// Move the last element into slot i, shrink, then restore
			// the heap invariant. Java only sifts up *or* down; we
			// always run both because the displaced element may be
			// out-of-order in either direction.
			pq.heap[i] = pq.heap[pq.size-1]
			pq.heap = pq.heap[:pq.size-1]
			pq.size--
			if i < pq.size {
				pq.upHeap(i)
				pq.downHeap(i)
			}
			return true
		}
	}
	return false
}

// All returns an iterator over the queue contents in heap order
// (NOT sorted). Mirrors Java's iterator() method using Go's
// range-over-func iterator pattern. Suitable for read-only traversal;
// callers must not mutate the queue during iteration.
func (pq *PriorityQueue[T]) All(yield func(T) bool) {
	for i := 0; i < pq.size; i++ {
		if !yield(pq.heap[i]) {
			return
		}
	}
}

// IntPriorityQueue is a convenience type for int priority queues.
type IntPriorityQueue = PriorityQueue[int]

// NewIntMinPriorityQueue creates a min-heap for ints.
func NewIntMinPriorityQueue(maxSize int) (*IntPriorityQueue, error) {
	return NewPriorityQueue(maxSize, func(a, b int) bool {
		return a < b
	})
}

// NewIntMaxPriorityQueue creates a max-heap for ints.
func NewIntMaxPriorityQueue(maxSize int) (*IntPriorityQueue, error) {
	return NewPriorityQueue(maxSize, func(a, b int) bool {
		return a > b
	})
}

// Float64PriorityQueue is a convenience type for float64 priority queues.
type Float64PriorityQueue = PriorityQueue[float64]

// NewFloat64MinPriorityQueue creates a min-heap for float64s.
func NewFloat64MinPriorityQueue(maxSize int) (*Float64PriorityQueue, error) {
	return NewPriorityQueue(maxSize, func(a, b float64) bool {
		return a < b
	})
}

// StringPriorityQueue is a convenience type for string priority queues.
type StringPriorityQueue = PriorityQueue[string]

// NewStringPriorityQueue creates a priority queue for strings (lexicographic order).
func NewStringPriorityQueue(maxSize int) (*StringPriorityQueue, error) {
	return NewPriorityQueue(maxSize, func(a, b string) bool {
		return a < b
	})
}
