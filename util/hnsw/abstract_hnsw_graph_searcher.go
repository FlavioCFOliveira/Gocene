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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// UnknownEntryPoint is the sentinel returned by FindBestEntryPoint /
// FindBestEntryPoints to signal that no entry point could be located
// (either the graph reports EntryNode == -1 or the visit budget was
// exhausted before a usable candidate was found). Mirrors the Java
// reference's AbstractHnswGraphSearcher.UNK_EP (-1).
const UnknownEntryPoint = -1

// AbstractHnswGraphSearcher is the base contract implemented by every
// concrete HNSW searcher (the regular [HnswGraphSearcher], the
// filtered searcher, and the seeded searcher). Mirrors the Java
// abstract class of the same name.
//
// Concrete searchers supply SearchLevel and FindBestEntryPoint; the
// shared Search method here orchestrates the two-phase top-level
// algorithm — first descend to the best entry point on level 0, then
// beam-search level 0 — and is the entry point invoked by callers.
type AbstractHnswGraphSearcher interface {
	// SearchLevel populates results with the nearest neighbours
	// found on level, starting from the entry points eps. eps
	// holds level-0 ordinals. acceptOrds is the filter applied at
	// collection time; nil accepts every node.
	SearchLevel(
		results KnnCollector,
		scorer RandomVectorScorer,
		level int,
		eps []int,
		graph HnswGraph,
		acceptOrds util.Bits,
	) error

	// FindBestEntryPoint descends the upper levels of graph from its
	// entry node to find the level-0 ordinal nearest to the query.
	// Returns a single-element slice containing the chosen entry
	// point, or [UnknownEntryPoint] when the graph has no entry node
	// or the collector's visit limit was exhausted during the
	// descent. The slice-valued return mirrors the Java contract
	// (Seeded searchers may return many entry points).
	FindBestEntryPoint(
		scorer RandomVectorScorer,
		graph HnswGraph,
		collector KnnCollector,
	) ([]int, error)
}

// Search orchestrates a full HNSW search by descending the upper
// levels to find the best entry point, then beam-searching level 0
// with that entry point. Mirrors the Java
// AbstractHnswGraphSearcher#search(KnnCollector, RandomVectorScorer,
// HnswGraph, Bits).
//
// Exposed as a free function so concrete searchers can share the
// orchestration without having to embed a base struct (Go's interface
// model preempts the Java single-inheritance shape).
func Search(
	s AbstractHnswGraphSearcher,
	results KnnCollector,
	scorer RandomVectorScorer,
	graph HnswGraph,
	acceptOrds util.Bits,
) error {
	eps, err := s.FindBestEntryPoint(scorer, graph, results)
	if err != nil {
		return err
	}
	if len(eps) == 0 {
		// Defensive: the Java assert eps != null && eps.length > 0
		// becomes a no-op here. An empty slice would be a bug in a
		// concrete implementation, so treat it as "no entry" rather
		// than panicking.
		return nil
	}
	if eps[0] == UnknownEntryPoint {
		return nil
	}
	return s.SearchLevel(results, scorer, 0, eps, graph, acceptOrds)
}

// scoreEntryPoints seeds the candidate heap and the results collector
// with eps' scores and marks every ep visited. Mirrors
// AbstractHnswGraphSearcher#scoreEntryPoints — exposed as a free
// helper since multiple concrete searchers reuse it.
func scoreEntryPoints(
	results KnnCollector,
	scorer RandomVectorScorer,
	visited util.BitSet,
	eps []int,
	acceptOrds util.Bits,
	candidates *NeighborQueue,
	scores []float32,
) error {
	if len(eps) == 0 {
		panic("hnsw: scoreEntryPoints requires at least one entry point")
	}
	if len(scores) < len(eps) {
		panic("hnsw: scoreEntryPoints scores buffer too small")
	}
	if _, err := scorer.BulkScore(eps, scores, len(eps)); err != nil {
		return err
	}
	results.IncVisitedCount(len(eps))
	for i, ep := range eps {
		score := scores[i]
		visited.Set(ep)
		candidates.Add(int32(ep), score)
		if acceptOrds == nil || acceptOrds.Get(ep) {
			results.Collect(ep, score)
		}
	}
	return nil
}

// graphSize returns the size of graph as the maxNodeId + 1 — the
// upper-bound ordinal the searcher must accommodate. Mirrors Java's
// HnswGraphSearcher.getGraphSize.
func graphSize(graph HnswGraph) int {
	return MaxNodeID(graph) + 1
}
