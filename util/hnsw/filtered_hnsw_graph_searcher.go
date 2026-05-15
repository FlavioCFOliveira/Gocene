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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilteredHnswGraphSearcher is an HNSW searcher specialised for
// filtered queries. It is the Go port of
// org.apache.lucene.util.hnsw.FilteredHnswGraphSearcher (Lucene 10.4.0)
// and is inspired by the ACORN-1 algorithm
// (https://arxiv.org/abs/2403.04871): when only a small fraction of
// nodes pass the filter, the regular beam search wastes work scoring
// rejected candidates. The filtered searcher avoids that by
// partitioning each candidate's neighbours into "to score" (accepted)
// and "to explore" (rejected) buckets, then optionally expanding into
// the neighbours-of-neighbours of the rejected set until enough
// accepted candidates have been collected.
//
// Two parameters tune the expansion:
//
//   - maxExplorationMultiplier caps how many additional accepted
//     candidates the searcher is willing to score on a single
//     candidate expansion, expressed as a multiple of the candidate's
//     own neighbour count.
//   - minToScore is the lower bound on accepted candidates the search
//     keeps looking for even when the local fraction looks bad; it
//     grows as the filter becomes more selective.
//
// Both are derived from filterSize / graph.Size() at construction
// time. See the Java javadoc for the full rationale.
//
// In Java this class extends HnswGraphSearcher and overrides
// searchLevel; the Go translation embeds *HnswGraphSearcher so the
// FindBestEntryPoint descent is inherited verbatim, and provides its
// own SearchLevel. Both implement [AbstractHnswGraphSearcher].
//
// FilteredHnswGraphSearcher is NOT safe for concurrent use; allocate
// one searcher per goroutine.
type FilteredHnswGraphSearcher struct {
	*HnswGraphSearcher

	// maxExplorationMultiplier bounds the number of extra accepted
	// candidates to score on a single expansion, expressed as a
	// multiple of the candidate's own neighbour count.
	maxExplorationMultiplier int

	// minToScore is the minimum number of accepted candidates the
	// search keeps looking for even when the local fraction looks
	// bad. Grows as the filter gets more selective.
	minToScore int

	// toScore / toExplore are the per-iteration scratch queues used
	// by SearchLevel. They are allocated once at construction time
	// and cleared between iterations.
	toScore   *intArrayQueue
	toExplore *intArrayQueue
}

// expandedExplorationLambda is the threshold above which the searcher
// will fan out into neighbours-of-neighbours. When more than 10% of a
// candidate's neighbours fail the filter, the search expands; below
// 10% the local neighbourhood is presumed informative enough.
//
// Mirrors EXPANDED_EXPLORATION_LAMBDA in the Java reference.
const expandedExplorationLambda float32 = 0.10

// NewFilteredHnswGraphSearcher constructs a filtered searcher for k
// nearest neighbours on the supplied graph. filterSize is the
// precomputed count of nodes the acceptOrds filter accepts; acceptOrds
// must be non-nil and filterSize must lie in (0, graph.Size()). The
// constructor mirrors FilteredHnswGraphSearcher.create in Java.
//
// The graph's MaxConn must be known ([UnknownMaxConn] is rejected with
// a panic); the filtered code path requires it to size the
// per-iteration scratch queues.
func NewFilteredHnswGraphSearcher(
	k int,
	graph HnswGraph,
	filterSize int,
	acceptOrds util.Bits,
) *FilteredHnswGraphSearcher {
	if acceptOrds == nil {
		panic("hnsw: acceptOrds must not be nil for filtered search")
	}
	gSize := graphSize(graph)
	if filterSize <= 0 || filterSize >= gSize {
		panic("hnsw: filterSize must be in (0, graph.Size())")
	}
	maxConn := graph.MaxConn()
	if maxConn <= 0 {
		panic("hnsw: graph must report a known MaxConn for filtered search")
	}

	candidates := NewNeighborQueue(k, true)
	visited := filteredBitSet(filterSize, gSize, k)

	base := NewHnswGraphSearcher(candidates, visited)

	filterRatio := float32(filterSize) / float32(gSize)
	// Java: Math.round(Math.min(1 / filterRatio, graph.maxConn() / 2.0))
	maxExpFloat := math.Min(1.0/float64(filterRatio), float64(maxConn)/2.0)
	maxExp := int(math.Round(maxExpFloat))
	// As the filter gets exceptionally restrictive, we must spread
	// out the exploration. Java:
	//   Math.round(Math.min(Math.max(0, 1.0 / filterRatio - 2.0 * maxConn), maxConn))
	minScoreFloat := math.Min(
		math.Max(0, 1.0/float64(filterRatio)-2.0*float64(maxConn)),
		float64(maxConn),
	)
	minToScore := int(math.Round(minScoreFloat))

	queueCapacity := maxConn * 2 * maxExp
	if queueCapacity < 1 {
		// Defensive: maxExp can round to zero on huge filters where
		// the filtered path is barely useful. A minimum capacity
		// keeps the queues legal and the poll/add helpers honest.
		queueCapacity = 1
	}

	return &FilteredHnswGraphSearcher{
		HnswGraphSearcher:        base,
		maxExplorationMultiplier: maxExp,
		minToScore:               minToScore,
		toScore:                  newIntArrayQueue(queueCapacity),
		toExplore:                newIntArrayQueue(queueCapacity),
	}
}

