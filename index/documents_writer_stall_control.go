package index

import (
	"sync"
	"sync/atomic"
	"time"
)

// DocumentsWriterStallControl controls the health status of a DocumentsWriter session.
// This class is used to block incoming indexing threads if flushing is significantly
// slower than indexing to ensure the DocumentsWriter's healthiness. If flushing is
// significantly slower than indexing, the net memory used within an IndexWriter
// session can increase very quickly and easily exceed the JVM's available memory.
//
// To prevent OOM Errors and ensure IndexWriter's stability, this class blocks
// incoming threads from indexing once the stall conditions are met.
type DocumentsWriterStallControl struct {
	mu         sync.Mutex
	stalled    atomic.Bool
	wasStalled bool
	numWaiting atomic.Int32
	notifier   chan struct{}
}

// NewDocumentsWriterStallControl creates a new DocumentsWriterStallControl.
func NewDocumentsWriterStallControl() *DocumentsWriterStallControl {
	return &DocumentsWriterStallControl{
		notifier: make(chan struct{}),
	}
}

// UpdateStalled updates the stalled flag status.
// This method will set the stalled flag to true iff the number of flushing
// DocumentsWriterPerThread is greater than the number of active
// DocumentsWriterPerThread. Otherwise, it will reset the control to healthy
// and release all threads waiting on WaitIfStalled.
func (s *DocumentsWriterStallControl) UpdateStalled(stalled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stalled.Load() != stalled {
		s.stalled.Store(stalled)
		if stalled {
			s.wasStalled = true
		} else {
			// Signal all waiting threads by closing the current notifier channel.
			close(s.notifier)
			// Create a new notifier channel for the next stalled period.
			s.notifier = make(chan struct{})
		}
	}
}

// WaitIfStalled blocks if documents writing is currently in a stalled state.
func (s *DocumentsWriterStallControl) WaitIfStalled() {
	if s.stalled.Load() {
		s.mu.Lock()
		if s.stalled.Load() {
			// Capture the current notifier channel while holding the lock to ensure
			// we wait on the channel that will be closed when the current stall ends.
			notifier := s.notifier
			s.mu.Unlock()

			s.numWaiting.Add(1)
			// Defensive: wait for up to 1 second here, and let the caller re-stall
			// if it's still needed. This mirrors the Java wait(1000) behavior.
			select {
			case <-notifier:
			case <-time.After(1 * time.Second):
			}
			s.numWaiting.Add(-1)
			return
		}
		s.mu.Unlock()
	}
}

// AnyStalledThreads returns true if the writer is currently stalled.
func (s *DocumentsWriterStallControl) AnyStalledThreads() bool {
	return s.stalled.Load()
}

// HasBlocked returns true if there are currently threads waiting.
func (s *DocumentsWriterStallControl) HasBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numWaiting.Load() > 0
}

// GetNumWaiting returns the current number of waiting threads.
func (s *DocumentsWriterStallControl) GetNumWaiting() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.numWaiting.Load())
}

// IsHealthy returns true if the writer is not stalled.
func (s *DocumentsWriterStallControl) IsHealthy() bool {
	return !s.stalled.Load()
}

// WasStalled returns true if the writer has been stalled at least once.
func (s *DocumentsWriterStallControl) WasStalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wasStalled
}
