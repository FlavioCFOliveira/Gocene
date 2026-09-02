package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ReadOnlyDirectoryReader is a read-only variant of DirectoryReader.
// It wraps a DirectoryReader and prevents all write operations.
// This is the Go port of Lucene's read-only directory reader pattern.
type ReadOnlyDirectoryReader struct {
	mu sync.RWMutex

	// delegate is the underlying DirectoryReader
	delegate *DirectoryReader

	// isClosed indicates if this reader has been closed
	isClosed bool
}

// NewReadOnlyDirectoryReader creates a new ReadOnlyDirectoryReader from a DirectoryReader.
func NewReadOnlyDirectoryReader(delegate *DirectoryReader) (*ReadOnlyDirectoryReader, error) {
	if delegate == nil {
		return nil, fmt.Errorf("delegate reader cannot be nil")
	}

	return &ReadOnlyDirectoryReader{
		delegate: delegate,
	}, nil
}

// NumDocs returns the number of live documents in the index.
func (r *ReadOnlyDirectoryReader) NumDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return 0
	}

	return r.delegate.NumDocs()
}

// MaxDoc returns the maximum document ID plus one.
func (r *ReadOnlyDirectoryReader) MaxDoc() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return 0
	}

	return r.delegate.MaxDoc()
}

// DocCount returns the total number of documents (including deleted).
func (r *ReadOnlyDirectoryReader) DocCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return 0
	}

	return r.delegate.DocCount()
}

// HasDeletions returns true if any documents have been deleted.
func (r *ReadOnlyDirectoryReader) HasDeletions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return false
	}

	return r.delegate.HasDeletions()
}

// NumDeletedDocs returns the number of deleted documents.
func (r *ReadOnlyDirectoryReader) NumDeletedDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return 0
	}

	return r.delegate.NumDeletedDocs()
}

// IsCurrent returns true if the index has not changed since this reader was opened.
func (r *ReadOnlyDirectoryReader) IsCurrent() (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return false, fmt.Errorf("reader is closed")
	}

	return r.delegate.IsCurrent()
}

// GetDirectory returns the Directory associated with this reader.
func (r *ReadOnlyDirectoryReader) GetDirectory() store.Directory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil
	}

	return r.delegate.GetDirectory()
}

// GetSegmentInfos returns the SegmentInfos for this reader.
func (r *ReadOnlyDirectoryReader) GetSegmentInfos() *SegmentInfos {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil
	}

	return r.delegate.GetSegmentInfos()
}

// GetIndexCommit returns the IndexCommit for this reader.
func (r *ReadOnlyDirectoryReader) GetIndexCommit() *IndexCommit {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil
	}

	return r.delegate.GetIndexCommit()
}

// GetSegmentReaders returns the SegmentReaders for this reader.
func (r *ReadOnlyDirectoryReader) GetSegmentReaders() []*SegmentReader {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil
	}

	return r.delegate.GetSegmentReaders()
}

// GetTermVectors returns the term vectors for the specified document.
func (r *ReadOnlyDirectoryReader) GetTermVectors(docID int) (Fields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	return r.delegate.GetTermVectors(docID)
}

// Terms returns the Terms for the specified field.
func (r *ReadOnlyDirectoryReader) Terms(field string) (Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	return r.delegate.Terms(field)
}

// GetLiveDocs returns the live documents bitset.
func (r *ReadOnlyDirectoryReader) GetLiveDocs() []bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil
	}

	return r.delegate.GetLiveDocs()
}

// StoredFields returns the StoredFields for this reader.
func (r *ReadOnlyDirectoryReader) StoredFields() (StoredFields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	return r.delegate.StoredFields()
}

// Close closes this reader.
func (r *ReadOnlyDirectoryReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return nil
	}

	r.isClosed = true

	// Close the delegate
	if r.delegate != nil {
		return r.delegate.Close()
	}

	return nil
}

// IsClosed returns true if this reader has been closed.
func (r *ReadOnlyDirectoryReader) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isClosed
}

// GetDelegate returns the underlying DirectoryReader.
func (r *ReadOnlyDirectoryReader) GetDelegate() *DirectoryReader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.delegate
}

// EnsureOpen throws an error if the reader is closed.
func (r *ReadOnlyDirectoryReader) EnsureOpen() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return fmt.Errorf("reader is closed")
	}

	return nil
}

// Leaves returns all leaf reader contexts.
func (r *ReadOnlyDirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	return r.delegate.Leaves()
}

// GetContext returns the reader context.
func (r *ReadOnlyDirectoryReader) GetContext() (IndexReaderContext, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	return r.delegate.GetContext()
}

// Clone creates a copy of this ReadOnlyDirectoryReader.
// The clone shares the same delegate but has its own closed state.
func (r *ReadOnlyDirectoryReader) Clone() (*ReadOnlyDirectoryReader, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	// Note: We don't clone the delegate, just wrap it again
	// This is intentional as the delegate itself should handle reference counting
	return &ReadOnlyDirectoryReader{
		delegate: r.delegate,
	}, nil
}

// Reopen returns a new ReadOnlyDirectoryReader with the latest index state.
// This is a read-only operation that doesn't modify the index.
func (r *ReadOnlyDirectoryReader) Reopen() (*ReadOnlyDirectoryReader, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return nil, fmt.Errorf("reader is closed")
	}

	// Reopen the delegate
	newDelegate, err := r.delegate.Reopen()
	if err != nil {
		return nil, err
	}

	return NewReadOnlyDirectoryReader(newDelegate)
}

// String returns a string representation of this reader.
func (r *ReadOnlyDirectoryReader) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.isClosed {
		return "ReadOnlyDirectoryReader(closed)"
	}

	return fmt.Sprintf("ReadOnlyDirectoryReader(%v)", r.delegate)
}
