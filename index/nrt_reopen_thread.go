package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTReopenThread periodically reopens NRT readers to provide near real-time search.
// This is a simpler alternative to ControlledRealTimeReopenThread.
type NRTReopenThread struct {
	mu sync.RWMutex

	// name is the thread name
	name string

	// reopenFunc is called to reopen the reader
	reopenFunc func() error

	// interval is the reopen interval
	interval time.Duration

	// isRunning indicates if the thread is running
	isRunning atomic.Bool

	// stopChan signals the thread to stop
	stopChan chan struct{}

	// wg waits for goroutines
	wg sync.WaitGroup

	// reopenCount tracks the number of reopens
	reopenCount int64

	// lastReopenTime tracks when the last reopen occurred
	lastReopenTime time.Time

	// lastException stores the last error
	lastException error

	// applyAllDeletes indicates if all deletes should be applied
	applyAllDeletes bool
}

// NewNRTReopenThread creates a new NRTReopenThread.
func NewNRTReopenThread(name string, interval time.Duration, reopenFunc func() error) (*NRTReopenThread, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive")
	}

	if reopenFunc == nil {
		return nil, fmt.Errorf("reopenFunc cannot be nil")
	}

	thread := &NRTReopenThread{
		name:            name,
		interval:        interval,
		reopenFunc:      reopenFunc,
		stopChan:        make(chan struct{}),
		lastReopenTime:  time.Now(),
		applyAllDeletes: true,
	}

	return thread, nil
}

// Start starts the reopen thread.
func (t *NRTReopenThread) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isRunning.Load() {
		return fmt.Errorf("thread %s is already running", t.name)
	}

	t.isRunning.Store(true)

	// Start background goroutine
	t.wg.Add(1)
	go t.run()

	return nil
}

// Stop stops the reopen thread.
func (t *NRTReopenThread) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isRunning.Load() {
		return nil
	}

	t.isRunning.Store(false)
	close(t.stopChan)
	t.wg.Wait()

	return nil
}

// IsRunning returns true if the thread is running.
func (t *NRTReopenThread) IsRunning() bool {
	return t.isRunning.Load()
}

// run is the main thread loop.
func (t *NRTReopenThread) run() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.doReopen()
		}
	}
}

// doReopen performs the reopen operation.
func (t *NRTReopenThread) doReopen() {
	err := t.reopenFunc()

	t.mu.Lock()
	defer t.mu.Unlock()

	if err != nil {
		t.lastException = err
	} else {
		t.reopenCount++
		t.lastReopenTime = time.Now()
		t.lastException = nil
	}
}

// Trigger manually triggers a reopen.
func (t *NRTReopenThread) Trigger() error {
	t.mu.RLock()
	if !t.isRunning.Load() {
		t.mu.RUnlock()
		return fmt.Errorf("thread %s is not running", t.name)
	}
	t.mu.RUnlock()

	t.doReopen()
	return nil
}

// GetReopenCount returns the number of reopens.
func (t *NRTReopenThread) GetReopenCount() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.reopenCount
}

// GetLastReopenTime returns the time of the last reopen.
func (t *NRTReopenThread) GetLastReopenTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.lastReopenTime
}

// GetLastException returns the last exception.
func (t *NRTReopenThread) GetLastException() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.lastException
}

// GetName returns the thread name.
func (t *NRTReopenThread) GetName() string {
	return t.name
}

// GetInterval returns the reopen interval.
func (t *NRTReopenThread) GetInterval() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.interval
}

// SetInterval sets the reopen interval.
func (t *NRTReopenThread) SetInterval(interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.interval = interval
	return nil
}

// GetApplyAllDeletes returns whether all deletes should be applied.
func (t *NRTReopenThread) GetApplyAllDeletes() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.applyAllDeletes
}

// SetApplyAllDeletes sets whether all deletes should be applied.
func (t *NRTReopenThread) SetApplyAllDeletes(apply bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.applyAllDeletes = apply
}

// GetTimeSinceLastReopen returns the time since the last reopen.
func (t *NRTReopenThread) GetTimeSinceLastReopen() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return time.Since(t.lastReopenTime)
}

// String returns a string representation of the NRTReopenThread.
func (t *NRTReopenThread) String() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return fmt.Sprintf("NRTReopenThread{name=%s, running=%v, interval=%v, reopens=%d}",
		t.name, t.isRunning.Load(), t.interval, t.reopenCount)
}
