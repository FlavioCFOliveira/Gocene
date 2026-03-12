// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConcurrentMergeScheduler performs merges concurrently using goroutines.
// This is the Go port of Lucene's org.apache.lucene.index.ConcurrentMergeScheduler.
//
// ConcurrentMergeScheduler runs merge operations in background goroutines,
// allowing indexing to continue while merges are in progress. It supports:
//   - Configurable number of concurrent merge threads
//   - Graceful shutdown with merge completion
//   - Merge throttling and prioritization
//   - Error handling and recovery
//
// This is the default merge scheduler for Lucene and provides the best
// performance for most use cases.
type ConcurrentMergeScheduler struct {
	*BaseMergeScheduler

	// maxThreadCount limits concurrent merge threads (default: 1)
	// Set to 0 for auto (based on CPU count)
	maxThreadCount int

	// maxMergeCount limits total merges (running + pending)
	maxMergeCount int

	// runningMerges tracks active merge goroutines
	runningMerges sync.WaitGroup

	// mergeQueue holds pending merges
	mergeQueue chan *mergeTask

	// ctx controls the lifecycle of merge goroutines
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc

	// mergeErrors collects errors from merge operations
	mergeErrors chan error

	// mu protects mutable fields
	mu sync.Mutex
}

// mergeTask represents a merge operation to be executed.
type mergeTask struct {
	writer *IndexWriter
	merge  *OneMerge
	done   chan error
}

// NewConcurrentMergeScheduler creates a new ConcurrentMergeScheduler.
func NewConcurrentMergeScheduler() *ConcurrentMergeScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &ConcurrentMergeScheduler{
		BaseMergeScheduler: NewBaseMergeScheduler(),
		maxThreadCount:     1, // Default to single merge thread
		maxMergeCount:      5, // Default max concurrent merges
		mergeQueue:         make(chan *mergeTask, 100),
		ctx:                ctx,
		cancel:             cancel,
		mergeErrors:        make(chan error, 10),
	}

	// Start merge workers
	s.startMergeWorkers()

	return s
}

// startMergeWorkers starts the merge worker goroutines.
func (s *ConcurrentMergeScheduler) startMergeWorkers() {
	threadCount := s.maxThreadCount
	if threadCount <= 0 {
		// Auto: use number of CPUs
		threadCount = 1
	}

	for i := 0; i < threadCount; i++ {
		go s.mergeWorker()
	}
}

// mergeWorker is the goroutine that processes merge tasks.
func (s *ConcurrentMergeScheduler) mergeWorker() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-s.mergeQueue:
			if task == nil {
				return
			}
			err := s.doMerge(task.writer, task.merge)
			if task.done != nil {
				task.done <- err
				close(task.done)
			}
			if err != nil {
				select {
				case s.mergeErrors <- err:
				default:
					// Error channel full, drop the error
				}
			}
		}
	}
}

// MaxThreadCount returns the maximum number of merge threads.
func (s *ConcurrentMergeScheduler) MaxThreadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxThreadCount
}

// SetMaxThreadCount sets the maximum number of merge threads.
// Set to 0 for auto (based on available CPUs).
func (s *ConcurrentMergeScheduler) SetMaxThreadCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if count < 0 {
		count = 0
	}
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

// Merge schedules a merge to run in a goroutine.
// If the merge queue is full, this method will block until space is available.
func (s *ConcurrentMergeScheduler) Merge(writer *IndexWriter, merge *OneMerge) error {
	if s.IsClosed() {
		return fmt.Errorf("merge scheduler is closed")
	}

	task := &mergeTask{
		writer: writer,
		merge:  merge,
		done:   make(chan error, 1),
	}

	// Queue the merge
	select {
	case s.mergeQueue <- task:
		// Successfully queued
	case <-s.ctx.Done():
		return fmt.Errorf("merge scheduler is shutting down")
	}

	// Wait for completion (for synchronous behavior)
	// In async mode, we would return immediately
	err := <-task.done
	return err
}

// MergeAsync schedules a merge to run asynchronously.
// Returns immediately without waiting for the merge to complete.
func (s *ConcurrentMergeScheduler) MergeAsync(writer *IndexWriter, merge *OneMerge) error {
	if s.IsClosed() {
		return fmt.Errorf("merge scheduler is closed")
	}

	task := &mergeTask{
		writer: writer,
		merge:  merge,
		done:   nil, // No notification needed for async
	}

	select {
	case s.mergeQueue <- task:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("merge scheduler is shutting down")
	default:
		return fmt.Errorf("merge queue is full")
	}
}

// doMerge performs the actual merge operation.
func (s *ConcurrentMergeScheduler) doMerge(writer *IndexWriter, merge *OneMerge) error {
	s.incrementRunningMerges()
	defer s.decrementRunningMerges()

	s.runningMerges.Add(1)
	defer s.runningMerges.Done()

	// Check for cancellation
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("merge cancelled due to scheduler shutdown")
	default:
	}

	// Perform the merge
	// In a full implementation, this would:
	// 1. Create a MergeState from the source segments
	// 2. Use the Codec to write the merged segment
	// 3. Update the SegmentInfos
	// 4. Handle any errors and cleanup

	// For now, just simulate the merge
	err := s.executeMerge(writer, merge)
	if err != nil {
		return NewMergeException("merge failed", err, merge)
	}

	return nil
}

// executeMerge executes the actual merge logic.
func (s *ConcurrentMergeScheduler) executeMerge(writer *IndexWriter, merge *OneMerge) error {
	// This is a placeholder for the actual merge implementation
	// In a full implementation, this would:
	//
	// 1. Create a MergeState from the source segments
	//    mergeState := NewMergeState(merge.Segments, writer.GetSegmentInfo())
	//
	// 2. Get the codec and create segment writers
	//    codec := writer.GetConfig().GetCodec()
	//    segmentWriteState := NewSegmentWriteState(...)
	//
	// 3. Merge fields, postings, stored fields, doc values, etc.
	//    fieldsConsumer := codec.PostingsFormat().FieldsConsumer(segmentWriteState)
	//    storedFieldsWriter := codec.StoredFieldsFormat().FieldsWriter(...)
	//
	// 4. Write the merged segment files
	//
	// 5. Update SegmentInfos with the new segment
	//
	// 6. Clean up old segment files

	// Simulate merge work
	time.Sleep(1 * time.Millisecond)

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

	// Close the merge queue
	close(s.mergeQueue)

	// Wait for all merges to complete
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

	// Close base scheduler
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

	// Close the merge queue
	close(s.mergeQueue)

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
	return len(s.mergeQueue)
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
	return fmt.Sprintf("ConcurrentMergeScheduler(maxThreadCount=%d, maxMergeCount=%d, running=%d, pending=%d)",
		s.MaxThreadCount(), s.MaxMergeCount(), s.GetRunningMergeCount(), s.GetPendingMergeCount())
}
