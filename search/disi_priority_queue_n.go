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

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisiPriorityQueueN.java

// DisiPriorityQueueN is a 0-indexed min-heap of DisiWrapper instances ordered
// by current document ID.
//
// This is the Go equivalent of the Java DisiPriorityQueueN (Lucene 10.4.0),
// which is the concrete 0-indexed implementation of the abstract
// DisiPriorityQueue class.  Gocene's original DisiPriorityQueue uses a
// 1-indexed heap; DisiPriorityQueueN is the separate 0-indexed variant.
//
// The 0-indexed heap navigation helpers (disiLeftNode, disiRightNode,
// disiParentNode) live in disi_wrapper.go and are shared with the heap
// management code in WANDScorer.
//
// Mirrors org.apache.lucene.search.DisiPriorityQueueN (Lucene 10.4.0).
type DisiPriorityQueueN struct {
	heap []*DisiWrapper
	size int
}

// NewDisiPriorityQueueN allocates a DisiPriorityQueueN for up to maxSize entries.
func NewDisiPriorityQueueN(maxSize int) *DisiPriorityQueueN {
	return &DisiPriorityQueueN{
		heap: make([]*DisiWrapper, maxSize),
	}
}

// Size returns the number of entries in the queue.
func (pq *DisiPriorityQueueN) Size() int { return pq.size }

// Top returns the entry with the smallest docID, or nil when empty.
func (pq *DisiPriorityQueueN) Top() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	return pq.heap[0]
}

// Top2 returns the second-smallest entry (by docID), or nil when size < 2.
//
// Mirrors DisiPriorityQueueN.top2().
func (pq *DisiPriorityQueueN) Top2() *DisiWrapper {
	switch pq.size {
	case 0, 1:
		return nil
	case 2:
		return pq.heap[1]
	default:
		if pq.heap[1].doc <= pq.heap[2].doc {
			return pq.heap[1]
		}
		return pq.heap[2]
	}
}

// TopList returns a linked list of all entries sharing the minimum docID.
// Entries are chained via their next pointer.
//
// Mirrors DisiPriorityQueueN.topList().
func (pq *DisiPriorityQueueN) TopList() *DisiWrapper {
	heap := pq.heap
	size := pq.size
	if size == 0 {
		return nil
	}
	list := heap[0]
	list.next = nil
	if size >= 3 {
		list = pq.topListAt(list, 1)
		list = pq.topListAt(list, 2)
	} else if size == 2 && heap[1].doc == list.doc {
		heap[1].next = list
		list = heap[1]
	}
	return list
}

func (pq *DisiPriorityQueueN) topListAt(list *DisiWrapper, i int) *DisiWrapper {
	heap := pq.heap
	size := pq.size
	w := heap[i]
	if w.doc == list.doc {
		w.next = list
		list = w
		left := disiLeftNode(i)
		right := disiRightNode(left)
		if right < size {
			list = pq.topListAt(list, left)
			list = pq.topListAt(list, right)
		} else if left < size && heap[left].doc == list.doc {
			heap[left].next = list
			list = heap[left]
		}
	}
	return list
}

// Add inserts w into the queue and returns the new top.
//
// Mirrors DisiPriorityQueueN.add(DisiWrapper).
func (pq *DisiPriorityQueueN) Add(w *DisiWrapper) *DisiWrapper {
	pq.heap[pq.size] = w
	pq.upHeap(pq.size)
	pq.size++
	return pq.heap[0]
}

// AddAll bulk-inserts len entries from entries[offset:offset+len] using
// O(n) Floyd heapify.
//
// Mirrors DisiPriorityQueueN.addAll(DisiWrapper[], int, int).
func (pq *DisiPriorityQueueN) AddAll(entries []*DisiWrapper, offset, length int) {
	if length == 0 {
		return
	}
	if pq.size+length > len(pq.heap) {
		panic("DisiPriorityQueueN.AddAll: insufficient capacity")
	}
	copy(pq.heap[pq.size:], entries[offset:offset+length])
	pq.size += length

	// Floyd O(n) heapify.
	firstLeaf := pq.size >> 1
	for rootIdx := firstLeaf - 1; rootIdx >= 0; rootIdx-- {
		parentIdx := rootIdx
		parent := pq.heap[parentIdx]
		for parentIdx < firstLeaf {
			childIdx := disiLeftNode(parentIdx)
			rightChildIdx := disiRightNode(childIdx)
			child := pq.heap[childIdx]
			if rightChildIdx < pq.size && pq.heap[rightChildIdx].doc < child.doc {
				child = pq.heap[rightChildIdx]
				childIdx = rightChildIdx
			}
			if child.doc >= parent.doc {
				break
			}
			pq.heap[parentIdx] = child
			parentIdx = childIdx
		}
		pq.heap[parentIdx] = parent
	}
}

// Pop removes and returns the entry with the smallest docID.
//
// Mirrors DisiPriorityQueueN.pop().
func (pq *DisiPriorityQueueN) Pop() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	result := pq.heap[0]
	pq.size--
	pq.heap[0] = pq.heap[pq.size]
	pq.heap[pq.size] = nil
	pq.downHeap(pq.size)
	return result
}

// UpdateTop re-heapifies after the top entry's doc field was modified.
// Returns the new top.
//
// Mirrors DisiPriorityQueueN.updateTop().
func (pq *DisiPriorityQueueN) UpdateTop() *DisiWrapper {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

// UpdateTopWith replaces the current top with w, re-heapifies, and returns
// the new top.
//
// Mirrors DisiPriorityQueueN.updateTop(DisiWrapper).
func (pq *DisiPriorityQueueN) UpdateTopWith(w *DisiWrapper) *DisiWrapper {
	pq.heap[0] = w
	return pq.UpdateTop()
}

// Clear removes all entries from the queue.
//
// Mirrors DisiPriorityQueueN.clear().
func (pq *DisiPriorityQueueN) Clear() {
	for i := 0; i < pq.size; i++ {
		pq.heap[i] = nil
	}
	pq.size = 0
}

// HeapAll returns a range-over-func iterator over all entries in heap order
// (not sorted by doc).
func (pq *DisiPriorityQueueN) HeapAll() func(yield func(*DisiWrapper) bool) {
	return func(yield func(*DisiWrapper) bool) {
		for i := 0; i < pq.size; i++ {
			if !yield(pq.heap[i]) {
				return
			}
		}
	}
}

// upHeap sifts the entry at index i upward to its correct heap position.
//
// Mirrors DisiPriorityQueueN.upHeap(int).
func (pq *DisiPriorityQueueN) upHeap(i int) {
	node := pq.heap[i]
	nodeDoc := node.doc
	j := disiParentNode(i)
	for j >= 0 && nodeDoc < pq.heap[j].doc {
		pq.heap[i] = pq.heap[j]
		i = j
		j = disiParentNode(j)
	}
	pq.heap[i] = node
}

// downHeap sifts the entry at position 0 downward to its correct heap position.
//
// Mirrors DisiPriorityQueueN.downHeap(int).
func (pq *DisiPriorityQueueN) downHeap(size int) {
	i := 0
	node := pq.heap[0]
	j := disiLeftNode(i)
	if j < size {
		k := disiRightNode(j)
		if k < size && pq.heap[k].doc < pq.heap[j].doc {
			j = k
		}
		if pq.heap[j].doc < node.doc {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = disiLeftNode(i)
				k := disiRightNode(j)
				if k < size && pq.heap[k].doc < pq.heap[j].doc {
					j = k
				}
				if j >= size || pq.heap[j].doc >= node.doc {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}