// SearchLevel runs the filtered beam search on level 0. Mirrors the
// override of searchLevel in FilteredHnswGraphSearcher.
//
// The Java assertion is preserved: filtered search only applies to
// level 0; higher-level descent is the inherited regular path. A
// panic is raised on any other level so misuse fails loudly.
func (f *FilteredHnswGraphSearcher) SearchLevel(
	results KnnCollector,
	scorer RandomVectorScorer,
	level int,
	eps []int,
	graph HnswGraph,
	acceptOrds util.Bits,
) error {
	if level != 0 {
		panic("hnsw: FilteredHnswGraphSearcher only supports level 0")
	}

	size := graphSize(graph)

	// Java: prepareScratchState() — reset candidates + visited.
	// The base struct exposes prepareScratchState which also grows
	// bulkNodes/bulkScores; we want only the candidates/visited
	// reset here so we don't disturb the buffers used by the
	// scoreEntryPoints call below.
	f.candidates.Clear()
	if f.visited.Length() < size {
		// Reallocate visited if the graph is larger than the
		// initial sizing predicted. Mirrors the implicit growth
		// that Java relies on FixedBitSet/SparseFixedBitSet for.
		f.visited = createBitSet(results.K(), size)
	} else {
		f.visited.ClearAll()
	}
	if cap(f.bulkScores) < len(eps) {
		f.bulkScores = make([]float32, len(eps))
	} else {
		f.bulkScores = f.bulkScores[:len(eps)]
	}

	if results.EarlyTerminated() {
		return nil
	}
	if err := scoreEntryPoints(
		results, scorer, f.visited, eps, acceptOrds, f.candidates, f.bulkScores,
	); err != nil {
		return err
	}
	if results.EarlyTerminated() {
		return nil
	}

	// minAcceptedSimilarity is the floor the next candidate must
	// clear. +ulp (Math.nextUp) bumps the floor past the current top
	// so equal-scoring duplicates do not pollute the heap.
	minAcceptedSimilarity := mathNextUp32(results.MinCompetitiveSimilarity())

	for f.candidates.Size() > 0 && !results.EarlyTerminated() {
		topCandidateSimilarity := f.candidates.TopScore()
		if minAcceptedSimilarity > topCandidateSimilarity {
			break
		}
		topCandidateNode := int(f.candidates.Pop())

		if err := f.seek(f.HnswGraphSearcher, graph, level, topCandidateNode); err != nil {
			return err
		}
		neighborCount := graph.NeighborCount()
		f.toScore.clear()
		f.toExplore.clear()

		// Partition the candidate's neighbours into "to score"
		// (accepted) and "to explore" (rejected). The Java loop
		// also stops once toScore fills up.
		for !f.toScore.isFull() {
			friendOrd, err := f.next(f.HnswGraphSearcher, graph)
			if err != nil {
				return err
			}
			if friendOrd == util.NO_MORE_DOCS {
				break
			}
			if friendOrd >= size {
				panic("hnsw: graph returned out-of-range neighbour")
			}
			if f.visited.GetAndSet(friendOrd) {
				continue
			}
			if acceptOrds.Get(friendOrd) {
				f.toScore.add(friendOrd)
			} else {
				f.toExplore.add(friendOrd)
			}
		}

		// Adjust the score budget to the local fraction of filtered
		// neighbours, capped by maxExplorationMultiplier. The Java
		// formula uses 1 / (1 - filteredAmount) so a fully filtered
		// neighbourhood (filteredAmount == 1) falls back to the
		// multiplier cap.
		filteredAmount := float32(0)
		if neighborCount > 0 {
			filteredAmount = float32(f.toExplore.count()) / float32(neighborCount)
		}
		multiplier := float32(f.maxExplorationMultiplier)
		if filteredAmount < 1 {
			inverse := 1.0 / (1.0 - filteredAmount)
			if inverse < multiplier {
				multiplier = inverse
			}
		}
		maxToScoreCount := int(float32(neighborCount) * multiplier)
		maxAdditionalToExploreCount := f.toExplore.capacity() - 1
		totalExplored := f.toScore.count() + f.toExplore.count()

		if f.toScore.count() < maxToScoreCount && filteredAmount > expandedExplorationLambda {
			for {
				exploreFriend := f.toExplore.poll()
				if exploreFriend == util.NO_MORE_DOCS {
					break
				}
				if totalExplored >= maxAdditionalToExploreCount {
					break
				}
				if f.toScore.count() >= maxToScoreCount {
					break
				}
				if err := f.seek(f.HnswGraphSearcher, graph, level, exploreFriend); err != nil {
					return err
				}
				for f.toScore.count() < maxToScoreCount {
					friendOfAFriendOrd, err := f.next(f.HnswGraphSearcher, graph)
					if err != nil {
						return err
					}
					if friendOfAFriendOrd == util.NO_MORE_DOCS {
						break
					}
					if f.visited.GetAndSet(friendOfAFriendOrd) {
						continue
					}
					totalExplored++
					if acceptOrds.Get(friendOfAFriendOrd) {
						// toScore.count() < maxToScoreCount on every
						// iteration of this inner loop (guarded by the
						// for-condition above), and maxToScoreCount
						// <= toScore.capacity()/2 by construction; the
						// add cannot overflow.
						f.toScore.add(friendOfAFriendOrd)
					} else if totalExplored < maxAdditionalToExploreCount &&
						f.toScore.count() < f.minToScore {
						// totalExplored < maxAdditionalToExploreCount
						// == toExplore.capacity() - 1 here; combined
						// with the partition-phase invariant
						// toExplore.size <= maxConn, this add stays
						// strictly under capacity. Mirrors the Java
						// reference, which performs the same add
						// without an overflow guard.
						f.toExplore.add(friendOfAFriendOrd)
					}
				}
			}
		}

		// Score the accepted candidates and feed them into the
		// candidates heap / collector.
		toScoreCount := f.toScore.count()
		if cap(f.bulkScores) < toScoreCount {
			f.bulkScores = make([]float32, toScoreCount)
		}
		var maxScore float32 = float32(math.Inf(-1))
		if toScoreCount > 0 {
			scores := f.bulkScores[:toScoreCount]
			ms, err := scorer.BulkScore(f.toScore.nodes[f.toScore.upto:f.toScore.size], scores, toScoreCount)
			if err != nil {
				return err
			}
			maxScore = ms
		}
		results.IncVisitedCount(toScoreCount)
		if maxScore > minAcceptedSimilarity {
			for i := 0; i < toScoreCount; i++ {
				friendSimilarity := f.bulkScores[i]
				if friendSimilarity > minAcceptedSimilarity {
					ord := f.toScore.nodes[f.toScore.upto+i]
					f.candidates.Add(int32(ord), friendSimilarity)
					if results.Collect(ord, friendSimilarity) {
						minAcceptedSimilarity = mathNextUp32(results.MinCompetitiveSimilarity())
					}
				}
			}
		}
		// Mark all collected entries as consumed.
		f.toScore.upto = f.toScore.size
		if strategy := results.GetSearchStrategy(); strategy != nil {
			strategy.NextVectorsBlock()
		}
	}
	return nil
}

