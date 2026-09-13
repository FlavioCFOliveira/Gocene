// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/document"
)

// Apache Lucene 10.5.0 has no BaseStoredFieldsReader, StoredFieldsReaderImpl or
// EmptyStoredFieldsReader; the three types that used to live here were invented
// by this port. org.apache.lucene.codecs.StoredFieldsReader is abstract in
// document(int, StoredFieldVisitor), clone(), checkIntegrity() and close(), so
// there is no default body for a base type to carry, and giving one a
// checkIntegrity() that does nothing let an invented type satisfy the real
// interface while hiding that the port of the reader is missing. The reader
// Lucene actually uses is
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsReader
// (reached through Lucene90StoredFieldsFormat, which is what Lucene104Codec
// returns from storedFieldsFormat()); Gocene's counterpart lives in
// codecs/lucene90/compressing.

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

// Field type constants for serialization
const (
	FieldTypeString = 1
	FieldTypeBinary = 2
	FieldTypeInt    = 3
	FieldTypeLong   = 4
	FieldTypeFloat  = 5
	FieldTypeDouble = 6
)

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
var _ StoredFieldVisitor = (*DocumentStoredFieldVisitor)(nil)
