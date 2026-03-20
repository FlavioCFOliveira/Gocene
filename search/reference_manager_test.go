package search

import (
	"sync"
	"testing"
)

// mockReference is a mock implementation of Reference for testing
type mockReference struct {
	mu       sync.Mutex
	refCount int
	id       string
	closed   bool
}

func newMockReference(id string) *mockReference {
	return &mockReference{
		refCount: 1, // Start with 1 reference
		id:       id,
	}
}

func (m *mockReference) IncRef() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refCount++
}

func (m *mockReference) DecRef() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refCount--
	if m.refCount <= 0 {
		m.closed = true
		return true
	}
	return false
}

func (m *mockReference) GetRefCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refCount
}

func (m *mockReference) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestNewReferenceManager(t *testing.T) {
	ref := newMockReference("test1")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	if rm.IsClosed() {
		t.Error("expected manager to not be closed")
	}

	// Reference count should be 2 (1 from creation + 1 from IncRef in NewReferenceManager)
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}
}

func TestNewReferenceManager_NilInitial(t *testing.T) {
	_, err := NewReferenceManager(nil, nil)
	if err == nil {
		t.Error("expected error for nil initial reference")
	}
}

func TestReferenceManager_Acquire(t *testing.T) {
	ref := newMockReference("test1")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	// Acquire the reference
	acquired, err := rm.Acquire()
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}

	// Reference count should be 3 (1 initial + 1 from NewReferenceManager + 1 from Acquire)
	if ref.GetRefCount() != 3 {
		t.Errorf("expected ref count 3, got %d", ref.GetRefCount())
	}

	// Release the reference
	if err := rm.Release(acquired); err != nil {
		t.Fatalf("failed to release: %v", err)
	}

	// Reference count should be back to 2
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}
}

func TestReferenceManager_AcquireClosed(t *testing.T) {
	ref := newMockReference("test1")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}

	// Close the manager
	if err := rm.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Try to acquire after close
	_, err = rm.Acquire()
	if err == nil {
		t.Error("expected error when acquiring from closed manager")
	}
}

func TestReferenceManager_SetCurrent(t *testing.T) {
	ref1 := newMockReference("ref1")
	ref2 := newMockReference("ref2")

	rm, err := NewReferenceManager(ref1, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	// Initial state: ref1 has count 2
	if ref1.GetRefCount() != 2 {
		t.Errorf("expected ref1 count 2, got %d", ref1.GetRefCount())
	}

	// Set new current
	if err := rm.SetCurrent(ref2); err != nil {
		t.Fatalf("failed to set current: %v", err)
	}

	// ref1 should be decremented to 1
	if ref1.GetRefCount() != 1 {
		t.Errorf("expected ref1 count 1, got %d", ref1.GetRefCount())
	}

	// ref2 should be incremented to 2
	if ref2.GetRefCount() != 2 {
		t.Errorf("expected ref2 count 2, got %d", ref2.GetRefCount())
	}

	// Current should be ref2
	current := rm.GetCurrent()
	if current != ref2 {
		t.Error("expected current to be ref2")
	}
}

func TestReferenceManager_SetCurrentNil(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	if err := rm.SetCurrent(nil); err == nil {
		t.Error("expected error when setting nil current")
	}
}

func TestReferenceManager_Close(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}

	// Initial ref count is 2
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}

	// Close the manager
	if err := rm.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !rm.IsClosed() {
		t.Error("expected manager to be closed")
	}

	// Reference should be decremented
	if ref.GetRefCount() != 1 {
		t.Errorf("expected ref count 1, got %d", ref.GetRefCount())
	}

	// Closing again should not error
	if err := rm.Close(); err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestReferenceManager_AfterClose(t *testing.T) {
	ref := newMockReference("test")
	closed := false

	afterClose := func(r Reference) {
		closed = true
	}

	rm, err := NewReferenceManager(ref, afterClose)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}

	// Decrement ref count until it reaches 0
	for ref.GetRefCount() > 1 {
		ref.DecRef()
	}

	// Now close the manager - this should trigger afterClose
	if err := rm.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !closed {
		t.Error("expected afterClose to be called")
	}
}

func TestReferenceManager_Release(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	// Acquire reference
	acquired, err := rm.Acquire()
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}

	// Release nil should error
	if err := rm.Release(nil); err == nil {
		t.Error("expected error when releasing nil")
	}

	// Release valid reference
	if err := rm.Release(acquired); err != nil {
		t.Fatalf("failed to release: %v", err)
	}
}

func TestReferenceManager_ConcurrentAccess(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	numIterations := 100

	// Concurrent acquire/release
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				acquired, err := rm.Acquire()
				if err != nil {
					t.Errorf("failed to acquire: %v", err)
					return
				}
				if err := rm.Release(acquired); err != nil {
					t.Errorf("failed to release: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	// Final ref count should be 2 (1 initial + 1 from NewReferenceManager)
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}
}

func TestReferenceManager_IncRefDecRef(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	// Initial count is 2
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}

	// IncRef
	if err := rm.IncRef(); err != nil {
		t.Fatalf("failed to inc ref: %v", err)
	}
	if ref.GetRefCount() != 3 {
		t.Errorf("expected ref count 3, got %d", ref.GetRefCount())
	}

	// DecRef
	if err := rm.DecRef(); err != nil {
		t.Fatalf("failed to dec ref: %v", err)
	}
	if ref.GetRefCount() != 2 {
		t.Errorf("expected ref count 2, got %d", ref.GetRefCount())
	}
}

func TestReferenceManager_IncRefDecRefClosed(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}

	rm.Close()

	if err := rm.IncRef(); err == nil {
		t.Error("expected error when IncRef on closed manager")
	}

	if err := rm.DecRef(); err == nil {
		t.Error("expected error when DecRef on closed manager")
	}
}

func TestReferenceManager_SetAfterClose(t *testing.T) {
	ref := newMockReference("test")
	rm, err := NewReferenceManager(ref, nil)
	if err != nil {
		t.Fatalf("failed to create ReferenceManager: %v", err)
	}
	defer rm.Close()

	called := false
	rm.SetAfterClose(func(r Reference) {
		called = true
	})

	// Trigger afterClose by closing
	ref.DecRef() // Reduce count to 1
	rm.Close()   // This should trigger afterClose

	if !called {
		t.Error("expected afterClose to be called")
	}
}
