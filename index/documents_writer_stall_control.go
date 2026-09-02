//go:build ignore

package index

import (
	"sync"
	"time"
	"sync/atomic"
)

// documentsWriterStallControl coordinates indexing threads with flushing.
// When flushing significantly lags indexing, incoming indexers are blocked.
//
// Port of org.apache.lucene.index.DocumentsWriterStallControl.
type documentsWriterStallControl struct {
	mu      sync.Mutex
	cond    *sync.Cond
	stalled atomic.Bool

	// assert-only fields
	numWaiting int
	wasStalledFlag bool
}

// newDocumentsWriterStallControl constructs a fresh stall controller.
func newDocumentsWriterStallControl() *documentsWriterStallControl {
	s := &documentsWriterStallControl{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// UpdateStalled toggles the stalled state. When transitioning to unstalled,
// all blocked waiters are released.
func (s *documentsWriterStallControl) UpdateStalled(stalled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stalled.Load() == stalled {
		return
	}
	s.stalled.Store(stalled)
	if stalled {
		s.wasStalledFlag = true
	}
	s.cond.Broadcast()
}

// WaitIfStalled blocks until the controller becomes unstalled or up to 1s.
func (s *documentsWriterStallControl) WaitIfStalled() {
	if !s.stalled.Load() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stalled.Load() {
		return
	}
	s.numWaiting++
	// Mirror Lucene: bounded wait with a 1s safety to recover from any
	// missed broadcast. We use a timer that broadcasts the cond on expiry.
	done := make(chan struct{})
	t := time.AfterFunc(time.Second, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
		close(done)
	})
	s.cond.Wait()
	t.Stop()
	select {
	case <-done:
	default:
	}
	s.numWaiting--
}

// AnyStalledThreads reports whether the controller is currently stalled.
func (s *documentsWriterStallControl) AnyStalledThreads() bool {
	return s.stalled.Load()
}

// HasBlocked reports whether any thread is currently blocked. Test-only.
func (s *documentsWriterStallControl) HasBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numWaiting > 0
}

// GetNumWaiting returns the number of currently blocked threads. Test-only.
func (s *documentsWriterStallControl) GetNumWaiting() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numWaiting
}

// IsHealthy reports the inverse of AnyStalledThreads. Test-only.
func (s *documentsWriterStallControl) IsHealthy() bool {
	return !s.stalled.Load()
}

// WasStalled reports whether the controller has ever been stalled. Test-only.
func (s *documentsWriterStallControl) WasStalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wasStalledFlag
}
