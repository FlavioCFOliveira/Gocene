// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// StoredFieldsFormat handles encoding/decoding of stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsFormat.
//
// Stored fields are kept in files like _X.fdt (data) and _X.fdx (index)
// and contain the original field values that can be retrieved at search time.
type StoredFieldsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsReader returns a reader for stored fields.
	// The caller should close the returned reader when done.
	FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error)

	// FieldsWriter returns a writer for stored fields.
	// The caller should close the returned writer when done.
	FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error)
}

// BaseStoredFieldsFormat provides common functionality.
type BaseStoredFieldsFormat struct {
	name string
}

// NewBaseStoredFieldsFormat creates a new BaseStoredFieldsFormat.
func NewBaseStoredFieldsFormat(name string) *BaseStoredFieldsFormat {
	return &BaseStoredFieldsFormat{name: name}
}

// Name returns the format name.
func (f *BaseStoredFieldsFormat) Name() string {
	return f.name
}

// FieldsReader returns a fields reader (must be implemented by subclasses).
func (f *BaseStoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error) {
	return nil, fmt.Errorf("FieldsReader not implemented")
}

// FieldsWriter returns a fields writer (must be implemented by subclasses).
func (f *BaseStoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error) {
	return nil, fmt.Errorf("FieldsWriter not implemented")
}

// Lucene104StoredFieldsFormat is the Lucene 10.4 stored fields format.
//
// This is a placeholder implementation. A full implementation would include:
//   - Block compression for stored fields
//   - Field-level compression options
//   - Chunk-based storage for better compression
type Lucene104StoredFieldsFormat struct {
	*BaseStoredFieldsFormat
}

// NewLucene104StoredFieldsFormat creates a new Lucene104StoredFieldsFormat.
func NewLucene104StoredFieldsFormat() *Lucene104StoredFieldsFormat {
	return &Lucene104StoredFieldsFormat{
		BaseStoredFieldsFormat: NewBaseStoredFieldsFormat("Lucene104StoredFieldsFormat"),
	}
}

// FieldsReader returns a stored fields reader.
func (f *Lucene104StoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error) {
	// TODO: Implement full stored fields reader
	return nil, fmt.Errorf("Lucene104StoredFieldsFormat.FieldsReader not yet implemented")
}

// FieldsWriter returns a stored fields writer.
func (f *Lucene104StoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error) {
	// TODO: Implement full stored fields writer
	return nil, fmt.Errorf("Lucene104StoredFieldsFormat.FieldsWriter not yet implemented")
}

// StoredFieldsReader is a reader for stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsReader.
type StoredFieldsReader interface {
	// VisitDocument visits the stored fields for a document.
	// The visitor is called for each stored field in the document.
	VisitDocument(docID int, visitor StoredFieldVisitor) error

	// Close releases resources.
	Close() error
}

// StoredFieldsWriter is a writer for stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsWriter.
type StoredFieldsWriter interface {
	// StartDocument starts writing a document.
	StartDocument() error

	// FinishDocument finishes writing the current document.
	FinishDocument() error

	// WriteField writes a field.
	WriteField(field document.IndexableField) error

	// Close releases resources.
	Close() error
}

// StoredFieldVisitor is called for each stored field when visiting a document.
type StoredFieldVisitor interface {
	// StringField is called for a stored string field.
	StringField(field string, value string)

	// BinaryField is called for a stored binary field.
	BinaryField(field string, value []byte)

	// IntField is called for a stored int field.
	IntField(field string, value int)

	// LongField is called for a stored long field.
	LongField(field string, value int64)

	// FloatField is called for a stored float field.
	FloatField(field string, value float32)

	// DoubleField is called for a stored double field.
	DoubleField(field string, value float64)
}
