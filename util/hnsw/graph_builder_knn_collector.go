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
	"math"
)

// GraphBuilderKnnCollector is a restricted, specialised [KnnCollector]
// used while building an HNSW graph. It is the Go port of
// org.apache.lucene.util.hnsw.HnswGraphBuilder.GraphBuilderKnnCollector
// (a public static final inner class of HnswGraphBuilder in Lucene
// 10.4.0).
//
// The collector wraps a min-heap [NeighborQueue] (descending order is
// false because the queue keeps the smallest score at the top, evicting
// the worst-of-the-kept entries during overflow). It does not produce
// TopDocs — calling [GraphBuilderKnnCollector.TopDocs] panics, matching
// Java's IllegalArgumentException.
//
// The collector exposes a few extra operations the builder uses to
// drain candidates back into a [NeighborArray]:
//
//   - PopNode pops the worst-scoring node id (a level-order pop from
//     the heap top).
//   - PopUntilNearestKNodes shrinks the heap to its top-k members and
//     returns the underlying node id slice.
//   - MinimumScore reports the heap top's score (the threshold a new
//     candidate must beat to overflow).
//
// Mirrors the Java class field-for-field; the visited-count is local
// state, the search strategy is fixed at nil, and earlyTerminated is
// always false (the builder polices its own visit limit through the
// graph's growth).
//
// Not safe for concurrent use.
type GraphBuilderKnnCollector struct {
	queue        *NeighborQueue
	k            int
	visitedCount int64
}

// NewGraphBuilderKnnCollector constructs a collector targeting k
// results. Mirrors Java's GraphBuilderKnnCollector(int k).
func NewGraphBuilderKnnCollector(k int) *GraphBuilderKnnCollector {
	return &GraphBuilderKnnCollector{
		queue: NewNeighborQueue(k, false),
		k:     k,
	}
}

// Size returns the number of items currently held.
func (c *GraphBuilderKnnCollector) Size() int { return c.queue.Size() }

// PopNode removes and returns the worst-scoring node (the heap top).
// Mirrors Java's public int popNode().
func (c *GraphBuilderKnnCollector) PopNode() int { return int(c.queue.Pop()) }

// PopUntilNearestKNodes drains entries from the top until at most k
// remain, then returns the surviving node ids in heap order. Mirrors
// Java's public int[] popUntilNearestKNodes(); Java returns the
// queue's internal int[] buffer, but the Go [NeighborQueue.Nodes]
// helper already widens int32 → int and allocates a fresh slice, so
// the Go return is a new slice the caller may keep across further
// collector operations.
func (c *GraphBuilderKnnCollector) PopUntilNearestKNodes() []int {
	for c.Size() > c.k {
		c.queue.Pop()
	}
	nodes := c.queue.Nodes()
	out := make([]int, len(nodes))
	for i, n := range nodes {
		out[i] = int(n)
	}
	return out
}

// MinimumScore returns the score at the top of the heap, the threshold
// a new candidate must beat to overflow. Mirrors Java's package-private
// float minimumScore().
//
// On an empty heap [NeighborQueue.TopScore] returns the float decoded
// from a zero heap value (typically 0.0); the Java reference performs
// no emptiness check either.
func (c *GraphBuilderKnnCollector) MinimumScore() float32 {
	return c.queue.TopScore()
}

// Clear resets the heap and the visited-count.
func (c *GraphBuilderKnnCollector) Clear() {
	c.queue.Clear()
	c.visitedCount = 0
}

// EarlyTerminated is always false for the builder collector; the
// builder enforces its own bounds through k and graph growth.
func (c *GraphBuilderKnnCollector) EarlyTerminated() bool { return false }

// IncVisitedCount records that count additional vectors have been
// visited.
func (c *GraphBuilderKnnCollector) IncVisitedCount(count int) {
	c.visitedCount += int64(count)
}

// VisitedCount returns the cumulative visited vector count.
func (c *GraphBuilderKnnCollector) VisitedCount() int64 { return c.visitedCount }

// VisitLimit is the maximum visit budget; the builder collector
// reports math.MaxInt64, matching Java's Long.MAX_VALUE.
func (c *GraphBuilderKnnCollector) VisitLimit() int64 { return math.MaxInt64 }

// K returns the configured k.
func (c *GraphBuilderKnnCollector) K() int { return c.k }

// Collect inserts (docID, similarity) into the bounded queue and
// returns true when the entry was retained.
func (c *GraphBuilderKnnCollector) Collect(docID int, similarity float32) bool {
	return c.queue.InsertWithOverflow(int32(docID), similarity)
}

// MinCompetitiveSimilarity is the smallest score the collector would
// still retain. Returns the heap top once the queue holds at least k
// entries; otherwise -Inf. Mirrors the Java implementation precisely.
func (c *GraphBuilderKnnCollector) MinCompetitiveSimilarity() float32 {
	if c.queue.Size() >= c.K() {
		return c.queue.TopScore()
	}
	return float32(math.Inf(-1))
}

// TopDocs panics: the builder collector does not support TopDocs,
// matching Java's @Override that throws IllegalArgumentException.
func (c *GraphBuilderKnnCollector) TopDocs() *TopDocs {
	panic("hnsw: GraphBuilderKnnCollector.TopDocs is not supported")
}

// GetSearchStrategy returns nil for the builder collector.
func (c *GraphBuilderKnnCollector) GetSearchStrategy() KnnSearchStrategy {
	return nil
}

// Compile-time guard: GraphBuilderKnnCollector satisfies KnnCollector.
var _ KnnCollector = (*GraphBuilderKnnCollector)(nil)
