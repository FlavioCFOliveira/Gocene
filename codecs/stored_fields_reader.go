// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
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

// NeedsField accepts every stored field: this visitor rebuilds the whole
// document. Mirrors the YES-for-everything needsField of a load-all
// StoredFieldVisitor.
func (v *DocumentStoredFieldVisitor) NeedsField(*spi.FieldInfo) (spi.StoredFieldVisitorStatus, error) {
	return spi.StoredFieldVisitorStatusYes, nil
}

// StringField adds a string field to the document.
func (v *DocumentStoredFieldVisitor) StringField(fieldInfo *spi.FieldInfo, value string) error {
	field, err := document.NewTextField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// BinaryField adds a binary field to the document.
func (v *DocumentStoredFieldVisitor) BinaryField(fieldInfo *spi.FieldInfo, value []byte) error {
	field, err := document.NewStoredFieldFromBytes(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// IntField adds an int field to the document.
func (v *DocumentStoredFieldVisitor) IntField(fieldInfo *spi.FieldInfo, value int) error {
	field, err := document.NewIntField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// LongField adds a long field to the document.
func (v *DocumentStoredFieldVisitor) LongField(fieldInfo *spi.FieldInfo, value int64) error {
	field, err := document.NewLongField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// FloatField adds a float field to the document.
func (v *DocumentStoredFieldVisitor) FloatField(fieldInfo *spi.FieldInfo, value float32) error {
	field, err := document.NewFloatField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// DoubleField adds a double field to the document.
func (v *DocumentStoredFieldVisitor) DoubleField(fieldInfo *spi.FieldInfo, value float64) error {
	field, err := document.NewDoubleField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(field)
	return nil
}

// GetDocument returns the built document.
func (v *DocumentStoredFieldVisitor) GetDocument() *document.Document {
	return v.doc
}

// Ensure implementations satisfy the interface
var _ StoredFieldVisitor = (*DocumentStoredFieldVisitor)(nil)
