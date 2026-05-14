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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Lucene's CloseableThreadLocal solves a JVM-specific problem: Java's
// ThreadLocal can hold dead values long after the owning instance is
// dereferenced, because every thread maintains a master map. The
// workaround stores WeakReferences in the ThreadLocal and hard refs in
// a side WeakHashMap.
//
// Goroutines are not threads. They have no stable identity, can
// migrate between OS threads, and there is no public API for
// goroutine-local storage. Faking it via runtime.GoID + sync.Map is
// non-idiomatic, brittle, and leaks slots whenever a goroutine exits
// without an explicit hook.
//
// The Go port replaces "thread-local" with "context-local": callers
// supply a context value (any comparable key — typically a per-segment
// or per-query identifier they already track) when storing or
// retrieving the cached value. Close() drops all cached values and
// renders subsequent Get calls inert.
//
// This preserves the *intent* (a reusable per-actor scratch buffer
// pool whose entire backing memory is released on a single Close call)
// while honoring Go's concurrency model. The Java name is kept so the
// porting trail from Lucene 10.4.0 remains readable.
// -----------------------------------------------------------------------------

package util

import "sync"

// PerContextCache is a per-context scratch slot generic over the
// cached value type. It is the Go port of
// org.apache.lucene.util.CloseableThreadLocal, retargeted from
// goroutine-local to caller-context-local storage.
//
// Typical use:
//
//	cache := util.NewPerContextCache[*Decoder](func() *Decoder { return NewDecoder() })
//	defer cache.Close()
//
//	d := cache.Get(segmentID)
//	d.Decode(...)
//
// PerContextCache is safe for concurrent use.
type PerContextCache[T any] struct {
	mu      sync.RWMutex
	closed  bool
	values  map[any]T
	initial func() T
}

// CloseableThreadLocal is kept as an alias of PerContextCache to keep
// the porting trail readable. New code should use PerContextCache
// directly.
type CloseableThreadLocal[T any] = PerContextCache[T]

// NewPerContextCache constructs a per-context cache. The initial
// function, if non-nil, supplies the value returned by Get when no
// entry exists for the supplied context. Passing a nil initial means
// Get returns the zero value of T for unknown contexts.
func NewPerContextCache[T any](initial func() T) *PerContextCache[T] {
	return &PerContextCache[T]{
		values:  make(map[any]T),
		initial: initial,
	}
}

// NewCloseableThreadLocal is a Java-name alias of NewPerContextCache.
// It exists to ease the migration of code paths translated literally
// from Lucene 10.4.0.
func NewCloseableThreadLocal[T any](initial func() T) *PerContextCache[T] {
	return NewPerContextCache[T](initial)
}

// Get returns the value cached for ctx, or the initial value (and
// caches it) when no entry exists. After Close, Get returns the zero
// value of T.
//
// ctx must be a comparable value (this is enforced statically by Go's
// map semantics on `any`).
func (c *PerContextCache[T]) Get(ctx any) T {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		var zero T
		return zero
	}
	v, ok := c.values[ctx]
	c.mu.RUnlock()
	if ok {
		return v
	}

	// Initialize lazily under the write lock to keep concurrent Gets
	// from racing to construct duplicate values.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		var zero T
		return zero
	}
	if v, ok := c.values[ctx]; ok {
		return v
	}
	var iv T
	if c.initial != nil {
		iv = c.initial()
	}
	c.values[ctx] = iv
	return iv
}

// Put replaces the cached value for ctx. After Close, Put is a no-op.
func (c *PerContextCache[T]) Put(ctx any, v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.values[ctx] = v
}

// Set is an alias of Put kept for parity with the Java API surface.
func (c *PerContextCache[T]) Set(ctx any, v T) { c.Put(ctx, v) }

// Remove evicts the cached value for ctx (mirroring
// ThreadLocal.remove). Returns true when an entry was removed.
func (c *PerContextCache[T]) Remove(ctx any) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	if _, ok := c.values[ctx]; !ok {
		return false
	}
	delete(c.values, ctx)
	return true
}

// Len returns the number of cached entries, or 0 after Close.
func (c *PerContextCache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return 0
	}
	return len(c.values)
}

// Close drops all cached entries and renders subsequent Get/Put calls
// inert. Close is idempotent. Mirrors java.io.Closeable.close.
func (c *PerContextCache[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = nil
	c.closed = true
	return nil
}
