// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// MergeSource provides access to new merges and executes the actual merge.
// This is the Go port of Lucene's org.apache.lucene.index.MergeScheduler.MergeSource.
//
// MergeSource is implemented by IndexWriter to provide merges to the MergeScheduler.
// The scheduler calls GetNextMerge to retrieve pending merges and calls Merge
// to execute them.
type MergeSource interface {
	// GetNextMerge returns the next pending merge, or nil if none.
	// The MergeScheduler calls this method repeatedly until it returns nil.
	GetNextMerge() *OneMerge

	// OnMergeFinished is called when a merge completes.
	// This allows the MergeSource to update its internal state.
	OnMergeFinished(merge *OneMerge)

	// HasPendingMerges returns true if there are pending merges.
	HasPendingMerges() bool

	// Merge executes the merge operation.
	// This is called by the MergeScheduler to perform the actual merge.
	Merge(merge *OneMerge) error
}

// MergeScheduler schedules background merges.
// This is the Go port of Lucene's org.apache.lucene.index.MergeScheduler.
//
// MergeScheduler is responsible for executing merge operations, typically
// in background threads/goroutines. The two main implementations are:
//   - SerialMergeScheduler: Runs merges synchronously in the calling thread
//   - ConcurrentMergeScheduler: Runs merges in background goroutines
//
// Users can also implement custom merge schedulers for specific needs.
type MergeScheduler interface {
	// Merge runs the merges provided by MergeSource.
	// The implementation may execute merges synchronously or asynchronously.
	Merge(source MergeSource, trigger MergeTrigger) error

	// Close closes the scheduler, waiting for any running merges to complete.
	Close() error

	// GetRunningMergeCount returns the number of currently running merges.
	GetRunningMergeCount() int

	// SetMaxMerges sets the maximum number of concurrent merges.
	SetMaxMerges(maxMerges int)

	// GetMaxMerges returns the maximum number of concurrent merges.
	GetMaxMerges() int
}

// MergeProgress tracks the progress of a merge operation.
type MergeProgress struct {
	// TotalDocs is the total number of documents to merge.
	TotalDocs int

	// MergedDocs is the number of documents merged so far.
	MergedDocs int

	// IsAborted is true if the merge was aborted.
	IsAborted bool

	// Error holds any error that occurred during the merge.
	Error error

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewMergeProgress creates a new MergeProgress.
func NewMergeProgress(totalDocs int) *MergeProgress {
	return &MergeProgress{
		TotalDocs:  totalDocs,
		MergedDocs: 0,
		IsAborted:  false,
		Error:      nil,
	}
}

// GetProgress returns the merge progress as a percentage (0.0 to 100.0).
func (mp *MergeProgress) GetProgress() float64 {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if mp.TotalDocs == 0 {
		return 100.0
	}

	return float64(mp.MergedDocs) / float64(mp.TotalDocs) * 100.0
}

// SetProgress sets the number of merged documents.
func (mp *MergeProgress) SetProgress(mergedDocs int) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.MergedDocs = mergedDocs
}

// IncrementProgress increments the merged document count.
func (mp *MergeProgress) IncrementProgress(delta int) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.MergedDocs += delta
}

// Abort marks the merge as aborted.
func (mp *MergeProgress) Abort() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.IsAborted = true
}

// SetError sets the error that occurred during the merge.
func (mp *MergeProgress) SetError(err error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.Error = err
}

// GetError returns any error that occurred during the merge.
func (mp *MergeProgress) GetError() error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.Error
}

// OneMergeProgress provides progress and state for an executing merge.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.OneMergeProgress.
//
// This class encapsulates the logic to pause and resume the merge thread
// or to abort the merge entirely.
type OneMergeProgress struct {
	// aborted indicates if the merge has been aborted
	aborted atomic.Bool

	// pauseLock protects pause-related state
	pauseLock sync.Mutex

	// pauseCond is used to wait/pause during merges
	pauseCond *sync.Cond

	// pauseTimesNS tracks pause times for each reason
	pauseTimesNS map[PauseReason]int64

	// owner is the goroutine that owns this merge (for sanity checking)
	owner goroutineID
}

// PauseReason represents the reason for pausing a merge.
type PauseReason int

const (
	// STOPPED indicates the merge is stopped (typically because throughput rate is 0).
	STOPPED PauseReason = iota
	// PAUSED indicates the merge is temporarily paused due to exceeded throughput rate.
	PAUSED
	// OTHER indicates other pause reasons.
	OTHER
)

// goroutineID is a simple identifier for the owning goroutine.
type goroutineID int64

var currentGoroutineID int64

func getGoroutineID() goroutineID {
	return goroutineID(atomic.AddInt64(&currentGoroutineID, 1))
}

// NewOneMergeProgress creates a new OneMergeProgress.
func NewOneMergeProgress() *OneMergeProgress {
	p := &OneMergeProgress{
		pauseTimesNS: make(map[PauseReason]int64),
	}
	p.pauseCond = sync.NewCond(&p.pauseLock)
	return p
}

