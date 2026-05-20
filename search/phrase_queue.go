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
//   lucene/core/src/java/org/apache/lucene/search/PhraseQueue.java

// PhraseQueue is a min-heap of PhrasePositions ordered by (position, offset, ord).
//
// Mirrors org.apache.lucene.search.PhraseQueue (Lucene 10.4.0), which extends
// Lucene's PriorityQueue<PhrasePositions>.
//
// The comparison follows Java's lessThan:
//  1. Smaller position wins.
//  2. On tie: smaller offset wins.
//  3. On double tie: smaller ord wins.
type PhraseQueue struct {
	heap []*PhrasePositions
	size int
}

// NewPhraseQueue allocates a PhraseQueue for up to size entries.
func NewPhraseQueue(size int) *PhraseQueue {
	return &PhraseQueue{heap: make([]*PhrasePositions, size+1)} // 1-indexed
}

// Size returns the number of entries.
func (pq *PhraseQueue) Size() int { return pq.size }

// Top returns the top (smallest) entry, or nil when empty.
func (pq *PhraseQueue) Top() *PhrasePositions {
	if pq.size == 0 {
		return nil
	}
	return pq.heap[1]
}

// Add inserts pp and returns the new top.
func (pq *PhraseQueue) Add(pp *PhrasePositions) *PhrasePositions {
	pq.size++
	if pq.size >= len(pq.heap) {
		grown := make([]*PhrasePositions, pq.size*2+1)
		copy(grown, pq.heap)
		pq.heap = grown
	}
	pq.heap[pq.size] = pp
	pq.upHeap(pq.size)
	return pq.heap[1]
}

// Pop removes and returns the top entry.
func (pq *PhraseQueue) Pop() *PhrasePositions {
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

// UpdateTop rebalances after the top element has been mutated.
func (pq *PhraseQueue) UpdateTop() *PhrasePositions {
	pq.downHeap(1)
	return pq.heap[1]
}

// lessThan mirrors PhraseQueue.lessThan: returns true when pp1 should be
// higher in the queue (i.e., pp1 comes before pp2).
func phraseQueueLessThan(pp1, pp2 *PhrasePositions) bool {
	if pp1.Position == pp2.Position {
		if pp1.Offset == pp2.Offset {
			return pp1.Ord < pp2.Ord
		}
		return pp1.Offset < pp2.Offset
	}
	return pp1.Position < pp2.Position
}

func (pq *PhraseQueue) upHeap(i int) {
	node := pq.heap[i]
	j := i >> 1
	for j > 0 && phraseQueueLessThan(node, pq.heap[j]) {
		pq.heap[i] = pq.heap[j]
		i = j
		j = i >> 1
	}
	pq.heap[i] = node
}

func (pq *PhraseQueue) downHeap(i int) {
	node := pq.heap[i]
	j := i << 1
	k := j + 1
	if k <= pq.size && phraseQueueLessThan(pq.heap[k], pq.heap[j]) {
		j = k
	}
	for j <= pq.size && phraseQueueLessThan(pq.heap[j], node) {
		pq.heap[i] = pq.heap[j]
		i = j
		j = i << 1
		k = j + 1
		if k <= pq.size && phraseQueueLessThan(pq.heap[k], pq.heap[j]) {
			j = k
		}
	}
	pq.heap[i] = node
}
