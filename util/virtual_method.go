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
	"sync"
	"sync/atomic"
)

// VirtualMethod tracks distinct implementations of a "virtual" function in
// Go. It is a Lucene-divergence port of org.apache.lucene.util.VirtualMethod.
//
// Lucene-divergence note: Java's VirtualMethod inspects a class hierarchy at
// runtime via reflection so the application can detect when a deprecated
// method has been overridden by a subclass — a backwards-compatibility tool
// for evolving abstract base classes. Go has neither class inheritance nor
// reflective access to overriding subclasses, so this port adopts the
// closest workable semantics:
//
//   - Each VirtualMethod is created once per "method slot" with a base name.
//     The singleton enforcement of the Java version is preserved through the
//     package-private registry below; creating two VirtualMethods with the
//     same baseClass+name panics, matching UnsupportedOperationException.
//
//   - Implementations register themselves by calling RegisterImpl with a
//     comparable token (typically a sentinel value of some type derived from
//     the base, or a string tag identifying the implementation). Distinct
//     registrations get incrementing distances starting at 1 — the same
//     convention Lucene uses for "distance of the overriding class".
//
//   - GetImplementationDistance returns 0 for unregistered tokens, mirroring
//     "not overridden". IsOverriddenAsOf returns distance > 0.
//
//   - CompareImplementationDistance preserves Lucene's API for callers that
//     need to know which of two methods is "more overridden".
//
// This divergence is intentional and load-bearing: in Lucene the class
// hierarchy is the source of truth; in Go the registration is explicit.
type VirtualMethod[C any] struct {
	baseClass string
	method    string

	distances atomic.Pointer[map[any]int]
	mu        sync.Mutex // serialises RegisterImpl.
}

// virtualMethodRegistry enforces the singleton constraint: at most one
// VirtualMethod per (baseClass, method) pair, just like the Java version's
// singletonSet of java.lang.reflect.Method.
var (
	virtualMethodRegistryMu sync.Mutex
	virtualMethodRegistry   = make(map[virtualMethodKey]struct{})
)

type virtualMethodKey struct {
	baseClass string
	method    string
}

// NewVirtualMethod creates a singleton tracker for the given baseClass and
// method names. Panics with an UnsupportedOperationException-equivalent if a
// VirtualMethod with the same (baseClass, method) pair already exists,
// matching the Java original's "singleton enforcement" rule.
func NewVirtualMethod[C any](baseClass, method string) *VirtualMethod[C] {
	key := virtualMethodKey{baseClass: baseClass, method: method}
	virtualMethodRegistryMu.Lock()
	if _, exists := virtualMethodRegistry[key]; exists {
		virtualMethodRegistryMu.Unlock()
		panic("VirtualMethod instances must be singletons and therefore assigned to static final members in the same class, they use as baseClass ctor param.")
	}
	virtualMethodRegistry[key] = struct{}{}
	virtualMethodRegistryMu.Unlock()

	vm := &VirtualMethod[C]{baseClass: baseClass, method: method}
	empty := make(map[any]int)
	vm.distances.Store(&empty)
	return vm
}

// RegisterImpl tags impl as a distinct implementation of this method. The
// returned distance — at least 1, monotonically increasing per VirtualMethod
// — mirrors Lucene's "distance of overriding class from baseClass".
// Re-registering the same impl is a no-op that returns its existing distance.
//
// Race-freedom: a mutex serialises writers while the map is published
// atomically so GetImplementationDistance never sees a partially-built map.
func (vm *VirtualMethod[C]) RegisterImpl(impl any) int {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	current := *vm.distances.Load()
	if d, ok := current[impl]; ok {
		return d
	}
	next := make(map[any]int, len(current)+1)
	for k, v := range current {
		next[k] = v
	}
	d := len(next) + 1
	next[impl] = d
	vm.distances.Store(&next)
	return d
}

// GetImplementationDistance returns the registered distance for impl, or 0
// when impl was never registered. The 0/non-0 distinction matches Lucene's
// "not overridden" / "overridden" semantics.
func (vm *VirtualMethod[C]) GetImplementationDistance(impl any) int {
	if d, ok := (*vm.distances.Load())[impl]; ok {
		return d
	}
	return 0
}

// IsOverriddenAsOf reports whether impl was registered as an override.
// Equivalent to GetImplementationDistance(impl) > 0.
func (vm *VirtualMethod[C]) IsOverriddenAsOf(impl any) bool {
	return vm.GetImplementationDistance(impl) > 0
}

// CompareImplementationDistance returns the signed difference between the
// distances of impl on m1 and m2: positive when impl is "further" along
// m1's chain, negative when it is "further" along m2's, zero when they
// match (including both unregistered).
func CompareImplementationDistance[C any](impl any, m1, m2 *VirtualMethod[C]) int {
	d1 := m1.GetImplementationDistance(impl)
	d2 := m2.GetImplementationDistance(impl)
	switch {
	case d1 > d2:
		return 1
	case d1 < d2:
		return -1
	default:
		return 0
	}
}

// resetVirtualMethodRegistry clears the singleton registry. Test-only helper:
// VirtualMethod's singleton constraint is global per process; this function
// lets tests re-create a tracker without leaking state across runs.
func resetVirtualMethodRegistry() {
	virtualMethodRegistryMu.Lock()
	virtualMethodRegistry = make(map[virtualMethodKey]struct{})
	virtualMethodRegistryMu.Unlock()
}
