package index

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTReplicationReader handles reading index data for NRT (Near Real-Time) replication.
// It coordinates with the replication source to receive consistent snapshots.
// This is the Go port of Lucene's NRT replication reader pattern.
type NRTReplicationReader struct {
	mu sync.RWMutex

	// currentRevision is the current index revision
	currentRevision *IndexRevision

	// pendingFiles tracks files that are being downloaded
	pendingFiles map[string]bool

	// completedFiles tracks files that have been successfully downloaded
	completedFiles map[string]bool

	// isOpen indicates if the reader is open
	isOpen atomic.Bool

	// lastSyncTime tracks when the last sync occurred
	lastSyncTime time.Time

	// syncCount tracks the number of syncs
	syncCount int64

	// source is the replication source (e.g., HTTP endpoint, file path)
	source string
}

// NewNRTReplicationReader creates a new NRTReplicationReader.
func NewNRTReplicationReader(source string) (*NRTReplicationReader, error) {
	if source == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}

	rr := &NRTReplicationReader{
		currentRevision:  &IndexRevision{},
		pendingFiles:   make(map[string]bool),
		completedFiles: make(map[string]bool),
		lastSyncTime:   time.Now(),
		source:         source,
	}

	rr.isOpen.Store(true)

	return rr, nil
}

// GetCurrentRevision returns the current index revision.
func (rr *NRTReplicationReader) GetCurrentRevision() (*IndexRevision, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if !rr.isOpen.Load() {
		return nil, fmt.Errorf("replication reader is closed")
	}

	return rr.currentRevision, nil
}

// UpdateRevision updates the current index revision after a sync.
func (rr *NRTReplicationReader) UpdateRevision(revision *IndexRevision) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return fmt.Errorf("replication reader is closed")
	}

	if revision == nil {
		return fmt.Errorf("revision cannot be nil")
	}

	rr.currentRevision = revision
	rr.lastSyncTime = time.Now()
	rr.syncCount++

	// Reset file tracking for new revision
	rr.pendingFiles = make(map[string]bool)
	rr.completedFiles = make(map[string]bool)

	return nil
}

// GetSource returns the replication source.
func (rr *NRTReplicationReader) GetSource() string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.source
}

// SetSource sets the replication source.
func (rr *NRTReplicationReader) SetSource(source string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return fmt.Errorf("replication reader is closed")
	}

	if source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	rr.source = source
	return nil
}

// StartFileDownload marks a file as being downloaded.
func (rr *NRTReplicationReader) StartFileDownload(filename string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return fmt.Errorf("replication reader is closed")
	}

	rr.pendingFiles[filename] = true
	return nil
}

// CompleteFileDownload marks a file as successfully downloaded.
func (rr *NRTReplicationReader) CompleteFileDownload(filename string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return fmt.Errorf("replication reader is closed")
	}

	delete(rr.pendingFiles, filename)
	rr.completedFiles[filename] = true
	return nil
}

// IsFilePending returns true if the file is being downloaded.
func (rr *NRTReplicationReader) IsFilePending(filename string) bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.pendingFiles[filename]
}

// IsFileComplete returns true if the file has been downloaded.
func (rr *NRTReplicationReader) IsFileComplete(filename string) bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.completedFiles[filename]
}

// GetPendingFiles returns the list of files being downloaded.
func (rr *NRTReplicationReader) GetPendingFiles() []string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	files := make([]string, 0, len(rr.pendingFiles))
	for file := range rr.pendingFiles {
		files = append(files, file)
	}
	return files
}

// GetCompletedFiles returns the list of completed files.
func (rr *NRTReplicationReader) GetCompletedFiles() []string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	files := make([]string, 0, len(rr.completedFiles))
	for file := range rr.completedFiles {
		files = append(files, file)
	}
	return files
}

// GetRequiredFiles returns the list of files needed for the current revision.
func (rr *NRTReplicationReader) GetRequiredFiles() ([]string, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if !rr.isOpen.Load() {
		return nil, fmt.Errorf("replication reader is closed")
	}

	return rr.currentRevision.Files, nil
}

// IsSyncComplete returns true if all files for the current revision are downloaded.
func (rr *NRTReplicationReader) IsSyncComplete() bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if !rr.isOpen.Load() {
		return false
	}

	for _, file := range rr.currentRevision.Files {
		if !rr.completedFiles[file] {
			return false
		}
	}
	return true
}

// Sync performs a sync with the replication source.
func (rr *NRTReplicationReader) Sync(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return fmt.Errorf("replication reader is closed")
	}

	// In a real implementation, this would:
	// 1. Fetch the latest revision from the source
	// 2. Compare with current revision
	// 3. Download any missing files
	// For now, just update the sync time
	rr.lastSyncTime = time.Now()
	rr.syncCount++

	return nil
}

// Close closes the NRTReplicationReader.
func (rr *NRTReplicationReader) Close() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if !rr.isOpen.Load() {
		return nil
	}

	rr.isOpen.Store(false)

	// Clear all tracking
	rr.pendingFiles = nil
	rr.completedFiles = nil
	rr.currentRevision = nil

	return nil
}

// IsOpen returns true if the reader is open.
func (rr *NRTReplicationReader) IsOpen() bool {
	return rr.isOpen.Load()
}

// GetSyncCount returns the number of syncs performed.
func (rr *NRTReplicationReader) GetSyncCount() int64 {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.syncCount
}

// GetLastSyncTime returns the time of the last sync.
func (rr *NRTReplicationReader) GetLastSyncTime() time.Time {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.lastSyncTime
}

// String returns a string representation of the NRTReplicationReader.
func (rr *NRTReplicationReader) String() string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return fmt.Sprintf("NRTReplicationReader{open=%v, source=%s, pending=%d, completed=%d}",
		rr.isOpen.Load(), rr.source, len(rr.pendingFiles), len(rr.completedFiles))
}
