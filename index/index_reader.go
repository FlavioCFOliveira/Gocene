// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexReaderInterface is the interface for reading indexes.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.
//
// IndexReaderInterface defines the common operations available on all IndexReaders.
type IndexReaderInterface interface {
	// DocCount returns the total number of documents (including deleted).
	DocCount() int

	// NumDocs returns the number of live documents (excluding deleted).
	NumDocs() int

	// MaxDoc returns the maximum document ID plus one.
	MaxDoc() int

	// Close releases resources associated with this reader.
	Close() error

	// HasDeletions returns true if this reader has deleted documents.
	HasDeletions() bool

	// NumDeletedDocs returns the number of deleted documents.
	NumDeletedDocs() int

	// EnsureOpen throws an error if the reader is closed.
	EnsureOpen() error

	// IncRef increments the reference count.
	IncRef() error

	// DecRef decrements the reference count, closing the reader when it reaches zero.
	DecRef() error

	// TryIncRef tries to increment the reference count.
	// Returns true if successful, false if the reader is already closed.
	TryIncRef() bool

	// GetRefCount returns the current reference count.
	GetRefCount() int32

	// GetContext returns the reader context.
	GetContext() (IndexReaderContext, error)

	// Leaves returns all leaf reader contexts.
	Leaves() ([]*LeafReaderContext, error)

	// StoredFields returns a StoredFields for this reader.
	StoredFields() (StoredFields, error)
}

// IndexReader is an abstract base class for reading indexes.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.
//
// IndexReader provides common functionality for both LeafReader and CompositeReader.
type IndexReader struct {
	// closed indicates if the reader has been closed
	closed atomic.Bool

	// refCount tracks references to this reader
	refCount atomic.Int32

	// docCount is the total document count (including deleted)
	docCount int

	// numDocs is the number of live documents (excluding deleted)
	numDocs int

	// maxDoc is the maximum document ID plus one
	maxDoc int

	// fieldInfos is the FieldInfos for this reader
	fieldInfos *FieldInfos

	// liveDocs indicates which documents are live (nil if all docs are live)
	liveDocs util.Bits

	// cacheHelper provides caching support
	cacheHelper *ReaderCacheHelper
}

// NewIndexReader creates a new IndexReader.
func NewIndexReader() *IndexReader {
	reader := &IndexReader{
		cacheHelper: NewReaderCacheHelper(),
	}
	reader.refCount.Store(1)
	return reader
}

// SetDocCount sets the total document count.
func (r *IndexReader) SetDocCount(docCount int) {
	r.docCount = docCount
}

// SetNumDocs sets the number of live documents.
func (r *IndexReader) SetNumDocs(numDocs int) {
	r.numDocs = numDocs
}

// SetMaxDoc sets the maximum document ID.
func (r *IndexReader) SetMaxDoc(maxDoc int) {
	r.maxDoc = maxDoc
}

// SetFieldInfos sets the FieldInfos.
func (r *IndexReader) SetFieldInfos(infos *FieldInfos) {
	r.fieldInfos = infos
}

// SetLiveDocs sets the live docs Bits.
func (r *IndexReader) SetLiveDocs(liveDocs util.Bits) {
	r.liveDocs = liveDocs
}

// DocCount returns the total number of documents.
func (r *IndexReader) DocCount() int {
	return r.docCount
}

// NumDocs returns the number of live documents.
func (r *IndexReader) NumDocs() int {
	return r.numDocs
}

// MaxDoc returns the maximum document ID.
func (r *IndexReader) MaxDoc() int {
	return r.maxDoc
}

// GetFieldInfos returns the FieldInfos for the index.
func (r *IndexReader) GetFieldInfos() *FieldInfos {
	return r.fieldInfos
}

// GetLiveDocs returns the live docs Bits, or nil if all docs are live.
func (r *IndexReader) GetLiveDocs() util.Bits {
	return r.liveDocs
}

// HasDeletions returns true if this reader has deleted documents.
func (r *IndexReader) HasDeletions() bool {
	return r.liveDocs != nil
}

// NumDeletedDocs returns the number of deleted documents.
func (r *IndexReader) NumDeletedDocs() int {
	return r.maxDoc - r.numDocs
}

// EnsureOpen throws an error if the reader is closed.
func (r *IndexReader) EnsureOpen() error {
	if r.closed.Load() {
		return ErrAlreadyClosed
	}
	return nil
}

