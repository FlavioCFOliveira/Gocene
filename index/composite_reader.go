// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// CompositeReader is an abstract base class for IndexReaders that are composed of
// multiple sub-readers. This is the Go port of Lucene's org.apache.lucene.index.CompositeReader.
//
// CompositeReader provides a unified view of multiple sub-readers, allowing operations
// to be performed across the entire index while the underlying data is stored in
// separate segments.
type CompositeReader struct {
	*IndexReader

	// subReaders holds the sub-readers
	subReaders []IndexReaderInterface

	// starts contains the starting doc ID for each sub-reader
	starts []int

	// totalMaxDoc is the total maxDoc across all sub-readers
	totalMaxDoc int

	// totalNumDocs is the total numDocs across all sub-readers
	totalNumDocs int
}

// NewCompositeReader creates a new CompositeReader.
// This should be called by subclasses.
func NewCompositeReader() *CompositeReader {
	return &CompositeReader{
		IndexReader: NewIndexReader(),
	}
}

// NewCompositeReaderWithSubReaders creates a CompositeReader with the given sub-readers.
// This is used by DirectoryReader and other composite reader implementations.
func NewCompositeReaderWithSubReaders(subReaders []IndexReaderInterface) (*CompositeReader, error) {
	if len(subReaders) == 0 {
		return nil, fmt.Errorf("subReaders array must be non-empty")
	}

	reader := &CompositeReader{
		IndexReader:  NewIndexReader(),
		subReaders:   make([]IndexReaderInterface, len(subReaders)),
		starts:       make([]int, len(subReaders)+1),
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
	reader.totalMaxDoc = maxDoc
	reader.totalNumDocs = numDocs

	// Set the document counts in the base IndexReader
	reader.SetMaxDoc(maxDoc)
	reader.SetNumDocs(numDocs)
	reader.SetDocCount(maxDoc)

	return reader, nil
}

// GetSequentialSubReaders returns the sub-readers in sequential order.
func (r *CompositeReader) GetSequentialSubReaders() []IndexReaderInterface {
	return r.subReaders
}

// ReaderIndex returns the index of the sub-reader that contains the given doc ID.
// Returns -1 if the doc ID is out of range.
func (r *CompositeReader) ReaderIndex(docID int) int {
	if docID < 0 || docID >= r.totalMaxDoc {
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
func (r *CompositeReader) ReaderBase(readerIndex int) int {
	if readerIndex < 0 || readerIndex >= len(r.starts) {
		return -1
	}
	return r.starts[readerIndex]
}

// NumDocs returns the number of live documents.
func (r *CompositeReader) NumDocs() int {
	return r.totalNumDocs
}

// MaxDoc returns the maximum document ID plus one.
func (r *CompositeReader) MaxDoc() int {
	return r.totalMaxDoc
}

// DocCount returns the total document count.
func (r *CompositeReader) DocCount() int {
	return r.totalMaxDoc
}

// HasDeletions returns true if any sub-reader has deletions.
func (r *CompositeReader) HasDeletions() bool {
	for _, subReader := range r.subReaders {
		if subReader.HasDeletions() {
			return true
		}
	}
	return false
}

// GetContext returns the reader context.
func (r *CompositeReader) GetContext() (IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	// This should be overridden by subclasses to return the proper context
	return nil, fmt.Errorf("GetContext must be implemented by subclass")
}

// Leaves returns all leaf reader contexts.
func (r *CompositeReader) Leaves() ([]*LeafReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	// This should be overridden by subclasses
	return nil, fmt.Errorf("Leaves must be implemented by subclass")
}

// CompositeReaderInterface defines the interface for composite readers.
type CompositeReaderInterface interface {
	IndexReaderInterface

	// GetSequentialSubReaders returns the sub-readers in sequential order.
	GetSequentialSubReaders() []IndexReaderInterface
}

// Ensure CompositeReader implements CompositeReaderInterface
var _ CompositeReaderInterface = (*CompositeReader)(nil)
