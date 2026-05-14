// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestMapOfSets_PutSingle(t *testing.T) {
	m := NewMapOfSets[string, int](nil)
	if got := m.Put("a", 1); got != 1 {
		t.Fatalf("Put: size=%d want 1", got)
	}
	if got := m.Put("a", 1); got != 1 {
		t.Fatalf("Put duplicate: size=%d want 1", got)
	}
	if got := m.Put("a", 2); got != 2 {
		t.Fatalf("Put new value: size=%d want 2", got)
	}
}

func TestMapOfSets_PutAll(t *testing.T) {
	m := NewMapOfSets[string, int](nil)
	if got := m.PutAll("k", []int{1, 2, 3}); got != 3 {
		t.Fatalf("PutAll: size=%d want 3", got)
	}
	if got := m.PutAll("k", []int{2, 4}); got != 4 {
		t.Fatalf("PutAll second: size=%d want 4", got)
	}
	if got := m.PutAll("k", nil); got != 4 {
		t.Fatalf("PutAll nil: size=%d want 4", got)
	}
}

func TestMapOfSets_Contains(t *testing.T) {
	m := NewMapOfSets[string, int](nil)
	if m.Contains("x", 1) {
		t.Fatalf("Contains on missing key should be false")
	}
	m.Put("x", 1)
	if !m.Contains("x", 1) {
		t.Fatalf("Contains after Put should be true")
	}
	if m.Contains("x", 2) {
		t.Fatalf("Contains for missing value should be false")
	}
}

func TestMapOfSets_GetMapAliasesBacking(t *testing.T) {
	backing := map[string]map[int]struct{}{}
	m := NewMapOfSets(backing)
	m.Put("a", 1)
	if _, ok := backing["a"][1]; !ok {
		t.Fatalf("Put did not propagate to caller-supplied backing map")
	}
	// Mutation through GetMap must be visible through the wrapper.
	m.GetMap()["b"] = map[int]struct{}{42: {}}
	if !m.Contains("b", 42) {
		t.Fatalf("GetMap mutation not visible through Contains")
	}
}

func TestMapOfSets_GetReturnsSet(t *testing.T) {
	m := NewMapOfSets[string, int](nil)
	m.PutAll("k", []int{10, 20, 30})
	set, ok := m.Get("k")
	if !ok {
		t.Fatalf("Get(k) missing")
	}
	if len(set) != 3 {
		t.Fatalf("set size=%d want 3", len(set))
	}
	for _, v := range []int{10, 20, 30} {
		if _, present := set[v]; !present {
			t.Fatalf("set missing %d", v)
		}
	}
	_, ok = m.Get("missing")
	if ok {
		t.Fatalf("Get(missing) should be false")
	}
}

func TestMapOfSets_NilBackingInitialises(t *testing.T) {
	m := NewMapOfSets[int, string](nil)
	if m.GetMap() == nil {
		t.Fatalf("nil backing should be replaced with a new map")
	}
}

func TestMapOfSets_IntKeysAndStringValues(t *testing.T) {
	m := NewMapOfSets[int, string](nil)
	m.Put(7, "alpha")
	m.Put(7, "beta")
	m.Put(8, "alpha")
	if !m.Contains(7, "alpha") || !m.Contains(7, "beta") || !m.Contains(8, "alpha") {
		t.Fatalf("missing expected values")
	}
	if m.Contains(8, "beta") {
		t.Fatalf("unexpected value present")
	}
}
