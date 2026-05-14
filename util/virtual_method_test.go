// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"testing"
)

type vmBase interface{ tag() string }

type implA struct{}

func (implA) tag() string { return "A" }

type implB struct{}

func (implB) tag() string { return "B" }

// TestVirtualMethod_DefaultDistance is zero for unregistered impls — the
// Lucene "not overridden" path.
func TestVirtualMethod_DefaultDistance(t *testing.T) {
	resetVirtualMethodRegistry()
	vm := NewVirtualMethod[vmBase]("vmBase", "tag-default")
	if got := vm.GetImplementationDistance(implA{}); got != 0 {
		t.Fatalf("default distance = %d, want 0", got)
	}
	if vm.IsOverriddenAsOf(implA{}) {
		t.Fatalf("IsOverriddenAsOf should be false for unregistered impl")
	}
}

// TestVirtualMethod_RegisterAndDistance covers the happy path: distance is
// 1, 2, 3, ... in registration order; re-registering returns the same value.
func TestVirtualMethod_RegisterAndDistance(t *testing.T) {
	resetVirtualMethodRegistry()
	vm := NewVirtualMethod[vmBase]("vmBase", "tag-register")
	if d := vm.RegisterImpl(implA{}); d != 1 {
		t.Fatalf("first registration distance = %d, want 1", d)
	}
	if d := vm.RegisterImpl(implB{}); d != 2 {
		t.Fatalf("second registration distance = %d, want 2", d)
	}
	if d := vm.RegisterImpl(implA{}); d != 1 {
		t.Fatalf("re-registration must be idempotent, got %d", d)
	}
	if d := vm.GetImplementationDistance(implA{}); d != 1 {
		t.Fatalf("GetImplementationDistance(A)=%d, want 1", d)
	}
	if !vm.IsOverriddenAsOf(implA{}) {
		t.Fatalf("IsOverriddenAsOf(A) should be true")
	}
}

// TestVirtualMethod_SingletonEnforcement panics on duplicate (baseClass, method).
func TestVirtualMethod_SingletonEnforcement(t *testing.T) {
	resetVirtualMethodRegistry()
	_ = NewVirtualMethod[vmBase]("vmBase", "tag-singleton")
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate VirtualMethod registration")
		}
	}()
	_ = NewVirtualMethod[vmBase]("vmBase", "tag-singleton")
}

// TestCompareImplementationDistance covers the three branches of the helper.
func TestCompareImplementationDistance(t *testing.T) {
	resetVirtualMethodRegistry()
	old := NewVirtualMethod[vmBase]("vmBase", "old")
	neu := NewVirtualMethod[vmBase]("vmBase", "new")
	a, b := implA{}, implB{}

	// Both unregistered: 0.
	if got := CompareImplementationDistance(a, old, neu); got != 0 {
		t.Fatalf("both unregistered: got %d, want 0", got)
	}
	// Register on old: A has distance 1 on old, 0 on new ⇒ +1.
	old.RegisterImpl(a)
	if got := CompareImplementationDistance(a, old, neu); got != 1 {
		t.Fatalf("old-only: got %d, want 1", got)
	}
	// Register on new with two impls so A's distance there is 2.
	neu.RegisterImpl(b)
	neu.RegisterImpl(a)
	if got := CompareImplementationDistance(a, old, neu); got != -1 {
		t.Fatalf("new-deeper: got %d, want -1", got)
	}
}

// TestVirtualMethod_Concurrent ensures repeated registrations from multiple
// goroutines never lose updates or hand out duplicate distances.
func TestVirtualMethod_Concurrent(t *testing.T) {
	resetVirtualMethodRegistry()
	vm := NewVirtualMethod[vmBase]("vmBase", "tag-concurrent")

	const goroutines = 16
	const perG = 32

	type tag struct{ id, k int }

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for k := 0; k < perG; k++ {
				vm.RegisterImpl(tag{id: g, k: k})
			}
		}()
	}
	wg.Wait()

	// Every (g, k) must map to a non-zero distance.
	seen := make(map[int]struct{}, goroutines*perG)
	for g := 0; g < goroutines; g++ {
		for k := 0; k < perG; k++ {
			d := vm.GetImplementationDistance(tag{id: g, k: k})
			if d <= 0 {
				t.Fatalf("tag(g=%d,k=%d) distance=%d, want >0", g, k, d)
			}
			if _, dup := seen[d]; dup {
				t.Fatalf("duplicate distance %d", d)
			}
			seen[d] = struct{}{}
		}
	}
	if len(seen) != goroutines*perG {
		t.Fatalf("seen %d unique distances, want %d", len(seen), goroutines*perG)
	}
}
