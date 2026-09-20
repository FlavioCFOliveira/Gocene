// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// DocumentStoredFieldVisitor is a StoredFieldVisitor that reconstructs a
// Document from visited stored-field callbacks.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.DocumentStoredFieldVisitor.
//
// Note: a similar struct exists in codecs/stored_fields_reader.go for
// back-compat with already-shipped Gocene code. This canonical version
// lives in the document package matching Lucene's package, and adds the
// fieldsToAdd filtering missing from the codecs/ variant.
//
// Divergences from Java:
//   - Adds StoredField-equivalent fields built via the existing NewXxxField
//     constructors to avoid coupling to a still-evolving StoredField API.
type DocumentStoredFieldVisitor struct {
	doc          *Document
	fieldsToAdd  map[string]struct{}
	loadAllField bool
}

// NewDocumentStoredFieldVisitor creates a visitor that accepts every
// stored field encountered.
func NewDocumentStoredFieldVisitor() *DocumentStoredFieldVisitor {
	return &DocumentStoredFieldVisitor{
		doc:          NewDocument(),
		loadAllField: true,
	}
}

// NewDocumentStoredFieldVisitorFor creates a visitor that only accepts
// fields whose names are present in fields. A nil/empty set selects no
// fields (mirroring Lucene's behaviour with an empty Set).
func NewDocumentStoredFieldVisitorFor(fields ...string) *DocumentStoredFieldVisitor {
	v := &DocumentStoredFieldVisitor{
		doc:         NewDocument(),
		fieldsToAdd: make(map[string]struct{}, len(fields)),
	}
	for _, f := range fields {
		v.fieldsToAdd[f] = struct{}{}
	}
	return v
}

// NeedsField reports whether the visitor wishes to receive the given field.
//
// Mirrors DocumentStoredFieldVisitor.needsField(FieldInfo)
// (DocumentStoredFieldVisitor.java:97-100): YES when no field filter was
// supplied or the filter contains the field name, NO otherwise.
func (v *DocumentStoredFieldVisitor) NeedsField(fieldInfo *spi.FieldInfo) (spi.StoredFieldVisitorStatus, error) {
	if v.loadAllField {
		return spi.StoredFieldVisitorStatusYes, nil
	}
	if _, ok := v.fieldsToAdd[fieldInfo.Name()]; ok {
		return spi.StoredFieldVisitorStatusYes, nil
	}
	return spi.StoredFieldVisitorStatusNo, nil
}

// StringField is invoked for each visited stored string field.
func (v *DocumentStoredFieldVisitor) StringField(fieldInfo *spi.FieldInfo, value string) error {
	f, err := NewStoredField(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// BinaryField is invoked for each visited stored binary field.
func (v *DocumentStoredFieldVisitor) BinaryField(fieldInfo *spi.FieldInfo, value []byte) error {
	f, err := NewStoredFieldFromBytes(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// IntField is invoked for each visited stored int field.
func (v *DocumentStoredFieldVisitor) IntField(fieldInfo *spi.FieldInfo, value int) error {
	f, err := NewIntField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// LongField is invoked for each visited stored long field.
func (v *DocumentStoredFieldVisitor) LongField(fieldInfo *spi.FieldInfo, value int64) error {
	f, err := NewLongField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// FloatField is invoked for each visited stored float field.
func (v *DocumentStoredFieldVisitor) FloatField(fieldInfo *spi.FieldInfo, value float32) error {
	f, err := NewFloatField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// DoubleField is invoked for each visited stored double field.
func (v *DocumentStoredFieldVisitor) DoubleField(fieldInfo *spi.FieldInfo, value float64) error {
	f, err := NewDoubleField(fieldInfo.Name(), value, true)
	if err != nil {
		return err
	}
	v.doc.Add(f)
	return nil
}

// GetDocument returns the reconstructed Document.
func (v *DocumentStoredFieldVisitor) GetDocument() *Document {
	return v.doc
}
