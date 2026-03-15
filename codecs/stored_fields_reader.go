// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BaseStoredFieldsReader provides a base implementation of StoredFieldsReader.
// This can be embedded in custom StoredFieldsReader implementations to get
// default implementations for common methods.
type BaseStoredFieldsReader struct {
	mu          sync.RWMutex
	closed      bool
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	fieldInfos  *index.FieldInfos
}

// NewBaseStoredFieldsReader creates a new BaseStoredFieldsReader.
func NewBaseStoredFieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *BaseStoredFieldsReader {
	return &BaseStoredFieldsReader{
		directory:   dir,
		segmentInfo:   segmentInfo,
		fieldInfos:    fieldInfos,
	}
}

// GetDirectory returns the directory.
func (r *BaseStoredFieldsReader) GetDirectory() store.Directory {
	return r.directory
}

// GetSegmentInfo returns the segment info.
func (r *BaseStoredFieldsReader) GetSegmentInfo() *index.SegmentInfo {
	return r.segmentInfo
}

// GetFieldInfos returns the field infos.
func (r *BaseStoredFieldsReader) GetFieldInfos() *index.FieldInfos {
	return r.fieldInfos
}

// IsClosed returns true if this reader has been closed.
func (r *BaseStoredFieldsReader) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
}

// Close releases resources.
// This implements the StoredFieldsReader interface.
func (r *BaseStoredFieldsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return nil
}

// VisitDocument visits the stored fields for a document.
// This must be implemented by subclasses.
func (r *BaseStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	return fmt.Errorf("VisitDocument not implemented")
}

// StoredFieldsReaderImpl is a concrete implementation of StoredFieldsReader
// that reads stored fields from memory.
type StoredFieldsReaderImpl struct {
	*BaseStoredFieldsReader
	docs []StoredDocument
}

// StoredDocument represents a document with its stored fields.
type StoredDocument struct {
	Fields []StoredField
}

// StoredField represents a single stored field.
type StoredField struct {
	Name  string
	Type  byte
	Value interface{}
}

// NewStoredFieldsReaderImpl creates a new StoredFieldsReaderImpl.
func NewStoredFieldsReaderImpl(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *StoredFieldsReaderImpl {
	return &StoredFieldsReaderImpl{
		BaseStoredFieldsReader: NewBaseStoredFieldsReader(dir, segmentInfo, fieldInfos),
		docs:                   make([]StoredDocument, 0),
	}
}

// AddDocument adds a document to the reader.
func (r *StoredFieldsReaderImpl) AddDocument(doc StoredDocument) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.closed {
		r.docs = append(r.docs, doc)
	}
}

// VisitDocument visits the stored fields for a document.
func (r *StoredFieldsReaderImpl) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return fmt.Errorf("StoredFieldsReader is closed")
	}

	if docID < 0 || docID >= len(r.docs) {
		return fmt.Errorf("document ID %d out of range [0, %d)", docID, len(r.docs))
	}

	doc := r.docs[docID]
	for _, field := range doc.Fields {
		switch field.Type {
		case FieldTypeString:
			if v, ok := field.Value.(string); ok {
				visitor.StringField(field.Name, v)
			}
		case FieldTypeBinary:
			if v, ok := field.Value.([]byte); ok {
				visitor.BinaryField(field.Name, v)
			}
		case FieldTypeInt:
			if v, ok := field.Value.(int); ok {
				visitor.IntField(field.Name, v)
			}
		case FieldTypeLong:
			if v, ok := field.Value.(int64); ok {
				visitor.LongField(field.Name, v)
			}
		case FieldTypeFloat:
			if v, ok := field.Value.(float32); ok {
				visitor.FloatField(field.Name, v)
			}
		case FieldTypeDouble:
			if v, ok := field.Value.(float64); ok {
				visitor.DoubleField(field.Name, v)
			}
		}
	}

	return nil
}

// Field type constants for serialization
const (
	FieldTypeString = 1
	FieldTypeBinary = 2
	FieldTypeInt    = 3
	FieldTypeLong   = 4
	FieldTypeFloat  = 5
	FieldTypeDouble = 6
)

// EmptyStoredFieldsReader is a StoredFieldsReader with no documents.
// This is useful for segments that have no stored fields.
type EmptyStoredFieldsReader struct {
	*BaseStoredFieldsReader
}

// NewEmptyStoredFieldsReader creates a new EmptyStoredFieldsReader.
func NewEmptyStoredFieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) *EmptyStoredFieldsReader {
	return &EmptyStoredFieldsReader{
		BaseStoredFieldsReader: NewBaseStoredFieldsReader(dir, segmentInfo, fieldInfos),
	}
}

// VisitDocument does nothing.
func (r *EmptyStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	return nil
}

// DocumentStoredFieldVisitor is a StoredFieldVisitor that builds a document.
type DocumentStoredFieldVisitor struct {
	doc *document.Document
}

// NewDocumentStoredFieldVisitor creates a new DocumentStoredFieldVisitor.
func NewDocumentStoredFieldVisitor() *DocumentStoredFieldVisitor {
	return &DocumentStoredFieldVisitor{
		doc: document.NewDocument(),
	}
}

// StringField adds a string field to the document.
func (v *DocumentStoredFieldVisitor) StringField(name string, value string) {
	field, _ := document.NewTextField(name, value, true)
	v.doc.Add(field)
}

// BinaryField adds a binary field to the document.
func (v *DocumentStoredFieldVisitor) BinaryField(name string, value []byte) {
	field, _ := document.NewStoredFieldFromBytes(name, value)
	v.doc.Add(field)
}

// IntField adds an int field to the document.
func (v *DocumentStoredFieldVisitor) IntField(name string, value int) {
	field, _ := document.NewIntField(name, value, true)
	v.doc.Add(field)
}

// LongField adds a long field to the document.
func (v *DocumentStoredFieldVisitor) LongField(name string, value int64) {
	field, _ := document.NewLongField(name, value, true)
	v.doc.Add(field)
}

// FloatField adds a float field to the document.
func (v *DocumentStoredFieldVisitor) FloatField(name string, value float32) {
	field, _ := document.NewFloatField(name, value, true)
	v.doc.Add(field)
}

// DoubleField adds a double field to the document.
func (v *DocumentStoredFieldVisitor) DoubleField(name string, value float64) {
	field, _ := document.NewDoubleField(name, value, true)
	v.doc.Add(field)
}

// GetDocument returns the built document.
func (v *DocumentStoredFieldVisitor) GetDocument() *document.Document {
	return v.doc
}

// Ensure implementations satisfy the interface
var _ StoredFieldsReader = (*BaseStoredFieldsReader)(nil)
var _ StoredFieldsReader = (*StoredFieldsReaderImpl)(nil)
var _ StoredFieldsReader = (*EmptyStoredFieldsReader)(nil)
var _ StoredFieldVisitor = (*DocumentStoredFieldVisitor)(nil)
