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

	// threadDoneCond is signaled when a merge thread completes
	// Used by waitForMergeThread to avoid busy-waiting
	threadDoneCond *sync.Cond

	// rateLimiter limits merge I/O
	rateLimiter store.RateLimiter

	// maxFullFlushMergeWaitMillis is the maximum time (in milliseconds) that
	// a full flush will wait for merges. A negative value means merges run
	// asynchronously without blocking the flush; 0 means do not wait.
	maxFullFlushMergeWaitMillis int64

	// suppressExceptions, when true, causes background merge threads to
	// swallow non-abort merge exceptions instead of reporting them to the
	// IndexWriter. This matches Lucene's ConcurrentMergeScheduler
	// setSuppressExceptions testing hook.
	suppressExceptions bool
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
		mergeThreads:       make([]*MergeThread, 0, 4),
		pendingMerges:      make([]*OneMerge, 0, 8),
		ctx:                ctx,
		cancel:             cancel,
		mergeErrors:        make(chan error, 10),
		rateLimiter:        store.NewSimpleRateLimiter(StartMBPerSec),
	}

	// Initialize condition variable for thread completion signaling
	s.threadDoneCond = sync.NewCond(&s.mergeMu)

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

// SetMaxFullFlushMergeWaitMillis sets the maximum time (in milliseconds) that a
// full flush will wait for merges to complete. Negative means do not wait.
func (s *ConcurrentMergeScheduler) SetMaxFullFlushMergeWaitMillis(ms int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxFullFlushMergeWaitMillis = ms
}

// GetMaxFullFlushMergeWaitMillis returns the maximum time (in milliseconds)
// that a full flush will wait for merges.
func (s *ConcurrentMergeScheduler) GetMaxFullFlushMergeWaitMillis() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxFullFlushMergeWaitMillis
}

// SetSuppressExceptions enables suppression of background merge exceptions.
// While suppressed, merge goroutines still fail but do not propagate errors
// to the IndexWriter. This is Lucene's testing-only hook.
func (s *ConcurrentMergeScheduler) SetSuppressExceptions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suppressExceptions = true
}

// ClearSuppressExceptions disables suppression of background merge exceptions.
func (s *ConcurrentMergeScheduler) ClearSuppressExceptions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suppressExceptions = false
}

// IsSuppressExceptions reports whether merge exceptions are currently suppressed.
func (s *ConcurrentMergeScheduler) IsSuppressExceptions() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.suppressExceptions
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
	return s.MergeInternal(source, trigger, false)
}

