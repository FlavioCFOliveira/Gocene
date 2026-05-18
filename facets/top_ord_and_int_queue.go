// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "container/heap"

// TopOrdAndIntQueue is the int32-typed twin of TopOrdAndFloatQueue. Mirrors
// org.apache.lucene.facet.TopOrdAndIntQueue.
type TopOrdAndIntQueue struct {
	cap  int
	heap topOrdIntHeap
}

// NewTopOrdAndIntQueue creates a queue holding up to capacity entries.
func NewTopOrdAndIntQueue(capacity int) *TopOrdAndIntQueue {
	return &TopOrdAndIntQueue{cap: capacity}
}

// Capacity returns the configured upper bound.
func (q *TopOrdAndIntQueue) Capacity() int { return q.cap }

// Size returns the number of entries currently held.
func (q *TopOrdAndIntQueue) Size() int { return q.heap.Len() }

// Clear empties the queue.
func (q *TopOrdAndIntQueue) Clear() { q.heap = q.heap[:0] }

// InsertInt adds (ord, value); when full, the smallest entry is evicted if
// the new value is larger. Returns whether the entry ended up inside.
func (q *TopOrdAndIntQueue) InsertInt(ord int, value int32) bool {
	if q.heap.Len() < q.cap {
		heap.Push(&q.heap, topOrdIntEntry{ord: ord, value: value})
		return true
	}
	if q.heap[0].value < value {
		q.heap[0] = topOrdIntEntry{ord: ord, value: value}
		heap.Fix(&q.heap, 0)
		return true
	}
	return false
}

// Insert satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndIntQueue) Insert(ord int, value float64) bool {
	return q.InsertInt(ord, int32(value))
}

// PopInt removes and returns the smallest entry.
func (q *TopOrdAndIntQueue) PopInt() (ord int, value int32, ok bool) {
	if q.heap.Len() == 0 {
		return 0, 0, false
	}
	e := heap.Pop(&q.heap).(topOrdIntEntry)
	return e.ord, e.value, true
}

// Pop satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndIntQueue) Pop() (ord int, value float64, ok bool) {
	o, v, k := q.PopInt()
	return o, float64(v), k
}

// TopInt returns the smallest entry without removing it.
func (q *TopOrdAndIntQueue) TopInt() (ord int, value int32, ok bool) {
	if q.heap.Len() == 0 {
		return 0, 0, false
	}
	return q.heap[0].ord, q.heap[0].value, true
}

// Top satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndIntQueue) Top() (ord int, value float64, ok bool) {
	o, v, k := q.TopInt()
	return o, float64(v), k
}

type topOrdIntEntry struct {
	ord   int
	value int32
}

type topOrdIntHeap []topOrdIntEntry

func (h topOrdIntHeap) Len() int           { return len(h) }
func (h topOrdIntHeap) Less(i, j int) bool { return h[i].value < h[j].value }
func (h topOrdIntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *topOrdIntHeap) Push(x any)        { *h = append(*h, x.(topOrdIntEntry)) }
func (h *topOrdIntHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

var _ TopOrdAndNumberQueue = (*TopOrdAndIntQueue)(nil)
