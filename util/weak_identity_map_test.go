// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"testing"
)

type wimKey struct{ tag string }

// TestWeakIdentityMap_PutGet covers the canonical lookup path with pointer
// keys.
func TestWeakIdentityMap_PutGet(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	k1 := &wimKey{tag: "k1"}
	k2 := &wimKey{tag: "k2"}
	m.Put(k1, 1)
	m.Put(k2, 2)

	if v, ok := m.Get(k1); !ok || v != 1 {
		t.Fatalf("Get(k1) = (%d,%v), want (1,true)", v, ok)
	}
	if v, ok := m.Get(k2); !ok || v != 2 {
		t.Fatalf("Get(k2) = (%d,%v), want (2,true)", v, ok)
	}
	if _, ok := m.Get(&wimKey{tag: "k1"}); ok {
		t.Fatalf("Get must use pointer identity, not structural equality")
	}
}

// TestWeakIdentityMap_Identity verifies the key contract: two distinct
// pointers with identical contents must map to distinct entries.
func TestWeakIdentityMap_Identity(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, string]()
	a := &wimKey{tag: "same"}
	b := &wimKey{tag: "same"}
	if *a != *b {
		t.Fatalf("test precondition: a and b must compare structurally equal")
	}
	m.Put(a, "A")
	m.Put(b, "B")
	if m.Size() != 2 {
		t.Fatalf("size=%d, want 2 (identity must distinguish a and b)", m.Size())
	}
	if v, _ := m.Get(a); v != "A" {
		t.Fatalf("a → %q, want %q", v, "A")
	}
	if v, _ := m.Get(b); v != "B" {
		t.Fatalf("b → %q, want %q", v, "B")
	}
}

// TestWeakIdentityMap_Remove deletes entries.
func TestWeakIdentityMap_Remove(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	k := &wimKey{tag: "r"}
	m.Put(k, 42)
	if v, ok := m.Remove(k); !ok || v != 42 {
		t.Fatalf("Remove(k) = (%d,%v), want (42,true)", v, ok)
	}
	if _, ok := m.Get(k); ok {
		t.Fatalf("Get after Remove should miss")
	}
	if _, ok := m.Remove(k); ok {
		t.Fatalf("second Remove(k) should report missing")
	}
}

// TestWeakIdentityMap_ContainsKey covers the boolean predicate.
func TestWeakIdentityMap_ContainsKey(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	k := &wimKey{tag: "c"}
	if m.ContainsKey(k) {
		t.Fatalf("empty map must not contain k")
	}
	m.Put(k, 7)
	if !m.ContainsKey(k) {
		t.Fatalf("Put-then-ContainsKey should hold")
	}
}

// TestWeakIdentityMap_ClearAndSize round-trips Put → Clear and reports size.
func TestWeakIdentityMap_ClearAndSize(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	if !m.IsEmpty() {
		t.Fatalf("new map must be empty")
	}
	for i := 0; i < 5; i++ {
		m.Put(&wimKey{tag: "k"}, i)
	}
	if m.Size() != 5 {
		t.Fatalf("Size=%d, want 5", m.Size())
	}
	m.Clear()
	if !m.IsEmpty() || m.Size() != 0 {
		t.Fatalf("Clear must zero the map")
	}
}

// TestWeakIdentityMap_KeyValueIterators returns snapshots of the contents.
func TestWeakIdentityMap_KeyValueIterators(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	k1 := &wimKey{tag: "k1"}
	k2 := &wimKey{tag: "k2"}
	m.Put(k1, 1)
	m.Put(k2, 2)

	keys := m.KeyIterator()
	if len(keys) != 2 {
		t.Fatalf("KeyIterator length=%d, want 2", len(keys))
	}
	saw := map[*wimKey]bool{}
	for _, k := range keys {
		saw[k] = true
	}
	if !saw[k1] || !saw[k2] {
		t.Fatalf("KeyIterator must yield both keys, got %v", saw)
	}

	values := m.ValueIterator()
	if len(values) != 2 {
		t.Fatalf("ValueIterator length=%d, want 2", len(values))
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	if sum != 3 {
		t.Fatalf("ValueIterator sum=%d, want 3", sum)
	}
}

// TestWeakIdentityMap_NilKey demonstrates that nil pointers act as a single
// identity slot (consistent with Lucene's NULL handling) without panicking.
func TestWeakIdentityMap_NilKey(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	var nilK *wimKey
	m.Put(nilK, 9)
	if v, ok := m.Get(nilK); !ok || v != 9 {
		t.Fatalf("Get(nil) = (%d,%v), want (9,true)", v, ok)
	}
}

// TestWeakIdentityMap_Concurrent runs concurrent Put/Get/Remove and confirms
// internal consistency at the end.
func TestWeakIdentityMap_Concurrent(t *testing.T) {
	m := NewWeakIdentityMap[*wimKey, int]()
	const n = 200
	keys := make([]*wimKey, n)
	for i := range keys {
		keys[i] = &wimKey{tag: "c"}
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			m.Put(keys[i], i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_ = m.ContainsKey(keys[i])
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_, _ = m.Get(keys[i])
		}
	}()
	wg.Wait()

	// After all Puts are done, every key must map to its index.
	for i, k := range keys {
		if v, ok := m.Get(k); !ok || v != i {
			t.Fatalf("post-race: key %d → (%d,%v), want (%d,true)", i, v, ok, i)
		}
	}
}