// filteredBitSet picks a bitset implementation sized for the expected
// visitation count of a filtered search. Mirrors
// FilteredHnswGraphSearcher.bitSet(long, int, int).
//
// The visitation count is estimated as log(graphSize) * k / fraction-
// passing-filter; if the estimate is < graphSize/128 a sparse bitset
// is used, otherwise a dense one.
func filteredBitSet(filterSize, graphSize, topK int) util.BitSet {
	if graphSize <= 0 {
		s, _ := util.NewSparseFixedBitSet(1)
		return s
	}
	percentFiltered := float64(filterSize) / float64(graphSize)
	totalOps := math.Log(float64(graphSize)) * float64(topK)
	approximateVisitation := int(totalOps / percentFiltered)
	return filteredBitSetPick(approximateVisitation, graphSize)
}

// filteredBitSetPick is the inner helper that maps an expected visit
// count to a sparse-vs-dense bitset. Mirrors FilteredHnswGraphSearcher.
// bitSet(int, int).
func filteredBitSetPick(expectedBits, totalBits int) util.BitSet {
	if totalBits <= 0 {
		s, _ := util.NewSparseFixedBitSet(1)
		return s
	}
	if expectedBits < (totalBits >> 7) {
		s, err := util.NewSparseFixedBitSet(totalBits)
		if err != nil {
			panic(err)
		}
		return s
	}
	f, err := util.NewFixedBitSet(totalBits)
	if err != nil {
		panic(err)
	}
	return f
}

