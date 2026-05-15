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

// HnswGraphSearcher is the default HNSW graph searcher. It tracks
// scratch state — a candidates min-heap, a visited bitset, and bulk
// scoring buffers — that is allocated once per searcher and reused on
// every SearchLevel call, so a single searcher can be used to satisfy
// many queries without paying the allocation cost twice.
//
// Port of org.apache.lucene.util.hnsw.HnswGraphSearcher (Lucene
// 10.4.0). HnswGraphSearcher is the concrete subclass that drives
// regular (non-filtered, non-seeded) HNSW search; the filtered and
// seeded variants are deferred — see [FilteredHnswGraphSearcher] /
// [SeededHnswGraphSearcher] TODOs in the package roadmap.
//
// HnswGraphSearcher is NOT safe for concurrent use: the candidates
// queue, the visited bitset, and the bulk buffers all carry per-call
// state. Concurrent search must use one searcher per goroutine. The
// [OnHeapHnswGraphSearcher] thread-safe wrapper is the concurrent
// counterpart for OnHeapHnswGraph.
type HnswGraphSearcher struct {
	// candidates is the min-heap of (node, score) pairs the caller
	// has not yet expanded. It is bounded by the search beam width
	// at construction time but grows beyond it via Add (Java's
	// unbounded heap path).
	candidates *NeighborQueue

	// visited tracks every node that has been touched on the
	// current search. The bitset may be replaced by a larger one
	// when the graph turns out to exceed the configured size; the
	// pointer is therefore mutable.
	visited util.BitSet

	// bulkNodes / bulkScores are scratch buffers used by both the
	// findBestEntryPoint descent and the level-0 beam search. They
	// are grown lazily to the maximum size required so far.
	bulkNodes  []int
	bulkScores []float32

	// seek / next are the policy functions used to walk the graph.
	// They default to graph.SeekLevel / graph.NextNeighbor, which
	// reuse the graph's intrinsic cursor; concurrent searchers on
	// OnHeapHnswGraph install thread-local replacements via
	// NewOnHeapHnswGraphSearcher so the per-graph cursor is left
	// untouched.
	seek seekFunc
	next nextFunc

	// userData lets seek/next policy functions carry their own
	// thread-local state (e.g. the OnHeapHnswGraph cursor) without
	// growing the public surface of the searcher.
	userData any
}

// seekFunc is the policy used by [HnswGraphSearcher] to position the
// neighbour iterator on (level, target). The default implementation
// delegates to graph.SeekLevel; the OnHeapHnswGraph thread-safe
// variant overrides this to maintain a private cursor.
type seekFunc func(s *HnswGraphSearcher, graph HnswGraph, level, target int) error

// nextFunc is the policy used by [HnswGraphSearcher] to advance the
// neighbour iterator. Mirrors graphNextNeighbor in the Java reference.
type nextFunc func(s *HnswGraphSearcher, graph HnswGraph) (int, error)

// defaultSeek delegates to graph.SeekLevel; the searcher inherits the
// graph's intrinsic cursor. This is the right choice for read-only
// graphs (codec-backed) and for any non-OnHeap implementation.
func defaultSeek(_ *HnswGraphSearcher, graph HnswGraph, level, target int) error {
	return graph.SeekLevel(level, target)
}

// defaultNext delegates to graph.NextNeighbor.
func defaultNext(_ *HnswGraphSearcher, graph HnswGraph) (int, error) {
	return graph.NextNeighbor()
}

// NewHnswGraphSearcher constructs a searcher backed by the supplied
// candidate queue and visited bitset. Most callers should prefer
// [SearchWithOnHeapGraph] or [SearchWithCollector], which build the
// scratch state for them.
func NewHnswGraphSearcher(candidates *NeighborQueue, visited util.BitSet) *HnswGraphSearcher {
	return &HnswGraphSearcher{
		candidates: candidates,
		visited:    visited,
		seek:       defaultSeek,
		next:       defaultNext,
	}
}

// ExpectedVisitedNodes returns a heuristic upper bound on the number
// of nodes a kNN search will touch in a graph of graphSize nodes.
// Mirrors HnswGraphSearcher#expectedVisitedNodes; HNSW is roughly
// logarithmic in the graph size, so the estimate is k * log(graphSize).
func ExpectedVisitedNodes(k, graphSize int) int {
	if graphSize <= 0 {
		return 0
	}
	return int(math.Log(float64(graphSize)) * float64(k))
}

