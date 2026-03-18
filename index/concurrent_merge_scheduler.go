// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// AutoDetectMergesAndThreads is a sentinel value for auto-detecting merge thread count.
const AutoDetectMergesAndThreads = -1

// MinMergeMBPerSec is the floor for IO write rate limit (minimum rate).
const MinMergeMBPerSec = 5.0

// MaxMergeMBPerSec is the ceiling for IO write rate limit (maximum rate).
const MaxMergeMBPerSec = 10240.0

// StartMBPerSec is the initial value for IO write rate limit when doAutoIOThrottle is true.
const StartMBPerSec = 20.0

// MinBigMergeMB is the threshold for what constitutes a "big" merge (below this size,
// merges don't count against maxThreadCount).
const MinBigMergeMB = 50.0

// ConcurrentMergeScheduler performs merges concurrently using goroutines.
// This is the Go port of Lucene's org.apache.lucene.index.ConcurrentMergeScheduler.
//
// ConcurrentMergeScheduler runs merge operations in background goroutines,
// allowing indexing to continue while merges are in progress. It supports:
//   - Configurable number of concurrent merge threads
//   - Graceful shutdown with merge completion
//   - Merge throttling and prioritization
//   - Rate limiting for I/O operations
//   - Error handling and recovery
//
// This is the default merge scheduler for Lucene and provides the best
// performance for most use cases.
type ConcurrentMergeScheduler struct {
	*BaseMergeScheduler

	// maxThreadCount limits concurrent merge threads (default: auto-detect)
	// Set to AutoDetectMergesAndThreads for auto (based on CPU count)
	maxThreadCount int

	// maxMergeCount limits total merges (running + pending)
	maxMergeCount int

	// doAutoIOThrottle enables automatic I/O throttling
	doAutoIOThrottle bool

	// targetMBPerSec is the current IO write throttle rate
	targetMBPerSec float64

	// forceMergeMBPerSec is the rate limit for forced merges
	forceMergeMBPerSec float64

	// mergeThreads tracks active merge goroutines
	mergeThreads []*MergeThread

	// mergeThreadCounter is used for naming threads
	mergeThreadCounter int

	// pendingMerges holds merges waiting to be executed
	pendingMerges []*OneMerge

	// runningMerges tracks active merge goroutines
	runningMerges sync.WaitGroup

	// ctx controls the lifecycle of merge goroutines
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc

	// mergeErrors collects errors from merge operations
	mergeErrors chan error

	// mu protects mutable fields
	mu sync.Mutex

	// mergeMu protects merge-related state
	mergeMu sync.Mutex

	// rateLimiter limits merge I/O
	rateLimiter *MergeRateLimiter

	// intraMergeExecutor provides parallelism within a single merge
	// (not implemented yet - placeholder for future enhancement)
}

// NewConcurrentMergeScheduler creates a new ConcurrentMergeScheduler.
func NewConcurrentMergeScheduler() *ConcurrentMergeScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &ConcurrentMergeScheduler{
		BaseMergeScheduler: NewBaseMergeScheduler(),
		maxThreadCount:     AutoDetectMergesAndThreads,
		maxMergeCount:      AutoDetectMergesAndThreads,
		doAutoIOThrottle:   false,
		targetMBPerSec:     StartMBPerSec,
		forceMergeMBPerSec: float64(0), // No limit by default
		mergeThreads:       make([]*MergeThread, 0),
		pendingMerges:      make([]*OneMerge, 0),
		ctx:                ctx,
		cancel:             cancel,
		mergeErrors:        make(chan error, 10),
		rateLimiter:        NewMergeRateLimiter(),
	}

	return s
}

// MaxThreadCount returns the maximum number of merge threads.
func (s *ConcurrentMergeScheduler) MaxThreadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxThreadCount
}

// SetMaxThreadCount sets the maximum number of merge threads.
// Set to AutoDetectMergesAndThreads for auto (based on available CPUs).
func (s *ConcurrentMergeScheduler) SetMaxThreadCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxThreadCount = count
}

