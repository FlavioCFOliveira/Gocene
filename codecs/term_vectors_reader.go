// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BaseTermVectorsReader provides a base implementation of TermVectorsReader.
// This can be embedded in custom TermVectorsReader implementations to get
// default implementations for common methods.
type BaseTermVectorsReader struct {
	mu          sync.RWMutex
	closed      bool
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	fieldInfos  *index.FieldInfos
}

// NewBaseTermVectorsReader creates a new BaseTermVectorsReader.
func NewBaseTermVectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *BaseTermVectorsReader {
	return &BaseTermVectorsReader{
		directory:   dir,
		segmentInfo: segmentInfo,
		fieldInfos:  fieldInfos,
	}
}

// GetDirectory returns the directory.
func (r *BaseTermVectorsReader) GetDirectory() store.Directory {
	return r.directory
}

// GetSegmentInfo returns the segment info.
func (r *BaseTermVectorsReader) GetSegmentInfo() *index.SegmentInfo {
	return r.segmentInfo
}

// GetFieldInfos returns the field infos.
func (r *BaseTermVectorsReader) GetFieldInfos() *index.FieldInfos {
	return r.fieldInfos
}

// IsClosed returns true if this reader has been closed.
func (r *BaseTermVectorsReader) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
}

// Close releases resources.
// This implements the TermVectorsReader interface.
func (r *BaseTermVectorsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return nil
}

// Get retrieves term vectors for the given document ID.
// This must be implemented by subclasses.
func (r *BaseTermVectorsReader) Get(docID int) (index.Fields, error) {
	return nil, fmt.Errorf("Get not implemented")
}

// GetField retrieves the term vector for a specific field in a document.
// This must be implemented by subclasses.
func (r *BaseTermVectorsReader) GetField(docID int, field string) (index.Terms, error) {
	return nil, fmt.Errorf("GetField not implemented")
}

// TermVectorsReaderImpl is a concrete implementation of TermVectorsReader
// that reads term vectors from memory.
type TermVectorsReaderImpl struct {
	*BaseTermVectorsReader
	docs []TermVectorDocument
}

// TermVectorDocument represents term vectors for a document.
type TermVectorDocument struct {
	Fields map[string]index.Terms
}

// NewTermVectorsReaderImpl creates a new TermVectorsReaderImpl.
func NewTermVectorsReaderImpl(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *TermVectorsReaderImpl {
	return &TermVectorsReaderImpl{
		BaseTermVectorsReader: NewBaseTermVectorsReader(dir, segmentInfo, fieldInfos),
		docs:                  make([]TermVectorDocument, 0),
	}
}

// AddDocument adds a document with term vectors.
func (r *TermVectorsReaderImpl) AddDocument(doc TermVectorDocument) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.closed {
		r.docs = append(r.docs, doc)
	}
}

// Get retrieves term vectors for the given document ID.
func (r *TermVectorsReaderImpl) Get(docID int) (index.Fields, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("TermVectorsReader is closed")
	}

	if docID < 0 || docID >= len(r.docs) {
		return nil, fmt.Errorf("document ID %d out of range [0, %d)", docID, len(r.docs))
	}

	doc := r.docs[docID]
	return &termVectorFields{fields: doc.Fields}, nil
}

// GetField retrieves the term vector for a specific field in a document.
func (r *TermVectorsReaderImpl) GetField(docID int, field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("TermVectorsReader is closed")
	}

	if docID < 0 || docID >= len(r.docs) {
		return nil, fmt.Errorf("document ID %d out of range [0, %d)", docID, len(r.docs))
	}

	doc := r.docs[docID]
	terms, ok := doc.Fields[field]
	if !ok {
		return nil, nil
	}

	return terms, nil
}

// termVectorFields implements index.Fields for term vectors.
type termVectorFields struct {
	fields map[string]index.Terms
}

// Iterator returns an iterator over all field names.
func (f *termVectorFields) Iterator() (index.FieldIterator, error) {
	fieldNames := make([]string, 0, len(f.fields))
	for name := range f.fields {
		fieldNames = append(fieldNames, name)
	}
	return &termVectorFieldIterator{fields: fieldNames, index: -1}, nil
}

// Get returns the terms for a field.
func (f *termVectorFields) Terms(name string) (index.Terms, error) {
	terms, ok := f.fields[name]
	if !ok {
		return nil, nil
	}
	return terms, nil
}

// Size returns the number of fields.
func (f *termVectorFields) Size() int {
	return len(f.fields)
}

// termVectorFieldIterator implements index.FieldIterator for term vectors.
type termVectorFieldIterator struct {
	fields []string
	index  int
}

// Next advances to the next field name.
func (i *termVectorFieldIterator) Next() (string, error) {
	i.index++
	if i.index >= len(i.fields) {
		return "", nil
	}
	return i.fields[i.index], nil
}

// HasNext returns true if there are more field names.
func (i *termVectorFieldIterator) HasNext() bool {
	return i.index+1 < len(i.fields)
}

// EmptyTermVectorsReader is a TermVectorsReader with no documents.
// This is useful for segments that have no term vectors.
type EmptyTermVectorsReader struct {
	*BaseTermVectorsReader
}

// NewEmptyTermVectorsReader creates a new EmptyTermVectorsReader.
func NewEmptyTermVectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *EmptyTermVectorsReader {
	return &EmptyTermVectorsReader{
		BaseTermVectorsReader: NewBaseTermVectorsReader(dir, segmentInfo, fieldInfos),
	}
}

// Get returns empty fields.
func (r *EmptyTermVectorsReader) Get(docID int) (index.Fields, error) {
	return &index.EmptyFields{}, nil
}

// GetField returns nil.
func (r *EmptyTermVectorsReader) GetField(docID int, field string) (index.Terms, error) {
	return nil, nil
}

// Ensure implementations satisfy the interface
var _ TermVectorsReader = (*BaseTermVectorsReader)(nil)
var _ TermVectorsReader = (*TermVectorsReaderImpl)(nil)
var _ TermVectorsReader = (*EmptyTermVectorsReader)(nil)