// createBitSet picks a bitset implementation appropriate for the
// expected visit count: a sparse bitset when the search is expected
// to touch fewer than graphSize/128 nodes, a dense bitset otherwise.
// Mirrors HnswGraphSearcher.createBitSet.
func createBitSet(k, gSize int) util.BitSet {
	if gSize <= 0 {
		// Defensive: the constructors all require length > 0 to
		// avoid empty-bitset panics; treat empty graphs as a
		// trivial sparse bitset so callers can short-circuit later.
		s, _ := util.NewSparseFixedBitSet(1)
		return s
	}
	if ExpectedVisitedNodes(k, gSize) < (gSize >> 7) {
		s, err := util.NewSparseFixedBitSet(gSize)
		if err != nil {
			// Constructor only errors when length <= 0; the gSize
			// > 0 guard above precludes that branch.
			panic(err)
		}
		return s
	}
	f, err := util.NewFixedBitSet(gSize)
	if err != nil {
		panic(err)
	}
	return f
}

// SearchWithCollector is the public entry point that mirrors
// HnswGraphSearcher.search(scorer, knnCollector, graph, acceptOrds).
// It constructs a fresh searcher sized for the collector's k, then
// orchestrates findBestEntryPoint → searchLevel.
//
// This signature delegates the filtered-vs-regular and seeded-vs-base
// dispatch to a single decision point; the filtered/seeded code paths
// are not yet ported, so a non-nil acceptOrds always takes the regular
// path with collection-time filtering.
//
// TODO(rmp): re-introduce the filtered/seeded dispatch when
// FilteredHnswGraphSearcher and SeededHnswGraphSearcher land.
func SearchWithCollector(
	scorer RandomVectorScorer,
	collector KnnCollector,
	graph HnswGraph,
	acceptOrds util.Bits,
) error {
	return SearchWithCollectorAndFilter(scorer, collector, graph, acceptOrds, 0)
}

// SearchWithCollectorAndFilter mirrors the five-arg
// HnswGraphSearcher.search overload that also receives a precomputed
// filteredDocCount. The filteredDocCount is currently unused (the
// filtered-search path is deferred); accepted to keep the call shape
// stable for downstream callers.
func SearchWithCollectorAndFilter(
	scorer RandomVectorScorer,
	collector KnnCollector,
	graph HnswGraph,
	acceptOrds util.Bits,
	filteredDocCount int,
) error {
	if filteredDocCount < 0 || filteredDocCount > graph.Size() {
		panic("hnsw: filteredDocCount must be in [0, graph.Size()]")
	}
	// TODO(rmp): once FilteredHnswGraphSearcher and
	// SeededHnswGraphSearcher land, dispatch here on the collector's
	// search strategy (HnswStrategy.UseFilteredSearch /
	// Seeded.numberOfEntryPoints). For now the regular searcher
	// handles every case correctly; the strategy is read by the
	// SearchLevel loop via NextVectorsBlock callbacks.
	searcher := NewHnswGraphSearcher(
		NewNeighborQueue(collector.K(), true),
		createBitSet(collector.K(), graphSize(graph)),
	)
	return Search(searcher, collector, scorer, graph, acceptOrds)
}

// SearchWithOnHeapGraph mirrors the static
// HnswGraphSearcher.search(scorer, topK, graph, acceptOrds,
// visitedLimit) overload that returns a fresh KnnCollector. It
// constructs a [TopKnnCollector] with the supplied visit budget,
// runs the search through a thread-safe [OnHeapHnswGraphSearcher],
// and returns the collector so callers can drain TopDocs.
//
// This entry point exists because OnHeapHnswGraph mutates the seek /
// neighbor cursors during traversal; the wrapper isolates that
// mutation so concurrent searchers on the same graph see consistent
// neighbour lists.
func SearchWithOnHeapGraph(
	scorer RandomVectorScorer,
	topK int,
	graph *OnHeapHnswGraph,
	acceptOrds util.Bits,
	visitedLimit int,
) (KnnCollector, error) {
	collector := NewTopKnnCollector(topK, visitedLimit, nil)
	gs := graphSize(graph)
	if gs <= 0 {
		// Empty graph: short-circuit to avoid SparseFixedBitSet's
		// "length needs to be >= 1" constructor error.
		return collector, nil
	}
	sparse, err := util.NewSparseFixedBitSet(gs)
	if err != nil {
		return nil, err
	}
	searcher := NewOnHeapHnswGraphSearcher(
		NewNeighborQueue(topK, true), sparse,
	)
	if err := Search(searcher, collector, scorer, graph, acceptOrds); err != nil {
		return nil, err
	}
	return collector, nil
}

