package search

import (
	"fmt"
	"sync"
)

// Reference is the base interface for reference-counted resources.
// Implementations must provide reference counting capabilities.
type Reference interface {
	// IncRef increments the reference count
	IncRef()
	// DecRef decrements the reference count and returns true if the reference reached zero
	DecRef() bool
	// GetRefCount returns the current reference count (for testing/debugging)
	GetRefCount() int
}

// ReferenceManager manages reference-counted resources with Acquire/Release semantics.
// It is the foundation for NRT (Near Real-Time) search capabilities.
// Thread-safe for concurrent access.
type ReferenceManager struct {
	mu sync.RWMutex

	// current holds the currently managed reference
	current Reference

	// afterClose is called when the current reference is replaced or the manager is closed
	afterClose func(Reference)

	// isClosed indicates if the manager has been closed
	isClosed bool
}

// NewReferenceManager creates a new ReferenceManager with the given initial reference.
// The initial reference must have a reference count of at least 1.
func NewReferenceManager(initial Reference, afterClose func(Reference)) (*ReferenceManager, error) {
	if initial == nil {
		return nil, fmt.Errorf("initial reference cannot be nil")
	}

	rm := &ReferenceManager{
		current:    initial,
		afterClose: afterClose,
	}

	// Ensure the initial reference has at least one reference
	initial.IncRef()

	return rm, nil
}

// Acquire returns the current reference, incrementing its reference count.
// The caller must call Release() when done with the reference.
// Returns error if the manager is closed.
func (rm *ReferenceManager) Acquire() (Reference, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.isClosed {
		return nil, fmt.Errorf("reference manager is closed")
	}

	if rm.current == nil {
		return nil, fmt.Errorf("no current reference available")
	}

	rm.current.IncRef()
	return rm.current, nil
}

// Release decrements the reference count of the given reference.
// This must be called for every Acquire call.
func (rm *ReferenceManager) Release(ref Reference) error {
	if ref == nil {
		return fmt.Errorf("cannot release nil reference")
	}

	if ref.DecRef() {
		// Reference count reached zero, call afterClose if provided
		if rm.afterClose != nil {
			rm.afterClose(ref)
		}
	}

	return nil
}

// IncRef increments the reference count of the current managed reference.
// This is typically used internally; external code should use Acquire.
func (rm *ReferenceManager) IncRef() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.isClosed {
		return fmt.Errorf("reference manager is closed")
	}

	if rm.current == nil {
		return fmt.Errorf("no current reference available")
	}

	rm.current.IncRef()
	return nil
}

// DecRef decrements the reference count of the current managed reference.
// This is typically used internally; external code should use Release.
func (rm *ReferenceManager) DecRef() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.isClosed {
		return fmt.Errorf("reference manager is closed")
	}

	if rm.current == nil {
		return fmt.Errorf("no current reference available")
	}

	if rm.current.DecRef() {
		// Reference count reached zero, call afterClose if provided
		if rm.afterClose != nil {
			rm.afterClose(rm.current)
		}
	}

	return nil
}

// GetCurrent returns the current managed reference without incrementing the reference count.
// The returned reference is only valid while holding the manager's lock.
// This is intended for internal use; external code should use Acquire.
func (rm *ReferenceManager) GetCurrent() Reference {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.current
}

// SetCurrent replaces the current managed reference with a new one.
// The old reference will have its reference count decremented.
// The new reference must have a reference count of at least 1.
func (rm *ReferenceManager) SetCurrent(newRef Reference) error {
	if newRef == nil {
		return fmt.Errorf("new reference cannot be nil")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isClosed {
		return fmt.Errorf("reference manager is closed")
	}

	oldRef := rm.current

	// Increment new reference before making it current
	newRef.IncRef()
	rm.current = newRef

	// Decrement old reference
	if oldRef != nil {
		if oldRef.DecRef() {
			// Reference count reached zero, call afterClose if provided
			if rm.afterClose != nil {
				rm.afterClose(oldRef)
			}
		}
	}

	return nil
}

// Close closes the reference manager and releases the current reference.
// After Close is called, Acquire will return an error.
func (rm *ReferenceManager) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isClosed {
		return nil // Already closed
	}

	rm.isClosed = true

	// Release the current reference
	if rm.current != nil {
		if rm.current.DecRef() {
			// Reference count reached zero, call afterClose if provided
			if rm.afterClose != nil {
				rm.afterClose(rm.current)
			}
		}
		rm.current = nil
	}

	return nil
}

// IsClosed returns true if the manager has been closed.
func (rm *ReferenceManager) IsClosed() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.isClosed
}

// SetAfterClose sets the callback function to be called when a reference is closed.
// This is useful for custom cleanup logic.
func (rm *ReferenceManager) SetAfterClose(afterClose func(Reference)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.afterClose = afterClose
}
