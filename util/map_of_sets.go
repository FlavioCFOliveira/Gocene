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

// MapOfSets is a helper that associates each key with a set of values.
// It is the Go analogue of org.apache.lucene.util.MapOfSets, which
// wraps Map<K, Set<V>> in Java. Values are stored as map[V]struct{} to
// give O(1) presence checks without burning memory on a value payload.
//
// Like the Java reference, MapOfSets is NOT safe for concurrent use.
type MapOfSets[K comparable, V comparable] struct {
	theMap map[K]map[V]struct{}
}

// NewMapOfSets returns a MapOfSets backed by the given map. The map is
// retained as the live backing store, matching Lucene's constructor
// that accepts a caller-provided Map<K, Set<V>>.
func NewMapOfSets[K, V comparable](backing map[K]map[V]struct{}) *MapOfSets[K, V] {
	if backing == nil {
		backing = make(map[K]map[V]struct{})
	}
	return &MapOfSets[K, V]{theMap: backing}
}

// GetMap returns direct access to the underlying map. Mutations to the
// returned map are visible through MapOfSets and vice-versa, matching
// the Lucene contract.
func (m *MapOfSets[K, V]) GetMap() map[K]map[V]struct{} {
	return m.theMap
}

// Put adds val to the set associated with key, creating the set if it
// doesn't already exist. Returns the size of the set after insertion.
func (m *MapOfSets[K, V]) Put(key K, val V) int {
	set, ok := m.theMap[key]
	if !ok {
		set = make(map[V]struct{}, 23)
		m.theMap[key] = set
	}
	set[val] = struct{}{}
	return len(set)
}

// PutAll adds every value in vals to the set associated with key,
// creating the set if it doesn't already exist. Returns the size of
// the set after insertion.
func (m *MapOfSets[K, V]) PutAll(key K, vals []V) int {
	set, ok := m.theMap[key]
	if !ok {
		set = make(map[V]struct{}, 23)
		m.theMap[key] = set
	}
	for _, v := range vals {
		set[v] = struct{}{}
	}
	return len(set)
}

// Get returns the set associated with key and a boolean indicating
// whether the key was present. The returned map aliases the backing
// store; mutations are visible through MapOfSets.
func (m *MapOfSets[K, V]) Get(key K) (map[V]struct{}, bool) {
	set, ok := m.theMap[key]
	return set, ok
}

// Contains reports whether the set associated with key contains val.
// Returns false when the key is not present.
func (m *MapOfSets[K, V]) Contains(key K, val V) bool {
	set, ok := m.theMap[key]
	if !ok {
		return false
	}
	_, present := set[val]
	return present
}
