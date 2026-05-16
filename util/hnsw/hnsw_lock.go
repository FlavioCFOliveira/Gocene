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

import "sync"

// hnswLockNumStripes is the number of stripes in the HnswLock. Mirrors
// the Java reference's NUM_LOCKS = 512. Tuning this constant trades
// memory (one sync.RWMutex per stripe) for contention; 512 was picked
// upstream because it keeps the cache footprint small while making
// per-(level, node) collisions rare in practice.
const hnswLockNumStripes = 512

// HnswLock provides read-and-write striped locks for access to nodes of
// an [OnHeapHnswGraph]. It is the Go port of the package-private
// org.apache.lucene.util.hnsw.HnswLock (Lucene 10.4.0) used by
// [HnswConcurrentMergeBuilder] and the HnswGraphBuilder instances it
// drives.
//
// The lock is striped: a fixed array of [hnswLockNumStripes]
// sync.RWMutex values, indexed by a deterministic hash of
// (level, node). Two ordinal pairs that collide on the hash share the
// same mutex, so unrelated callers may serialise more than strictly
// required — the stripe count is sized to make that rare.
//
// Thread-safety: HnswLock is safe for concurrent use. The receiver is
// not copied (do not pass HnswLock by value; always use *HnswLock).
//
// Lifecycle: an HnswLock instance carries the same lifetime as the
// graph being mutated. There is no Close or Reset.
type HnswLock struct {
	// locks is the array of striped read/write mutexes. The slice is
	// allocated once at construction with length hnswLockNumStripes
	// and never reallocated — index access is the only operation.
	// Indexing happens on the hot path of every concurrent neighbour
	// mutation and every concurrent graph seek; bounds elision is
	// driven by the constant length.
	locks [hnswLockNumStripes]sync.RWMutex
}

// NewHnswLock constructs an HnswLock with the canonical number of
// stripes. Mirrors Java's package-private constructor.
func NewHnswLock() *HnswLock {
	return &HnswLock{}
}

// ReadLock acquires the read lock for the (level, node) stripe and
// returns a function the caller invokes to release it. The release
// function is safe to call exactly once; passing it through defer is
// the idiomatic usage.
//
// Mirrors Java's Lock read(int level, int node) — the caller's
// try / finally pattern translates to defer in Go.
func (h *HnswLock) ReadLock(level, node int) (release func()) {
	idx := hnswStripeIndex(level, node)
	h.locks[idx].RLock()
	return h.locks[idx].RUnlock
}

// WriteLock acquires the write lock for the (level, node) stripe and
// returns a function the caller invokes to release it. The release
// function is safe to call exactly once; passing it through defer is
// the idiomatic usage.
//
// Mirrors Java's Lock write(int level, int node).
func (h *HnswLock) WriteLock(level, node int) (release func()) {
	idx := hnswStripeIndex(level, node)
	h.locks[idx].Lock()
	return h.locks[idx].Unlock
}

// hnswStripeIndex returns the stripe index for the (level, node) pair.
//
// The Java reference computes `hash(v1, v2) = v1 * 31 + v2` and then
// `% NUM_LOCKS`. Java's `%` on a possibly-negative int can produce a
// negative result, but in practice level >= 0 and node >= 0 for HNSW,
// so the result is always non-negative. Go's `%` on int has the same
// sign-preserving semantics; this helper folds the result through
// `uint` to guarantee a non-negative index regardless of input — a
// belt-and-braces guard a future caller passing a synthetic negative
// level would not silently corrupt.
//
// The multiplier 31 matches Java verbatim.
func hnswStripeIndex(level, node int) int {
	const multiplier = 31
	h := uint(level*multiplier + node)
	return int(h % uint(hnswLockNumStripes))
}
