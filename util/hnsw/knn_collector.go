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

// knn_collector.go — temporary local surface for the kNN collector
// types that HnswGraphSearcher depends on. The canonical types live in
// org.apache.lucene.search.KnnCollector / TopKnnCollector /
// AbstractKnnCollector / KnnSearchStrategy and will move out of this
// package when the search-base sprint ports them. The shape here is
// kept deliberately minimal and structurally identical to the Java
// reference so the migration can be a rename rather than a redesign.

package hnsw

import (
	"math"
)

// KnnCollector is the local stub of
// org.apache.lucene.search.KnnCollector. HnswGraphSearcher only uses
// the small subset captured below: result collection, visit-count /
// limit accounting, k() inspection, minCompetitiveSimilarity for
// early termination, and getSearchStrategy() for strategy-driven
// dispatch.
//
// TODO(rmp): unify with the canonical org.apache.lucene.search
// KnnCollector once that package lands.
type KnnCollector interface {
	// EarlyTerminated reports whether the collector has exhausted
	// its visit budget and the caller should stop.
	EarlyTerminated() bool

	// IncVisitedCount records that count additional vectors have
	// been visited.
	IncVisitedCount(count int)

	// VisitedCount returns the current visited vector count.
	VisitedCount() int64

	// VisitLimit returns the configured ceiling on visited vectors.
	VisitLimit() int64

	// K returns the expected number of collected results.
	K() int

	// Collect collects (docID, similarity); returns true when the
	// pair was retained (either a fresh insertion or an overflow
	// replacement).
	Collect(docID int, similarity float32) bool

	// MinCompetitiveSimilarity returns the smallest similarity score
	// that the collector would still retain. Returns
	// math.Inf(-1) (as float32) when the collector is not yet full.
	MinCompetitiveSimilarity() float32

	// TopDocs drains the collected results into a TopDocs. This is
	// generally a destructive call; subsequent collector use is
	// undefined.
	TopDocs() *TopDocs

	// GetSearchStrategy returns the search strategy bound to this
	// collector, or nil.
	GetSearchStrategy() KnnSearchStrategy
}

// KnnSearchStrategy is the local stub of
// org.apache.lucene.search.knn.KnnSearchStrategy. Only the per-block
// hook is currently invoked by HnswGraphSearcher; equality/hash
// semantics are deferred until a concrete strategy needs them.
//
// TODO(rmp): unify with the canonical KnnSearchStrategy in the
// search-base sprint.
type KnnSearchStrategy interface {
	// NextVectorsBlock signals the strategy that another block of
	// candidate vectors is about to be examined.
	NextVectorsBlock()
}

// HnswStrategy mirrors org.apache.lucene.search.knn.KnnSearchStrategy.Hnsw.
// It encodes a filtered-search threshold expressed as an integer
// percentage in [0, 100]: 0 means never use filtered search, 100 means
// always. HnswGraphSearcher reads this to decide between the regular
// and the filtered code paths.
type HnswStrategy struct {
	filteredSearchThreshold int
}

// NewHnswStrategy constructs an HnswStrategy. filteredSearchThreshold
// must be in [0, 100] or the constructor panics, mirroring Java's
// IllegalArgumentException.
func NewHnswStrategy(filteredSearchThreshold int) *HnswStrategy {
	if filteredSearchThreshold < 0 || filteredSearchThreshold > 100 {
		panic("hnsw: filteredSearchThreshold must be >= 0 and <= 100")
	}
	return &HnswStrategy{filteredSearchThreshold: filteredSearchThreshold}
}

// DefaultHnswStrategy is the package-level default Hnsw strategy:
// filteredSearchThreshold == 0 (never use filtered search). Mirrors
// Java's KnnSearchStrategy.Hnsw.DEFAULT, which is the value
// HnswGraphSearcher falls back to when the collector exposes none.
var DefaultHnswStrategy = NewHnswStrategy(0)

// FilteredSearchThreshold returns the configured threshold.
func (s *HnswStrategy) FilteredSearchThreshold() int { return s.filteredSearchThreshold }

// UseFilteredSearch reports whether the filtered-search path should be
// taken for a graph whose ratioPassingFilter (fraction of accepted
// nodes) is the supplied value. ratioPassingFilter must be in [0, 1].
//
// Mirrors KnnSearchStrategy.Hnsw#useFilteredSearch.
func (s *HnswStrategy) UseFilteredSearch(ratioPassingFilter float32) bool {
	if ratioPassingFilter < 0 || ratioPassingFilter > 1 {
		panic("hnsw: ratioPassingFilter out of [0,1]")
	}
	return ratioPassingFilter*100 < float32(s.filteredSearchThreshold)
}

// NextVectorsBlock is a no-op for the plain HNSW strategy.
func (s *HnswStrategy) NextVectorsBlock() {}

// ScoreDoc is the local stub of org.apache.lucene.search.ScoreDoc.
// It carries a doc id, the similarity score, and an unused shard
// index field for parity with the canonical type.
//
// TODO(rmp): unify with search.ScoreDoc once util/hnsw is wired to
// the search package without a cycle.
type ScoreDoc struct {
	Doc        int
	Score      float32
	ShardIndex int
}

// NewScoreDoc constructs a ScoreDoc with ShardIndex == -1, the
// convention used by the Java reference for results that have not yet
// been assigned to a shard.
func NewScoreDoc(doc int, score float32) *ScoreDoc {
	return &ScoreDoc{Doc: doc, Score: score, ShardIndex: -1}
}

