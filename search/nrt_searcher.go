package search

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// NRTSearcher provides Near Real-Time (NRT) search capabilities.
// It wraps an IndexSearcher and allows for real-time search results.
type NRTSearcher struct {
	mu sync.RWMutex

	// searcher is the underlying IndexSearcher
	searcher *IndexSearcher

	// version is the searcher version for NRT tracking
	version int64

	// isOpen indicates if the searcher is open
	isOpen atomic.Bool

	// lastRefreshTime tracks when the searcher was last refreshed
	lastRefreshTime time.Time

	// refreshCount tracks the number of refreshes
	refreshCount int64

	// manager is the SearcherManager for acquiring searchers
	manager *SearcherManager

	// autoRefresh indicates if auto-refresh is enabled
	autoRefresh bool

	// refreshInterval is the auto-refresh interval
	refreshInterval time.Duration

	// stopChan signals the auto-refresh goroutine to stop
	stopChan chan struct{}

	// wg waits for goroutines
	wg sync.WaitGroup
}

// NewNRTSearcher creates a new NRTSearcher.
func NewNRTSearcher(manager *SearcherManager) (*NRTSearcher, error) {
	if manager == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}

	// Acquire initial searcher
	searcher, err := manager.Acquire()
	if err != nil {
		return nil, fmt.Errorf("acquiring searcher: %w", err)
	}

	nrt := &NRTSearcher{
		searcher:        searcher,
		version:         1,
		lastRefreshTime: time.Now(),
		manager:         manager,
		stopChan:        make(chan struct{}),
	}

	nrt.isOpen.Store(true)

	return nrt, nil
}

// GetSearcher returns the underlying IndexSearcher.
func (nrt *NRTSearcher) GetSearcher() *IndexSearcher {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.searcher
}

// GetVersion returns the searcher version.
func (nrt *NRTSearcher) GetVersion() int64 {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.version
}

// IncrementVersion increments the searcher version.
func (nrt *NRTSearcher) IncrementVersion() {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	nrt.version++
}

// Refresh refreshes the searcher to see the latest changes.
func (nrt *NRTSearcher) Refresh(ctx context.Context) error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.isOpen.Load() {
		return fmt.Errorf("searcher is closed")
	}

	// Release current searcher
	if nrt.searcher != nil {
		nrt.manager.Release(nrt.searcher)
	}

	// Acquire new searcher
	newSearcher, err := nrt.manager.Acquire()
	if err != nil {
		return fmt.Errorf("acquiring new searcher: %w", err)
	}

	nrt.searcher = newSearcher
	nrt.version++
	nrt.lastRefreshTime = time.Now()
	nrt.refreshCount++

	return nil
}

// GetRefreshCount returns the number of refreshes.
func (nrt *NRTSearcher) GetRefreshCount() int64 {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.refreshCount
}

// GetLastRefreshTime returns the time of the last refresh.
func (nrt *NRTSearcher) GetLastRefreshTime() time.Time {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.lastRefreshTime
}

// StartAutoRefresh starts automatic refreshing at the given interval.
func (nrt *NRTSearcher) StartAutoRefresh(interval time.Duration) error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.isOpen.Load() {
		return fmt.Errorf("searcher is closed")
	}

	if nrt.autoRefresh {
		return fmt.Errorf("auto-refresh is already running")
	}

	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	nrt.autoRefresh = true
	nrt.refreshInterval = interval

	// Start auto-refresh goroutine
	nrt.wg.Add(1)
	go nrt.autoRefreshLoop()

	return nil
}

// StopAutoRefresh stops automatic refreshing.
func (nrt *NRTSearcher) StopAutoRefresh() error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.autoRefresh {
		return nil
	}

	nrt.autoRefresh = false
	close(nrt.stopChan)
	nrt.wg.Wait()

	return nil
}

// IsAutoRefreshRunning returns true if auto-refresh is running.
func (nrt *NRTSearcher) IsAutoRefreshRunning() bool {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.autoRefresh
}

// autoRefreshLoop is the auto-refresh goroutine.
func (nrt *NRTSearcher) autoRefreshLoop() {
	defer nrt.wg.Done()

	ticker := time.NewTicker(nrt.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-nrt.stopChan:
			return
		case <-ticker.C:
			ctx := context.Background()
			nrt.Refresh(ctx)
		}
	}
}

// Search performs a search with the given query.
func (nrt *NRTSearcher) Search(query Query, topN int) (*TopDocs, error) {
	nrt.mu.RLock()
	if !nrt.isOpen.Load() {
		nrt.mu.RUnlock()
		return nil, fmt.Errorf("searcher is closed")
	}
	searcher := nrt.searcher
	nrt.mu.RUnlock()

	return searcher.Search(query, topN)
}

// GetIndexReader returns the underlying IndexReader.
func (nrt *NRTSearcher) GetIndexReader() index.IndexReaderInterface {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	if nrt.searcher == nil {
		return nil
	}

	return nrt.searcher.GetIndexReader()
}

// GetManager returns the SearcherManager.
func (nrt *NRTSearcher) GetManager() *SearcherManager {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.manager
}

// IsOpen returns true if the searcher is open.
func (nrt *NRTSearcher) IsOpen() bool {
	return nrt.isOpen.Load()
}

// Close closes the NRTSearcher.
func (nrt *NRTSearcher) Close() error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.isOpen.Load() {
		return nil
	}

	nrt.isOpen.Store(false)

	// Stop auto-refresh if running
	if nrt.autoRefresh {
		close(nrt.stopChan)
		nrt.wg.Wait()
	}

	// Release searcher
	if nrt.searcher != nil {
		nrt.manager.Release(nrt.searcher)
		nrt.searcher = nil
	}

	return nil
}

// String returns a string representation of the NRTSearcher.
func (nrt *NRTSearcher) String() string {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return fmt.Sprintf("NRTSearcher{open=%v, version=%d, refreshes=%d, auto=%v}",
		nrt.isOpen.Load(), nrt.version, nrt.refreshCount, nrt.autoRefresh)
}
