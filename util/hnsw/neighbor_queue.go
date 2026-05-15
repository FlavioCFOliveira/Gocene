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

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// NeighborQueue stores HNSW graph arcs as (score, nodeID) pairs packed into a
// sortable int64 and held in a 3-ary heap. It supports both a min-heap and a
// max-heap mode and exposes the bounded [NeighborQueue.InsertWithOverflow]
// alongside the unbounded [NeighborQueue.Add].
//
// This is a port of org.apache.lucene.util.hnsw.NeighborQueue (Lucene 10.4.0)
// and is byte-for-byte semantically compatible with the reference: the same
// (score, node) inputs produce the same heap layout, the same top-of-heap
// answers, and the same tie-breaking decisions. The encoding is the one
// described in the Java javadoc:
//
//   - The most significant 32 bits hold the float score, converted to a
//     sortable int via [util.FloatToSortableInt]; lexicographic order on the
//     encoded long therefore matches the natural float ordering.
//   - The least significant 32 bits hold the bitwise complement of the node
//     id, masked to 32 bits. Complementing the node id makes the encoded long
//     comparator break score ties in favour of the smaller node id (i.e. on
//     an overflow with equal scores, the smaller id displaces the larger).
//   - For the max-heap variant, the entire long is bit-flipped (-1 - v ==
//     ^v) so that the underlying min-heap orders larger original scores
//     first. The complement form is used in lieu of negation because
//     Long.MIN_VALUE has no positive counterpart; ^v always round-trips.
//
// NeighborQueue is not safe for concurrent use.
type NeighborQueue struct {
	heap  *util.TernaryLongHeap
	order neighborOrder

	// visitedCount tracks the number of neighbors visited during a single
	// graph traversal; carried alongside the heap purely as bookkeeping.
	visitedCount int
	// incomplete is set when a search stopped early because it reached the
	// visited-nodes limit; reset by Clear.
	incomplete bool
}

// neighborOrder selects the encoding transform that turns a packed
// (score, node) long into its heap-sortable form.
type neighborOrder uint8

const (
	minHeapOrder neighborOrder = iota
	maxHeapOrder
)

// apply returns the heap-sortable form of v for this order.
//
// For minHeapOrder this is the identity. For maxHeapOrder this is the bit
// complement of v, i.e. -1 - v in two's-complement arithmetic; this matches
// the Java Order.MAX_HEAP.apply implementation and, unlike plain negation,
// has the property of being its own inverse and of mapping the full int64
// range onto itself (including Long.MIN_VALUE / Long.MAX_VALUE).
func (o neighborOrder) apply(v int64) int64 {
	if o == maxHeapOrder {
		return ^v
	}
	return v
}

// NewNeighborQueue returns an empty NeighborQueue with the given initial
// capacity. initialSize must be > 0; a non-positive value panics, mirroring
// Java's IllegalArgumentException. When maxHeap is true the queue keeps the
// largest score at the top (so InsertWithOverflow on a full queue evicts the
// largest); otherwise it keeps the smallest at the top.
//
// The initial capacity is also the bound used by InsertWithOverflow; Add
// grows the queue beyond it without limit.
func NewNeighborQueue(initialSize int, maxHeap bool) *NeighborQueue {
	order := minHeapOrder
	if maxHeap {
		order = maxHeapOrder
	}
	return &NeighborQueue{
		heap:  util.NewTernaryLongHeap(initialSize),
		order: order,
	}
}

// Size returns the number of elements currently in the queue.
func (q *NeighborQueue) Size() int { return q.heap.Size() }

// Add inserts a new (nodeID, score) arc into the queue, growing the backing
// heap as needed. Use Add when the queue is meant to be unbounded.
func (q *NeighborQueue) Add(newNode int32, newScore float32) {
	q.heap.Push(q.encode(newNode, newScore))
}

// InsertWithOverflow inserts (newNode, newScore) into the queue if it is not
// yet at initial capacity, or if the new value would displace the current top
// (i.e. the worst-of-the-kept element). Returns true when the entry was
// retained — either as a fresh insertion or by replacing the top — and false
// when the queue was full and the candidate failed to improve on the top.
func (q *NeighborQueue) InsertWithOverflow(newNode int32, newScore float32) bool {
	return q.heap.InsertWithOverflow(q.encode(newNode, newScore))
}