// MaxMergeCount returns the maximum number of concurrent merges.
func (s *ConcurrentMergeScheduler) MaxMergeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxMergeCount
}

// SetMaxMergeCount sets the maximum number of concurrent merges.
func (s *ConcurrentMergeScheduler) SetMaxMergeCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if count < 1 {
		count = 1
	}
	s.maxMergeCount = count
	s.SetMaxMerges(count)
}

// SetMaxMergesAndThreads sets both maxMergeCount and maxThreadCount.
// If both are set to AutoDetectMergesAndThreads, values are auto-detected.
func (s *ConcurrentMergeScheduler) SetMaxMergesAndThreads(maxMergeCount, maxThreadCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxMergeCount == AutoDetectMergesAndThreads && maxThreadCount == AutoDetectMergesAndThreads {
		s.maxMergeCount = AutoDetectMergesAndThreads
		s.maxThreadCount = AutoDetectMergesAndThreads
		return nil
	}

	if maxMergeCount == AutoDetectMergesAndThreads || maxThreadCount == AutoDetectMergesAndThreads {
		return fmt.Errorf("both maxMergeCount and maxThreadCount must be AutoDetectMergesAndThreads or both must be positive")
	}

	if maxThreadCount < 1 {
		return fmt.Errorf("maxThreadCount should be at least 1")
	}
	if maxMergeCount < 1 {
		return fmt.Errorf("maxMergeCount should be at least 1")
	}
	if maxThreadCount > maxMergeCount {
		return fmt.Errorf("maxThreadCount should be <= maxMergeCount (= %d)", maxMergeCount)
	}

	s.maxThreadCount = maxThreadCount
	s.maxMergeCount = maxMergeCount
	return nil
}

// SetDefaultMaxMergesAndThreads sets defaults for rotational or non-rotational storage.
func (s *ConcurrentMergeScheduler) SetDefaultMaxMergesAndThreads(spins bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if spins {
		// Traditional spinning disk: single merge thread
		s.maxThreadCount = 1
		s.maxMergeCount = 6
	} else {
		// SSD or similar: use CPU count
		coreCount := runtime.NumCPU()
		s.maxThreadCount = max(1, coreCount/2)
		s.maxMergeCount = s.maxThreadCount + 5
	}
}

// SetForceMergeMBPerSec sets the per-merge IO throttle rate for forced merges.
func (s *ConcurrentMergeScheduler) SetForceMergeMBPerSec(mbPerSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forceMergeMBPerSec = mbPerSec
}

// GetForceMergeMBPerSec returns the per-merge IO throttle rate for forced merges.
func (s *ConcurrentMergeScheduler) GetForceMergeMBPerSec() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forceMergeMBPerSec
}

// SetAutoIOThrottle enables or disables automatic I/O throttling.
func (s *ConcurrentMergeScheduler) SetAutoIOThrottle(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doAutoIOThrottle = enabled
}

// GetAutoIOThrottle returns whether automatic I/O throttling is enabled.
func (s *ConcurrentMergeScheduler) GetAutoIOThrottle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doAutoIOThrottle
}

// getEffectiveMaxThreadCount gets the effective thread count, handling auto-detect.
func (s *ConcurrentMergeScheduler) getEffectiveMaxThreadCount() int {
	s.mu.Lock()
	threadCount := s.maxThreadCount
	s.mu.Unlock()

	if threadCount == AutoDetectMergesAndThreads {
		// Auto-detect: use half of CPU cores
		coreCount := runtime.NumCPU()
		threadCount = max(1, coreCount/2)
	}
	return threadCount
}

