// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LazyDocument defers actually loading a field's value until you ask for it.
// You must not use the returned Field instances after the provided reader has been closed.
//
// This is the Go port of Lucene's org.apache.lucene.misc.document.LazyDocument.
type LazyDocument struct {
	reader     index.IndexReaderInterface
	docID      int
	doc        *Document
	fields     map[int][]*LazyField
	fieldNames map[string]struct{}
	mu         sync.Mutex
}

// NewLazyDocument creates a new LazyDocument for the given reader and document ID.
func NewLazyDocument(reader index.IndexReaderInterface, docID int) *LazyDocument {
	return &LazyDocument{
		reader:     reader,
		docID:      docID,
		fields:     make(map[int][]*LazyField),
		fieldNames: make(map[string]struct{}),
	}
}

// GetField creates a field whose value will be lazy loaded if and when it is used.
//
// NOTE: This method must be called once for each value of the field name specified in
// sequence that the values exist. This method may not be used to generate multiple, lazy,
// field instances referring to the same underlying field instance.
//
// The lazy loading of field values from all instances of field objects returned by
// this method are all backed by a single Document per LazyDocument instance.
func (ld *LazyDocument) GetField(fieldInfo *index.FieldInfo) *LazyField {
	ld.fieldNames[fieldInfo.Name()] = struct{}{}

	values, exists := ld.fields[fieldInfo.Number()]
	if !exists {
		values = []*LazyField{}
		ld.fields[fieldInfo.Number()] = values
	}

	value := &LazyField{
		lazyDoc:  ld,
		name:     fieldInfo.Name(),
		fieldNum: fieldInfo.Number(),
	}
	ld.fields[fieldInfo.Number()] = append(values, value)

	// Edge case: if someone asks this LazyDoc for more LazyFields
	// after other LazyFields from the same LazyDoc have been
	// actualized, we need to force the doc to be re-fetched
	// so the new LazyFields are also populated.
	ld.mu.Lock()
	ld.doc = nil
	ld.mu.Unlock()

	return value
}

// GetDocument returns the underlying document, loading it if necessary.
// This is non-private for test access only.
func (ld *LazyDocument) GetDocument() (*Document, error) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	if ld.doc == nil {
		// Collect field names to load
		names := make([]string, 0, len(ld.fieldNames))
		for name := range ld.fieldNames {
			names = append(names, name)
		}

		// Create a visitor that collects fields into a document
		visitor := &documentCollector{}
		sf, err := ld.reader.StoredFields()
		if err != nil {
			return nil, fmt.Errorf("failed to get stored fields: %w", err)
		}
		if sf == nil {
			return NewDocument(), nil
		}

		err = sf.Document(ld.docID, visitor)
		if err != nil {
			return nil, fmt.Errorf("unable to load document: %w", err)
		}
		ld.doc = visitor.doc
	}

	return ld.doc, nil
}

// fetchRealValues loads the real values for the given field.
func (ld *LazyDocument) fetchRealValues(name string, fieldNum int) error {
	d, err := ld.GetDocument()
	if err != nil {
		return err
	}

	lazyValues, exists := ld.fields[fieldNum]
	if !exists {
		return nil
	}

	realValues := d.GetFieldsByName(name)

	if len(realValues) < len(lazyValues) {
		return fmt.Errorf("more lazy values than real values for field: %s", name)
	}

	for i, lazyField := range lazyValues {
		if lazyField != nil && i < len(realValues) {
			lazyField.realValue = realValues[i]
		}
	}

	return nil
}

// LazyField is a field that loads its value on demand.
// This is the Go port of Lucene's LazyDocument.LazyField.
type LazyField struct {
	lazyDoc   *LazyDocument
	name      string
	fieldNum  int
	realValue IndexableField
	mu        sync.RWMutex
}

// HasBeenLoaded returns true if this field's value has been loaded.
// This is non-private for test access only.
func (lf *LazyField) HasBeenLoaded() bool {
	lf.mu.RLock()
	defer lf.mu.RUnlock()
	return lf.realValue != nil
}

// getRealValue returns the real field value, loading it if necessary.
func (lf *LazyField) getRealValue() (IndexableField, error) {
	lf.mu.RLock()
	if lf.realValue != nil {
		defer lf.mu.RUnlock()
		return lf.realValue, nil
	}
	lf.mu.RUnlock()

	lf.mu.Lock()
	defer lf.mu.Unlock()

	// Double-check after acquiring write lock
	if lf.realValue != nil {
		return lf.realValue, nil
	}

	if err := lf.lazyDoc.fetchRealValues(lf.name, lf.fieldNum); err != nil {
		return nil, err
	}

	if lf.realValue == nil {
		return nil, fmt.Errorf("field value was not lazy loaded")
	}

	return lf.realValue, nil
}

// Name returns the name of the field.
func (lf *LazyField) Name() string {
	return lf.name
}

// StringValue returns the string value of the field.
func (lf *LazyField) StringValue() string {
	real, err := lf.getRealValue()
	if err != nil {
		return ""
	}
	if real == nil {
		return ""
	}
	return real.StringValue()
}

// BinaryValue returns the binary value of the field.
func (lf *LazyField) BinaryValue() []byte {
	real, err := lf.getRealValue()
	if err != nil {
		return nil
	}
	if real == nil {
		return nil
	}
	return real.BinaryValue()
}

// NumericValue returns the numeric value of the field.
func (lf *LazyField) NumericValue() interface{} {
	real, err := lf.getRealValue()
	if err != nil {
		return nil
	}
	if real == nil {
		return nil
	}
	return real.NumericValue()
}

// FieldType returns the field type.
func (lf *LazyField) FieldType() *FieldType {
	real, err := lf.getRealValue()
	if err != nil {
		return nil
	}
	if real == nil {
		return nil
	}
	return real.FieldType()
}

// ReaderValue returns a reader for the field value.
func (lf *LazyField) ReaderValue() io.Reader {
	real, err := lf.getRealValue()
	if err != nil {
		return nil
	}
	if real == nil {
		return nil
	}
	return real.ReaderValue()
}

// documentCollector is a StoredFieldVisitor that collects fields into a Document.
type documentCollector struct {
	doc *Document
}

// Ensure documentCollector implements StoredFieldVisitor
var _ index.StoredFieldVisitor = (*documentCollector)(nil)

// StringField is called for a stored string field.
func (dc *documentCollector) StringField(field string, value string) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// BinaryField is called for a stored binary field.
func (dc *documentCollector) BinaryField(field string, value []byte) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// IntField is called for a stored int field.
func (dc *documentCollector) IntField(field string, value int) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// LongField is called for a stored long field.
func (dc *documentCollector) LongField(field string, value int64) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// FloatField is called for a stored float field.
func (dc *documentCollector) FloatField(field string, value float32) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// DoubleField is called for a stored double field.
func (dc *documentCollector) DoubleField(field string, value float64) {
	if dc.doc == nil {
		dc.doc = NewDocument()
	}
	ft := NewFieldType()
	ft.SetStored(true)
	f, _ := NewField(field, value, ft)
	dc.doc.Add(f)
}

// Ensure LazyField implements IndexableField
var _ IndexableField = (*LazyField)(nil)