// IncRef increments the reference count.
// Returns an error if the reader is already closed.
func (r *IndexReader) IncRef() error {
	for {
		count := r.refCount.Load()
		if count <= 0 {
			return ErrAlreadyClosed
		}
		if r.refCount.CompareAndSwap(count, count+1) {
			return nil
		}
	}
}

// DecRef decrements the reference count.
// When the count reaches zero, the reader is closed.
func (r *IndexReader) DecRef() error {
	if r.refCount.Add(-1) == 0 {
		return r.closeInternal()
	}
	return nil
}

// TryIncRef tries to increment the reference count.
// Returns true if successful, false if the reader is already closed.
func (r *IndexReader) TryIncRef() bool {
	for {
		count := r.refCount.Load()
		if count <= 0 {
			return false
		}
		if r.refCount.CompareAndSwap(count, count+1) {
			return true
		}
	}
}

// GetRefCount returns the current reference count.
func (r *IndexReader) GetRefCount() int32 {
	return r.refCount.Load()
}

// Close closes the reader.
// This decrements the reference count and closes the reader if it reaches zero.
func (r *IndexReader) Close() error {
	return r.DecRef()
}

// closeInternal closes the reader and releases resources.
func (r *IndexReader) closeInternal() error {
	if r.closed.Swap(true) {
		return nil // Already closed
	}

	// Notify cache helper listeners
	if r.cacheHelper != nil {
		r.cacheHelper.NotifyClosedListeners()
	}

	return nil
}

// IsClosed returns true if the reader is closed.
func (r *IndexReader) IsClosed() bool {
	return r.closed.Load()
}

// IsCurrent returns true if the reader is still up to date with the index.
func (r *IndexReader) IsCurrent() (bool, error) {
	return true, nil
}

// GetCacheHelper returns the CacheHelper for this reader.
func (r *IndexReader) GetCacheHelper() CacheHelper {
	return r.cacheHelper
}

// GetContext returns the reader context (must be implemented by subclasses).
func (r *IndexReader) GetContext() (IndexReaderContext, error) {
	return nil, fmt.Errorf("GetContext must be implemented by subclass")
}

// Leaves returns all leaf reader contexts (must be implemented by subclasses).
func (r *IndexReader) Leaves() ([]*LeafReaderContext, error) {
	return nil, fmt.Errorf("Leaves must be implemented by subclass")
}

// GetCoreCacheKey returns a CacheKey for this reader.
// This is used for caching per-reader data structures.
func (r *IndexReader) GetCoreCacheKey() interface{} {
	return r.cacheHelper.CacheKey()
}

// StoredFields returns a StoredFields instance for accessing stored fields.
// This must be implemented by subclasses that support stored fields.
func (r *IndexReader) StoredFields() (StoredFields, error) {
	return nil, fmt.Errorf("StoredFields must be implemented by subclass")
}

// TermVectors returns a TermVectors instance for accessing term vectors.
// This must be implemented by subclasses that support term vectors.
func (r *IndexReader) TermVectors() (TermVectors, error) {
	return nil, fmt.Errorf("TermVectors must be implemented by subclass")
}

// Terms returns the Terms for a field.
// This must be implemented by subclasses.
func (r *IndexReader) Terms(field string) (Terms, error) {
	return nil, fmt.Errorf("Terms must be implemented by subclass")
}

// GetTermVectors returns term vectors for a document.
// This is a convenience method that delegates to TermVectors().Get().
func (r *IndexReader) GetTermVectors(docID int) (Fields, error) {
	tv, err := r.TermVectors()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return tv.Get(docID)
}

// GetTermVector returns the term vector for a specific field.
// This is a convenience method that delegates to TermVectors().GetField().
func (r *IndexReader) GetTermVector(docID int, field string) (Terms, error) {
	tv, err := r.TermVectors()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return tv.GetField(docID, field)
}

// Document retrieves stored fields for a document using the visitor pattern.
// This is a convenience method that delegates to StoredFields().Document().
func (r *IndexReader) Document(docID int, visitor StoredFieldVisitor) error {
	sf, err := r.StoredFields()
	if err != nil {
		return err
	}
	if sf == nil {
		return nil
	}
	return sf.Document(docID, visitor)
}

// ErrAlreadyClosed is returned when attempting to use a closed reader.
var ErrAlreadyClosed = &AlreadyClosedError{}

// AlreadyClosedError indicates that an IndexReader has been closed.
type AlreadyClosedError struct{}

// Error returns the error message.
func (e *AlreadyClosedError) Error() string {
	return "this IndexReader is closed"
}

// Ensure IndexReader implements IndexReaderInterface
var _ IndexReaderInterface = (*IndexReader)(nil)