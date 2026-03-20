package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NRTReader provides Near Real-Time (NRT) search capabilities.
// It wraps an IndexReader and allows for real-time visibility of indexed documents.
type NRTReader struct {
	mu sync.RWMutex

	// reader is the underlying IndexReader
	reader *DirectoryReader

	// version is the reader version for NRT tracking
	version int64

	// isOpen indicates if the reader is open
	isOpen atomic.Bool

	// lastRefreshTime tracks when the reader was last refreshed
	lastRefreshTime time.Time

	// refreshCount tracks the number of refreshes
	refreshCount int64

	// writer is the IndexWriter for NRT operations (may be nil)
	writer *IndexWriter

	// nrtSegmentReaders holds NRT segment readers
	nrtSegmentReaders []*NRTSegmentReader
}

// NewNRTReader creates a new NRTReader.
func NewNRTReader(reader *DirectoryReader, writer *IndexWriter) (*NRTReader, error) {
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}

	nrt := &NRTReader{
		reader:            reader,
		version:           1,
		lastRefreshTime:   time.Now(),
		nrtSegmentReaders: make([]*NRTSegmentReader, 0),
		writer:            writer,
	}

	nrt.isOpen.Store(true)

	return nrt, nil
}

// GetReader returns the underlying DirectoryReader.
func (nrt *NRTReader) GetReader() *DirectoryReader {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.reader
}

// GetVersion returns the reader version.
func (nrt *NRTReader) GetVersion() int64 {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.version
}

// IncrementVersion increments the reader version.
func (nrt *NRTReader) IncrementVersion() {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	nrt.version++
}

// Refresh refreshes the reader to see the latest changes.
func (nrt *NRTReader) Refresh() error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.isOpen.Load() {
		return fmt.Errorf("reader is closed")
	}

	// Reopen the underlying reader
	newReader, err := nrt.reader.Reopen()
	if err != nil {
		return fmt.Errorf("reopening reader: %w", err)
	}

	nrt.reader = newReader
	nrt.version++
	nrt.lastRefreshTime = time.Now()
	nrt.refreshCount++

	return nil
}

// GetRefreshCount returns the number of refreshes.
func (nrt *NRTReader) GetRefreshCount() int64 {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.refreshCount
}

// GetLastRefreshTime returns the time of the last refresh.
func (nrt *NRTReader) GetLastRefreshTime() time.Time {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.lastRefreshTime
}

// NumDocs returns the number of documents.
func (nrt *NRTReader) NumDocs() int {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	if !nrt.isOpen.Load() {
		return 0
	}

	return nrt.reader.NumDocs()
}

// MaxDoc returns the maximum document ID.
func (nrt *NRTReader) MaxDoc() int {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	if !nrt.isOpen.Load() {
		return 0
	}

	return nrt.reader.MaxDoc()
}

// HasDeletions returns true if there are deleted documents.
func (nrt *NRTReader) HasDeletions() bool {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	if !nrt.isOpen.Load() {
		return false
	}

	return nrt.reader.HasDeletions()
}

// GetWriter returns the IndexWriter for NRT operations.
func (nrt *NRTReader) GetWriter() *IndexWriter {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return nrt.writer
}

// AddNRTSegmentReader adds an NRT segment reader.
func (nrt *NRTReader) AddNRTSegmentReader(segmentReader *NRTSegmentReader) {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	nrt.nrtSegmentReaders = append(nrt.nrtSegmentReaders, segmentReader)
}

// GetNRTSegmentReaders returns the NRT segment readers.
func (nrt *NRTReader) GetNRTSegmentReaders() []*NRTSegmentReader {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	result := make([]*NRTSegmentReader, len(nrt.nrtSegmentReaders))
	copy(result, nrt.nrtSegmentReaders)
	return result
}

// IsOpen returns true if the reader is open.
func (nrt *NRTReader) IsOpen() bool {
	return nrt.isOpen.Load()
}

// Close closes the NRTReader.
func (nrt *NRTReader) Close() error {
	nrt.mu.Lock()
	defer nrt.mu.Unlock()

	if !nrt.isOpen.Load() {
		return nil
	}

	nrt.isOpen.Store(false)

	// Close underlying reader
	nrt.reader.Close()

	// Close NRT segment readers
	for _, sr := range nrt.nrtSegmentReaders {
		sr.Close()
	}
	nrt.nrtSegmentReaders = nil

	return nil
}

// String returns a string representation of the NRTReader.
func (nrt *NRTReader) String() string {
	nrt.mu.RLock()
	defer nrt.mu.RUnlock()

	return fmt.Sprintf("NRTReader{open=%v, version=%d, docs=%d, segments=%d}",
		nrt.isOpen.Load(), nrt.version, nrt.reader.NumDocs(), len(nrt.nrtSegmentReaders))
}
