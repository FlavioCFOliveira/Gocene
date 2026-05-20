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

// Package knn provides the kNN (k-nearest-neighbour) vector-search
// infrastructure: per-leaf collector factories, search strategies, and
// the cross-leaf MultiLeafKnnCollector.
//
// It is the Go port of org.apache.lucene.search.knn (Lucene 10.4.0).
//
// The canonical KnnCollector / AbstractKnnCollector / TopKnnCollector
// types live in github.com/FlavioCFOliveira/Gocene/util/hnsw; this
// package re-uses them rather than redefining them, matching the
// existing Gocene convention where hnsw owns the local stub until the
// search-base sprint migrates the canonical types out.
package knn

import (
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// DefaultFilteredSearchThreshold is the default value of the
// filteredSearchThreshold knob carried by the [Hnsw] search strategy.
// Zero means: never use filtered search regardless of the ratio of
// vectors passing the filter.
//
// Mirrors KnnSearchStrategy.DEFAULT_FILTERED_SEARCH_THRESHOLD.
const DefaultFilteredSearchThreshold = 0

// KnnSearchStrategy provides additional kNN search configuration. It
// is the Go port of the abstract base class
// org.apache.lucene.search.knn.KnnSearchStrategy (Lucene 10.4.0).
//
// Concrete strategies must implement [NextVectorsBlock] and carry
// their own value-equality semantics. The Equals method takes any so
// callers can compare heterogenous strategies without a runtime
// downcast at the call site; implementations should perform their own
// type check.
//
// Mirrors KnnSearchStrategy: the Java type forces subclasses to
// override equals/hashCode/nextVectorsBlock. The Go interface enforces
// the equivalent contract.
type KnnSearchStrategy interface {
	// NextVectorsBlock signals the strategy that another block of
	// candidate vectors is about to be examined.
	NextVectorsBlock()

	// Equals reports whether this strategy is structurally equal to
	// other. Implementations should narrow to their concrete type
	// before comparing fields.
	Equals(other any) bool

	// HashCode returns a value-stable hash, sufficient for use as a
	// map key when paired with [Equals].
	HashCode() uint64
}

// Compile-time guards that the concrete strategies satisfy both the
// local interface and the hnsw stub interface (so that any code path
// dispatching through hnsw can accept a strategy declared in this
// package without an adapter).
var (
	_ KnnSearchStrategy      = (*Hnsw)(nil)
	_ KnnSearchStrategy      = (*Seeded)(nil)
	_ hnsw.KnnSearchStrategy = (*Hnsw)(nil)
	_ hnsw.KnnSearchStrategy = (*Seeded)(nil)
)

// Hnsw is the HNSW kNN search strategy. It carries an integer
// percentage in [0, 100] controlling when the filtered-search code
// path is preferred over the regular one. Mirrors
// KnnSearchStrategy.Hnsw.
type Hnsw struct {
	filteredSearchThreshold int
}

// DefaultHnsw is the package-level Hnsw default
// (filteredSearchThreshold == 0). Mirrors
// KnnSearchStrategy.Hnsw.DEFAULT.
var DefaultHnsw = NewHnsw(DefaultFilteredSearchThreshold)

// NewHnsw constructs an Hnsw strategy.
// filteredSearchThreshold must be in [0, 100]; outside that range the
// constructor panics, mirroring Java's IllegalArgumentException.
func NewHnsw(filteredSearchThreshold int) *Hnsw {
	if filteredSearchThreshold < 0 || filteredSearchThreshold > 100 {
		panic("knn: filteredSearchThreshold must be >= 0 and <= 100")
	}
	return &Hnsw{filteredSearchThreshold: filteredSearchThreshold}
}

// FilteredSearchThreshold returns the configured percentage.
func (h *Hnsw) FilteredSearchThreshold() int { return h.filteredSearchThreshold }

// UseFilteredSearch reports whether the filtered-search path should
// be taken for a graph whose ratioPassingFilter is the supplied
// value. ratioPassingFilter must be in [0, 1]; outside that range the
// method panics, mirroring Java's assert.
//
// Mirrors KnnSearchStrategy.Hnsw#useFilteredSearch.
func (h *Hnsw) UseFilteredSearch(ratioPassingFilter float32) bool {
	if ratioPassingFilter < 0 || ratioPassingFilter > 1 {
		panic("knn: ratioPassingFilter out of [0,1]")
	}
	return ratioPassingFilter*100 < float32(h.filteredSearchThreshold)
}

// NextVectorsBlock is a no-op for the plain HNSW strategy.
func (h *Hnsw) NextVectorsBlock() {}

// Equals reports whether other is an *Hnsw with the same
// filteredSearchThreshold.
func (h *Hnsw) Equals(other any) bool {
	if h == other {
		return true
	}
	o, ok := other.(*Hnsw)
	if !ok || o == nil {
		return false
	}
	return h.filteredSearchThreshold == o.filteredSearchThreshold
}

// HashCode mirrors java.util.Objects.hashCode(Integer) — the boxed
// hash of the threshold value.
func (h *Hnsw) HashCode() uint64 {
	return uint64(uint32(h.filteredSearchThreshold))
}

// Seeded is a kNN search strategy that primes the search with an
// explicit set of entry-point doc IDs before falling back to the
// underlying strategy. Mirrors KnnSearchStrategy.Seeded.
type Seeded struct {
	entryPoints         util.DocIdSetIterator
	numberOfEntryPoints int
	originalStrategy    KnnSearchStrategy
}

// NewSeeded constructs a Seeded strategy. numberOfEntryPoints must be
// >= 0; if it is > 0 then entryPoints must be non-nil. When
// entryPoints is nil and numberOfEntryPoints == 0 the strategy uses
// the empty iterator, matching Java's DocIdSetIterator.empty().
//
// Mirrors KnnSearchStrategy.Seeded(DocIdSetIterator, int,
// KnnSearchStrategy).
func NewSeeded(entryPoints util.DocIdSetIterator, numberOfEntryPoints int, originalStrategy KnnSearchStrategy) *Seeded {
	if numberOfEntryPoints < 0 {
		panic("knn: numberOfEntryPoints must be >= 0")
	}
	if numberOfEntryPoints > 0 && entryPoints == nil {
		panic("knn: entryPoints must not be nil")
	}
	if entryPoints == nil {
		entryPoints = util.EmptyDocIdSetIterator()
	}
	return &Seeded{
		entryPoints:         entryPoints,
		numberOfEntryPoints: numberOfEntryPoints,
		originalStrategy:    originalStrategy,
	}
}

// EntryPoints returns the iterator of valid entry-point doc IDs.
func (s *Seeded) EntryPoints() util.DocIdSetIterator { return s.entryPoints }

// NumberOfEntryPoints returns the configured count.
func (s *Seeded) NumberOfEntryPoints() int { return s.numberOfEntryPoints }

// OriginalStrategy returns the strategy used after seeding completes.
// May be nil.
func (s *Seeded) OriginalStrategy() KnnSearchStrategy { return s.originalStrategy }

// NextVectorsBlock delegates to the original strategy.
func (s *Seeded) NextVectorsBlock() {
	if s.originalStrategy != nil {
		s.originalStrategy.NextVectorsBlock()
	}
}

// Equals reports whether other is a *Seeded with identical
// entryPoints reference, numberOfEntryPoints, and original strategy.
// Iterator equality follows reference identity, matching Java's
// Objects.equals on the DocIdSetIterator instance.
func (s *Seeded) Equals(other any) bool {
	if s == other {
		return true
	}
	o, ok := other.(*Seeded)
	if !ok || o == nil {
		return false
	}
	if s.numberOfEntryPoints != o.numberOfEntryPoints {
		return false
	}
	if s.entryPoints != o.entryPoints { //nolint:staticcheck // reference equality matches Java
		return false
	}
	switch {
	case s.originalStrategy == nil && o.originalStrategy == nil:
		// equal
	case s.originalStrategy == nil || o.originalStrategy == nil:
		return false
	default:
		if !s.originalStrategy.Equals(o.originalStrategy) {
			return false
		}
	}
	return true
}

// HashCode combines the field hashes the same way
// java.util.Objects.hash does: a rolling 31-multiplier accumulator
// seeded at 1.
func (s *Seeded) HashCode() uint64 {
	const prime = uint64(31)
	h := uint64(1)
	// entryPoints: identity hash (use pointer bits when convertible).
	h = prime*h + identityHash(s.entryPoints)
	h = prime*h + uint64(uint32(s.numberOfEntryPoints))
	if s.originalStrategy != nil {
		h = prime*h + s.originalStrategy.HashCode()
	} else {
		h = prime * h
	}
	return h
}

// identityHash returns a value-stable hash for an interface that
// mimics Java's System.identityHashCode (collapsed to zero for nil).
// It folds the (type, value) tuple into a single 64-bit word using
// the runtime representation of the interface header.
func identityHash(v any) uint64 {
	if v == nil {
		return 0
	}
	type iface struct {
		typ  uintptr
		data uintptr
	}
	i := *(*iface)(unsafe.Pointer(&v))
	return uint64(i.typ) ^ uint64(i.data)
}
