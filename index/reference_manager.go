// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ReferenceManager manages a reference to a resource with reference counting.
// It provides thread-safe access and lifecycle management.
// This is the Go port of Lucene's org.apache.lucene.search.ReferenceManager.
//
// Type parameter T is the type of resource being managed (e.g., *IndexReader, *IndexSearcher).
// The resource must implement Closable interface for proper cleanup.
type ReferenceManager[T any] struct {
	mu sync.RWMutex

	// current holds the current resource reference.
	// Access must be protected by mu.
	current T

	// generation is incremented on each refresh.
	generation int64

	// refreshListeners are notified when the reference is refreshed.
	refreshListeners []RefreshListener

	// closed indicates if the manager has been closed.
	closed atomic.Bool

	// acquireFunc is called to acquire a new reference.
	// This is typically set by subclasses to customize acquisition behavior.
	acquireFunc func(T) T

	// releaseFunc is called to release a reference.
	// This is typically set by subclasses to customize release behavior.
	releaseFunc func(T) error
}

// RefreshListener is notified when the reference is refreshed.
type RefreshListener interface {
	// BeforeRefresh is called before the reference is refreshed.
	BeforeRefresh()
	// AfterRefresh is called after the reference is refreshed with the new generation.
	AfterRefresh(generation int64)
}

// RefreshListenerFunc is a function type that implements RefreshListener.
type RefreshListenerFunc struct {
	beforeFunc func()
	afterFunc  func(int64)
}

// NewRefreshListenerFunc creates a RefreshListener from functions.
func NewRefreshListenerFunc(before func(), after func(int64)) *RefreshListenerFunc {
	return &RefreshListenerFunc{
		beforeFunc: before,
		afterFunc:  after,
	}
}

// BeforeRefresh implements RefreshListener.
func (l *RefreshListenerFunc) BeforeRefresh() {
	if l.beforeFunc != nil {
		l.beforeFunc()
	}
}

// AfterRefresh implements RefreshListener.
func (l *RefreshListenerFunc) AfterRefresh(generation int64) {
	if l.afterFunc != nil {
		l.afterFunc(generation)
	}
}

// NewReferenceManager creates a new ReferenceManager with the given initial reference.
func NewReferenceManager[T any](initial T) *ReferenceManager[T] {
	return &ReferenceManager[T]{
		current:    initial,
		generation: 1,
	}
}

// NewReferenceManagerWithFuncs creates a new ReferenceManager with custom acquire/release functions.
func NewReferenceManagerWithFuncs[T any](
	initial T,
	acquireFunc func(T) T,
	releaseFunc func(T) error,
) *ReferenceManager[T] {
	return &ReferenceManager[T]{
		current:       initial,
		generation:    1,
		acquireFunc:   acquireFunc,
		releaseFunc:   releaseFunc,
	}
}

// Acquire returns the current reference and increments its reference count.
// The caller MUST call Release when done with the reference.
// Returns error if the manager is closed.
func (rm *ReferenceManager[T]) Acquire() (T, error) {
	var zero T

	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.closed.Load() {
		return zero, fmt.Errorf("reference manager is closed")
	}

	if rm.acquireFunc != nil {
		return rm.acquireFunc(rm.current), nil
	}

	return rm.current, nil
}

// Release decrements the reference count and potentially closes the resource.
// Must be called once for each Acquire.
func (rm *ReferenceManager[T]) Release(reference T) error {
	if rm.releaseFunc != nil {
		return rm.releaseFunc(reference)
	}
	return nil
}

// GetCurrent returns the current reference without incrementing the reference count.
// This is useful for read-only operations where the caller doesn't need to keep the reference alive.
// The returned reference may be closed by another goroutine, so use with caution.
func (rm *ReferenceManager[T]) GetCurrent() T {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.current
}

// GetGeneration returns the current generation number.
// The generation is incremented each time the reference is refreshed.
func (rm *ReferenceManager[T]) GetGeneration() int64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.generation
}

// MaybeRefresh checks if a refresh is needed and performs it if so.
// Returns true if a refresh was performed, false otherwise.
// Subclasses should override doRefresh to implement the actual refresh logic.
func (rm *ReferenceManager[T]) MaybeRefresh() (bool, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.closed.Load() {
		return false, fmt.Errorf("reference manager is closed")
	}

	// Subclasses implement actual refresh logic
	return false, nil
}

// Refresh refreshes the reference if needed and returns the new generation.
// Blocks until the refresh is complete.
func (rm *ReferenceManager[T]) Refresh() (int64, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.closed.Load() {
		return 0, fmt.Errorf("reference manager is closed")
	}

	// Notify listeners before refresh
	rm.notifyBeforeRefresh()

	// Subclasses implement actual refresh logic here
	// For now, just increment generation
	rm.generation++

	// Notify listeners after refresh
	rm.notifyAfterRefresh()

	return rm.generation, nil
}

// Close closes the reference manager and releases the current reference.
func (rm *ReferenceManager[T]) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.closed.Load() {
		return nil
	}

	rm.closed.Store(true)

	// Release current reference
	if rm.releaseFunc != nil {
		return rm.releaseFunc(rm.current)
	}

	return nil
}

// IsOpen returns true if the manager is open.
func (rm *ReferenceManager[T]) IsOpen() bool {
	return !rm.closed.Load()
}

// AddRefreshListener adds a listener to be notified on refresh.
func (rm *ReferenceManager[T]) AddRefreshListener(listener RefreshListener) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.refreshListeners = append(rm.refreshListeners, listener)
}

// RemoveRefreshListener removes a refresh listener.
func (rm *ReferenceManager[T]) RemoveRefreshListener(listener RefreshListener) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, l := range rm.refreshListeners {
		if l == listener {
			rm.refreshListeners = append(rm.refreshListeners[:i], rm.refreshListeners[i+1:]...)
			return
		}
	}
}

// notifyBeforeRefresh notifies all listeners before refresh.
func (rm *ReferenceManager[T]) notifyBeforeRefresh() {
	for _, listener := range rm.refreshListeners {
		listener.BeforeRefresh()
	}
}

// notifyAfterRefresh notifies all listeners after refresh.
func (rm *ReferenceManager[T]) notifyAfterRefresh() {
	for _, listener := range rm.refreshListeners {
		listener.AfterRefresh(rm.generation)
	}
}

// Swap swaps the current reference with a new one.
// This should be called by subclasses to update the reference.
// Returns the old reference which should be released by the caller.
func (rm *ReferenceManager[T]) Swap(newReference T) T {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	old := rm.current
	rm.current = newReference
	rm.generation++

	return old
}

// SetCurrent sets the current reference without incrementing generation.
// Use with caution - prefer Swap for normal updates.
func (rm *ReferenceManager[T]) SetCurrent(reference T) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.current = reference
}

// String returns a string representation of the manager.
func (rm *ReferenceManager[T]) String() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return fmt.Sprintf("ReferenceManager{generation=%d, closed=%v}", rm.generation, rm.closed.Load())
}