// TotalHitsRelation indicates whether a TotalHits value is exact or a
// lower bound.
type TotalHitsRelation int

const (
	// EqualTo means the TotalHits value is exact.
	EqualTo TotalHitsRelation = iota
	// GreaterThanOrEqualTo means the TotalHits value is a lower
	// bound; the true count may be larger.
	GreaterThanOrEqualTo
)

// TotalHits is the local stub of org.apache.lucene.search.TotalHits.
type TotalHits struct {
	Value    int64
	Relation TotalHitsRelation
}

// NewTotalHits constructs a TotalHits.
func NewTotalHits(value int64, relation TotalHitsRelation) *TotalHits {
	return &TotalHits{Value: value, Relation: relation}
}

// TopDocs is the local stub of org.apache.lucene.search.TopDocs.
// ScoreDocs is presented in score-descending order by [TopKnnCollector.TopDocs].
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
}

// NewTopDocs constructs a TopDocs.
func NewTopDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc) *TopDocs {
	return &TopDocs{TotalHits: totalHits, ScoreDocs: scoreDocs}
}

// AbstractKnnCollector captures the visit-budget bookkeeping shared
// across concrete KnnCollector implementations. Mirrors
// org.apache.lucene.search.AbstractKnnCollector.
//
// Subclasses embed *AbstractKnnCollector and supply Collect,
// MinCompetitiveSimilarity, TopDocs, and (optionally) GetSearchStrategy.
type AbstractKnnCollector struct {
	visitedCount   int64
	visitLimit     int64
	k              int
	searchStrategy KnnSearchStrategy
}

// NewAbstractKnnCollector constructs the base collector.
func NewAbstractKnnCollector(k int, visitLimit int64, strategy KnnSearchStrategy) *AbstractKnnCollector {
	return &AbstractKnnCollector{
		k:              k,
		visitLimit:     visitLimit,
		searchStrategy: strategy,
	}
}

// EarlyTerminated returns true once the visited-count reaches the
// visit limit.
func (a *AbstractKnnCollector) EarlyTerminated() bool {
	return a.visitedCount >= a.visitLimit
}

// IncVisitedCount adds count to the visited-count counter.
func (a *AbstractKnnCollector) IncVisitedCount(count int) {
	if count < 0 {
		panic("hnsw: IncVisitedCount with negative count")
	}
	a.visitedCount += int64(count)
}

// VisitedCount returns the cumulative visited vector count.
func (a *AbstractKnnCollector) VisitedCount() int64 { return a.visitedCount }

// VisitLimit returns the configured visit-budget ceiling.
func (a *AbstractKnnCollector) VisitLimit() int64 { return a.visitLimit }

// K returns the expected number of results.
func (a *AbstractKnnCollector) K() int { return a.k }

// GetSearchStrategy returns the configured strategy, possibly nil.
func (a *AbstractKnnCollector) GetSearchStrategy() KnnSearchStrategy {
	return a.searchStrategy
}

// TopKnnCollector is the local stub of
// org.apache.lucene.search.TopKnnCollector: a KnnCollector backed by a
// min-heap NeighborQueue that retains the top-k highest similarity
// (docID, score) pairs.
//
// TODO(rmp): unify with search.TopKnnCollector when the search-base
// port lands.
type TopKnnCollector struct {
	*AbstractKnnCollector
	queue *NeighborQueue
}

// NewTopKnnCollector constructs a TopKnnCollector for k results with
// the supplied visit budget and (optional) search strategy.
func NewTopKnnCollector(k int, visitLimit int, strategy KnnSearchStrategy) *TopKnnCollector {
	return &TopKnnCollector{
		AbstractKnnCollector: NewAbstractKnnCollector(k, int64(visitLimit), strategy),
		queue:                NewNeighborQueue(k, false),
	}
}

// Collect inserts (docID, similarity) and returns true when the entry
// was retained (either as a fresh insertion or by displacing the
// previous heap top).
func (c *TopKnnCollector) Collect(docID int, similarity float32) bool {
	return c.queue.InsertWithOverflow(int32(docID), similarity)
}

// MinCompetitiveSimilarity returns the lowest similarity the
// collector would still retain; -Inf until k results have been seen.
func (c *TopKnnCollector) MinCompetitiveSimilarity() float32 {
	if c.queue.Size() >= c.K() {
		return c.queue.TopScore()
	}
	return float32(math.Inf(-1))
}

// TopDocs drains the heap into a score-descending TopDocs. The
// collector should not be used after this call.
func (c *TopKnnCollector) TopDocs() *TopDocs {
	size := c.queue.Size()
	if size > c.K() {
		// Java's assert. Should never happen because InsertWithOverflow
		// keeps the heap bounded.
		panic("hnsw: TopKnnCollector collected more than k results")
	}
	scoreDocs := make([]*ScoreDoc, size)
	// Pop yields ascending scores in min-heap order, so we fill the
	// slice from the back to land on score-descending order.
	for i := 1; i <= size; i++ {
		topNode := c.queue.TopNode()
		topScore := c.queue.TopScore()
		scoreDocs[size-i] = NewScoreDoc(int(topNode), topScore)
		c.queue.Pop()
	}
	relation := EqualTo
	if c.EarlyTerminated() {
		relation = GreaterThanOrEqualTo
	}
	return NewTopDocs(NewTotalHits(c.VisitedCount(), relation), scoreDocs)
}

// NumCollected returns the current heap size, exposed for testing.
func (c *TopKnnCollector) NumCollected() int { return c.queue.Size() }

// Compile-time guard.
var _ KnnCollector = (*TopKnnCollector)(nil)