// Abort aborts the merge at the next possible moment.
func (p *OneMergeProgress) Abort() {
	p.aborted.Store(true)
	p.Wakeup() // wakeup any paused merge thread
}

// IsAborted returns true if the merge has been aborted.
func (p *OneMergeProgress) IsAborted() bool {
	return p.aborted.Load()
}

// CheckAborted returns an error if the merge has been aborted.
func (p *OneMergeProgress) CheckAborted() error {
	if p.IsAborted() {
		return fmt.Errorf("merge aborted")
	}
	return nil
}

// PauseNanos pauses the calling goroutine for at least pauseNanos nanoseconds.
// Returns immediately if the merge is aborted or if condition returns false.
func (p *OneMergeProgress) PauseNanos(pauseNanos int64, reason PauseReason, condition func() bool) error {
	start := int64(0) // would be System.nanoTime() in Java

	p.pauseLock.Lock()
	defer p.pauseLock.Unlock()

	remaining := pauseNanos
	for remaining > 0 && !p.IsAborted() && (condition == nil || condition()) {
		// In Go, we use a different approach for waiting
		// Convert to milliseconds for the wait
		waitMs := remaining / 1_000_000
		if waitMs < 1 {
			waitMs = 1
		}
		p.pauseCond.Wait()
		remaining -= pauseNanos // This is simplified; in real implementation we'd track actual time
	}

	// Update pause time
	p.pauseTimesNS[reason] += start // Simplified

	if p.IsAborted() {
		return fmt.Errorf("merge aborted")
	}
	return nil
}

// Wakeup requests a wakeup for any goroutines paused in PauseNanos.
func (p *OneMergeProgress) Wakeup() {
	p.pauseLock.Lock()
	defer p.pauseLock.Unlock()
	p.pauseCond.Broadcast()
}

// GetPauseTimes returns the pause times for each reason in nanoseconds.
func (p *OneMergeProgress) GetPauseTimes() map[PauseReason]int64 {
	p.pauseLock.Lock()
	defer p.pauseLock.Unlock()

	result := make(map[PauseReason]int64)
	for k, v := range p.pauseTimesNS {
		result[k] = v
	}
	return result
}

// SetMergeThread sets the owner of this merge progress.
func (p *OneMergeProgress) SetMergeThread(owner goroutineID) {
	p.owner = owner
}

// BaseMergeScheduler provides common functionality for merge schedulers.
type BaseMergeScheduler struct {
	// maxMerges is the maximum number of concurrent merges
	maxMerges int

	// runningMerges is the count of currently running merges
	runningMerges int32

	// mu protects mutable fields
	mu sync.Mutex

	// closed indicates if the scheduler is closed
	closed bool

	// infoStream for logging
	infoStream InfoStream
}

// NewBaseMergeScheduler creates a new BaseMergeScheduler.
func NewBaseMergeScheduler() *BaseMergeScheduler {
	return &BaseMergeScheduler{
		maxMerges:     1,
		runningMerges: 0,
		closed:        false,
	}
}

// Merge runs the merges from the source (must be implemented by subclasses).
func (s *BaseMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
	return fmt.Errorf("Merge not implemented")
}

// Close closes the scheduler.
func (s *BaseMergeScheduler) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

// IsClosed returns true if the scheduler is closed.
func (s *BaseMergeScheduler) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// SetMaxMerges sets the maximum number of concurrent merges.
func (s *BaseMergeScheduler) SetMaxMerges(maxMerges int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if maxMerges < 1 {
		maxMerges = 1
	}
	s.maxMerges = maxMerges
}

// GetMaxMerges returns the maximum number of concurrent merges.
func (s *BaseMergeScheduler) GetMaxMerges() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxMerges
}

// GetRunningMergeCount returns the number of currently running merges.
func (s *BaseMergeScheduler) GetRunningMergeCount() int {
	return int(atomic.LoadInt32(&s.runningMerges))
}

// IncrementRunningMerges increments the count of running merges.
func (s *BaseMergeScheduler) IncrementRunningMerges() int {
	return int(atomic.AddInt32(&s.runningMerges, 1))
}

// DecrementRunningMerges decrements the count of running merges.
func (s *BaseMergeScheduler) DecrementRunningMerges() int {
	return int(atomic.AddInt32(&s.runningMerges, -1))
}

// SetInfoStream sets the info stream for logging.
func (s *BaseMergeScheduler) SetInfoStream(infoStream InfoStream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoStream = infoStream
}

// GetInfoStream returns the info stream for logging.
func (s *BaseMergeScheduler) GetInfoStream() InfoStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoStream
}

// Verbose returns true if info stream messages are enabled.
func (s *BaseMergeScheduler) Verbose() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoStream != nil && s.infoStream.IsEnabled("MS")
}

// Message outputs a message to the info stream.
func (s *BaseMergeScheduler) Message(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.infoStream != nil {
		s.infoStream.Message("MS", msg)
	}
}

// InfoStream is an interface for logging messages.
type InfoStream interface {
	IsEnabled(component string) bool
	Message(component string, msg string)
}