// intArrayQueue is a small fixed-capacity FIFO of ints used as the
// per-iteration scratch buffer in the filtered search loop. It mirrors
// FilteredHnswGraphSearcher.IntArrayQueue in the Java reference.
//
// The queue is single-producer / single-consumer per iteration: callers
// fill it with add, then drain it with poll. Capacity is fixed at
// construction; add on a full queue panics (Java throws
// UnsupportedOperationException).
type intArrayQueue struct {
	nodes []int
	upto  int
	size  int
}

// newIntArrayQueue returns an empty queue with the given fixed
// capacity. capacity must be > 0; a non-positive value panics.
func newIntArrayQueue(capacity int) *intArrayQueue {
	if capacity <= 0 {
		panic("hnsw: intArrayQueue capacity must be > 0")
	}
	return &intArrayQueue{nodes: make([]int, capacity)}
}

// capacity returns the maximum number of elements the queue can hold.
func (q *intArrayQueue) capacity() int { return len(q.nodes) }

// count returns the number of unread elements (size - upto).
func (q *intArrayQueue) count() int { return q.size - q.upto }

// add appends node to the queue. Panics when the queue is at capacity,
// mirroring Java's UnsupportedOperationException.
func (q *intArrayQueue) add(node int) {
	if q.isFull() {
		panic("hnsw: intArrayQueue add past capacity")
	}
	q.nodes[q.size] = node
	q.size++
}

// isFull reports whether the queue is at capacity.
func (q *intArrayQueue) isFull() bool { return q.size == len(q.nodes) }

// poll removes and returns the next unread element, or
// util.NO_MORE_DOCS when the queue is empty. Mirrors Java's poll which
// returns DocIdSetIterator.NO_MORE_DOCS for the same condition.
func (q *intArrayQueue) poll() int {
	if q.upto == q.size {
		return util.NO_MORE_DOCS
	}
	v := q.nodes[q.upto]
	q.upto++
	return v
}

// clear resets the queue to empty. The backing slice is reused.
func (q *intArrayQueue) clear() {
	q.upto = 0
	q.size = 0
}

// Compile-time guard: FilteredHnswGraphSearcher must satisfy the
// abstract searcher contract.
var _ AbstractHnswGraphSearcher = (*FilteredHnswGraphSearcher)(nil)
