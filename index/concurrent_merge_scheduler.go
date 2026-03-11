// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
)

// ConcurrentMergeScheduler performs merges concurrently using goroutines.
type ConcurrentMergeScheduler struct {
	*BaseMergeScheduler

	// maxThreadCount limits concurrent merge threads
	maxThreadCount int

	// runningMerges tracks active merge goroutines
	runningMerges sync.WaitGroup

	// mu protects mutable fields
	mu sync.Mutex
}

// NewConcurrentMergeScheduler creates a new ConcurrentMergeScheduler.
func NewConcurrentMergeScheduler() *ConcurrentMergeScheduler {
	return &ConcurrentMergeScheduler{
		BaseMergeScheduler: &BaseMergeScheduler{},
		maxThreadCount:     1, // Default to single merge thread
	}
}

// MaxThreadCount returns the maximum number of merge threads.
func (s *ConcurrentMergeScheduler) MaxThreadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxThreadCount
}

// SetMaxThreadCount sets the maximum number of merge threads.
func (s *ConcurrentMergeScheduler) SetMaxThreadCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxThreadCount = count
}

// Merge schedules a merge to run in a goroutine.
func (s *ConcurrentMergeScheduler) Merge(writer *IndexWriter, merge *OneMerge) error {
	s.runningMerges.Add(1)
	go func() {
		defer s.runningMerges.Done()
		s.doMerge(writer, merge)
	}()
	return nil
}

// doMerge performs the actual merge.
func (s *ConcurrentMergeScheduler) doMerge(writer *IndexWriter, merge *OneMerge) {
	// Merge implementation would go here
}

// Close waits for all running merges to complete.
func (s *ConcurrentMergeScheduler) Close() error {
	s.runningMerges.Wait()
	return nil
}