// MergeInternal is a helper that allows Merge to be called with a flag indicating
// if the caller is a merge thread, avoiding deadlock in maybeStall.
func (s *ConcurrentMergeScheduler) MergeInternal(source MergeSource, trigger MergeTrigger, isMergeThread bool) error {
	if s.IsClosed() {
		return NewAlreadyClosedException("merge scheduler is closed", nil)
	}

	// Get effective thread and merge counts
	maxThreadCount := s.getEffectiveMaxThreadCount()
	maxMergeCount := s.getEffectiveMaxMergeCount()

	// Main merge loop. Drain the scheduler's internal pending queue first so
	// merges that were re-queued because the thread limit was reached are not
	// lost when source.GetNextMerge runs dry.
	for {
		// Maybe stall if too many pending merges
		if err := s.maybeStall(source, maxMergeCount, isMergeThread); err != nil {
			return err
		}

		// Prefer internal pending merges over asking the source so that
		// re-queued merges are processed before new ones.
		var merge *OneMerge
		s.mergeMu.Lock()
		if len(s.pendingMerges) > 0 {
			merge = s.pendingMerges[0]
			s.pendingMerges = s.pendingMerges[1:]
		}
		s.mergeMu.Unlock()

		if merge == nil {
			merge = source.GetNextMerge()
			if merge == nil {
				break
			}
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

// MergeWithSpec runs the specific merges provided by MergeSpecification.
func (s *ConcurrentMergeScheduler) MergeWithSpec(source MergeSource, spec *MergeSpecification, doWait bool) error {
	if s.IsClosed() {
		return NewAlreadyClosedException("merge scheduler is closed", nil)
	}

	s.mergeMu.Lock()
	// Add all merges from the specification to the pending queue
	s.pendingMerges = append(s.pendingMerges, spec.Merges...)
	s.mergeMu.Unlock()

	// Trigger a merge run to start processing the newly added merges
	// Since Merge() is the main entry point that spawns threads, we can just call it
	// with a dummy source that returns nil.
	dummySource := &dummyMergeSource{}
	if err := s.Merge(dummySource, MergeTriggerExplicit); err != nil {
		return err
	}

	if doWait {
		if !spec.Await() {
			return fmt.Errorf("one or more merges failed")
		}
	}

	return nil
}

type dummyMergeSource struct{}

func (d *dummyMergeSource) GetNextMerge() *OneMerge     { return nil }
func (d *dummyMergeSource) OnMergeFinished(*OneMerge)   {}
func (d *dummyMergeSource) HasPendingMerges() bool      { return false }
func (d *dummyMergeSource) Merge(merge *OneMerge) error { return fmt.Errorf("not implemented") }

// maybeStall stalls the calling goroutine if there are too many pending merges.
// This implements the deadlock prevention logic from Lucene's maybeStall() (lines 616-651),
// ensuring that merge threads are never stalled.
func (s *ConcurrentMergeScheduler) maybeStall(source MergeSource, maxMergeCount int, isMergeThread bool) error {
	if isMergeThread {
		return nil
	}

	for {
		s.mergeMu.Lock()
		activeThreads := len(s.mergeThreads)
		s.mergeMu.Unlock()

		if activeThreads < maxMergeCount || !source.HasPendingMerges() {
			break
		}

		s.doStall()
	}

	return nil
}

// doStall pauses the calling goroutine when merging has fallen behind.
// This is the Go analogue of Lucene's protected
// ConcurrentMergeScheduler.doStall() hook; subclasses may override it to
// implement custom throttling. The default implementation waits on the
// scheduler's condition variable until at least one merge thread finishes.
func (s *ConcurrentMergeScheduler) doStall() {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()

	// Wait briefly so that a finishing thread can notify us. If no thread
	// finishes within the defensive timeout we loop and re-check.
	if s.threadDoneCond != nil {
		s.threadDoneCond.Wait()
	}
}

// spawnMergeThread starts a new goroutine to execute a merge.
func (s *ConcurrentMergeScheduler) spawnMergeThread(source MergeSource, merge *OneMerge) {
	s.mu.Lock()
	s.mergeThreadCounter++
	threadName := fmt.Sprintf("MergeThread-%d", s.mergeThreadCounter)
	s.mu.Unlock()

	thread := NewMergeThread(threadName, merge)
	thread.RateLimiter = NewMergeRateLimiter(merge.Progress)
	thread.SetRunning(true)

	s.updateIOThrottle(merge, thread.RateLimiter)

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
		if err != nil {
			merge.Error = err
		}

		// In Java, runOnMergeFinished is called after doMerge.
		// It handles removing the thread, calling merge() again for new merges,
		// and notifying waiters.
		s.runOnMergeFinished(source, merge, thread)

		// Always notify the merge source so bookkeeping (and observer
		// notification) happens regardless of success or failure.
		source.OnMergeFinished(merge)

		if err != nil {
			s.mu.Lock()
			suppress := s.suppressExceptions
			s.mu.Unlock()
			if suppress {
				// Swallow the error, matching Lucene's setSuppressExceptions.
				return
			}
			select {
			case s.mergeErrors <- err:
			default:
				// Error channel full, drop the error
			}
			return
		}
	}()
}

// removeMergeThread removes a thread from the active list.
// Signals threadDoneCond to wake up any waiters.
func (s *ConcurrentMergeScheduler) removeMergeThread(thread *MergeThread) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()

	for i, t := range s.mergeThreads {
		if t == thread {
			s.mergeThreads = append(s.mergeThreads[:i], s.mergeThreads[i+1:]...)
			break
		}
	}

	// Signal that a thread completed - wakes up waitForMergeThread
	if s.threadDoneCond != nil {
		s.threadDoneCond.Broadcast()
	}
}

// waitForMergeThread waits for any merge thread to complete.
// Uses sync.Cond for efficient waiting instead of busy-wait with sleep.
func (s *ConcurrentMergeScheduler) waitForMergeThread() {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()

	// Wait while there are active threads
	// The condition is checked in a loop because spurious wakeups can occur
	for len(s.mergeThreads) > 0 {
		s.threadDoneCond.Wait()
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

	// Execute the merge via the overridable hook (Lucene's protected
	// ConcurrentMergeScheduler.doMerge() extension point).
	if err := s.doMerge(source, merge); err != nil {
		return NewMergeException("merge failed", err, merge)
	}

	s.mergeSuccess(source, merge)
	return nil
}

// doMerge performs the actual merge by calling source.Merge(merge).
// Subclasses may override this hook to wrap or intercept merge execution,
// matching Lucene's protected ConcurrentMergeScheduler.doMerge() hook.
func (s *ConcurrentMergeScheduler) doMerge(source MergeSource, merge *OneMerge) error {
	return source.Merge(merge)
}

// mergeSuccess is called when a merge successfully completes.
// Subclasses may override this hook to perform additional bookkeeping.
func (s *ConcurrentMergeScheduler) mergeSuccess(source MergeSource, merge *OneMerge) {
	// Default: no-op
}

// runOnMergeFinished is called when a merge thread finishes.
// Mirrors Lucene's ConcurrentMergeScheduler.runOnMergeFinished().
func (s *ConcurrentMergeScheduler) runOnMergeFinished(source MergeSource, merge *OneMerge, thread *MergeThread) {
	// Let CMS run new merges if necessary:
	s.MergeInternal(source, MergeTriggerMergeFinished, true)

	// Remove the thread from the active list.
	s.removeMergeThread(thread)
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

	// Wait for all running merges to drain, bounded by a timeout.
	if err := s.awaitMergesOrTimeout(60 * time.Second); err != nil {
		return err
	}

	return s.BaseMergeScheduler.Close()
}

// awaitMergesOrTimeout blocks until all running merges have drained or the
// timeout elapses, returning an error on timeout.
//
// The watcher goroutine must not outlive this call. A goroutine blocked in
// sync.WaitGroup.Wait cannot be cancelled, so on the timeout path it would leak
// (surviving until the stuck merges happened to finish, or forever). Instead a
// context is cancelled when this method returns and the watcher polls the
// running-merge count, exiting promptly when either the count reaches zero or
// the context is cancelled. This guarantees no goroutine is leaked on timeout.
func (s *ConcurrentMergeScheduler) awaitMergesOrTimeout(timeout time.Duration) error {
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if s.GetRunningMergeCount() == 0 {
				return
			}
			select {
			case <-watchCtx.Done():
				// The caller is returning (e.g. on timeout); stop watching so this
				// goroutine exits cleanly instead of leaking.
				return
			case <-ticker.C:
			}
		}
	}()

	// On timeout the deferred cancelWatch unblocks the watcher before we return.
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for merges to complete")
	}
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

	// Wait for running merges to drain, bounded by the caller's context. Poll on
	// this goroutine (selecting on ctx.Done) rather than spawning a watcher that
	// blocks in runningMerges.Wait(): the old approach leaked that goroutine when
	// the context was cancelled before merges finished (rmp #4748).
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for s.GetRunningMergeCount() != 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for merges: %w", ctx.Err())
		case <-ticker.C:
		}
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

