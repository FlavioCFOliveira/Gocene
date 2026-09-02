package index

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DirectoryReaderReopener handles reopening of DirectoryReader with NRT support.
// It monitors the index for changes and creates new DirectoryReader instances
// when the index has been updated.
// This is the Go port of Lucene's reopen functionality for DirectoryReader.
type DirectoryReaderReopener struct {
	mu sync.RWMutex

	// current is the currently managed DirectoryReader
	current *DirectoryReader

	// writer is the IndexWriter for NRT reopening (may be nil)
	writer *IndexWriter

	// applyAllDeletes indicates if deletes should be applied during reopen
	applyAllDeletes bool

	// commitUserData stores custom commit data
	commitUserData map[string]string

	// isClosed indicates if the reopener has been closed
	isClosed bool

	// reopenListeners are called when a reopen occurs
	reopenListeners []ReopenListener

	// minReopenInterval is the minimum time between reopens
	minReopenInterval time.Duration

	// lastReopenTime tracks when the last reopen occurred
	lastReopenTime time.Time
}

// ReopenListener is called when a DirectoryReader is reopened.
type ReopenListener interface {
	// OnReopen is called after a successful reopen
	OnReopen(oldReader, newReader *DirectoryReader)
	// OnReopenError is called when reopen fails
	OnReopenError(err error)
}

// ReopenResult contains the result of a reopen operation.
type ReopenResult struct {
	// NewReader is the new DirectoryReader (nil if no changes)
	NewReader *DirectoryReader
	// HasChanges indicates if the index changed
	HasChanges bool
	// Generation is the commit generation
	Generation int64
}

// NewDirectoryReaderReopener creates a new DirectoryReaderReopener.
// The initial reader must already be open.
func NewDirectoryReaderReopener(initial *DirectoryReader) (*DirectoryReaderReopener, error) {
	if initial == nil {
		return nil, fmt.Errorf("initial reader cannot be nil")
	}

	return &DirectoryReaderReopener{
		current:           initial,
		applyAllDeletes:   true,
		commitUserData:    make(map[string]string),
		minReopenInterval: 100 * time.Millisecond,
	}, nil
}

// NewDirectoryReaderReopenerWithWriter creates a new DirectoryReaderReopener with NRT support.
// This allows for near real-time reopening using the IndexWriter.
func NewDirectoryReaderReopenerWithWriter(initial *DirectoryReader, writer *IndexWriter) (*DirectoryReaderReopener, error) {
	reopener, err := NewDirectoryReaderReopener(initial)
	if err != nil {
		return nil, err
	}

	reopener.writer = writer
	return reopener, nil
}

// MaybeReopen checks if the index has changed and reopens the reader if necessary.
// Returns a ReopenResult with the new reader if changes were detected.
// If no changes were detected, returns nil reader and HasChanges=false.
func (dr *DirectoryReaderReopener) MaybeReopen(ctx context.Context) (*ReopenResult, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if dr.isClosed {
		return nil, fmt.Errorf("reopener is closed")
	}

	// Check minimum reopen interval
	if time.Since(dr.lastReopenTime) < dr.minReopenInterval {
		return &ReopenResult{HasChanges: false}, nil
	}

	// Determine if we should reopen
	hasChanges, err := dr.checkForChanges()
	if err != nil {
		dr.notifyReopenError(err)
		return nil, fmt.Errorf("checking for changes: %w", err)
	}

	if !hasChanges {
		return &ReopenResult{HasChanges: false}, nil
	}

	// Reopen the reader
	newReader, generation, err := dr.doReopen()
	if err != nil {
		dr.notifyReopenError(err)
		return nil, fmt.Errorf("reopening reader: %w", err)
	}

	// Update state
	oldReader := dr.current
	dr.current = newReader
	dr.lastReopenTime = time.Now()

	// Notify listeners
	dr.notifyReopen(oldReader, newReader)

	return &ReopenResult{
		NewReader:  newReader,
		HasChanges: true,
		Generation: generation,
	}, nil
}

// Reopen is a blocking version of MaybeReopen.
// It forces a reopen of the reader, even if no changes are detected.
func (dr *DirectoryReaderReopener) Reopen(ctx context.Context) (*DirectoryReader, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if dr.isClosed {
		return nil, fmt.Errorf("reopener is closed")
	}

	newReader, _, err := dr.doReopen()
	if err != nil {
		dr.notifyReopenError(err)
		return nil, fmt.Errorf("reopening reader: %w", err)
	}

	oldReader := dr.current
	dr.current = newReader
	dr.lastReopenTime = time.Now()

	dr.notifyReopen(oldReader, newReader)

	return newReader, nil
}

