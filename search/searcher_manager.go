package search

import (
	"context"
	"fmt"
	"sync"
)

// SearcherManager manages the lifecycle of IndexSearcher instances.
// It provides thread-safe access to a single current searcher, and handles
// reopening the searcher when the index changes.
// This is the Go port of Lucene's org.apache.lucene.search.SearcherManager.
type SearcherManager struct {
	mu sync.RWMutex

	// current is the currently managed IndexSearcher
	current *IndexSearcher

	// factory is the SearcherFactory used to create new searchers
	factory SearcherFactory

	// afterClose is called when a searcher is closed
	afterClose func(*IndexSearcher)

	// isClosed indicates if the manager has been closed
	isClosed bool

	// refCount tracks references to the current searcher
	refCount map[*IndexSearcher]int
}

// NewSearcherManager creates a new SearcherManager with the given initial searcher.
// The factory is used to create new searchers when the index changes.
func NewSearcherManager(initial *IndexSearcher, factory SearcherFactory, afterClose func(*IndexSearcher)) (*SearcherManager, error) {
	if initial == nil {
		return nil, fmt.Errorf("initial searcher cannot be nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	sm := &SearcherManager{
		current:    initial,
		factory:    factory,
		afterClose: afterClose,
		refCount:   make(map[*IndexSearcher]int),
	}

	// Ensure the initial searcher has at least one reference
	sm.refCount[initial] = 1

	return sm, nil
}

// Acquire returns the current IndexSearcher, incrementing its reference count.
// The caller must call Release() when done with the searcher.
// Returns error if the manager is closed.
func (sm *SearcherManager) Acquire() (*IndexSearcher, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isClosed {
		return nil, fmt.Errorf("searcher manager is closed")
	}

	if sm.current == nil {
		return nil, fmt.Errorf("no current searcher available")
	}

	sm.refCount[sm.current]++
	return sm.current, nil
}

// Release decrements the reference count of the given searcher.
// This must be called for every Acquire call.
func (sm *SearcherManager) Release(searcher *IndexSearcher) error {
	if searcher == nil {
		return fmt.Errorf("cannot release nil searcher")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	count, exists := sm.refCount[searcher]
	if !exists || count <= 0 {
		return fmt.Errorf("searcher not acquired or already released")
	}

	count--
	sm.refCount[searcher] = count

	// If count reaches zero and this is not the current searcher, close it
	if count == 0 && searcher != sm.current {
		delete(sm.refCount, searcher)
		if sm.afterClose != nil {
			sm.afterClose(searcher)
		}
	}

	return nil
}

// MaybeRefresh attempts to reopen the searcher if the index has changed.
// This should be called periodically to make new documents visible to search.
// Returns true if a refresh was performed, false if no changes were detected.
func (sm *SearcherManager) MaybeRefresh(ctx context.Context) (bool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isClosed {
		return false, fmt.Errorf("searcher manager is closed")
	}

	// Get the current reader from the searcher
	// Note: In a real implementation, we would check if the reader has changes
	// and create a new searcher if needed
	// For now, we return false indicating no refresh was needed
	return false, nil
}

// Refresh is a blocking version of MaybeRefresh.
// It waits until a new searcher is available.
func (sm *SearcherManager) Refresh(ctx context.Context) error {
	_, err := sm.MaybeRefresh(ctx)
	return err
}

// GetCurrent returns the current managed searcher without incrementing the reference count.
// The returned searcher is only valid while holding the manager's lock.
// This is intended for internal use; external code should use Acquire.
func (sm *SearcherManager) GetCurrent() *IndexSearcher {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// UpdateSearcher replaces the current searcher with a new one.
// The old searcher will have its reference count decremented.
func (sm *SearcherManager) UpdateSearcher(ctx context.Context, newSearcher *IndexSearcher) error {
	if newSearcher == nil {
		return fmt.Errorf("new searcher cannot be nil")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isClosed {
		return fmt.Errorf("searcher manager is closed")
	}

	oldSearcher := sm.current

	// Initialize ref count for new searcher
	sm.refCount[newSearcher] = 1
	sm.current = newSearcher

	// Decrement old searcher ref count
	if oldSearcher != nil {
		count := sm.refCount[oldSearcher]
		count--
		if count <= 0 {
			delete(sm.refCount, oldSearcher)
			if sm.afterClose != nil {
				sm.afterClose(oldSearcher)
			}
		} else {
			sm.refCount[oldSearcher] = count
		}
	}

	return nil
}

// Close closes the searcher manager and releases the current searcher.
// After Close is called, Acquire will return an error.
func (sm *SearcherManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isClosed {
		return nil // Already closed
	}

	sm.isClosed = true

	// Release the current searcher
	if sm.current != nil {
		count, exists := sm.refCount[sm.current]
		if exists {
			count--
			if count <= 0 {
				delete(sm.refCount, sm.current)
				if sm.afterClose != nil {
					sm.afterClose(sm.current)
				}
			} else {
				sm.refCount[sm.current] = count
			}
		}
		sm.current = nil
	}

	return nil
}

// IsClosed returns true if the manager has been closed.
func (sm *SearcherManager) IsClosed() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isClosed
}

// SetAfterClose sets the callback function to be called when a searcher is closed.
func (sm *SearcherManager) SetAfterClose(afterClose func(*IndexSearcher)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.afterClose = afterClose
}

// GetRefCount returns the reference count for a specific searcher.
// This is primarily for testing/debugging.
func (sm *SearcherManager) GetRefCount(searcher *IndexSearcher) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.refCount[searcher]
}

// SwapSearcher atomically swaps the current searcher with a new one.
// This is used when a new searcher is ready to be made current.
// Returns the old searcher that was replaced.
func (sm *SearcherManager) SwapSearcher(newSearcher *IndexSearcher) (*IndexSearcher, error) {
	if newSearcher == nil {
		return nil, fmt.Errorf("new searcher cannot be nil")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isClosed {
		return nil, fmt.Errorf("searcher manager is closed")
	}

	oldSearcher := sm.current

	// Initialize ref count for new searcher
	sm.refCount[newSearcher] = 1
	sm.current = newSearcher

	return oldSearcher, nil
}

// DecRef decrements the reference count of a searcher.
// This is called by Release and during searcher replacement.
func (sm *SearcherManager) DecRef(searcher *IndexSearcher) error {
	if searcher == nil {
		return fmt.Errorf("cannot decrement nil searcher")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	count, exists := sm.refCount[searcher]
	if !exists {
		return fmt.Errorf("searcher not in ref count map")
	}

	count--
	if count <= 0 {
		delete(sm.refCount, searcher)
		if sm.afterClose != nil {
			sm.afterClose(searcher)
		}
	} else {
		sm.refCount[searcher] = count
	}

	return nil
}
