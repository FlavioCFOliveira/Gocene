// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// NRTReferenceManager manages NRT (Near Real-Time) reader references.
//
// This is the Go port of Lucene's org.apache.lucene.search.NRTReferenceManager.
//
// NRTReferenceManager is a specialized ReferenceManager that works with IndexWriter
// to provide near real-time search capabilities. It refreshes the reference from
// the IndexWriter's uncommitted changes.
type NRTReferenceManager struct {
	*ReferenceManager[*DirectoryReader]

	// writer is the IndexWriter to obtain NRT readers from
	writer *IndexWriter

	// reopening tracks if a reopen is in progress
	reopening atomic.Bool

	// reopenComplete is signaled when reopen completes
	reopenComplete chan struct{}

	// mu protects reopening state
	mu sync.Mutex
}

// NewNRTReferenceManager creates a new NRTReferenceManager from the given IndexWriter.
//
// The manager takes ownership of the provided reader and will close it when
// a new reader is obtained or when the manager is closed.
func NewNRTReferenceManager(writer *IndexWriter, reader *DirectoryReader) (*NRTReferenceManager, error) {
	if writer == nil {
		return nil, fmt.Errorf("writer cannot be nil")
	}
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}

	rm := NewReferenceManagerWithFuncs(reader,
		func(r *DirectoryReader) *DirectoryReader {
			// Acquire increments reference count
			if r != nil {
				r.IncRef()
			}
			return r
		},
		func(r *DirectoryReader) error {
			// Release decrements reference count
			if r != nil {
				return r.DecRef()
			}
			return nil
		},
	)

	nrt := &NRTReferenceManager{
		ReferenceManager: rm,
		writer:          writer,
		reopenComplete:  make(chan struct{}),
	}

	return nrt, nil
}

// RefreshIfNeeded refreshes the reader if there are changes in the IndexWriter.
// Returns true if a refresh was performed.
func (rm *NRTReferenceManager) RefreshIfNeeded() (bool, error) {
	rm.mu.Lock()
	if rm.reopening.Load() {
		rm.mu.Unlock()
		// Wait for current reopen to complete
		<-rm.reopenComplete
		return true, nil
	}

	rm.reopening.Store(true)
	rm.reopenComplete = make(chan struct{})
	rm.mu.Unlock()

	// Signal completion when done
	defer func() {
		rm.mu.Lock()
		rm.reopening.Store(false)
		close(rm.reopenComplete)
		rm.mu.Unlock()
	}()

	// Notify listeners before refresh
	rm.notifyBeforeRefresh()

	// Get current reader to check if reopen is needed
	currentReader, err := rm.Acquire()
	if err != nil {
		return false, err
	}

	// Check if reopen is needed
	isCurrent, err := currentReader.IsCurrent()
	if err != nil {
		rm.Release(currentReader)
		return false, err
	}

	if isCurrent {
		// No changes, release and return
		rm.Release(currentReader)
		return false, nil
	}

	// Reopen the reader
	newReader, err := currentReader.Reopen()
	if err != nil {
		rm.Release(currentReader)
		return false, fmt.Errorf("failed to reopen reader: %w", err)
	}

	// Swap references
	oldReader := rm.Swap(newReader)

	// Release the old references
	rm.Release(currentReader)
	if oldReader != nil {
		rm.Release(oldReader)
	}

	// Notify listeners after refresh
	rm.notifyAfterRefresh()

	return true, nil
}

// MaybeRefresh refreshes the reader if there are changes.
// This is an alias for RefreshIfNeeded for compatibility.
func (rm *NRTReferenceManager) MaybeRefresh() (bool, error) {
	return rm.RefreshIfNeeded()
}

// WaitForGeneration waits for the reference to reach the specified generation.
// Returns immediately if the current generation is already >= target.
// This method is useful for waiting for specific updates to become visible.
func (rm *NRTReferenceManager) WaitForGeneration(targetGeneration int64, timeoutMs int) (bool, error) {
	startGeneration := rm.GetGeneration()

	if startGeneration >= targetGeneration {
		return true, nil
	}

	// Try to refresh to catch up
	refreshed, err := rm.RefreshIfNeeded()
	if err != nil {
		return false, err
	}

	// Check if we've reached the target
	if rm.GetGeneration() >= targetGeneration {
		return true, nil
	}

	// If we refreshed but still haven't reached the target,
	// the generation might not be available yet
	if refreshed {
		return rm.GetGeneration() >= targetGeneration, nil
	}

	// No changes, haven't reached target
	return false, nil
}

// GetWriter returns the IndexWriter associated with this manager.
func (rm *NRTReferenceManager) GetWriter() *IndexWriter {
	return rm.writer
}

// IsReopening returns true if a reopen is currently in progress.
func (rm *NRTReferenceManager) IsReopening() bool {
	return rm.reopening.Load()
}

// Close closes the manager and releases all resources.
func (rm *NRTReferenceManager) Close() error {
	if rm.ReferenceManager == nil {
		return nil
	}

	// Wait for any in-progress reopen
	rm.mu.Lock()
	if rm.reopening.Load() {
		ch := rm.reopenComplete
		rm.mu.Unlock()
		<-ch
	} else {
		rm.mu.Unlock()
	}

	return rm.ReferenceManager.Close()
}

// Acquire acquires the current reader reference.
// The caller must call Release when done.
func (rm *NRTReferenceManager) Acquire() (*DirectoryReader, error) {
	if rm.ReferenceManager == nil {
		return nil, fmt.Errorf("reference manager is nil")
	}
	return rm.ReferenceManager.Acquire()
}

// Release releases a previously acquired reader reference.
func (rm *NRTReferenceManager) Release(reader *DirectoryReader) error {
	if rm.ReferenceManager == nil {
		return fmt.Errorf("reference manager is nil")
	}
	return rm.ReferenceManager.Release(reader)
}

// notifyBeforeRefresh notifies all listeners before refresh.
func (rm *NRTReferenceManager) notifyBeforeRefresh() {
	// Access embedded ReferenceManager's listeners through the embedded type
	if rm.ReferenceManager != nil {
		rm.ReferenceManager.notifyBeforeRefresh()
	}
}

// notifyAfterRefresh notifies all listeners after refresh.
func (rm *NRTReferenceManager) notifyAfterRefresh() {
	if rm.ReferenceManager != nil {
		rm.ReferenceManager.notifyAfterRefresh()
	}
}

// Swap swaps the current reference with a new one.
func (rm *NRTReferenceManager) Swap(newReader *DirectoryReader) *DirectoryReader {
	if rm.ReferenceManager == nil {
		return nil
	}
	return rm.ReferenceManager.Swap(newReader)
}

// GetGeneration returns the current generation.
func (rm *NRTReferenceManager) GetGeneration() int64 {
	if rm.ReferenceManager == nil {
		return 0
	}
	return rm.ReferenceManager.GetGeneration()
}

// IsOpen returns true if the manager is open.
func (rm *NRTReferenceManager) IsOpen() bool {
	if rm.ReferenceManager == nil {
		return false
	}
	return rm.ReferenceManager.IsOpen()
}

// AddRefreshListener adds a refresh listener.
func (rm *NRTReferenceManager) AddRefreshListener(listener RefreshListener) {
	if rm.ReferenceManager != nil {
		rm.ReferenceManager.AddRefreshListener(listener)
	}
}

// RemoveRefreshListener removes a refresh listener.
func (rm *NRTReferenceManager) RemoveRefreshListener(listener RefreshListener) {
	if rm.ReferenceManager != nil {
		rm.ReferenceManager.RemoveRefreshListener(listener)
	}
}
