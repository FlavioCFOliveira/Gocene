// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// ParallelLeafReader is a LeafReader that aggregates several parallel sub-readers
// which share the same docID space but contribute different fields. Mirrors
// org.apache.lucene.index.ParallelLeafReader from Apache Lucene 10.4.0.
//
// The semantic contract: for the same docID across all parallel sub-readers,
// the document is the union of the fields contributed by each sub-reader.
// Field names must be unique across sub-readers.
//
// Gocene skeleton: this initial port wires the construction API and the
// per-field dispatch table. Full Lucene-parity behaviour (StoredFields and
// TermVectors aggregation, CacheHelper composition, doClose cascade) is
// deferred to a follow-up task; see backlog #2702.
type ParallelLeafReader struct {
	*LeafReader

	closeSubReaders     bool
	parallelReaders     []*LeafReader
	storedFieldsReaders []*LeafReader

	// fieldToReader maps each field name to the sub-reader that contributed it.
	// Built once at construction; immutable thereafter.
	fieldToReader map[string]*LeafReader

	closeMu sync.Mutex
	closed  bool
}

// NewParallelLeafReader constructs a ParallelLeafReader over the given sub-readers
// (defaults: closeSubReaders=true, storedFieldsReaders=readers).
func NewParallelLeafReader(readers ...*LeafReader) (*ParallelLeafReader, error) {
	return NewParallelLeafReaderFull(true, readers, readers)
}

// NewParallelLeafReaderWithClose mirrors the (closeSubReaders, readers...) Java overload.
func NewParallelLeafReaderWithClose(closeSubReaders bool, readers ...*LeafReader) (*ParallelLeafReader, error) {
	return NewParallelLeafReaderFull(closeSubReaders, readers, readers)
}

// NewParallelLeafReaderFull mirrors the (closeSubReaders, readers, storedFieldsReaders)
// Java overload. All sub-readers must have the same MaxDoc; field names must
// be unique across readers.
func NewParallelLeafReaderFull(closeSubReaders bool, readers, storedFieldsReaders []*LeafReader) (*ParallelLeafReader, error) {
	if len(readers) == 0 {
		return nil, fmt.Errorf("at least one sub-reader is required")
	}
	baseMaxDoc := readers[0].MaxDoc()
	fieldMap := make(map[string]*LeafReader, len(readers)*8)
	for _, r := range readers {
		if r.MaxDoc() != baseMaxDoc {
			return nil, fmt.Errorf("parallel sub-readers must have the same MaxDoc; got %d and %d",
				baseMaxDoc, r.MaxDoc())
		}
		if fi := r.GetFieldInfos(); fi != nil {
			for _, name := range fi.Names() {
				if _, dup := fieldMap[name]; dup {
					return nil, fmt.Errorf("duplicate field %q across parallel sub-readers", name)
				}
				fieldMap[name] = r
			}
		}
	}
	// Construct the embedded LeafReader from the first sub-reader's segmentInfo
	// as a placeholder; per-field dispatch routes through fieldToReader.
	base := NewLeafReader(readers[0].GetSegmentInfo())
	return &ParallelLeafReader{
		LeafReader:          base,
		closeSubReaders:     closeSubReaders,
		parallelReaders:     readers,
		storedFieldsReaders: storedFieldsReaders,
		fieldToReader:       fieldMap,
	}, nil
}

// GetParallelReaders returns the underlying parallel sub-readers.
func (p *ParallelLeafReader) GetParallelReaders() []*LeafReader {
	return p.parallelReaders
}

// readerFor returns the sub-reader that contributed the given field, or nil
// if the field is not present in any sub-reader.
func (p *ParallelLeafReader) readerFor(field string) *LeafReader {
	return p.fieldToReader[field]
}

// GetNumericDocValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetNumericDocValues(field)
	}
	return nil, nil
}

// GetBinaryDocValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetBinaryDocValues(field)
	}
	return nil, nil
}

// GetSortedDocValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetSortedDocValues(field)
	}
	return nil, nil
}

// GetSortedNumericDocValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetSortedNumericDocValues(field)
	}
	return nil, nil
}

// GetSortedSetDocValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetSortedSetDocValues(field)
	}
	return nil, nil
}

// GetNormValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetNormValues(field string) (NumericDocValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetNormValues(field)
	}
	return nil, nil
}

// GetPointValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetPointValues(field string) (PointValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetPointValues(field)
	}
	return nil, nil
}

// GetFloatVectorValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetFloatVectorValues(field)
	}
	return nil, nil
}

// GetByteVectorValues dispatches to the sub-reader that owns the field.
func (p *ParallelLeafReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	if r := p.readerFor(field); r != nil {
		return r.GetByteVectorValues(field)
	}
	return nil, nil
}

// DoClose closes parallel sub-readers if closeSubReaders is true. Idempotent.
func (p *ParallelLeafReader) DoClose() error {
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