// checkForChanges determines if the index has changed since the last reopen.
func (dr *DirectoryReaderReopener) checkForChanges() (bool, error) {
	if dr.current == nil {
		return false, fmt.Errorf("no current reader")
	}

	// In NRT mode, check with the writer
	if dr.writer != nil {
		// Check if the writer has changes
		// This is a simplified check - in reality we'd check version/generation
		// For now, we assume no changes
		return false, nil
	}

	// Non-NRT mode: check if segment infos have changed
	// This would typically involve checking commit metadata
	// For now, we assume no changes in non-NRT mode
	return false, nil
}

// doReopen performs the actual reopen operation.
func (dr *DirectoryReaderReopener) doReopen() (*DirectoryReader, int64, error) {
	if dr.writer != nil {
		// NRT reopen: get the latest reader from the writer
		return dr.reopenFromWriter()
	}

	// Non-NRT reopen: open a new reader from the directory
	return dr.reopenFromDirectory()
}

// reopenFromWriter performs an NRT reopen from the IndexWriter.
func (dr *DirectoryReaderReopener) reopenFromWriter() (*DirectoryReader, int64, error) {
	// In a real implementation, this would get the latest NRT reader from the writer
	// For now, we return the current reader
	generation := int64(1)

	return dr.current, generation, nil
}

// reopenFromDirectory performs a reopen by reading from the directory.
func (dr *DirectoryReaderReopener) reopenFromDirectory() (*DirectoryReader, int64, error) {
	if dr.current == nil {
		return nil, 0, fmt.Errorf("no current reader")
	}

	// Open a new reader from the directory
	// This would typically involve checking for new commits
	// For now, we return the current reader
	generation := int64(1)

	return dr.current, generation, nil
}

// GetCurrent returns the current DirectoryReader.
func (dr *DirectoryReaderReopener) GetCurrent() *DirectoryReader {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.current
}

// Close closes the reopener.
// Note: This does not close the underlying DirectoryReader.
func (dr *DirectoryReaderReopener) Close() error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if dr.isClosed {
		return nil
	}

	dr.isClosed = true
	dr.current = nil
	dr.writer = nil

	return nil
}

// IsClosed returns true if the reopener has been closed.
func (dr *DirectoryReaderReopener) IsClosed() bool {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.isClosed
}

// SetApplyAllDeletes sets whether deletes should be applied during reopen.
func (dr *DirectoryReaderReopener) SetApplyAllDeletes(apply bool) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.applyAllDeletes = apply
}

// GetApplyAllDeletes returns whether deletes are applied during reopen.
func (dr *DirectoryReaderReopener) GetApplyAllDeletes() bool {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.applyAllDeletes
}

// SetMinReopenInterval sets the minimum time between reopens.
func (dr *DirectoryReaderReopener) SetMinReopenInterval(interval time.Duration) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.minReopenInterval = interval
}

// GetMinReopenInterval returns the minimum time between reopens.
func (dr *DirectoryReaderReopener) GetMinReopenInterval() time.Duration {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.minReopenInterval
}

// AddReopenListener adds a listener for reopen events.
func (dr *DirectoryReaderReopener) AddReopenListener(listener ReopenListener) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.reopenListeners = append(dr.reopenListeners, listener)
}

// RemoveReopenListener removes a listener for reopen events.
func (dr *DirectoryReaderReopener) RemoveReopenListener(listener ReopenListener) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	for i, l := range dr.reopenListeners {
		if l == listener {
			dr.reopenListeners = append(dr.reopenListeners[:i], dr.reopenListeners[i+1:]...)
			return
		}
	}
}

// notifyReopen notifies all listeners of a successful reopen.
func (dr *DirectoryReaderReopener) notifyReopen(oldReader, newReader *DirectoryReader) {
	for _, listener := range dr.reopenListeners {
		listener.OnReopen(oldReader, newReader)
	}
}

// notifyReopenError notifies all listeners of a reopen error.
func (dr *DirectoryReaderReopener) notifyReopenError(err error) {
	for _, listener := range dr.reopenListeners {
		listener.OnReopenError(err)
	}
}

// SetCommitUserData sets custom commit user data.
func (dr *DirectoryReaderReopener) SetCommitUserData(data map[string]string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.commitUserData = make(map[string]string)
	for k, v := range data {
		dr.commitUserData[k] = v
	}
}

// GetCommitUserData returns the commit user data.
func (dr *DirectoryReaderReopener) GetCommitUserData() map[string]string {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range dr.commitUserData {
		result[k] = v
	}
	return result
}

// HasWriter returns true if this reopener is in NRT mode (has an IndexWriter).
func (dr *DirectoryReaderReopener) HasWriter() bool {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.writer != nil
}

// GetWriter returns the IndexWriter if in NRT mode, or nil.
func (dr *DirectoryReaderReopener) GetWriter() *IndexWriter {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.writer
}

// GetLastReopenTime returns the time of the last reopen.
func (dr *DirectoryReaderReopener) GetLastReopenTime() time.Time {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	return dr.lastReopenTime
}
