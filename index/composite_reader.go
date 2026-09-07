// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strings"
	"sync"
)

// CompositeReader is an abstract base class for IndexReaders that are composed of
// multiple sub-readers. This is the Go port of Lucene's org.apache.lucene.index.CompositeReader.
//
// CompositeReader provides a unified view of multiple sub-readers, allowing operations
// to be performed across the entire index while the underlying data is stored in
// separate segments.
type CompositeReader struct {
	*IndexReader

	// subReaders holds the sub-readers.
	// In Lucene 10.5.0, this is managed by subclasses via getSequentialSubReaders().
	// In Gocene, we keep it here for compatibility with DirectoryReader and BaseCompositeReader.
	subReaders []IndexReaderInterface

	// starts contains the starting doc ID for each sub-reader.
	starts []int

	// totalMaxDoc is the total maxDoc across all sub-readers.
	totalMaxDoc int

	// totalNumDocs is the total numDocs across all sub-readers.
	totalNumDocs int

	// readerContext is the reader context for this composite reader.
	readerContext *CompositeReaderContext

	// mu protects context initialization.
	mu sync.RWMutex
}

// NewCompositeReader creates a new CompositeReader.
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
		IndexReader: NewIndexReader(),
		subReaders:  make([]IndexReaderInterface, len(subReaders)),
		starts:      make([]int, len(subReaders)+1),
	}

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
// This is the Go port of Lucene's getSequentialSubReaders().
func (r *CompositeReader) GetSequentialSubReaders() []IndexReaderInterface {
	return r.subReaders
}

// GetContext returns the reader context.
// This is a faithful port of Lucene's final getContext() method.
func (r *CompositeReader) GetContext() (IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	if r.readerContext != nil {
		ctx := r.readerContext
		r.mu.RUnlock()
		return ctx, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.readerContext == nil {
		// In Java: readerContext = CompositeReaderContext.create(this);
		// In Gocene, this is implemented via the CompositeReaderContextBuilder.
		builder := NewCompositeReaderContextBuilder(r)
		ctx, err := builder.Build()
		if err != nil {
			return nil, err
		}
		r.readerContext = ctx
	}
	return r.readerContext, nil
}

// String returns a string representation of the reader.
// This is a faithful port of Lucene's toString() method.
func (r *CompositeReader) String() string {
	var sb strings.Builder

	// In Go, since CompositeReader is embedded, we use "CompositeReader" as the base name.
	// In Java, getClass().getSimpleName() would return the name of the concrete subclass.
	sb.WriteString("CompositeReader")
	sb.WriteByte('(')

	subReaders := r.GetSequentialSubReaders()
	if len(subReaders) > 0 {
		for i, sr := range subReaders {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(fmt.Sprintf("%v", sr))
		}
	}
	sb.WriteByte(')')
	return sb.String()
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

// CompositeReaderInterface defines the interface for composite readers.
type CompositeReaderInterface interface {
	IndexReaderInterface
	GetSequentialSubReaders() []IndexReaderInterface
}

// Ensure CompositeReader implements CompositeReaderInterface
var _ CompositeReaderInterface = (*CompositeReader)(nil)
