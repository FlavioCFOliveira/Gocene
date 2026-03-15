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
}

// NewCompositeReader creates a new CompositeReader.
// This should be called by subclasses.
func NewCompositeReader() *CompositeReader {
	return &CompositeReader{
		IndexReader: NewIndexReader(),
	}
}

// GetSequentialSubReaders returns the sub-readers in sequential order.
// This must be implemented by subclasses.
func (r *CompositeReader) GetSequentialSubReaders() []IndexReaderInterface {
	return nil
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