// SearchLevel runs a beam search over the supplied level, seeded at
// eps. Results are appended to the collector. Mirrors the package-
// private searchLevel method on HnswGraphSearcher.
//
// HnswGraphSearcher implements AbstractHnswGraphSearcher; this is one
// of the two methods Java's abstract class declares.
func (s *HnswGraphSearcher) SearchLevel(
	results KnnCollector,
	scorer RandomVectorScorer,
	level int,
	eps []int,
	graph HnswGraph,
	acceptOrds util.Bits,
) error {
	size := graphSize(graph)

	s.prepareScratchState(size, graph.MaxConn()*2)
	if cap(s.bulkScores) < len(eps) {
		s.bulkScores = make([]float32, len(eps))
	} else {
		s.bulkScores = s.bulkScores[:len(eps)]
	}
	if results.EarlyTerminated() {
		return nil
	}
	if err := scoreEntryPoints(
		results, scorer, s.visited, eps, acceptOrds, s.candidates, s.bulkScores,
	); err != nil {
		return err
	}
	if results.EarlyTerminated() {
		return nil
	}

	// minAcceptedSimilarity is the floor the next candidate must
	// clear before being considered. The +ulp (Math.nextUp) bumps
	// the floor past the current top so equally-scoring duplicates
	// do not pollute the heap unbounded — see shouldExploreMinSim
	// below for the one-shot escape hatch.
	minAcceptedSimilarity := mathNextUp32(results.MinCompetitiveSimilarity())
	shouldExploreMinSim := true

	for s.candidates.Size() > 0 && !results.EarlyTerminated() {
		topCandidateSimilarity := s.candidates.TopScore()
		if topCandidateSimilarity < minAcceptedSimilarity {
			// Allow one exploration when the candidate is exactly
			// one ulp below the floor; beyond that, exit.
			if shouldExploreMinSim &&
				mathNextUp32(topCandidateSimilarity) == minAcceptedSimilarity {
				shouldExploreMinSim = false
			} else {
				break
			}
		}

		topCandidateNode := int(s.candidates.Pop())
		if err := s.seek(s, graph, level, topCandidateNode); err != nil {
			return err
		}

		numNodes := 0
		for {
			friendOrd, err := s.next(s, graph)
			if err != nil {
				return err
			}
			if friendOrd == util.NO_MORE_DOCS {
				break
			}
			if friendOrd >= size {
				panic("hnsw: graph returned out-of-range neighbour")
			}
			if s.visited.GetAndSet(friendOrd) {
				continue
			}
			if results.EarlyTerminated() {
				break
			}
			if numNodes >= len(s.bulkNodes) {
				// Defensive: graphs may report MaxConn() as
				// UnknownMaxConn; prepareScratchState then sizes
				// bulk buffers from a fallback. Grow if a seek
				// surfaces more neighbours than expected.
				s.bulkNodes = append(s.bulkNodes, friendOrd)
				numNodes++
				continue
			}
			s.bulkNodes[numNodes] = friendOrd
			numNodes++
		}

		// Trim numNodes so the bulk score below stays within the
		// visit budget. Java casts to int via the Math.min long
		// path; the Go arithmetic is identical because both sides
		// fit in int.
		remainingBudget := results.VisitLimit() - results.VisitedCount()
		if int64(numNodes) > remainingBudget {
			if remainingBudget < 0 {
				numNodes = 0
			} else {
				numNodes = int(remainingBudget)
			}
		}
		results.IncVisitedCount(numNodes)

		if numNodes > 0 {
			if cap(s.bulkScores) < numNodes {
				s.bulkScores = make([]float32, numNodes)
			}
			scores := s.bulkScores[:numNodes]
			maxScore, err := scorer.BulkScore(s.bulkNodes[:numNodes], scores, numNodes)
			if err != nil {
				return err
			}
			if maxScore > results.MinCompetitiveSimilarity() {
				for i := 0; i < numNodes; i++ {
					node := s.bulkNodes[i]
					score := scores[i]
					if score >= minAcceptedSimilarity {
						s.candidates.Add(int32(node), score)
						if acceptOrds == nil || acceptOrds.Get(node) {
							if results.Collect(node, score) {
								oldMin := minAcceptedSimilarity
								minAcceptedSimilarity =
									mathNextUp32(results.MinCompetitiveSimilarity())
								if minAcceptedSimilarity > oldMin {
									shouldExploreMinSim = true
								}
							}
						}
					}
				}
			}
		}
		if strategy := results.GetSearchStrategy(); strategy != nil {
			strategy.NextVectorsBlock()
		}
	}
	return nil
}

