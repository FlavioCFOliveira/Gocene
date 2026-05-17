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

package knn

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// defaultMultiLeafGreediness is the default greediness of globally
// non-competitive search: (0, 1]. Matches Java's
// MultiLeafKnnCollector.DEFAULT_GREEDINESS.
const defaultMultiLeafGreediness float32 = 0.9

// defaultMultiLeafInterval is the default visited-count bit mask used
// to trigger periodic global-queue updates. Matches Java's
// MultiLeafKnnCollector.DEFAULT_INTERVAL (0xFF).
const defaultMultiLeafInterval = 0xff

// AbstractCollector is the structural interface a [MultiLeafKnnCollector]
// sub-collector must satisfy: the regular [hnsw.KnnCollector] surface
// plus NumCollected, which the Java reference reads from
// AbstractKnnCollector. The bundled [hnsw.TopKnnCollector] already
// satisfies this interface.
type AbstractCollector interface {
	hnsw.KnnCollector

	// NumCollected returns the number of (docID, score) pairs the
	// collector has retained so far. Mirrors
	// AbstractKnnCollector#numCollected.
	NumCollected() int
}

// MultiLeafKnnCollector is a [hnsw.KnnCollector] that exchanges the
// top collected results across segments through a shared global
// queue. It is the Go port of
// org.apache.lucene.search.knn.MultiLeafKnnCollector (Lucene 10.4.0).
//
// In Java the type extends KnnCollector.Decorator, delegating all
// KnnCollector methods to the wrapped subCollector except for the
// overridden Collect, MinCompetitiveSimilarity, and ToString. The Go
// port mirrors the same delegation by composition: the embedded
// AbstractCollector forwards every method that is not overridden in
// this file.
//
// Not safe for concurrent use; the cross-segment coordination point
// is the shared [hnsw.BlockingFloatHeap], which is itself thread-safe.
type MultiLeafKnnCollector struct {
	// Embed the sub-collector so all unoverridden KnnCollector
	// methods (EarlyTerminated, IncVisitedCount, VisitedCount,
	// VisitLimit, K, TopDocs, GetSearchStrategy) fall through.
	AbstractCollector

	// Mirrors the Java field of the same name. Retained as a
	// distinct reference so the overridden methods can call the
	// AbstractKnnCollector-specific NumCollected without an extra
	// interface assertion.
	subCollector AbstractCollector

	// globalSimilarityQueue is the shared min-heap of the top
	// similarities seen across all segments.
	globalSimilarityQueue *hnsw.BlockingFloatHeap

	// nonCompetitiveQueue holds the recent similarities that did
	// not beat the global threshold. Its capacity controls how
	// aggressively the collector aborts ("greediness").
	nonCompetitiveQueue *hnsw.FloatHeap

	// updatesQueue accumulates per-collect similarities for the
	// next periodic global-queue sync. Its capacity is k.
	updatesQueue *hnsw.FloatHeap

	// updatesScratch is the reusable ascending-sorted buffer
	// presented to globalSimilarityQueue.OfferMany. Capacity k.
	updatesScratch []float32

	// interval is the bit mask used to throttle global-queue syncs
	// (sync fires when visitedCount & interval == 0).
	interval int

	// kResultsCollected latches true once subCollector has retained
	// k entries. Mirrors Java's boolean field.
	kResultsCollected bool

	// cachedGlobalMinSim is the most recent value returned by
	// globalSimilarityQueue.OfferMany, used by MinCompetitiveSimilarity
	// to avoid taking the queue mutex on every call.
	cachedGlobalMinSim float32
}

// NewMultiLeafKnnCollector constructs a MultiLeafKnnCollector with
// Java's defaults (greediness = 0.9, interval = 0xff). Mirrors the
// three-arg Java constructor.
func NewMultiLeafKnnCollector(k int, globalSimilarityQueue *hnsw.BlockingFloatHeap, subCollector AbstractCollector) *MultiLeafKnnCollector {
	return NewMultiLeafKnnCollectorWithConfig(k, defaultMultiLeafGreediness, defaultMultiLeafInterval, globalSimilarityQueue, subCollector)
}

