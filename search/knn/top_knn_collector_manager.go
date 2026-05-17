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
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// TopKnnCollectorManager is the default [KnnCollectorManager]: it
// produces [hnsw.TopKnnCollector] instances for the configured k.
//
// Mirrors org.apache.lucene.search.knn.TopKnnCollectorManager.
//
// The Java constructor accepts an IndexSearcher that the canonical
// implementation never reads; it is preserved as the second argument
// so subclasses can use it. The Go port accepts the same parameter
// as any: the canonical implementation ignores it, but it is part
// of the public surface so call-sites remain Java-compatible. Pass
// nil when no searcher is available.
type TopKnnCollectorManager struct {
	k int
	// searcher is retained but unused by the canonical implementation.
	// It is kept on the struct so subclasses (and future Gocene call
	// sites) can read it back via the Searcher accessor.
	searcher any
}

// NewTopKnnCollectorManager constructs a TopKnnCollectorManager
// targeting k results. searcher is stored for parity with the Java
// constructor (IndexSearcher) but is not consulted by the canonical
// implementation; pass nil if no searcher is available.
func NewTopKnnCollectorManager(k int, searcher any) *TopKnnCollectorManager {
	return &TopKnnCollectorManager{k: k, searcher: searcher}
}

// K returns the configured neighbour count.
func (m *TopKnnCollectorManager) K() int { return m.k }

// Searcher returns the IndexSearcher reference supplied at
// construction. May be nil.
func (m *TopKnnCollectorManager) Searcher() any { return m.searcher }

// NewCollector returns a fresh [hnsw.TopKnnCollector] bound to the
// configured k, the supplied visited limit, and the optional search
// strategy. The leaf context parameter is ignored, matching the Java
// reference.
func (m *TopKnnCollectorManager) NewCollector(visitedLimit int, searchStrategy KnnSearchStrategy, _ *index.LeafReaderContext) (hnsw.KnnCollector, error) {
	return hnsw.NewTopKnnCollector(m.k, visitedLimit, asHnswStrategy(searchStrategy)), nil
}

// NewOptimisticCollector returns a fresh [hnsw.TopKnnCollector]
// scaled to the supplied k. Mirrors the Java override.
func (m *TopKnnCollectorManager) NewOptimisticCollector(visitedLimit int, searchStrategy KnnSearchStrategy, _ *index.LeafReaderContext, k int) (hnsw.KnnCollector, error) {
	return hnsw.NewTopKnnCollector(k, visitedLimit, asHnswStrategy(searchStrategy)), nil
}

// IsOptimistic returns true: TopKnnCollectorManager always exposes
// the optimistic collector path.
func (m *TopKnnCollectorManager) IsOptimistic() bool { return true }

// asHnswStrategy adapts a knn.KnnSearchStrategy to the narrower
// hnsw.KnnSearchStrategy interface accepted by the hnsw collector
// constructors. Returns nil for a nil input.
//
// Because Hnsw and Seeded both satisfy hnsw.KnnSearchStrategy (the
// compile-time guards in knn_search_strategy.go assert this), this
// is a straightforward interface narrowing. The branch on nil keeps
// the typed-nil pitfall at bay: passing a nil knn.KnnSearchStrategy
// must produce a nil hnsw.KnnSearchStrategy.
func asHnswStrategy(s KnnSearchStrategy) hnsw.KnnSearchStrategy {
	if s == nil {
		return nil
	}
	if h, ok := s.(hnsw.KnnSearchStrategy); ok {
		return h
	}
	// Should be unreachable: KnnSearchStrategy implementations in
	// this package are required by var-guards to also satisfy
	// hnsw.KnnSearchStrategy. Fall back to a thin wrapper.
	return strategyAdapter{wrapped: s}
}

// strategyAdapter narrows an arbitrary knn.KnnSearchStrategy down to
// the hnsw.KnnSearchStrategy interface. Only ever instantiated when
// asHnswStrategy receives a strategy implemented outside this package
// that does not directly satisfy hnsw.KnnSearchStrategy.
type strategyAdapter struct{ wrapped KnnSearchStrategy }

// NextVectorsBlock forwards to the wrapped strategy.
func (s strategyAdapter) NextVectorsBlock() { s.wrapped.NextVectorsBlock() }

// Compile-time guards.
var (
	_ KnnCollectorManager           = (*TopKnnCollectorManager)(nil)
	_ OptimisticKnnCollectorManager = (*TopKnnCollectorManager)(nil)
)
