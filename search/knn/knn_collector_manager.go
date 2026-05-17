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

// KnnCollectorManager creates [hnsw.KnnCollector] instances. Useful
// to create collectors that share global state across leaves, such
// as a global queue of results collected so far.
//
// Mirrors org.apache.lucene.search.knn.KnnCollectorManager.
//
// In Java the contract is split into a required NewCollector and a
// pair of default methods (NewOptimisticCollector / IsOptimistic).
// Go has no concept of default interface methods, so the optimistic
// hook is exposed as a separate optional interface:
// [OptimisticKnnCollectorManager]. The convenience helpers
// [NewOptimisticCollector] and [IsOptimistic] perform the optional
// dispatch with the Java-default fallback so callers can mirror the
// Java code path without a runtime type-switch at every call site.
type KnnCollectorManager interface {
	// NewCollector returns a new KnnCollector configured for the
	// supplied leaf, visit budget, and (optional) search strategy.
	//
	// visitedLimit is the maximum number of nodes the search is
	// allowed to visit. searchStrategy may be nil. context is the
	// leaf reader context the collector will be wired to.
	NewCollector(visitedLimit int, searchStrategy KnnSearchStrategy, context *index.LeafReaderContext) (hnsw.KnnCollector, error)
}

// OptimisticKnnCollectorManager extends [KnnCollectorManager] with
// the optional optimistic-collector hook used by per-segment k
// rescaling. Managers that do not need this hook simply omit it; the
// helper [NewOptimisticCollector] falls back to Java's default
// (return nil) for such managers.
//
// Mirrors KnnCollectorManager#newOptimisticCollector +
// KnnCollectorManager#isOptimistic.
type OptimisticKnnCollectorManager interface {
	KnnCollectorManager

	// NewOptimisticCollector returns a collector scaled to the
	// per-leaf k value supplied. May return nil to signal the
	// caller should fall back to the regular NewCollector path.
	NewOptimisticCollector(visitedLimit int, searchStrategy KnnSearchStrategy, context *index.LeafReaderContext, k int) (hnsw.KnnCollector, error)

	// IsOptimistic reports whether this manager exposes an
	// optimistic path.
	IsOptimistic() bool
}

// NewOptimisticCollector dispatches to the optimistic-collector path
// when manager implements [OptimisticKnnCollectorManager] and
// otherwise returns (nil, nil), matching the Java default of
// KnnCollectorManager#newOptimisticCollector.
func NewOptimisticCollector(manager KnnCollectorManager, visitedLimit int, searchStrategy KnnSearchStrategy, context *index.LeafReaderContext, k int) (hnsw.KnnCollector, error) {
	if opt, ok := manager.(OptimisticKnnCollectorManager); ok {
		return opt.NewOptimisticCollector(visitedLimit, searchStrategy, context, k)
	}
	return nil, nil
}

// IsOptimistic reports whether manager exposes an optimistic
// collector path. Mirrors the Java default of false.
func IsOptimistic(manager KnnCollectorManager) bool {
	if opt, ok := manager.(OptimisticKnnCollectorManager); ok {
		return opt.IsOptimistic()
	}
	return false
}
