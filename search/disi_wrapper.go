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
//   lucene/core/src/java/org/apache/lucene/search/DisiWrapper.java
//   lucene/core/src/java/org/apache/lucene/search/DisiPriorityQueue.java

// DisiWrapper wraps a Scorer for use in disjunction priority queues.
// It caches the iterator, cost, current docID, and optional two-phase
// match cost for ordering.
//
// Mirrors org.apache.lucene.search.DisiWrapper (Lucene 10.4.0).
//
// Deviations from Java:
//   - scorable is the Scorer directly (Gocene's Scorer is also Scorable).
//   - twoPhaseView and matchCost are populated only when the scorer
//     satisfies the scorerTwoPhaseProvider interface; otherwise both
//     are left at their zero values (nil / 0).
//   - iterator exposes the raw DISI (approximation when two-phase, else scorer)
//     so that WANDScorer can advance it directly as Java does.
//   - scaledMaxScore is the max score of this clause scaled as a long integer,
//     used by WANDScorer for block-max pruning.
type DisiWrapper struct {
	// scorer is the original Scorer this wrapper was built from.
	scorer Scorer
	// scorable is used for scoring; in Gocene Scorer satisfies Scorable.
	scorable Scorer
	// iterator is the raw DocIdSetIterator used for direct advancement (same
	// as approximation but named to mirror the Java field).
	iterator DocIdSetIterator
	// The DISI to use for approximation-based iteration.
	approximation DocIdSetIterator
	// cost is the estimated iteration cost (number of matching docs).
	cost int64
	// doc is the current document ID.
	doc int
	// next links wrappers sharing the same doc in a topList linked list.
	next *DisiWrapper
	// twoPhaseView is the optional TwoPhaseIterator, or nil.
	twoPhaseView *TwoPhaseIterator
	// matchCost is twoPhaseView.MatchCost() when twoPhaseView != nil.
	matchCost float32
	// scaledMaxScore is the max score of this clause scaled as a long, used
	// by WANDScorer for block-max pruning.  Zero when not in TOP_SCORES mode.
	scaledMaxScore int64
	// maxWindowScore is the maximum score this clause can contribute in the
	// current scoring window.  Used by MaxScoreBulkScorer for partitioning.
	maxWindowScore float32
}

// Doc returns the current document ID cached in this wrapper.
func (w *DisiWrapper) Doc() int { return w.doc }

// SetDoc sets the current document ID cached in this wrapper.
// Callers must call DisiPriorityQueue.UpdateTop after modifying the top.
func (w *DisiWrapper) SetDoc(doc int) { w.doc = doc }

// Next returns the next DisiWrapper in a topList chain, or nil.
// The field is set by TopList / topListAt; callers should treat the
// chain as read-only after retrieval.
func (w *DisiWrapper) Next() *DisiWrapper { return w.next }

// NewDisiWrapper constructs a DisiWrapper for scorer.
// If scorer exposes a TwoPhaseIterator via the scorerTwoPhaseProvider
// interface, the wrapper captures it and sets matchCost accordingly.
// The impacts parameter is ignored in this port (no ImpactsDISI yet).
func NewDisiWrapper(scorer Scorer, _ bool) *DisiWrapper {
	w := &DisiWrapper{
		scorer:   scorer,
		scorable: scorer,
		cost:     scorer.Cost(),
		doc:      -1,
	}
	if sp, ok := scorer.(scorerTwoPhaseProvider); ok {
		w.twoPhaseView = sp.TwoPhaseIterator()
	}
	if w.twoPhaseView != nil {
		w.approximation = w.twoPhaseView.Approximation()
		w.matchCost = w.twoPhaseView.MatchCost()
	} else {
		w.approximation = scorer
	}
	w.iterator = w.approximation
	return w
}

// ─── DisiPriorityQueue ───────────────────────────────────────────────────────

// DisiPriorityQueue is a min-heap of DisiWrapper instances ordered by
// current document ID.
//
// Mirrors org.apache.lucene.search.DisiPriorityQueue (Lucene 10.4.0).
type DisiPriorityQueue struct {
	heap []*DisiWrapper
	size int
}

// NewDisiPriorityQueue allocates a DisiPriorityQueue for up to maxSize entries.
func NewDisiPriorityQueue(maxSize int) *DisiPriorityQueue {
	return &DisiPriorityQueue{
		heap: make([]*DisiWrapper, maxSize+1), // 1-indexed
	}
}

// Size returns the number of entries in the queue.
func (pq *DisiPriorityQueue) Size() int { return pq.size }

// Add inserts w into the queue.
func (pq *DisiPriorityQueue) Add(w *DisiWrapper) *DisiWrapper {
	pq.size++
	pq.heap[pq.size] = w
	pq.upHeap(pq.size)
	return pq.heap[1]
}

// Top returns the entry with the smallest docID, or nil when empty.
func (pq *DisiPriorityQueue) Top() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	return pq.heap[1]
}

// Pop removes and returns the entry with the smallest docID.
func (pq *DisiPriorityQueue) Pop() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	top := pq.heap[1]
	pq.heap[1] = pq.heap[pq.size]
	pq.heap[pq.size] = nil
	pq.size--
	if pq.size > 0 {
		pq.downHeap(1)
	}
	return top
}

