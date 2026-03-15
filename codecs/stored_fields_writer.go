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

// BaseStoredFieldsWriter provides a base implementation of StoredFieldsWriter.
// This can be embedded in custom StoredFieldsWriter implementations to get
// default implementations for common methods.
type BaseStoredFieldsWriter struct {
	mu          sync.Mutex
	closed      bool
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	currentDoc  int
}

// NewBaseStoredFieldsWriter creates a new BaseStoredFieldsWriter.
func NewBaseStoredFieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo) *BaseStoredFieldsWriter {
	return &BaseStoredFieldsWriter{
		directory:   dir,
		segmentInfo: segmentInfo,
		currentDoc:  -1,
	}
}

// GetDirectory returns the directory.
func (w *BaseStoredFieldsWriter) GetDirectory() store.Directory {
	return w.directory
}

// GetSegmentInfo returns the segment info.
func (w *BaseStoredFieldsWriter) GetSegmentInfo() *index.SegmentInfo {
	return w.segmentInfo
}

// GetCurrentDoc returns the current document number.
func (w *BaseStoredFieldsWriter) GetCurrentDoc() int {
	return w.currentDoc
}

// IsClosed returns true if this writer has been closed.
func (w *BaseStoredFieldsWriter) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// StartDocument starts writing a document.
// This implements the StoredFieldsWriter interface.
func (w *BaseStoredFieldsWriter) StartDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("StoredFieldsWriter is closed")
	}

	w.currentDoc++
	return nil
}

// FinishDocument finishes writing the current document.
// This must be implemented by subclasses.
func (w *BaseStoredFieldsWriter) FinishDocument() error {
	return nil
}

// WriteField writes a field.
// This must be implemented by subclasses.
func (w *BaseStoredFieldsWriter) WriteField(field document.IndexableField) error {
	return fmt.Errorf("WriteField not implemented")
}

// Close releases resources.
// This implements the StoredFieldsWriter interface.
func (w *BaseStoredFieldsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	return nil
}

// StoredFieldsWriterImpl is a concrete implementation of StoredFieldsWriter
// that writes stored fields to memory and then to disk.
type StoredFieldsWriterImpl struct {
	*BaseStoredFieldsWriter
	docs       []StoredDocumentWriter
	currentDoc *StoredDocumentWriter
}

// StoredDocumentWriter represents a document being written.
type StoredDocumentWriter struct {
	Fields []StoredFieldWriter
}

// StoredFieldWriter represents a field being written.
type StoredFieldWriter struct {
	Name  string
	Type  byte
	Value interface{}
}

// NewStoredFieldsWriterImpl creates a new StoredFieldsWriterImpl.
func NewStoredFieldsWriterImpl(dir store.Directory, segmentInfo *index.SegmentInfo) *StoredFieldsWriterImpl {
	return &StoredFieldsWriterImpl{
		BaseStoredFieldsWriter: NewBaseStoredFieldsWriter(dir, segmentInfo),
		docs:                   make([]StoredDocumentWriter, 0),
	}
}

// StartDocument starts writing a document.
func (w *StoredFieldsWriterImpl) StartDocument() error {
	if err := w.BaseStoredFieldsWriter.StartDocument(); err != nil {
		return err
	}

	w.currentDoc = &StoredDocumentWriter{Fields: make([]StoredFieldWriter, 0)}
	return nil
}

// FinishDocument finishes writing the current document.
func (w *StoredFieldsWriterImpl) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("StoredFieldsWriter is closed")
	}

	if w.currentDoc == nil {
		return fmt.Errorf("no document started")
	}

	w.docs = append(w.docs, *w.currentDoc)
	w.currentDoc = nil
	return nil
}

// WriteField writes a field.
func (w *StoredFieldsWriterImpl) WriteField(field document.IndexableField) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("StoredFieldsWriter is closed")
	}

	if w.currentDoc == nil {
		return fmt.Errorf("no document started")
	}

	sf := StoredFieldWriter{Name: field.Name()}

	// Determine field type and value
	if field.StringValue() != "" {
		sf.Type = FieldTypeString
		sf.Value = field.StringValue()
	} else if field.BinaryValue() != nil && len(field.BinaryValue()) > 0 {
		sf.Type = FieldTypeBinary
		sf.Value = field.BinaryValue()
	} else if field.NumericValue() != nil {
		switch v := field.NumericValue().(type) {
		case int:
			sf.Type = FieldTypeInt
			sf.Value = v
		case int32:
			sf.Type = FieldTypeInt
			sf.Value = int(v)
		case int64:
			sf.Type = FieldTypeLong
			sf.Value = v
		case float32:
			sf.Type = FieldTypeFloat
			sf.Value = v
		case float64:
			sf.Type = FieldTypeDouble
			sf.Value = v
		default:
			// Default to storing as string
			sf.Type = FieldTypeString
			sf.Value = fmt.Sprintf("%v", v)
		}
	} else {
		// Empty field - skip
		return nil
	}

	w.currentDoc.Fields = append(w.currentDoc.Fields, sf)
	return nil
}

// GetDocuments returns the documents that have been written.
func (w *StoredFieldsWriterImpl) GetDocuments() []StoredDocumentWriter {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.docs
}

// NoOpStoredFieldsWriter is a StoredFieldsWriter that does nothing.
// This is useful for testing or when stored fields are not needed.
type NoOpStoredFieldsWriter struct {
	*BaseStoredFieldsWriter
}

// NewNoOpStoredFieldsWriter creates a new NoOpStoredFieldsWriter.
func NewNoOpStoredFieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo) *NoOpStoredFieldsWriter {
	return &NoOpStoredFieldsWriter{
		BaseStoredFieldsWriter: NewBaseStoredFieldsWriter(dir, segmentInfo),
	}
}

// StartDocument does nothing.
func (w *NoOpStoredFieldsWriter) StartDocument() error {
	return nil
}

// FinishDocument does nothing.
func (w *NoOpStoredFieldsWriter) FinishDocument() error {
	return nil
}

// WriteField does nothing.
func (w *NoOpStoredFieldsWriter) WriteField(field document.IndexableField) error {
	return nil
}

// Close does nothing.
func (w *NoOpStoredFieldsWriter) Close() error {
	return nil
}

// Ensure implementations satisfy the interface
var _ StoredFieldsWriter = (*BaseStoredFieldsWriter)(nil)
var _ StoredFieldsWriter = (*StoredFieldsWriterImpl)(nil)
var _ StoredFieldsWriter = (*NoOpStoredFieldsWriter)(nil)