// SerialMergeScheduler is a MergeScheduler that runs merges sequentially
// in the current goroutine.
// This is the Go port of Lucene's org.apache.lucene.index.SerialMergeScheduler.
//
// SerialMergeScheduler is useful for testing or when you want merges to
// run synchronously without background goroutines.
type SerialMergeScheduler struct {
	*BaseMergeScheduler

	// mu ensures only one merge runs at a time
	mu sync.Mutex
}

// NewSerialMergeScheduler creates a new SerialMergeScheduler.
func NewSerialMergeScheduler() *SerialMergeScheduler {
	return &SerialMergeScheduler{
		BaseMergeScheduler: NewBaseMergeScheduler(),
	}
}

// Merge runs the merges from the source sequentially.
// This is synchronized so that only one merge runs at a time.
func (s *SerialMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		merge := source.GetNextMerge()
		if merge == nil {
			break
		}

		if err := source.Merge(merge); err != nil {
			return err
		}

		source.OnMergeFinished(merge)
	}

	return nil
}

// Close closes the scheduler.
func (s *SerialMergeScheduler) Close() error {
	return s.BaseMergeScheduler.Close()
}

// GetRunningMergeCount returns 0 since SerialMergeScheduler runs merges synchronously.
func (s *SerialMergeScheduler) GetRunningMergeCount() int {
	return 0
}

// MergeException represents an error that occurred during a merge.
type MergeException struct {
	Message string
	Cause   error
	Merge   *OneMerge
}

// Error returns the error message.
func (e *MergeException) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying cause.
func (e *MergeException) Unwrap() error {
	return e.Cause
}

// NewMergeException creates a new MergeException.
func NewMergeException(message string, cause error, merge *OneMerge) *MergeException {
	return &MergeException{
		Message: message,
		Cause:   cause,
		Merge:   merge,
	}
}

// MergeThread represents a goroutine that is executing a merge.
// This is used by ConcurrentMergeScheduler to track active merges.
type MergeThread struct {
	// Name is a descriptive name for the merge thread
	Name string

	// Merge is the OneMerge being executed
	Merge *OneMerge

	// Running indicates if the thread is still running
	Running bool

	// StartedNS is the start time in nanoseconds
	StartedNS int64

	// ctx is the context for cancellation
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc

	// done is closed when the merge completes
	done chan struct{}

	// err holds any error that occurred
	err error

	// mu protects mutable fields
	mu sync.Mutex
}

// NewMergeThread creates a new MergeThread.
func NewMergeThread(name string, merge *OneMerge) *MergeThread {
	ctx, cancel := context.WithCancel(context.Background())
	return &MergeThread{
		Name:      name,
		Merge:     merge,
		Running:   false,
		StartedNS: 0,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// IsRunning returns true if the thread is still running.
func (t *MergeThread) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Running
}

// SetRunning sets the running state.
func (t *MergeThread) SetRunning(running bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Running = running
}

// Cancel cancels the merge thread.
func (t *MergeThread) Cancel() {
	t.cancel()
}

// Done returns a channel that is closed when the merge completes.
func (t *MergeThread) Done() <-chan struct{} {
	return t.done
}

// Wait waits for the merge to complete.
func (t *MergeThread) Wait() error {
	<-t.done
	return t.GetError()
}

// GetError returns any error that occurred.
func (t *MergeThread) GetError() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// SetError sets the error.
func (t *MergeThread) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.err = err
}

// MergeRateLimiter limits the rate of merge I/O.
// This is the Go port of Lucene's MergeRateLimiter.
type MergeRateLimiter struct {
	// mbPerSec is the target MB per second
	mbPerSec float64

	// minPauseCheckBytes is the minimum bytes between pause checks
	minPauseCheckBytes int64

	// totalBytesWritten is the total bytes written
	totalBytesWritten int64

	// lastPauseNS is the last pause time in nanoseconds
	lastPauseNS int64

	// mu protects mutable fields
	mu sync.Mutex
}

// NewMergeRateLimiter creates a new MergeRateLimiter.
func NewMergeRateLimiter() *MergeRateLimiter {
	return &MergeRateLimiter{
		mbPerSec:           float64(20.0), // Default: 20 MB/s
		minPauseCheckBytes: 100 * 1024,    // Check every 100KB
	}
}

// SetMBPerSec sets the target rate in MB per second.
func (r *MergeRateLimiter) SetMBPerSec(mbPerSec float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mbPerSec = mbPerSec
}

// GetMBPerSec returns the target rate in MB per second.
func (r *MergeRateLimiter) GetMBPerSec() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mbPerSec
}

// Pause pauses if necessary to maintain the target rate.
// This is a simplified implementation.
func (r *MergeRateLimiter) Pause(bytesWritten int64, progress *OneMergeProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalBytesWritten += bytesWritten

	// If rate is 0 or negative, don't throttle
	if r.mbPerSec <= 0 {
		return nil
	}

	// Check if we need to pause
	// This is a simplified implementation; real rate limiting would track time
	// and pause based on bytes written vs time elapsed

	return nil
}

// TotalBytesWritten returns the total bytes written.
func (r *MergeRateLimiter) TotalBytesWritten() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalBytesWritten
}