// AbortAll cancels all currently running merges and clears the pending merge queue.
func (s *ConcurrentMergeScheduler) AbortAll() {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	for _, thread := range s.mergeThreads {
		thread.Abort()
	}
	s.pendingMerges = s.pendingMerges[:0]
	if s.threadDoneCond != nil {
		s.threadDoneCond.Broadcast()
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
// Each CreateOutput call on the returned directory wraps the underlying output
// with a RateLimitedIndexOutput driven by a MergeRateLimiter created for this merge.
func (s *ConcurrentMergeScheduler) WrapForMerge(merge *OneMerge, directory store.Directory) store.Directory {
	rl := NewMergeRateLimiter(merge.Progress)
	s.mu.Lock()
	rate := s.targetMBPerSec
	s.mu.Unlock()
	rl.SetMBPerSec(rate)

	return newRateLimitedMergeDirectory(directory, rl)
}

// rateLimitedMergeDirectory is a FilterDirectory that wraps every CreateOutput
// with a RateLimitedIndexOutput using the supplied RateLimiter.
type rateLimitedMergeDirectory struct {
	*store.FilterDirectory
	adapter store.RateLimiter
}

func newRateLimitedMergeDirectory(dir store.Directory, rl store.RateLimiter) *rateLimitedMergeDirectory {
	return &rateLimitedMergeDirectory{
		FilterDirectory: store.NewFilterDirectory(dir),
		adapter:         rl,
	}
}

// CreateOutput returns a rate-limited index output for merge writes.
func (d *rateLimitedMergeDirectory) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	out, err := d.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	return store.NewRateLimitedIndexOutput(d.adapter, out), nil
}

// updateMergeThreads updates the merge thread scheduling, particularly for IO throttling.
// This method sorts merge threads by size in descending order and ensures
// smaller merges run before larger ones when thread count exceeds maxThreadCount.
// Based on Lucene's ConcurrentMergeScheduler.updateMergeThreads() (lines 319-441).
//
// In this Go port, the rate limiting logic is simplified compared to Lucene,
// as goroutines and channels do not require the same explicit thread-pausing mechanism.
func (s *ConcurrentMergeScheduler) updateMergeThreads() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only look at threads that are alive and not in the process of stopping.
	activeMerges := make([]*MergeThread, 0, len(s.mergeThreads))
	for _, t := range s.mergeThreads {
		if t != nil && t.IsRunning() {
			activeMerges = append(activeMerges, t)
		}
	}

	// Sort merge threads by estimated merge bytes in descending order (largest first).
	// This matches Lucene's timSort with a Comparator.
	// We use insertion sort for stability and to match Lucene's behavior.
	for i := 1; i < len(activeMerges); i++ {
		key := activeMerges[i]
		j := i - 1
		for j >= 0 && activeMerges[j].Merge.EstimatedMergeBytes.Load() < key.Merge.EstimatedMergeBytes.Load() {
			activeMerges[j+1] = activeMerges[j]
			j--
		}
		activeMerges[j+1] = key
	}

	// Count how many merges are "big" (exceed MIN_BIG_MERGE_MB threshold).
	activeMergeCount := len(activeMerges)
	bigMergeCount := 0
	for threadIdx := activeMergeCount - 1; threadIdx >= 0; threadIdx-- {
		if float64(activeMerges[threadIdx].Merge.EstimatedMergeBytes.Load()) > MinBigMergeMB*1024*1024 {
			bigMergeCount = 1 + threadIdx
			break
		}
	}

	// Log or track throttling decisions if verbose.
	// In this Go port, we don't perform explicit thread pausing; instead,
	// the scheduler respects the thread limits via maybeStall().
	// Future enhancement: implement rate limiting similar to Lucene if needed.
	_ = bigMergeCount // Suppress unused variable warning; kept for Lucene fidelity
}

