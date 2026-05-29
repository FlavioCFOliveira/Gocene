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
//   lucene/core/src/java/org/apache/lucene/search/VectorSimilarityCollector.java

import "math"

// KnnSearchStrategy is the lightweight search-strategy hook used by the
// VectorSimilarityCollector subsystem. It is distinct from
// knn.KnnSearchStrategy (the HNSW search-strategy used by the KNN vector
// queries); this one only needs a name for diagnostics.
type KnnSearchStrategy interface {
	// StrategyName returns the canonical strategy name used in toString.
	StrategyName() string
}

// vectorSimilarityDefaultStrategy is the search strategy used when none is
// specified. It mirrors AbstractVectorSimilarityQuery.DEFAULT_STRATEGY
// (KnnSearchStrategy.Hnsw with filteredSearchThreshold=0).
type vectorSimilarityDefaultStrategy struct{}

func (vectorSimilarityDefaultStrategy) StrategyName() string { return "Hnsw(0)" }

// vectorSimilarityDefaultStrategyInstance is the singleton default strategy.
var vectorSimilarityDefaultStrategyInstance KnnSearchStrategy = vectorSimilarityDefaultStrategy{}

// VectorSimilarityCollector performs a similarity-based graph search.
// The graph is traversed until the best candidate falls below
// traversalSimilarity. All visited nodes with similarity ≥ resultSimilarity
// are returned as results.
//
// Mirrors org.apache.lucene.search.VectorSimilarityCollector (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java extends AbstractKnnCollector which tracks visitedCount/visitLimit/k/strategy.
//     Go embeds these fields directly (AbstractKnnCollector is a stub in Gocene).
//   - KnnSearchStrategy in search package is a simple interface; the default strategy
//     (AbstractVectorSimilarityQuery.DEFAULT_STRATEGY) is inlined as
//     vectorSimilarityDefaultStrategy (Hnsw threshold=0).
//   - Results are collected in insertion order, matching Java's ArrayList (not sorted).
//
// @lucene.experimental
type VectorSimilarityCollector struct {
	// AbstractKnnCollector state.
	visitedCount int64
	visitLimit   int64
	k            int
	strategy     KnnSearchStrategy

	traversalSimilarity float32
	resultSimilarity    float32
	maxSimilarity       float32
	scoreDocList        []*ScoreDoc
}

// NewVectorSimilarityCollector creates a collector for similarity-based graph search.
//
//   - traversalSimilarity: lower threshold for graph traversal continuation.
//   - resultSimilarity: higher threshold for result collection.
//   - visitLimit: maximum number of nodes to visit.
//
// Panics if traversalSimilarity > resultSimilarity, mirroring Java's
// IllegalArgumentException.
//
// Mirrors VectorSimilarityCollector(float, float, long).
func NewVectorSimilarityCollector(traversalSimilarity, resultSimilarity float32, visitLimit int64) *VectorSimilarityCollector {
	if traversalSimilarity > resultSimilarity {
		panic("VectorSimilarityCollector: traversalSimilarity should be <= resultSimilarity")
	}
	return &VectorSimilarityCollector{
		visitLimit:          visitLimit,
		k:                   1,
		strategy:            vectorSimilarityDefaultStrategyInstance,
		traversalSimilarity: traversalSimilarity,
		resultSimilarity:    resultSimilarity,
		maxSimilarity:       float32(math.Inf(-1)),
	}
}

// Collect records a candidate node. Returns true always (traversal continues
// until the visit budget is exhausted or EarlyTerminated is detected by the
// caller).
//
// Mirrors VectorSimilarityCollector.collect(int, float).
func (c *VectorSimilarityCollector) Collect(docID int, similarity float32) bool {
	if similarity > c.maxSimilarity {
		c.maxSimilarity = similarity
	}
	if similarity >= c.resultSimilarity {
		c.scoreDocList = append(c.scoreDocList, &ScoreDoc{Doc: docID, Score: similarity})
	}
	return true
}

// MinCompetitiveSimilarity returns the minimum of traversalSimilarity and
// the best similarity encountered so far, which controls when the caller
// stops traversing.
//
// Mirrors VectorSimilarityCollector.minCompetitiveSimilarity().
func (c *VectorSimilarityCollector) MinCompetitiveSimilarity() float32 {
	if c.traversalSimilarity < c.maxSimilarity {
		return c.traversalSimilarity
	}
	return c.maxSimilarity
}

// TopDocs returns the collected results as TopDocs in insertion order.
// Results are NOT sorted (mirrors Java's ArrayList order).
//
// Mirrors VectorSimilarityCollector.topDocs().
func (c *VectorSimilarityCollector) TopDocs() *TopDocs {
	var relation Relation
	if c.EarlyTerminated() {
		relation = GREATER_THAN_OR_EQUAL_TO
	} else {
		relation = EQUAL_TO
	}
	return NewTopDocs(
		NewTotalHits(c.VisitedCount(), relation),
		c.scoreDocList,
	)
}

// NumCollected returns the number of results collected above resultSimilarity.
//
// Mirrors VectorSimilarityCollector.numCollected().
func (c *VectorSimilarityCollector) NumCollected() int {
	return len(c.scoreDocList)
}

// ─── AbstractKnnCollector state delegation ────────────────────────────────────

// EarlyTerminated reports whether the visit budget is exhausted.
func (c *VectorSimilarityCollector) EarlyTerminated() bool {
	return c.visitedCount >= c.visitLimit
}

// IncVisitedCount increments the visited node counter.
func (c *VectorSimilarityCollector) IncVisitedCount(count int) {
	c.visitedCount += int64(count)
}

// VisitedCount returns the current visited node count.
func (c *VectorSimilarityCollector) VisitedCount() int64 {
	return c.visitedCount
}

// VisitLimit returns the configured visit limit.
func (c *VectorSimilarityCollector) VisitLimit() int64 {
	return c.visitLimit
}

// K returns the k parameter (always 1 for similarity-based collection).
func (c *VectorSimilarityCollector) K() int {
	return c.k
}

// GetSearchStrategy returns the search strategy.
func (c *VectorSimilarityCollector) GetSearchStrategy() KnnSearchStrategy {
	return c.strategy
}
