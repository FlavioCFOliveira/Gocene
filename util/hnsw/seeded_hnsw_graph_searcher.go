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
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// seededHnswGraphSearcher is an [AbstractHnswGraphSearcher] that
// short-circuits the upper-level descent and uses a caller-supplied
// list of level-0 ordinals as entry points. The level-0 beam search
// is delegated to an inner searcher (regular or filtered). Mirrors
// org.apache.lucene.util.hnsw.SeededHnswGraphSearcher from Lucene
// 10.4.0.
//
// The type is package-private in the Java reference (final class with
// package-private constructor and factory); we keep the same scope in
// Go so the only legal construction path mirrors Java's
// fromEntryPoints factory invoked by [HnswGraphSearcher.Search].
//
// Not safe for concurrent use; allocate one searcher per goroutine.
type seededHnswGraphSearcher struct {
	delegate AbstractHnswGraphSearcher
	seedOrds []int
}

// errSeededNoEntryPoints is returned by [seededFromEntryPoints] when
// the caller asks for zero or negative entry points. Mirrors the
// Java IllegalArgumentException("The number of entry points must be
// > 0").
var errSeededNoEntryPoints = errors.New(
	"hnsw: the number of entry points must be > 0",
)

// errSeededTooFewEntryPoints is returned by [seededFromEntryPoints]
// when the supplied iterator yields fewer ordinals than the caller
// requested. Mirrors the Java IllegalArgumentException("The number of
// entry points provided is less than the number of entry points
// requested").
var errSeededTooFewEntryPoints = errors.New(
	"hnsw: the number of entry points provided is less than the number of entry points requested",
)

// seededFromEntryPoints drains numEps ordinals from eps and wraps
// delegate in a seeded searcher anchored on them. graphSize is the
// upper-bound ordinal used as a sanity check on each entry point;
// any value at or beyond it is treated as a programming error and
// panics, mirroring the Java assert.
//
// Returns [errSeededNoEntryPoints] when numEps <= 0 and
// [errSeededTooFewEntryPoints] when eps is exhausted before numEps
// ordinals have been collected.
func seededFromEntryPoints(
	delegate AbstractHnswGraphSearcher,
	numEps int,
	eps util.DocIdSetIterator,
	graphSize int,
) (*seededHnswGraphSearcher, error) {
	if numEps <= 0 {
		return nil, errSeededNoEntryPoints
	}
	entryPoints := make([]int, numEps)
	for idx := 0; idx < numEps; idx++ {
		ord, err := eps.NextDoc()
		if err != nil {
			return nil, fmt.Errorf("hnsw: seeded entry-point iterator failed: %w", err)
		}
		if ord == util.NO_MORE_DOCS {
			return nil, errSeededTooFewEntryPoints
		}
		if ord >= graphSize {
			// Mirrors the Java `assert entryPointOrdInt < graphSize`.
			// An out-of-range ordinal is a programmer bug in the
			// upstream search strategy, not a recoverable runtime
			// condition; fail loudly.
			panic(fmt.Sprintf(
				"hnsw: seeded entry point %d out of range (graph size %d)",
				ord, graphSize,
			))
		}
		entryPoints[idx] = ord
	}
	return newSeededHnswGraphSearcher(delegate, entryPoints), nil
}

// newSeededHnswGraphSearcher wraps delegate so the next Search call
// uses seedOrds as the level-0 entry points instead of descending
// from the graph's top-level entry node. seedOrds is retained by
// reference and must not be mutated by the caller after this call.
func newSeededHnswGraphSearcher(
	delegate AbstractHnswGraphSearcher,
	seedOrds []int,
) *seededHnswGraphSearcher {
	return &seededHnswGraphSearcher{
		delegate: delegate,
		seedOrds: seedOrds,
	}
}

// SearchLevel forwards to the wrapped delegate verbatim. The seeded
// wrapper only customises the entry-point selection; the per-level
// beam search itself is unchanged.
func (s *seededHnswGraphSearcher) SearchLevel(
	results KnnCollector,
	scorer RandomVectorScorer,
	level int,
	eps []int,
	graph HnswGraph,
	acceptOrds util.Bits,
) error {
	return s.delegate.SearchLevel(results, scorer, level, eps, graph, acceptOrds)
}

// FindBestEntryPoint returns the caller-supplied seed ordinals
// directly. Unlike the regular path it does not descend upper
// levels, does not consult collector, and never returns
// [UnknownEntryPoint]. The returned slice is the searcher's own
// backing storage; callers must not mutate it.
func (s *seededHnswGraphSearcher) FindBestEntryPoint(
	scorer RandomVectorScorer,
	graph HnswGraph,
	collector KnnCollector,
) ([]int, error) {
	return s.seedOrds, nil
}

// Compile-time guard: seededHnswGraphSearcher must satisfy the
// abstract searcher contract.
var _ AbstractHnswGraphSearcher = (*seededHnswGraphSearcher)(nil)
