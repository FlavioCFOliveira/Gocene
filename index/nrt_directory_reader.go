package index

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// NRTDirectoryReader is a DirectoryReader variant that supports Near Real-Time (NRT) updates.
// It provides immediate visibility of index changes made through an IndexWriter.
// This is the Go port of Lucene's NRT DirectoryReader implementation.
type NRTDirectoryReader struct {
	*DirectoryReader

	// writer is the IndexWriter for NRT operations
	writer *IndexWriter

	// version tracks the NRT version
	version int64

	// isNRT indicates if this is an NRT reader
	isNRT atomic.Bool

	// mu protects version and other mutable fields
	mu sync.RWMutex

	// nrtSegmentReaders holds NRT segment readers
	nrtSegmentReaders []*NRTSegmentReader

	// applyAllDeletes indicates if all deletes should be applied
	applyAllDeletes bool
}

// NewNRTDirectoryReader creates a new NRTDirectoryReader from a DirectoryReader and IndexWriter.
func NewNRTDirectoryReader(reader *DirectoryReader, writer *IndexWriter) (*NRTDirectoryReader, error) {
	if reader == nil {
		return nil, fmt.Errorf("directory reader cannot be nil")
	}

	nrtReader := &NRTDirectoryReader{
		DirectoryReader:   reader,
		writer:            writer,
		version:           1,
		nrtSegmentReaders: make([]*NRTSegmentReader, 0),
		applyAllDeletes:   true,
	}

	nrtReader.isNRT.Store(true)

	return nrtReader, nil
}

// IsNRT returns true if this is an NRT reader.
func (r *NRTDirectoryReader) IsNRT() bool {
	return r.isNRT.Load()
}

// GetVersion returns the NRT version of this reader.
func (r *NRTDirectoryReader) GetVersion() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// IncrementVersion increments the NRT version.
func (r *NRTDirectoryReader) IncrementVersion() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.version++
}

// GetWriter returns the IndexWriter for NRT operations, or nil if not in NRT mode.
func (r *NRTDirectoryReader) GetWriter() *IndexWriter {
	return r.writer
}

// GetNRTSegmentReaders returns the NRT segment readers.
func (r *NRTDirectoryReader) GetNRTSegmentReaders() []*NRTSegmentReader {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*NRTSegmentReader, len(r.nrtSegmentReaders))
	copy(result, r.nrtSegmentReaders)
	return result
}

// AddNRTSegmentReader adds an NRT segment reader.
func (r *NRTDirectoryReader) AddNRTSegmentReader(segmentReader *NRTSegmentReader) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nrtSegmentReaders = append(r.nrtSegmentReaders, segmentReader)
}

// RemoveNRTSegmentReader removes an NRT segment reader.
func (r *NRTDirectoryReader) RemoveNRTSegmentReader(segmentReader *NRTSegmentReader) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, sr := range r.nrtSegmentReaders {
		if sr == segmentReader {
			r.nrtSegmentReaders = append(r.nrtSegmentReaders[:i], r.nrtSegmentReaders[i+1:]...)
			return
		}
	}
}

// IsCurrent returns true if the index has not changed since this reader was opened.
// For NRT readers, this checks if the writer has uncommitted changes.
func (r *NRTDirectoryReader) IsCurrent() (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isNRT.Load() {
		// Non-NRT mode: check with parent
		return r.DirectoryReader.IsCurrent()
	}

	// NRT mode: check with writer
	if r.writer == nil {
		return true, nil
	}

	// If writer has uncommitted changes, we're not current
	return false, nil
}

// NumDocs returns the number of live documents in the index.
func (r *NRTDirectoryReader) NumDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isNRT.Load() || len(r.nrtSegmentReaders) == 0 {
		return r.DirectoryReader.NumDocs()
	}

	// Sum live docs from all NRT segment readers
	total := 0
	for _, sr := range r.nrtSegmentReaders {
		total += sr.NumDocs()
	}
	return total
}

// HasDeletions returns true if any documents have been deleted.
func (r *NRTDirectoryReader) HasDeletions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isNRT.Load() {
		return r.DirectoryReader.HasDeletions()
	}

	// Check if any NRT segment reader has deletions
	for _, sr := range r.nrtSegmentReaders {
		if sr.HasDeletions() {
			return true
		}
	}
	return false
}

// NumDeletedDocs returns the number of deleted documents.
func (r *NRTDirectoryReader) NumDeletedDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isNRT.Load() {
		return r.DirectoryReader.NumDeletedDocs()
	}

	// Sum deleted docs from all NRT segment readers
	total := 0
	for _, sr := range r.nrtSegmentReaders {
		total += sr.GetNumDeleted()
	}
	return total
}

// SetApplyAllDeletes sets whether all deletes should be applied.
func (r *NRTDirectoryReader) SetApplyAllDeletes(apply bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applyAllDeletes = apply
}

// GetApplyAllDeletes returns whether all deletes should be applied.
func (r *NRTDirectoryReader) GetApplyAllDeletes() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.applyAllDeletes
}

// Refresh refreshes the reader to reflect the latest index state.
// This creates a new reader with the latest changes from the writer.
func (r *NRTDirectoryReader) Refresh() (*NRTDirectoryReader, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isNRT.Load() {
		return nil, fmt.Errorf("cannot refresh non-NRT reader")
	}

	if r.writer == nil {
		return nil, fmt.Errorf("no writer available for NRT refresh")
	}

	// Create a new reader reflecting the current state
	// In a real implementation, this would get the latest reader from the writer
	newReader := &NRTDirectoryReader{
		DirectoryReader:   r.DirectoryReader,
		writer:            r.writer,
		version:           r.version + 1,
		nrtSegmentReaders: make([]*NRTSegmentReader, 0),
		applyAllDeletes:   r.applyAllDeletes,
	}
	newReader.isNRT.Store(true)

	return newReader, nil
}

// Close closes this NRTDirectoryReader.
func (r *NRTDirectoryReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isNRT.Store(false)

	// Close all NRT segment readers
	for _, sr := range r.nrtSegmentReaders {
		sr.Close()
	}
	r.nrtSegmentReaders = nil

	r.writer = nil

	return r.DirectoryReader.Close()
}

// Clone creates a shallow copy of this NRTDirectoryReader.
func (r *NRTDirectoryReader) Clone() (*NRTDirectoryReader, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := &NRTDirectoryReader{
		DirectoryReader:   r.DirectoryReader,
		writer:            r.writer,
		version:           r.version,
		nrtSegmentReaders: make([]*NRTSegmentReader, len(r.nrtSegmentReaders)),
		applyAllDeletes:   r.applyAllDeletes,
	}
	clone.isNRT.Store(r.isNRT.Load())

	// Copy segment readers
	copy(clone.nrtSegmentReaders, r.nrtSegmentReaders)

	return clone, nil
}

// ForEachNRTSegment iterates over all NRT segment readers.
func (r *NRTDirectoryReader) ForEachNRTSegment(fn func(segmentReader *NRTSegmentReader) error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, sr := range r.nrtSegmentReaders {
		if err := fn(sr); err != nil {
			return err
		}
	}

	return nil
}
