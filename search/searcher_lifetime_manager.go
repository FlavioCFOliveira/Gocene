package search

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Warmer is called to warm up a searcher before it's used.
type Warmer interface {
	// Warm warms up the searcher
	Warm(ctx context.Context, searcher *IndexSearcher) error
}

// WarmerFunc is an adapter to allow using a function as a Warmer.
type WarmerFunc func(ctx context.Context, searcher *IndexSearcher) error

// Warm implements the Warmer interface for WarmerFunc.
func (f WarmerFunc) Warm(ctx context.Context, searcher *IndexSearcher) error {
	return f(ctx, searcher)
}

// SearcherLifetimeManager manages the lifecycle of IndexSearcher instances,
// including warming, caching, and expiration policies.
// This is the Go port of Lucene's searcher lifetime management pattern.
type SearcherLifetimeManager struct {
	mu sync.RWMutex

	// manager is the underlying SearcherManager
	manager *SearcherManager

	// maxSearcherAge is the maximum time a searcher can be used
	maxSearcherAge time.Duration

	// maxSearchers is the maximum number of searchers to keep open
	maxSearchers int

	// searchers holds all managed searchers with their metadata
	searchers []*managedSearcher

	// warmer is called to warm up searchers before use
	warmer Warmer

	// isOpen indicates if the manager is open
	isOpen atomic.Bool

	// cleanupTicker triggers periodic cleanup
	cleanupTicker *time.Ticker

	// stopChan signals cleanup goroutine to stop
	stopChan chan struct{}

	// wg waits for cleanup goroutine
	wg sync.WaitGroup
}

// managedSearcher wraps an IndexSearcher with lifetime metadata
type managedSearcher struct {
	searcher  *IndexSearcher
	createdAt time.Time
	lastUsed  time.Time
	useCount  atomic.Int64
}

// NewSearcherLifetimeManager creates a new SearcherLifetimeManager.
func NewSearcherLifetimeManager(manager *SearcherManager, maxAge time.Duration, maxSearchers int) (*SearcherLifetimeManager, error) {
	if manager == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}

	if maxAge <= 0 {
		return nil, fmt.Errorf("maxAge must be positive")
	}

	if maxSearchers <= 0 {
		return nil, fmt.Errorf("maxSearchers must be positive")
	}

	lm := &SearcherLifetimeManager{
		manager:        manager,
		maxSearcherAge: maxAge,
		maxSearchers:   maxSearchers,
		searchers:      make([]*managedSearcher, 0),
		stopChan:       make(chan struct{}),
	}

	lm.isOpen.Store(true)

	// Start cleanup goroutine
	lm.cleanupTicker = time.NewTicker(maxAge / 2)
	lm.wg.Add(1)
	go lm.cleanupLoop()

	return lm, nil
}

// Acquire returns a searcher, creating a new one if necessary.
func (lm *SearcherLifetimeManager) Acquire(ctx context.Context) (*IndexSearcher, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if !lm.isOpen.Load() {
		return nil, fmt.Errorf("lifetime manager is closed")
	}

	// Try to get searcher from manager
	searcher, err := lm.manager.Acquire()
	if err != nil {
		return nil, fmt.Errorf("acquiring searcher: %w", err)
	}

	// Find or create managed searcher
	var ms *managedSearcher
	for _, s := range lm.searchers {
		if s.searcher == searcher {
			ms = s
			break
		}
	}

	if ms == nil {
		ms = &managedSearcher{
			searcher:  searcher,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}
		lm.searchers = append(lm.searchers, ms)
	}

	ms.lastUsed = time.Now()
	ms.useCount.Add(1)

	return searcher, nil
}

// Release releases a previously acquired searcher.
func (lm *SearcherLifetimeManager) Release(searcher *IndexSearcher) error {
	if searcher == nil {
		return nil
	}

	return lm.manager.Release(searcher)
}

// WarmSearcher warms up a searcher using the configured warmer.
func (lm *SearcherLifetimeManager) WarmSearcher(ctx context.Context, searcher *IndexSearcher) error {
	lm.mu.RLock()
	warmer := lm.warmer
	lm.mu.RUnlock()

	if warmer == nil {
		return nil
	}

	return warmer.Warm(ctx, searcher)
}

// SetWarmer sets the warmer for searchers.
func (lm *SearcherLifetimeManager) SetWarmer(warmer Warmer) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.warmer = warmer
}

// GetWarmer returns the current warmer.
func (lm *SearcherLifetimeManager) GetWarmer() Warmer {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.warmer
}

// cleanupLoop runs periodically to clean up expired searchers.
func (lm *SearcherLifetimeManager) cleanupLoop() {
	defer lm.wg.Done()

	for {
		select {
		case <-lm.stopChan:
			return
		case <-lm.cleanupTicker.C:
			lm.cleanup()
		}
	}
}

// cleanup removes expired searchers.
func (lm *SearcherLifetimeManager) cleanup() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if !lm.isOpen.Load() {
		return
	}

	now := time.Now()
	active := make([]*managedSearcher, 0)

	for _, ms := range lm.searchers {
		age := now.Sub(ms.createdAt)
		if age > lm.maxSearcherAge {
			// Expired - close it
			lm.manager.Release(ms.searcher)
		} else {
			active = append(active, ms)
		}
	}

	lm.searchers = active
}

// GetSearcherCount returns the number of active searchers.
func (lm *SearcherLifetimeManager) GetSearcherCount() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return len(lm.searchers)
}

// GetMaxSearcherAge returns the maximum searcher age.
func (lm *SearcherLifetimeManager) GetMaxSearcherAge() time.Duration {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.maxSearcherAge
}

// SetMaxSearcherAge sets the maximum searcher age.
func (lm *SearcherLifetimeManager) SetMaxSearcherAge(maxAge time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.maxSearcherAge = maxAge
	lm.cleanupTicker.Reset(maxAge / 2)
}

// GetMaxSearchers returns the maximum number of searchers.
func (lm *SearcherLifetimeManager) GetMaxSearchers() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.maxSearchers
}

// SetMaxSearchers sets the maximum number of searchers.
func (lm *SearcherLifetimeManager) SetMaxSearchers(maxSearchers int) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.maxSearchers = maxSearchers
}

// Close closes the SearcherLifetimeManager.
func (lm *SearcherLifetimeManager) Close() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if !lm.isOpen.Load() {
		return nil
	}

	lm.isOpen.Store(false)

	// Stop cleanup goroutine
	close(lm.stopChan)
	lm.cleanupTicker.Stop()
	lm.wg.Wait()

	// Release all searchers
	for _, ms := range lm.searchers {
		lm.manager.Release(ms.searcher)
	}
	lm.searchers = nil

	return nil
}

// IsOpen returns true if the manager is open.
func (lm *SearcherLifetimeManager) IsOpen() bool {
	return lm.isOpen.Load()
}

// String returns a string representation of the SearcherLifetimeManager.
func (lm *SearcherLifetimeManager) String() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return fmt.Sprintf("SearcherLifetimeManager{searchers=%d, maxAge=%v}",
		len(lm.searchers), lm.maxSearcherAge)
}
