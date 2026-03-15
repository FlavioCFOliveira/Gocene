// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// BaseCompositeReader is a base implementation of CompositeReader that manages
// sub-readers and provides document ID mapping.
// This is the Go port of Lucene's org.apache.lucene.index.BaseCompositeReader.
//
// BaseCompositeReader handles the complexity of mapping global document IDs to
// local document IDs within each sub-reader.
type BaseCompositeReader struct {
	*CompositeReader

	// subReaders is the list of sub-readers
	subReaders []IndexReaderInterface

	// starts contains the starting doc ID for each sub-reader
	// starts[i] is the first doc ID of subReaders[i]
	starts []int

	// numDocs is the total number of live documents
	numDocs int

	// maxDoc is the maximum document ID plus one
	maxDoc int

	// readerContext is the reader context for this composite reader
	readerContext *CompositeReaderContext

	// leafContexts are the leaf reader contexts
	leafContexts []*LeafReaderContext

	// mu protects context initialization
	mu sync.RWMutex
}

// NewBaseCompositeReader creates a new BaseCompositeReader.
//
// The subReaders array must be non-empty and in sequential order.
func NewBaseCompositeReader(subReaders []IndexReaderInterface) (*BaseCompositeReader, error) {
	if len(subReaders) == 0 {
		return nil, fmt.Errorf("subReaders array must be non-empty")
	}

	reader := &BaseCompositeReader{
		CompositeReader: NewCompositeReader(),
		subReaders:      make([]IndexReaderInterface, len(subReaders)),
		starts:          make([]int, len(subReaders)+1),
	}

	// Copy sub-readers and calculate starts
	maxDoc := 0
	numDocs := 0
	for i, subReader := range subReaders {
		reader.subReaders[i] = subReader
		reader.starts[i] = maxDoc
		maxDoc += subReader.MaxDoc()
		numDocs += subReader.NumDocs()
	}
	reader.starts[len(subReaders)] = maxDoc
	reader.maxDoc = maxDoc
	reader.numDocs = numDocs

	// Set the document counts in the base IndexReader
	reader.SetMaxDoc(maxDoc)
	reader.SetNumDocs(numDocs)
	reader.SetDocCount(maxDoc)

	return reader, nil
}

// GetSequentialSubReaders returns the sub-readers in sequential order.
func (r *BaseCompositeReader) GetSequentialSubReaders() []IndexReaderInterface {
	return r.subReaders
}

// GetSubReader returns the sub-reader for the given document ID.
func (r *BaseCompositeReader) GetSubReader(docID int) IndexReaderInterface {
	idx := r.ReaderIndex(docID)
	if idx < 0 || idx >= len(r.subReaders) {
		return nil
	}
	return r.subReaders[idx]
}

// ReaderIndex returns the index of the sub-reader that contains the given doc ID.
// Returns -1 if the doc ID is out of range.
func (r *BaseCompositeReader) ReaderIndex(docID int) int {
	if docID < 0 || docID >= r.maxDoc {
		return -1
	}

	// Binary search for the correct sub-reader
	lo, hi := 0, len(r.subReaders)
	for lo < hi {
		mid := (lo + hi) >> 1
		if docID < r.starts[mid] {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo - 1
}

// ReaderBase returns the document ID offset for the given sub-reader index.
func (r *BaseCompositeReader) ReaderBase(readerIndex int) int {
	if readerIndex < 0 || readerIndex >= len(r.starts) {
		return -1
	}
	return r.starts[readerIndex]
}

// GetContext returns the reader context.
func (r *BaseCompositeReader) GetContext() (IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	if r.readerContext != nil {
		r.mu.RUnlock()
		return r.readerContext, nil
	}
	r.mu.RUnlock()

	// Initialize context
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if r.readerContext != nil {
		return r.readerContext, nil
	}

	// Create leaf contexts
	r.leafContexts = make([]*LeafReaderContext, len(r.subReaders))
	for i, subReader := range r.subReaders {
		leafReader, ok := subReader.(*LeafReader)
		if !ok {
			return nil, fmt.Errorf("sub-reader %d is not a LeafReader", i)
		}
		r.leafContexts[i] = NewLeafReaderContext(leafReader, nil, i, r.starts[i])
	}

	// Create composite context - pass the composite reader itself
	// The CompositeReader embeds IndexReader which implements IndexReaderInterface
	compReader := r.CompositeReader.IndexReader
	// Convert to IndexReaderInterface
	var readerIf IndexReaderInterface = compReader
	r.readerContext = NewCompositeReaderContextWithChildren(readerIf, nil, nil, r.leafContexts)

	return r.readerContext, nil
}

// Leaves returns all leaf reader contexts.
func (r *BaseCompositeReader) Leaves() ([]*LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	compCtx, ok := ctx.(*CompositeReaderContext)
	if !ok {
		return nil, fmt.Errorf("context is not a CompositeReaderContext")
	}
	return compCtx.Leaves(), nil
}

// NumDocs returns the number of live documents.
func (r *BaseCompositeReader) NumDocs() int {
	return r.numDocs
}

// MaxDoc returns the maximum document ID plus one.
func (r *BaseCompositeReader) MaxDoc() int {
	return r.maxDoc
}

// HasDeletions returns true if any sub-reader has deletions.
func (r *BaseCompositeReader) HasDeletions() bool {
	for _, subReader := range r.subReaders {
		if subReader.HasDeletions() {
			return true
		}
	}
	return false
}

// closeInternal closes the reader and all sub-readers.
func (r *BaseCompositeReader) closeInternal() error {
	// Close all sub-readers
	var lastErr error
	for _, subReader := range r.subReaders {
		if err := subReader.Close(); err != nil {
			lastErr = err
		}
	}

	// Call parent close
	if err := r.IndexReader.Close(); err != nil {
		lastErr = err
	}

	return lastErr
}

// BaseCompositeReaderInterface defines the interface for base composite readers.
type BaseCompositeReaderInterface interface {
	CompositeReaderInterface

	// ReaderIndex returns the index of the sub-reader for the given doc ID.
	ReaderIndex(docID int) int

	// ReaderBase returns the document ID offset for the given sub-reader index.
	ReaderBase(readerIndex int) int
}

// Ensure BaseCompositeReader implements BaseCompositeReaderInterface
var _ BaseCompositeReaderInterface = (*BaseCompositeReader)(nil)
