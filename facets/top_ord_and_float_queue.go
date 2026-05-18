// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "container/heap"

// TopOrdAndFloatQueue is a bounded priority queue holding (ord, float32)
// pairs with ascending order on the value (so popping yields the smallest
// remaining entry — keeping the top-N largest values requires the caller to
// inspect Top() before inserting). Mirrors
// org.apache.lucene.facet.TopOrdAndFloatQueue.
type TopOrdAndFloatQueue struct {
	cap  int
	heap topOrdFloatHeap
}

// NewTopOrdAndFloatQueue creates a queue holding up to capacity entries.
func NewTopOrdAndFloatQueue(capacity int) *TopOrdAndFloatQueue {
	return &TopOrdAndFloatQueue{cap: capacity}
}

// Capacity returns the configured upper bound.
func (q *TopOrdAndFloatQueue) Capacity() int { return q.cap }

// Size returns the number of entries currently held.
func (q *TopOrdAndFloatQueue) Size() int { return q.heap.Len() }

// Clear empties the queue.
func (q *TopOrdAndFloatQueue) Clear() { q.heap = q.heap[:0] }

// InsertFloat adds (ord, value); when the queue is full the smallest entry
// is evicted if value is larger. Returns whether (ord, value) ended up inside.
func (q *TopOrdAndFloatQueue) InsertFloat(ord int, value float32) bool {
	if q.heap.Len() < q.cap {
		heap.Push(&q.heap, topOrdFloatEntry{ord: ord, value: value})
		return true
	}
	if q.heap[0].value < value {
		q.heap[0] = topOrdFloatEntry{ord: ord, value: value}
		heap.Fix(&q.heap, 0)
		return true
	}
	return false
}

// Insert satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndFloatQueue) Insert(ord int, value float64) bool {
	return q.InsertFloat(ord, float32(value))
}

// PopFloat removes and returns the smallest entry.
func (q *TopOrdAndFloatQueue) PopFloat() (ord int, value float32, ok bool) {
	if q.heap.Len() == 0 {
		return 0, 0, false
	}
	e := heap.Pop(&q.heap).(topOrdFloatEntry)
	return e.ord, e.value, true
}

// Pop satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndFloatQueue) Pop() (ord int, value float64, ok bool) {
	o, v, k := q.PopFloat()
	return o, float64(v), k
}

// TopFloat returns the smallest entry without removing it.
func (q *TopOrdAndFloatQueue) TopFloat() (ord int, value float32, ok bool) {
	if q.heap.Len() == 0 {
		return 0, 0, false
	}
	return q.heap[0].ord, q.heap[0].value, true
}

// Top satisfies TopOrdAndNumberQueue.
func (q *TopOrdAndFloatQueue) Top() (ord int, value float64, ok bool) {
	o, v, k := q.TopFloat()
	return o, float64(v), k
}

type topOrdFloatEntry struct {
	ord   int
	value float32
}

type topOrdFloatHeap []topOrdFloatEntry

func (h topOrdFloatHeap) Len() int           { return len(h) }
func (h topOrdFloatHeap) Less(i, j int) bool { return h[i].value < h[j].value }
func (h topOrdFloatHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *topOrdFloatHeap) Push(x any)        { *h = append(*h, x.(topOrdFloatEntry)) }
func (h *topOrdFloatHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

var _ TopOrdAndNumberQueue = (*TopOrdAndFloatQueue)(nil)
