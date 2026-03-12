// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
)

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
	// Merge schedules a merge to be executed.
	// The implementation may execute the merge synchronously or asynchronously.
	Merge(writer *IndexWriter, merge *OneMerge) error

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
}

// NewBaseMergeScheduler creates a new BaseMergeScheduler.
func NewBaseMergeScheduler() *BaseMergeScheduler {
	return &BaseMergeScheduler{
		maxMerges:     1,
		runningMerges: 0,
		closed:        false,
	}
}

// Merge schedules a merge (must be implemented by subclasses).
func (s *BaseMergeScheduler) Merge(writer *IndexWriter, merge *OneMerge) error {
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

// incrementRunningMerges increments the count of running merges.
func (s *BaseMergeScheduler) incrementRunningMerges() int {
	return int(atomic.AddInt32(&s.runningMerges, 1))
}

// decrementRunningMerges decrements the count of running merges.
func (s *BaseMergeScheduler) decrementRunningMerges() int {
	return int(atomic.AddInt32(&s.runningMerges, -1))
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