// getEffectiveMaxMergeCount gets the effective merge count, handling auto-detect.
func (s *ConcurrentMergeScheduler) getEffectiveMaxMergeCount() int {
	s.mu.Lock()
	mergeCount := s.maxMergeCount
	threadCount := s.maxThreadCount
	s.mu.Unlock()

	if mergeCount == AutoDetectMergesAndThreads {
		// Auto-detect
		if threadCount == AutoDetectMergesAndThreads {
			coreCount := runtime.NumCPU()
			threadCount = max(1, coreCount/2)
		}
		mergeCount = threadCount + 5
	}
	return mergeCount
}

// Merge runs the merges from the source using background goroutines.
// This implements the MergeScheduler interface.
func (s *ConcurrentMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
	if s.IsClosed() {
		return NewAlreadyClosedException("merge scheduler is closed", nil)
	}

	// Get effective thread and merge counts
	maxThreadCount := s.getEffectiveMaxThreadCount()
	maxMergeCount := s.getEffectiveMaxMergeCount()

	// Main merge loop
	for {
		// Maybe stall if too many pending merges
		if err := s.maybeStall(source, maxMergeCount); err != nil {
			return err
		}

		// Get next merge
		merge := source.GetNextMerge()
		if merge == nil {
			break
		}

		// Check if we should spawn a new merge thread
		s.mergeMu.Lock()
		activeThreads := len(s.mergeThreads)
		s.mergeMu.Unlock()

		if activeThreads < maxThreadCount {
			// Spawn new merge thread
			s.spawnMergeThread(source, merge)
		} else {
			// Wait for a thread to finish, then continue
			s.waitForMergeThread()
			// Re-queue this merge for later
			s.mergeMu.Lock()
			s.pendingMerges = append(s.pendingMerges, merge)
			s.mergeMu.Unlock()
		}
	}

	// Wait for all running merges to complete
	s.waitForAllMerges()

	return nil
}

// maybeStall stalls the calling goroutine if there are too many pending merges.
func (s *ConcurrentMergeScheduler) maybeStall(source MergeSource, maxMergeCount int) error {
	s.mergeMu.Lock()
	pendingCount := len(s.mergeThreads) + len(s.pendingMerges)
	s.mergeMu.Unlock()

	// If we're over the limit, wait
	for pendingCount >= maxMergeCount {
		if s.IsClosed() {
			return NewAlreadyClosedException("merge scheduler is closed", nil)
		}

		// Wait for a merge to complete
		s.waitForMergeThread()

		s.mergeMu.Lock()
		pendingCount = len(s.mergeThreads) + len(s.pendingMerges)
		s.mergeMu.Unlock()
	}

	return nil
}

// spawnMergeThread starts a new goroutine to execute a merge.
func (s *ConcurrentMergeScheduler) spawnMergeThread(source MergeSource, merge *OneMerge) {
	s.mu.Lock()
	s.mergeThreadCounter++
	threadName := fmt.Sprintf("MergeThread-%d", s.mergeThreadCounter)
	s.mu.Unlock()

	thread := NewMergeThread(threadName, merge)
	thread.SetRunning(true)

	s.mergeMu.Lock()
	s.mergeThreads = append(s.mergeThreads, thread)
	s.mergeMu.Unlock()

	s.runningMerges.Add(1)
	s.IncrementRunningMerges()

	go func() {
		defer s.runningMerges.Done()
		defer s.DecrementRunningMerges()
		defer func() { thread.SetRunning(false); close(thread.done) }()

		// Execute the merge
		err := s.executeMerge(source, merge)
		thread.SetError(err)

		// Signal completion
		source.OnMergeFinished(merge)

		// Remove from active threads
		s.removeMergeThread(thread)

		if err != nil {
			select {
			case s.mergeErrors <- err:
			default:
				// Error channel full, drop the error
			}
		}
	}()
}

// removeMergeThread removes a thread from the active list.
func (s *ConcurrentMergeScheduler) removeMergeThread(thread *MergeThread) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()

	for i, t := range s.mergeThreads {
		if t == thread {
			s.mergeThreads = append(s.mergeThreads[:i], s.mergeThreads[i+1:]...)
			break
		}
	}
}

