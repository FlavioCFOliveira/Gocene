// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/DisiPriorityQueue.java

package intervals

// DisiPriorityQueue is a priority queue of DocIdSetIterators that orders by
// current doc ID.
//
// Mirrors org.apache.lucene.queries.intervals.DisiPriorityQueue (Lucene 10.4.0).
type DisiPriorityQueue struct {
	heap []*DisiWrapper
	size int
}

// NewDisiPriorityQueue creates a queue with the given maximum size.
func NewDisiPriorityQueue(maxSize int) *DisiPriorityQueue {
	return &DisiPriorityQueue{heap: make([]*DisiWrapper, maxSize)}
}

// Size returns the number of elements.
func (pq *DisiPriorityQueue) Size() int { return pq.size }

// Top returns the element with the smallest doc ID.
func (pq *DisiPriorityQueue) Top() *DisiWrapper { return pq.heap[0] }

// TopList returns a linked list of wrappers at the minimum doc ID.
func (pq *DisiPriorityQueue) TopList() *DisiWrapper {
	heap := pq.heap
	size := pq.size
	list := heap[0]
	list.Next = nil
	if size >= 3 {
		list = pq.topList(list, heap, size, 1)
		list = pq.topList(list, heap, size, 2)
	} else if size == 2 && heap[1].Doc == list.Doc {
		heap[1].Next = list
		list = heap[1]
	}
	return list
}

func (pq *DisiPriorityQueue) topList(list *DisiWrapper, heap []*DisiWrapper, size, i int) *DisiWrapper {
	w := heap[i]
	if w.Doc == list.Doc {
		w.Next = list
		list = w
		left := (i+1)*2 - 1
		right := left + 1
		if right < size {
			list = pq.topList(list, heap, size, left)
			list = pq.topList(list, heap, size, right)
		} else if left < size && heap[left].Doc == list.Doc {
			heap[left].Next = list
			list = heap[left]
		}
	}
	return list
}

// Add adds an entry and returns the new top.
func (pq *DisiPriorityQueue) Add(entry *DisiWrapper) *DisiWrapper {
	size := pq.size
	pq.heap[size] = entry
	pq.upHeap(size)
	pq.size++
	return pq.heap[0]
}

// Pop removes and returns the top element.
func (pq *DisiPriorityQueue) Pop() *DisiWrapper {
	result := pq.heap[0]
	pq.size--
	pq.heap[0] = pq.heap[pq.size]
	pq.heap[pq.size] = nil
	pq.downHeap(pq.size)
	return result
}

// UpdateTop re-heapifies and returns the new top.
func (pq *DisiPriorityQueue) UpdateTop() *DisiWrapper {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

// All returns all wrappers (for iteration).
func (pq *DisiPriorityQueue) All() []*DisiWrapper {
	return pq.heap[:pq.size]
}

func disiLeftNode(node int) int  { return ((node + 1) << 1) - 1 }
func disiRightNode(left int) int { return left + 1 }
func disiParent(node int) int    { return ((node + 1) >> 1) - 1 }

func (pq *DisiPriorityQueue) upHeap(i int) {
	node := pq.heap[i]
	nodeDoc := node.Doc
	j := disiParent(i)
	for j >= 0 && nodeDoc < pq.heap[j].Doc {
		pq.heap[i] = pq.heap[j]
		i = j
		j = disiParent(j)
	}
	pq.heap[i] = node
}

func (pq *DisiPriorityQueue) downHeap(size int) {
	i := 0
	node := pq.heap[0]
	j := disiLeftNode(i)
	if j < size {
		k := disiRightNode(j)
		if k < size && pq.heap[k].Doc < pq.heap[j].Doc {
			j = k
		}
		if pq.heap[j].Doc < node.Doc {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = disiLeftNode(i)
				k = disiRightNode(j)
				if k < size && pq.heap[k].Doc < pq.heap[j].Doc {
					j = k
				}
				if !(j < size && pq.heap[j].Doc < node.Doc) {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}