// encode packs (node, score) into the order-applied int64 representation
// stored in the heap. The encoding mirrors Lucene exactly:
//
//	long raw = (long(sortableInt(score)) << 32) | (^node & 0xFFFFFFFF)
//	long out = order.apply(raw)
//
// Score occupies the high 32 bits as a sortable int; ^node occupies the low
// 32 bits to break score ties in favour of the smaller node id.
func (q *NeighborQueue) encode(node int32, score float32) int64 {
	scoreBits := int64(util.FloatToSortableInt(score)) << 32
	nodeBits := int64(uint32(^node))
	return q.order.apply(scoreBits | nodeBits)
}

// decodeScore extracts the float score from a heap-sortable int64.
func (q *NeighborQueue) decodeScore(heapValue int64) float32 {
	// Use arithmetic right shift to recover the (sign-extended) sortable int.
	return util.SortableIntToFloat(int32(q.order.apply(heapValue) >> 32))
}

// decodeNodeID extracts the node id from a heap-sortable int64. The low 32
// bits hold ^node, so the original node id is recovered by complementing
// once more and truncating to int32.
func (q *NeighborQueue) decodeNodeID(heapValue int64) int32 {
	return int32(^q.order.apply(heapValue))
}

// Pop removes the top element and returns its node id. Calling Pop on an
// empty queue panics (the panic originates in TernaryLongHeap.Pop, matching
// Java's IllegalStateException for the equivalent operation).
func (q *NeighborQueue) Pop() int32 {
	return q.decodeNodeID(q.heap.Pop())
}

// Nodes returns a freshly allocated slice containing every node id currently
// in the queue, in INTERNAL heap order (not score-sorted). The slice is safe
// to mutate by the caller; subsequent queue operations are unaffected.
func (q *NeighborQueue) Nodes() []int32 {
	size := q.Size()
	out := make([]int32, size)
	for i := 0; i < size; i++ {
		out[i] = q.decodeNodeID(q.heap.Get(i + 1))
	}
	return out
}

// TopNode returns the node id of the top element without removing it.
// For a min-heap this is the node with the smallest score; for a max-heap
// it is the node with the largest score.
func (q *NeighborQueue) TopNode() int32 {
	return q.decodeNodeID(q.heap.Top())
}

// TopScore returns the score of the top element without removing it.
// For a min-heap this is the minimum score; for a max-heap the maximum.
//
// On an empty queue TopScore returns the float decoded from a zero heap
// value (0.0 for either order), matching the Java reference which performs
// no emptiness check.
func (q *NeighborQueue) TopScore() float32 {
	return q.decodeScore(q.heap.Top())
}

// Clear removes every element from the queue and resets the visited-count
// and incomplete flags. The backing array is preserved.
func (q *NeighborQueue) Clear() {
	q.heap.Clear()
	q.visitedCount = 0
	q.incomplete = false
}

// VisitedCount returns the number of neighbors recorded as visited during the
// last traversal; the queue itself never updates this value — callers are
// responsible for calling SetVisitedCount.
func (q *NeighborQueue) VisitedCount() int { return q.visitedCount }

// SetVisitedCount sets the visited-count counter.
func (q *NeighborQueue) SetVisitedCount(visitedCount int) { q.visitedCount = visitedCount }

// Incomplete reports whether the most recent traversal was marked as
// incomplete (typically because it hit a visited-nodes limit).
func (q *NeighborQueue) Incomplete() bool { return q.incomplete }

// MarkIncomplete records that the current traversal stopped early. The flag
// is cleared by Clear.
func (q *NeighborQueue) MarkIncomplete() { q.incomplete = true }

// String returns the same textual form as the Java NeighborQueue.toString:
// "Neighbors[<size>]". Intended for debugging only.
func (q *NeighborQueue) String() string {
	return fmt.Sprintf("Neighbors[%d]", q.heap.Size())
}
