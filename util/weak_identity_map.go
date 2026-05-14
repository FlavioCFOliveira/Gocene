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

package util

import (
	"reflect"
	"sync"
)

// WeakIdentityMap is a Go-divergent port of
// org.apache.lucene.util.WeakIdentityMap. Java's original combines
// WeakHashMap with IdentityHashMap: keys are compared by reference equality
// and dropped automatically when garbage collected.
//
// Lucene-divergence note: Go's standard library exposes no weak references,
// and runtime.SetFinalizer is brittle (it forbids self-reference cycles,
// runs at unpredictable times, and rejects unsafe.Pointer keys). The most
// honest port is therefore an explicit-cleanup map: callers must invoke
// Remove (or Clear) to drop entries. The "identity" half of the contract is
// preserved exactly — keys are compared by their underlying pointer
// identity, never by a structural equality function.
//
// API parity with the Java version covers Put / Get / Remove / ContainsKey /
// Size / IsEmpty / Clear / KeyIterator / ValueIterator. The "reapOnRead"
// switch from Lucene is omitted because there is nothing to reap without
// weak references.
//
// Concurrency: this map is safe for use from multiple goroutines. Reads
// hold an RLock; writes hold the full lock. The Java equivalents
// (WeakIdentityMap.newHashMap / newConcurrentHashMap) collapse into a single
// implementation here.
type WeakIdentityMap[K any, V any] struct {
	mu   sync.RWMutex
	data map[uintptr]weakIdentityEntry[K, V]
}

type weakIdentityEntry[K any, V any] struct {
	// key is the original key as supplied by the caller. The map is keyed
	// by the uintptr derived from this value, but we retain the typed key
	// so KeyIterator can yield it back.
	key   K
	value V
}

// NewWeakIdentityMap constructs an empty map. The generic K parameter must
// be a pointer or interface type; storing value types makes identity keying
// meaningless because the uintptr derived from a value is the address of a
// transient copy.
func NewWeakIdentityMap[K any, V any]() *WeakIdentityMap[K, V] {
	return &WeakIdentityMap[K, V]{data: make(map[uintptr]weakIdentityEntry[K, V])}
}

// Put inserts (or replaces) the (key, value) mapping. The key's pointer
// identity is the lookup index.
func (m *WeakIdentityMap[K, V]) Put(key K, value V) {
	id := identityOf(key)
	m.mu.Lock()
	m.data[id] = weakIdentityEntry[K, V]{key: key, value: value}
	m.mu.Unlock()
}

// Get returns the value associated with key by identity and a boolean
// telling whether the entry exists.
func (m *WeakIdentityMap[K, V]) Get(key K) (V, bool) {
	id := identityOf(key)
	m.mu.RLock()
	entry, ok := m.data[id]
	m.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	return entry.value, true
}

// ContainsKey reports whether the map contains a mapping for key.
func (m *WeakIdentityMap[K, V]) ContainsKey(key K) bool {
	id := identityOf(key)
	m.mu.RLock()
	_, ok := m.data[id]
	m.mu.RUnlock()
	return ok
}

// Remove deletes the mapping for key and returns the previous value (or
// the zero value and false).
func (m *WeakIdentityMap[K, V]) Remove(key K) (V, bool) {
	id := identityOf(key)
	m.mu.Lock()
	entry, ok := m.data[id]
	if ok {
		delete(m.data, id)
	}
	m.mu.Unlock()
	if !ok {
		var zero V
		return zero, false
	}
	return entry.value, true
}

// Size returns the number of mappings.
func (m *WeakIdentityMap[K, V]) Size() int {
	m.mu.RLock()
	n := len(m.data)
	m.mu.RUnlock()
	return n
}

// IsEmpty reports whether the map contains no mappings.
func (m *WeakIdentityMap[K, V]) IsEmpty() bool {
	return m.Size() == 0
}

// Clear removes all mappings.
func (m *WeakIdentityMap[K, V]) Clear() {
	m.mu.Lock()
	m.data = make(map[uintptr]weakIdentityEntry[K, V])
	m.mu.Unlock()
}

// KeyIterator returns a snapshot slice of all keys, taken under the read
// lock. Iteration over the snapshot is safe to interleave with Put / Remove
// on other goroutines.
func (m *WeakIdentityMap[K, V]) KeyIterator() []K {
	m.mu.RLock()
	out := make([]K, 0, len(m.data))
	for _, entry := range m.data {
		out = append(out, entry.key)
	}
	m.mu.RUnlock()
	return out
}

// ValueIterator returns a snapshot slice of all values, taken under the
// read lock. Iteration over the snapshot is safe to interleave with Put /
// Remove on other goroutines.
func (m *WeakIdentityMap[K, V]) ValueIterator() []V {
	m.mu.RLock()
	out := make([]V, 0, len(m.data))
	for _, entry := range m.data {
		out = append(out, entry.value)
	}
	m.mu.RUnlock()
	return out
}

// identityOf extracts the pointer-identity index for a key. It uses
// reflection to obtain the underlying pointer:
//
//   - Pointer / unsafe.Pointer kinds: their pointer value.
//   - Interface kinds (rare since Go unwraps automatically): the boxed
//     pointer when the dynamic type is a pointer, otherwise the address
//     of the storage cell.
//   - All other kinds: the address of the reflect.Value's storage. This
//     case is meaningful only when the caller stores a pointer-shaped
//     wrapper; storing a plain value type yields the address of a
//     transient copy and breaks identity semantics.
//
// Callers should keep K as a pointer or interface-of-pointer to match
// Lucene's System.identityHashCode contract.
func identityOf[K any](key K) uintptr {
	v := reflect.ValueOf(&key).Elem()
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.UnsafePointer:
		return v.Pointer()
	default:
		// As a last resort fall back to the address of the reflect.Value's
		// storage cell. This keeps the helper total but is only meaningful
		// for pointer-shaped wrappers.
		return v.UnsafeAddr()
	}
}