// NewMultiLeafKnnCollectorWithConfig constructs a
// MultiLeafKnnCollector with custom greediness and interval. Mirrors
// the five-arg Java constructor.
//
// greediness must be in [0, 1]; interval must be > 0. Either of these
// conditions failing panics with the same message Java would raise.
// subCollector must be non-nil.
func NewMultiLeafKnnCollectorWithConfig(k int, greediness float32, interval int, globalSimilarityQueue *hnsw.BlockingFloatHeap, subCollector AbstractCollector) *MultiLeafKnnCollector {
	if greediness < 0 || greediness > 1 {
		panic("knn: greediness must be in [0,1]")
	}
	if interval <= 0 {
		panic("knn: interval must be positive")
	}
	if subCollector == nil {
		panic("knn: subCollector must be non-nil")
	}
	// math.Round on float32 rounds half-up; Java's Math.round(float)
	// rounds half toward positive infinity. The expression below
	// matches that: floor(x + 0.5).
	nonCompCap := int(math.Floor(float64((1-greediness)*float32(k)) + 0.5))
	if nonCompCap < 1 {
		nonCompCap = 1
	}
	return &MultiLeafKnnCollector{
		AbstractCollector:     subCollector,
		subCollector:          subCollector,
		globalSimilarityQueue: globalSimilarityQueue,
		nonCompetitiveQueue:   hnsw.NewFloatHeap(nonCompCap),
		updatesQueue:          hnsw.NewFloatHeap(k),
		updatesScratch:        make([]float32, k),
		interval:              interval,
		cachedGlobalMinSim:    float32(math.Inf(-1)),
	}
}

// Collect mirrors Java's MultiLeafKnnCollector#collect.
//
// The local subCollector consumes the (docID, similarity) pair
// unconditionally; the similarity is also added to the
// nonCompetitiveQueue and updatesQueue. Once k results have been
// retained, the global queue is synchronised either on the very
// first transition (firstKResultsCollected) or once every interval
// visited vectors. The synchronisation uses an ascending-sorted
// snapshot of updatesQueue (drained via Poll) as required by
// BlockingFloatHeap#OfferMany; failing to sort first triggers the
// regression covered by TestGlobalScoreCoordination (GH#13462).
func (c *MultiLeafKnnCollector) Collect(docID int, similarity float32) bool {
	localSimUpdated := c.subCollector.Collect(docID, similarity)
	firstKResultsCollected := !c.kResultsCollected && c.subCollector.NumCollected() == c.subCollector.K()
	if firstKResultsCollected {
		c.kResultsCollected = true
	}
	c.updatesQueue.Offer(similarity)
	globalSimUpdated := c.nonCompetitiveQueue.Offer(similarity)

	if c.kResultsCollected {
		// Visited-count is a counter; the period check fires when
		// (visitedCount & interval) == 0, matching Java exactly.
		// On the first-k transition we always fire to seed the
		// global queue with the initial top-k.
		if firstKResultsCollected || (c.subCollector.VisitedCount()&int64(c.interval)) == 0 {
			length := c.updatesQueue.Size()
			if length > 0 {
				// Drain updatesQueue into updatesScratch in
				// ascending order. NewFloatHeap is a min-heap, so
				// successive Polls produce ascending values, the
				// precondition OfferMany requires.
				for i := 0; i < length; i++ {
					c.updatesScratch[i] = c.updatesQueue.Poll()
				}
				if c.updatesQueue.Size() != 0 {
					// Mirrors the Java assert; bug if reached.
					panic(fmt.Sprintf("knn: updatesQueue not drained, size=%d", c.updatesQueue.Size()))
				}
				c.cachedGlobalMinSim = c.globalSimilarityQueue.OfferMany(c.updatesScratch, length)
				globalSimUpdated = true
			}
		}
	}
	return localSimUpdated || globalSimUpdated
}

// MinCompetitiveSimilarity returns the lowest similarity that this
// collector would still consider competitive, considering both the
// per-leaf threshold and the global threshold.
//
// Until k results have been collected the threshold is -Inf so every
// candidate is accepted, matching Java.
func (c *MultiLeafKnnCollector) MinCompetitiveSimilarity() float32 {
	if !c.kResultsCollected {
		return float32(math.Inf(-1))
	}
	subThreshold := c.subCollector.MinCompetitiveSimilarity()
	nonCompPeek := c.nonCompetitiveQueue.Peek()
	globalCandidate := nonCompPeek
	if c.cachedGlobalMinSim < globalCandidate {
		globalCandidate = c.cachedGlobalMinSim
	}
	if subThreshold > globalCandidate {
		return subThreshold
	}
	return globalCandidate
}

// String mirrors Java's toString override.
func (c *MultiLeafKnnCollector) String() string {
	return fmt.Sprintf("MultiLeafKnnCollector[subCollector=%v]", c.subCollector)
}

// Compile-time guard.
var _ hnsw.KnnCollector = (*MultiLeafKnnCollector)(nil)
