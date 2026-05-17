// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// ParallelCompositeReader is a CompositeReader that combines several parallel
// composite sub-readers. Each sub-reader must share the same leaf structure
// (same number of leaves with matching doc counts). Mirrors
// org.apache.lucene.index.ParallelCompositeReader from Apache Lucene 10.4.0.
//
// Gocene skeleton: this initial port wires construction, accessors and
// idempotent doClose. The fully wrapped getSequentialSubReaders that produces
// per-leaf ParallelLeafReader composition is deferred (see backlog #2702).
type ParallelCompositeReader struct {
	*CompositeReader

	closeSubReaders     bool
	parallelReaders     []*CompositeReader
	storedFieldsReaders []*CompositeReader

	closeMu sync.Mutex
	closed  bool
}

// NewParallelCompositeReader mirrors the (CompositeReader...) Java overload.
func NewParallelCompositeReader(readers ...*CompositeReader) (*ParallelCompositeReader, error) {
	return NewParallelCompositeReaderFull(true, readers, readers)
}

// NewParallelCompositeReaderWithClose mirrors the (closeSubReaders, CompositeReader...) overload.
func NewParallelCompositeReaderWithClose(closeSubReaders bool, readers ...*CompositeReader) (*ParallelCompositeReader, error) {
	return NewParallelCompositeReaderFull(closeSubReaders, readers, readers)
}

// NewParallelCompositeReaderFull mirrors the full Java constructor with both
// the main and stored-fields parallel reader sets.
func NewParallelCompositeReaderFull(closeSubReaders bool, readers, storedFieldsReaders []*CompositeReader) (*ParallelCompositeReader, error) {
	if len(readers) == 0 {
		return nil, fmt.Errorf("at least one sub-reader is required")
	}
	baseMaxDoc := readers[0].MaxDoc()
	for _, r := range readers {
		if r.MaxDoc() != baseMaxDoc {
			return nil, fmt.Errorf("parallel composite sub-readers must have the same MaxDoc; got %d and %d",
				baseMaxDoc, r.MaxDoc())
		}
	}
	return &ParallelCompositeReader{
		CompositeReader:     readers[0],
		closeSubReaders:     closeSubReaders,
		parallelReaders:     readers,
		storedFieldsReaders: storedFieldsReaders,
	}, nil
}

// GetParallelReaders returns the underlying parallel sub-readers.
func (p *ParallelCompositeReader) GetParallelReaders() []*CompositeReader {
	return p.parallelReaders
}

// DoClose closes all parallel sub-readers if closeSubReaders is true. Idempotent.
func (p *ParallelCompositeReader) DoClose() error {
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if !p.closeSubReaders {
		return nil
	}
	var firstErr error
	for _, r := range p.parallelReaders {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