// Top2 returns the second-smallest entry (by docID), or nil when size < 2.
//
// Mirrors DisiPriorityQueueN.top2().
func (pq *DisiPriorityQueue) Top2() *DisiWrapper {
	switch pq.size {
	case 0, 1:
		return nil
	case 2:
		return pq.heap[2]
	default:
		// In the 1-indexed heap heap[2] and heap[3] are the left and right children.
		if pq.heap[2] == nil {
			return pq.heap[3]
		}
		if pq.heap[3] == nil {
			return pq.heap[2]
		}
		if pq.heap[2].doc <= pq.heap[3].doc {
			return pq.heap[2]
		}
		return pq.heap[3]
	}
}

// Clear removes all entries from the queue.
func (pq *DisiPriorityQueue) Clear() {
	for i := 1; i <= pq.size; i++ {
		pq.heap[i] = nil
	}
	pq.size = 0
}

// UpdateTop re-heapifies after the top entry's doc field was modified.
// Returns the new top.
func (pq *DisiPriorityQueue) UpdateTop() *DisiWrapper {
	pq.downHeap(1)
	return pq.heap[1]
}

// UpdateTopWith replaces the current top with w, re-heapifies, and returns
// the new top.
//
// Mirrors DisiPriorityQueueN.updateTop(DisiWrapper) (Lucene 10.4.0).
func (pq *DisiPriorityQueue) UpdateTopWith(w *DisiWrapper) *DisiWrapper {
	pq.heap[1] = w
	pq.downHeap(1)
	return pq.heap[1]
}

// ─── 0-indexed heap helpers (DisiPriorityQueueN) ────────────────────────────

// disiLeftNode returns the left child index in a 0-indexed binary heap.
// Mirrors DisiPriorityQueueN.leftNode(int).
func disiLeftNode(node int) int { return ((node + 1) << 1) - 1 }

// disiRightNode returns the right child index given the left child index.
// Mirrors DisiPriorityQueueN.rightNode(int).
func disiRightNode(leftNode int) int { return leftNode + 1 }

// disiParentNode returns the parent index in a 0-indexed binary heap.
// Mirrors DisiPriorityQueueN.parentNode(int).
func disiParentNode(node int) int { return ((node + 1) >> 1) - 1 }

// AddAll bulk-inserts len entries from wrappers[offset:offset+len] using
// O(n) Floyd build-heap.  Fails if the insertion would exceed the allocated
// capacity.
//
// Mirrors DisiPriorityQueueN.addAll(DisiWrapper[], int, int) (Lucene 10.4.0).
func (pq *DisiPriorityQueue) AddAll(wrappers []*DisiWrapper, offset, length int) {
	if pq.size+length > len(pq.heap)-1 {
		panic("DisiPriorityQueue.AddAll: insufficient capacity")
	}
	for i := 0; i < length; i++ {
		pq.size++
		pq.heap[pq.size] = wrappers[offset+i]
	}
	// Floyd build-heap from the last non-leaf downward.
	for i := pq.size / 2; i >= 1; i-- {
		pq.downHeap(i)
	}
}

// HeapAll returns a range-over-func iterator over all entries in the heap
// in heap order (not sorted by doc).  Suitable for read-only traversal.
func (pq *DisiPriorityQueue) HeapAll() func(yield func(*DisiWrapper) bool) {
	return func(yield func(*DisiWrapper) bool) {
		for i := 1; i <= pq.size; i++ {
			if !yield(pq.heap[i]) {
				return
			}
		}
	}
}

// TopList returns a linked list of all entries sharing the minimum docID.
// Entries are chained via their next pointer; caller must reset next after use.
func (pq *DisiPriorityQueue) TopList() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	topDoc := pq.heap[1].doc
	var list *DisiWrapper
	pq.addToTopList(1, topDoc, &list)
	return list
}

func (pq *DisiPriorityQueue) addToTopList(i, topDoc int, list **DisiWrapper) {
	w := pq.heap[i]
	if w == nil || w.doc != topDoc {
		return
	}
	w.next = *list
	*list = w
	left := i * 2
	if left <= pq.size {
		pq.addToTopList(left, topDoc, list)
		right := left + 1
		if right <= pq.size {
			pq.addToTopList(right, topDoc, list)
		}
	}
}

func (pq *DisiPriorityQueue) upHeap(i int) {
	for i > 1 {
		parent := i / 2
		if pq.heap[parent].doc <= pq.heap[i].doc {
			break
		}
		pq.heap[parent], pq.heap[i] = pq.heap[i], pq.heap[parent]
		i = parent
	}
}

func (pq *DisiPriorityQueue) downHeap(i int) {
	for {
		left := i * 2
		if left > pq.size {
			break
		}
		smallest := i
		if pq.heap[left].doc < pq.heap[smallest].doc {
			smallest = left
		}
		if right := left + 1; right <= pq.size && pq.heap[right].doc < pq.heap[smallest].doc {
			smallest = right
		}
		if smallest == i {
			break
		}
		pq.heap[i], pq.heap[smallest] = pq.heap[smallest], pq.heap[i]
		i = smallest
	}
}
