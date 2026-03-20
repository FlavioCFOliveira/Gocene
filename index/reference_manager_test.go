// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
	"testing"
)

// mockResource is a mock closable resource for testing
type mockResource struct {
	id       int
	refCount int
	mu       sync.Mutex
	closed   bool
}

func newMockResource(id int) *mockResource {
	return &mockResource{id: id, refCount: 1}
}

func (m *mockResource) IncRef() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refCount++
}

func (m *mockResource) DecRef() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refCount--
	if m.refCount <= 0 {
		m.closed = true
		return true
	}
	return false
}

func (m *mockResource) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestReferenceManager_BasicOperations(t *testing.T) {
	// Create a mock resource
	resource := newMockResource(1)

	// Create manager with custom acquire/release functions
	acquireFunc := func(r *mockResource) *mockResource {
		r.IncRef()
		return r
	}

	releaseFunc := func(r *mockResource) error {
		r.DecRef()
		return nil
	}

	manager := NewReferenceManagerWithFuncs(resource, acquireFunc, releaseFunc)

	// Test Acquire
	ref, err := manager.Acquire()
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if ref != resource {
		t.Error("Acquired reference should be the same resource")
	}

	// Test GetGeneration
	gen := manager.GetGeneration()
	if gen != 1 {
		t.Errorf("Expected generation 1, got %d", gen)
	}

	// Test GetCurrent
	current := manager.GetCurrent()
	if current != resource {
		t.Error("GetCurrent should return the same resource")
	}

	// Test Release
	err = manager.Release(ref)
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Test Close
	err = manager.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Acquire after close should fail
	_, err = manager.Acquire()
	if err == nil {
		t.Error("Acquire should fail after Close")
	}
}

func TestReferenceManager_Swap(t *testing.T) {
	resource1 := newMockResource(1)
	resource2 := newMockResource(2)

	manager := NewReferenceManager(resource1)

	// Swap to resource2
	old := manager.Swap(resource2)
	if old != resource1 {
		t.Error("Swap should return the old resource")
	}

	// Generation should be incremented
	gen := manager.GetGeneration()
	if gen != 2 {
		t.Errorf("Expected generation 2 after swap, got %d", gen)
	}

	// Current should be resource2
	current := manager.GetCurrent()
	if current != resource2 {
		t.Error("Current should be resource2 after swap")
	}
}

func TestReferenceManager_RefreshListener(t *testing.T) {
	resource := newMockResource(1)
	manager := NewReferenceManager(resource)

	// Track listener calls
	var beforeCalled, afterCalled bool
	var afterGen int64

	listener := &RefreshListenerFunc{
		beforeFunc: func() {
			beforeCalled = true
		},
		afterFunc: func(gen int64) {
			afterCalled = true
			afterGen = gen
		},
	}

	manager.AddRefreshListener(listener)

	// Call Refresh
	gen, err := manager.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if !beforeCalled {
		t.Error("BeforeRefresh should be called")
	}
	if !afterCalled {
		t.Error("AfterRefresh should be called")
	}
	if afterGen != gen {
		t.Errorf("AfterRefresh generation should match returned generation: got %d, expected %d", afterGen, gen)
	}

	// Remove listener and refresh again
	manager.RemoveRefreshListener(listener)
	beforeCalled = false
	afterCalled = false

	_, _ = manager.Refresh()

	if beforeCalled || afterCalled {
		t.Error("Listener should not be called after removal")
	}
}

func TestReferenceManager_ConcurrentAccess(t *testing.T) {
	resource := newMockResource(1)

	acquireFunc := func(r *mockResource) *mockResource {
		r.IncRef()
		return r
	}

	releaseFunc := func(r *mockResource) error {
		r.DecRef()
		return nil
	}

	manager := NewReferenceManagerWithFuncs(resource, acquireFunc, releaseFunc)

	var wg sync.WaitGroup
	numGoroutines := 100
	acquiresPerGoroutine := 100

	// Concurrent acquires
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < acquiresPerGoroutine; j++ {
				ref, err := manager.Acquire()
				if err != nil {
					t.Errorf("Acquire failed: %v", err)
					return
				}
				err = manager.Release(ref)
				if err != nil {
					t.Errorf("Release failed: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	// Resource should still have 1 reference (the initial one)
	resource.mu.Lock()
	if resource.refCount != 1 {
		t.Errorf("Expected refCount 1, got %d", resource.refCount)
	}
	resource.mu.Unlock()
}

func TestReferenceManager_IsOpen(t *testing.T) {
	resource := newMockResource(1)
	manager := NewReferenceManager(resource)

	if !manager.IsOpen() {
		t.Error("Manager should be open initially")
	}

	err := manager.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if manager.IsOpen() {
		t.Error("Manager should be closed after Close()")
	}
}

func TestReferenceManager_MultipleListeners(t *testing.T) {
	resource := newMockResource(1)
	manager := NewReferenceManager(resource)

	callCount := 0
	var mu sync.Mutex

	listener1 := &RefreshListenerFunc{
		afterFunc: func(gen int64) {
			mu.Lock()
			callCount++
			mu.Unlock()
		},
	}

	listener2 := &RefreshListenerFunc{
		afterFunc: func(gen int64) {
			mu.Lock()
			callCount++
			mu.Unlock()
		},
	}

	manager.AddRefreshListener(listener1)
	manager.AddRefreshListener(listener2)

	_, _ = manager.Refresh()

	mu.Lock()
	if callCount != 2 {
		t.Errorf("Expected both listeners to be called, got %d calls", callCount)
	}
	mu.Unlock()
}

func TestReferenceManager_String(t *testing.T) {
	resource := newMockResource(1)
	manager := NewReferenceManager(resource)

	str := manager.String()
	if str == "" {
		t.Error("String() should return non-empty string")
	}

	// Should contain generation
	err := manager.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	str = manager.String()
	if str == "" {
		t.Error("String() should return non-empty string after close")
	}
}

func TestReferenceManager_GenericTypes(t *testing.T) {
	// Test with int type
	intManager := NewReferenceManager(42)
	val, err := intManager.Acquire()
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}

	// Test with string type
	stringManager := NewReferenceManager("test")
	str, err := stringManager.Acquire()
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if str != "test" {
		t.Errorf("Expected 'test', got %s", str)
	}
}
