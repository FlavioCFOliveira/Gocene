// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanDisiPriorityQueue.java

package spans

// SpanDisiPriorityQueue is a priority queue of DocIdSetIterators that orders
// by current doc ID. This specialization avoids the overhead of a pluggable
// comparison function found in a generic PriorityQueue.
//
// Mirrors org.apache.lucene.queries.spans.SpanDisiPriorityQueue (Lucene 10.4.0).
type SpanDisiPriorityQueue struct {
	heap []*SpanDisiWrapper
	size int
}

// NewSpanDisiPriorityQueue creates a new queue with the given maximum size.
func NewSpanDisiPriorityQueue(maxSize int) *SpanDisiPriorityQueue {
	return &SpanDisiPriorityQueue{
		heap: make([]*SpanDisiWrapper, maxSize),
		size: 0,
	}
}

// Size returns the number of elements in the queue.
func (pq *SpanDisiPriorityQueue) Size() int { return pq.size }

// Top returns the element with the lowest doc ID.
func (pq *SpanDisiPriorityQueue) Top() *SpanDisiWrapper { return pq.heap[0] }

// TopList returns a linked list of wrappers sharing the current minimum doc ID.
func (pq *SpanDisiPriorityQueue) TopList() *SpanDisiWrapper {
	heap := pq.heap
	size := pq.size
	list := heap[0]
	list.Next = nil
	if size >= 3 {
		list = pq.topList(list, heap, size, 1)
		list = pq.topList(list, heap, size, 2)
	} else if size == 2 && heap[1].Doc == list.Doc {
		list = pq.prepend(heap[1], list)
	}
	return list
}

func (pq *SpanDisiPriorityQueue) prepend(w1, w2 *SpanDisiWrapper) *SpanDisiWrapper {
	w1.Next = w2
	return w1
}

func (pq *SpanDisiPriorityQueue) topList(list *SpanDisiWrapper, heap []*SpanDisiWrapper, size, i int) *SpanDisiWrapper {
	w := heap[i]
	if w.Doc == list.Doc {
		list = pq.prepend(w, list)
		left := leftNode(i)
		right := left + 1
		if right < size {
			list = pq.topList(list, heap, size, left)
			list = pq.topList(list, heap, size, right)
		} else if left < size && heap[left].Doc == list.Doc {
			list = pq.prepend(heap[left], list)
		}
	}
	return list
}

// Add adds an entry to the queue and returns the new top.
func (pq *SpanDisiPriorityQueue) Add(entry *SpanDisiWrapper) *SpanDisiWrapper {
	heap := pq.heap
	size := pq.size
	heap[size] = entry
	pq.upHeap(size)
	pq.size = size + 1
	return heap[0]
}

// Pop removes and returns the top element.
func (pq *SpanDisiPriorityQueue) Pop() *SpanDisiWrapper {
	heap := pq.heap
	result := heap[0]
	i := pq.size - 1
	pq.size = i
	heap[0] = heap[i]
	heap[i] = nil
	pq.downHeap(i)
	return result
}

// UpdateTop re-heapifies after the top element has been mutated and returns the new top.
func (pq *SpanDisiPriorityQueue) UpdateTop() *SpanDisiWrapper {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

// UpdateTopWithReplacement replaces the top element and returns the new top.
func (pq *SpanDisiPriorityQueue) UpdateTopWithReplacement(topReplacement *SpanDisiWrapper) *SpanDisiWrapper {
	pq.heap[0] = topReplacement
	return pq.UpdateTop()
}

func (pq *SpanDisiPriorityQueue) upHeap(i int) {
	heap := pq.heap
	node := heap[i]
	nodeDoc := node.Doc
	j := parentNode(i)
	for j >= 0 && nodeDoc < heap[j].Doc {
		heap[i] = heap[j]
		i = j
		j = parentNode(j)
	}
	heap[i] = node
}

func (pq *SpanDisiPriorityQueue) downHeap(size int) {
	heap := pq.heap
	i := 0
	node := heap[0]
	j := leftNode(i)
	if j < size {
		k := rightNode(j)
		if k < size && heap[k].Doc < heap[j].Doc {
			j = k
		}
		if heap[j].Doc < node.Doc {
			for {
				heap[i] = heap[j]
				i = j
				j = leftNode(i)
				k = rightNode(j)
				if k < size && heap[k].Doc < heap[j].Doc {
					j = k
				}
				if !(j < size && heap[j].Doc < node.Doc) {
					break
				}
			}
			heap[i] = node
		}
	}
}

// All returns all wrappers in an unordered slice (for iteration).
func (pq *SpanDisiPriorityQueue) All() []*SpanDisiWrapper {
	return pq.heap[:pq.size]
}

// heap index helpers (same as Java's static methods)
func leftNode(node int) int   { return ((node + 1) << 1) - 1 }
func rightNode(left int) int  { return left + 1 }
func parentNode(node int) int { return ((node + 1) >> 1) - 1 }