// isBacklog returns true if there are other similarly sized merges running
// that have been active for more than 3 seconds, suggesting a backlog.
// Based on Lucene's ConcurrentMergeScheduler.isBacklog() (lines 805-823).
func (s *ConcurrentMergeScheduler) isBacklog(now int64, merge *OneMerge) bool {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	return s.isBacklogLocked(now, merge)
}

func (s *ConcurrentMergeScheduler) isBacklogLocked(now int64, merge *OneMerge) bool {
	mergeMB := float64(merge.EstimatedMergeBytes.Load()) / 1024.0 / 1024.0
	for _, t := range s.mergeThreads {
		startNS := t.Merge.MergeStartNS
		if t.IsRunning() &&
			t.Merge != merge &&
			startNS != -1 &&
			float64(t.Merge.EstimatedMergeBytes.Load()) > MinBigMergeMB*1024*1024 &&
			float64(now-startNS)/1e9 > 3.0 {

			otherMB := float64(t.Merge.EstimatedMergeBytes.Load()) / 1024.0 / 1024.0
			ratio := otherMB / mergeMB
			if ratio > 0.3 && ratio < 3.0 {
				return true
			}
		}
	}
	return false
}

// updateIOThrottle tunes the IO throttle rate when a new merge starts.
// Based on Lucene's ConcurrentMergeScheduler.updateIOThrottle() (lines 825-930).
func (s *ConcurrentMergeScheduler) updateIOThrottle(newMerge *OneMerge, rateLimiter store.RateLimiter) error {
	s.mu.Lock()
	doAuto := s.doAutoIOThrottle
	maxThreadCount := s.maxThreadCount
	s.mu.Unlock()

	if !doAuto {
		return nil
	}

	mergeMB := float64(newMerge.EstimatedMergeBytes.Load()) / 1024.0 / 1024.0
	if mergeMB < MinBigMergeMB {
		return nil
	}

	now := time.Now().UnixNano()
	newBacklog := s.isBacklog(now, newMerge)
	curBacklog := false

	if !newBacklog {
		s.mergeMu.Lock()
		if len(s.mergeThreads) > maxThreadCount {
			curBacklog = true
		} else {
			for _, t := range s.mergeThreads {
				if s.isBacklogLocked(now, t.Merge) {
					curBacklog = true
					break
				}
			}
		}
		s.mergeMu.Unlock()
	}

	s.mu.Lock()
	if newBacklog {
		s.targetMBPerSec *= 1.20
		if s.targetMBPerSec > MaxMergeMBPerSec {
			s.targetMBPerSec = MaxMergeMBPerSec
		}
	} else if !curBacklog {
		s.targetMBPerSec /= 1.10
		if s.targetMBPerSec < MinMergeMBPerSec {
			s.targetMBPerSec = MinMergeMBPerSec
		}
	}

	target := s.targetMBPerSec
	force := s.forceMergeMBPerSec
	s.mu.Unlock()

	var rate float64
	if newMerge.MaxNumSegments != -1 {
		rate = force
	} else {
		rate = target
	}
	rateLimiter.SetMBPerSec(rate)
	s.targetMBPerSecChanged()

	return nil
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
