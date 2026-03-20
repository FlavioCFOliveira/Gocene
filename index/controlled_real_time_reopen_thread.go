package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ControlledRealTimeReopenThread is a background thread that periodically reopens
// the NRT reader to make new documents visible for search. It provides controlled
// real-time reopening with configurable intervals and rate limiting.
// This is the Go port of Lucene's ControlledRealTimeReopenThread.
type ControlledRealTimeReopenThread struct {
	mu sync.RWMutex

	// manager is the NRTManager to reopen
	manager *NRTManager

	// minStaleSec is the minimum time between reopens when stale
	minStaleSec time.Duration

	// maxStaleSec is the maximum time between reopens when stale
	maxStaleSec time.Duration

	// isRunning indicates if the thread is running
	isRunning atomic.Bool

	// stopChan is used to signal the thread to stop
	stopChan chan struct{}

	// wg waits for the goroutine to finish
	wg sync.WaitGroup

	// lastReopenTime is the time of the last reopen
	lastReopenTime time.Time

	// reopenCount tracks the number of reopens
	reopenCount int64

	// errorHandler is called when reopen fails
	errorHandler func(error)
}

// NewControlledRealTimeReopenThread creates a new ControlledRealTimeReopenThread.
// minStaleSec is the minimum time between reopens when the reader is stale.
// maxStaleSec is the maximum time between reopens (acts as a safety net).
func NewControlledRealTimeReopenThread(manager *NRTManager, minStaleSec, maxStaleSec time.Duration) (*ControlledRealTimeReopenThread, error) {
	if manager == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}

	if minStaleSec <= 0 {
		return nil, fmt.Errorf("minStaleSec must be positive")
	}

	if maxStaleSec <= 0 {
		return nil, fmt.Errorf("maxStaleSec must be positive")
	}

	if maxStaleSec < minStaleSec {
		return nil, fmt.Errorf("maxStaleSec must be >= minStaleSec")
	}

	return &ControlledRealTimeReopenThread{
		manager:     manager,
		minStaleSec: minStaleSec,
		maxStaleSec: maxStaleSec,
		stopChan:    make(chan struct{}),
	}, nil
}

// Start starts the reopen thread.
func (t *ControlledRealTimeReopenThread) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRunning.Load() {
		return fmt.Errorf("thread is already running")
	}

	t.isRunning.Store(true)
	t.stopChan = make(chan struct{})
	t.wg.Add(1)

	go t.run()

	return nil
}

// Stop stops the reopen thread.
// Blocks until the thread has finished.
func (t *ControlledRealTimeReopenThread) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning.Load() {
		return nil
	}

	t.isRunning.Store(false)
	close(t.stopChan)

	// Wait for goroutine to finish
	t.wg.Wait()

	return nil
}

// IsRunning returns true if the thread is running.
func (t *ControlledRealTimeReopenThread) IsRunning() bool {
	return t.isRunning.Load()
}

// run is the main loop of the reopen thread.
func (t *ControlledRealTimeReopenThread) run() {
	defer t.wg.Done()

	// Calculate initial sleep time
	sleepTime := t.minStaleSec

	for {
		select {
		case <-t.stopChan:
			return
		case <-time.After(sleepTime):
			// Time to check for reopen
		}

		// Check if we should reopen
		shouldReopen, nextSleepTime := t.shouldReopen()

		if shouldReopen {
			// Perform reopen
			if err := t.doReopen(); err != nil {
				if t.errorHandler != nil {
					t.errorHandler(err)
				}
			}
		}

		sleepTime = nextSleepTime
	}
}

// shouldReopen determines if we should reopen and how long to sleep next.
func (t *ControlledRealTimeReopenThread) shouldReopen() (bool, time.Duration) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Check if enough time has passed since last reopen
	timeSinceLastReopen := time.Since(t.lastReopenTime)
	if timeSinceLastReopen < t.minStaleSec {
		// Not enough time passed, sleep for remaining time
		return false, t.minStaleSec - timeSinceLastReopen
	}

	// Check if reader is stale
	reader, err := t.manager.GetReader()
	if err != nil {
		// Error getting reader, try again after minStaleSec
		return false, t.minStaleSec
	}

	isCurrent, err := reader.IsCurrent()
	if err != nil {
		// Error checking if current, try again after minStaleSec
		return false, t.minStaleSec
	}

	if isCurrent {
		// Reader is current, no need to reopen
		// Sleep for maxStaleSec as a safety check
		return false, t.maxStaleSec
	}

	// Reader is stale and enough time has passed, should reopen
	return true, t.minStaleSec
}

// doReopen performs the actual reopen operation.
func (t *ControlledRealTimeReopenThread) doReopen() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	refreshed, err := t.manager.MaybeRefresh(ctx)
	if err != nil {
		return err
	}

	if refreshed {
		t.mu.Lock()
		t.lastReopenTime = time.Now()
		t.reopenCount++
		t.mu.Unlock()
	}

	return nil
}

// ForceReopen forces an immediate reopen.
func (t *ControlledRealTimeReopenThread) ForceReopen() error {
	if !t.isRunning.Load() {
		return fmt.Errorf("thread is not running")
	}

	return t.doReopen()
}

// GetReopenCount returns the number of reopens performed.
func (t *ControlledRealTimeReopenThread) GetReopenCount() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.reopenCount
}

// GetLastReopenTime returns the time of the last reopen.
func (t *ControlledRealTimeReopenThread) GetLastReopenTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastReopenTime
}

// SetErrorHandler sets the error handler for reopen failures.
func (t *ControlledRealTimeReopenThread) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorHandler = handler
}

// GetMinStaleSec returns the minimum stale time.
func (t *ControlledRealTimeReopenThread) GetMinStaleSec() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.minStaleSec
}

// GetMaxStaleSec returns the maximum stale time.
func (t *ControlledRealTimeReopenThread) GetMaxStaleSec() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.maxStaleSec
}

// WaitForGeneration waits for the manager to reach the specified generation.
// This is a convenience method that delegates to the manager.
func (t *ControlledRealTimeReopenThread) WaitForGeneration(ctx context.Context, generation int64) (*NRTDirectoryReader, error) {
	if !t.isRunning.Load() {
		return nil, fmt.Errorf("thread is not running")
	}

	return t.manager.WaitForGeneration(ctx, generation)
}
