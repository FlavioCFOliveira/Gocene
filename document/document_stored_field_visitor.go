// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

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
//   - The visitor methods take a field name (string) rather than a
//     FieldInfo, mirroring the Gocene index.StoredFieldVisitor interface.
//     NeedsField therefore also takes a string. When the FieldInfo type
//     gains a richer public surface this can be revisited.
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

// NeedsField reports whether the visitor wishes to receive the named field.
// Mirrors Lucene's needsField(FieldInfo) returning YES/NO.
func (v *DocumentStoredFieldVisitor) NeedsField(name string) bool {
	if v.loadAllField {
		return true
	}
	_, ok := v.fieldsToAdd[name]
	return ok
}

// StringField is invoked for each visited stored string field.
func (v *DocumentStoredFieldVisitor) StringField(name string, value string) {
	f, err := NewStoredField(name, value)
	if err == nil {
		v.doc.Add(f)
	}
}

// BinaryField is invoked for each visited stored binary field.
func (v *DocumentStoredFieldVisitor) BinaryField(name string, value []byte) {
	f, err := NewStoredFieldFromBytes(name, value)
	if err == nil {
		v.doc.Add(f)
	}
}

// IntField is invoked for each visited stored int field.
func (v *DocumentStoredFieldVisitor) IntField(name string, value int) {
	f, err := NewIntField(name, value, true)
	if err == nil {
		v.doc.Add(f)
	}
}

// LongField is invoked for each visited stored long field.
func (v *DocumentStoredFieldVisitor) LongField(name string, value int64) {
	f, err := NewLongField(name, value, true)
	if err == nil {
		v.doc.Add(f)
	}
}

// FloatField is invoked for each visited stored float field.
func (v *DocumentStoredFieldVisitor) FloatField(name string, value float32) {
	f, err := NewFloatField(name, value, true)
	if err == nil {
		v.doc.Add(f)
	}
}

// DoubleField is invoked for each visited stored double field.
func (v *DocumentStoredFieldVisitor) DoubleField(name string, value float64) {
	f, err := NewDoubleField(name, value, true)
	if err == nil {
		v.doc.Add(f)
	}
}

// GetDocument returns the reconstructed Document.
func (v *DocumentStoredFieldVisitor) GetDocument() *Document {
	return v.doc
}