// waitForMergeThread waits for any merge thread to complete.
func (s *ConcurrentMergeScheduler) waitForMergeThread() {
	s.mergeMu.Lock()
	// Get a reference to one of the running threads
	var threadToWait *MergeThread
	if len(s.mergeThreads) > 0 {
		threadToWait = s.mergeThreads[0]
	}
	s.mergeMu.Unlock()

	if threadToWait != nil {
		// Wait for the thread's done channel
		select {
		case <-threadToWait.Done():
			// Thread completed
		case <-time.After(30 * time.Second):
			// Timeout - continue anyway
		}
	} else {
		// No threads running, just yield
		time.Sleep(1 * time.Millisecond)
	}
}

// waitForAllMerges waits for all running merges to complete.
func (s *ConcurrentMergeScheduler) waitForAllMerges() {
	s.runningMerges.Wait()
}

// executeMerge performs the actual merge operation.
func (s *ConcurrentMergeScheduler) executeMerge(source MergeSource, merge *OneMerge) error {
	// Check for cancellation
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("merge cancelled due to scheduler shutdown")
	default:
	}

	// Execute the merge via the source
	err := source.Merge(merge)
	if err != nil {
		return NewMergeException("merge failed", err, merge)
	}

	return nil
}

// Close waits for all running merges to complete and shuts down the scheduler.
func (s *ConcurrentMergeScheduler) Close() error {
	s.mu.Lock()
	if s.IsClosed() {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Signal shutdown
	s.cancel()

	// Wait for all merges to complete with timeout
	done := make(chan struct{})
	go func() {
		s.runningMerges.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// All merges completed
	case <-time.After(60 * time.Second):
		// Timeout waiting for merges
		return fmt.Errorf("timeout waiting for merges to complete")
	}

	return s.BaseMergeScheduler.Close()
}

// CloseWithContext closes the scheduler with a custom context for timeout control.
func (s *ConcurrentMergeScheduler) CloseWithContext(ctx context.Context) error {
	s.mu.Lock()
	if s.IsClosed() {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Signal shutdown
	s.cancel()

	// Wait for all merges to complete
	done := make(chan struct{})
	go func() {
		s.runningMerges.Wait()
		close(done)
	}()

	// Wait with context
	select {
	case <-done:
		// All merges completed
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for merges: %w", ctx.Err())
	}

	return s.BaseMergeScheduler.Close()
}

// GetPendingMergeCount returns the number of pending merges in the queue.
func (s *ConcurrentMergeScheduler) GetPendingMergeCount() int {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	return len(s.pendingMerges)
}

// GetActiveThreadCount returns the number of active merge threads.
func (s *ConcurrentMergeScheduler) GetActiveThreadCount() int {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	return len(s.mergeThreads)
}

// GetMergeErrors returns any errors that occurred during merges.
// This drains the error channel.
func (s *ConcurrentMergeScheduler) GetMergeErrors() []error {
	var errors []error
	for {
		select {
		case err := <-s.mergeErrors:
			errors = append(errors, err)
		default:
			return errors
		}
	}
}

// String returns a string representation of the ConcurrentMergeScheduler.
func (s *ConcurrentMergeScheduler) String() string {
	return fmt.Sprintf("ConcurrentMergeScheduler(maxThreadCount=%d, maxMergeCount=%d, activeThreads=%d, running=%d, pending=%d)",
		s.getEffectiveMaxThreadCount(),
		s.getEffectiveMaxMergeCount(),
		s.GetActiveThreadCount(),
		s.GetRunningMergeCount(),
		s.GetPendingMergeCount())
}

// WrapForMerge wraps a Directory for merge operations with rate limiting.
// This is a simplified implementation.
func (s *ConcurrentMergeScheduler) WrapForMerge(merge *OneMerge, directory store.Directory) store.Directory {
	// In a full implementation, this would wrap the directory with rate limiting
	return directory
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