// FindBestEntryPoint descends the upper levels of graph greedily
// from its entry node, returning a single-element slice with the
// best-scoring level-0 ordinal. Returns [UnknownEntryPoint] when no
// entry point exists or the collector's visit budget is exhausted
// during the descent.
//
// HnswGraphSearcher implements AbstractHnswGraphSearcher; this is the
// second of the two methods Java's abstract class declares.
func (s *HnswGraphSearcher) FindBestEntryPoint(
	scorer RandomVectorScorer,
	graph HnswGraph,
	collector KnnCollector,
) ([]int, error) {
	currentEp, err := graph.EntryNode()
	if err != nil {
		return nil, err
	}
	numLevels, err := graph.NumLevels()
	if err != nil {
		return nil, err
	}
	if currentEp == -1 || numLevels == 1 {
		return []int{currentEp}, nil
	}

	size := graphSize(graph)
	s.prepareScratchState(size, graph.MaxConn()*2)

	currentScore, err := scorer.Score(currentEp)
	if err != nil {
		return nil, err
	}
	collector.IncVisitedCount(1)

	for level := numLevels - 1; level >= 1; level-- {
		foundBetter := true
		s.visited.Set(currentEp)
		for foundBetter {
			foundBetter = false
			if err := s.seek(s, graph, level, currentEp); err != nil {
				return nil, err
			}

			numNodes := 0
			for {
				friendOrd, err := s.next(s, graph)
				if err != nil {
					return nil, err
				}
				if friendOrd == util.NO_MORE_DOCS {
					break
				}
				if friendOrd >= size {
					panic("hnsw: graph returned out-of-range neighbour")
				}
				if s.visited.GetAndSet(friendOrd) {
					continue
				}
				if collector.EarlyTerminated() {
					return []int{UnknownEntryPoint}, nil
				}
				if numNodes >= len(s.bulkNodes) {
					s.bulkNodes = append(s.bulkNodes, friendOrd)
					numNodes++
					continue
				}
				s.bulkNodes[numNodes] = friendOrd
				numNodes++
			}

			maxScore := float32(math.Inf(-1))
			if numNodes > 0 {
				// prepareScratchState ensured s.bulkScores is sized
				// for at least maxConn*2 entries, but defensively
				// grow the buffer if a graph somehow surfaces more
				// neighbours than expected on a given seek.
				if cap(s.bulkScores) < numNodes {
					s.bulkScores = make([]float32, numNodes)
				}
				scores := s.bulkScores[:numNodes]
				m, err := scorer.BulkScore(s.bulkNodes[:numNodes], scores, numNodes)
				if err != nil {
					return nil, err
				}
				maxScore = m
			}
			collector.IncVisitedCount(numNodes)
			if maxScore > currentScore {
				for i := 0; i < numNodes; i++ {
					score := s.bulkScores[i]
					if score > currentScore {
						currentScore = score
						currentEp = s.bulkNodes[i]
						foundBetter = true
					}
				}
			}
		}
	}
	if collector.EarlyTerminated() {
		return []int{UnknownEntryPoint}, nil
	}
	return []int{currentEp}, nil
}

// prepareScratchState resets the candidates queue and the visited
// bitset, growing the latter when capacity falls short of the
// requested graph size. The bulk buffers are grown to at least
// bulkScoreSize entries — typically maxConn*2 for the level-0 search
// and the same for the upper-level descent.
//
// When the graph reports MaxConn == UnknownMaxConn the supplied
// bulkScoreSize will be negative; in that case a small fallback
// capacity is used and the buffers grow on demand inside the search
// loops.
func (s *HnswGraphSearcher) prepareScratchState(capacity, bulkScoreSize int) {
	if bulkScoreSize <= 0 {
		// Fallback capacity for graphs with unknown maxConn; the
		// search loops grow these buffers further when needed.
		bulkScoreSize = 16
	}
	s.candidates.Clear()
	switch v := s.visited.(type) {
	case *util.FixedBitSet:
		if v.Length() < capacity {
			next, err := util.NewFixedBitSet(capacity)
			if err != nil {
				panic(err)
			}
			s.visited = next
		} else {
			v.ClearAll()
		}
	default:
		// SparseFixedBitSet (or any custom BitSet); reuse if it has
		// enough capacity, otherwise allocate a new sparse bitset of
		// the requested size.
		if s.visited.Length() < capacity {
			next, err := util.NewSparseFixedBitSet(capacity)
			if err != nil {
				panic(err)
			}
			s.visited = next
		} else {
			s.visited.ClearAll()
		}
	}
	if cap(s.bulkNodes) < bulkScoreSize {
		s.bulkNodes = make([]int, bulkScoreSize)
	} else {
		s.bulkNodes = s.bulkNodes[:bulkScoreSize]
	}
	if cap(s.bulkScores) < bulkScoreSize {
		s.bulkScores = make([]float32, bulkScoreSize)
	} else {
		s.bulkScores = s.bulkScores[:bulkScoreSize]
	}
}

// mathNextUp32 returns the smallest float32 strictly greater than v.
// Mirrors Java's Math.nextUp(float). NaN propagates and ±Infinity is
// handled per IEEE-754.
func mathNextUp32(v float32) float32 {
	return float32(math.Nextafter(float64(v), math.Inf(1)))
}

// Compile-time guard.
var _ AbstractHnswGraphSearcher = (*HnswGraphSearcher)(nil)
