// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SortedDocValuesField is a field that stores a binary value as sorted DocValues.
// The value is stored in a sorted, deduplicated columnar structure for efficient
// lookups and sorting by value.
//
// This is the Go port of Lucene's org.apache.lucene.document.SortedDocValuesField.
type SortedDocValuesField struct {
	*Field
}

var (
	// SortedDocValuesFieldType is the FieldType for a SortedDocValuesField.
	// The field is not indexed or stored, but has sorted DocValues.
	SortedDocValuesFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	SortedDocValuesFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeSorted)
	SortedDocValuesFieldType.Freeze()
}

// NewSortedDocValuesField creates a new SortedDocValuesField with the given name and value.
// The field value is not indexed or stored, but is available via the DocValues API
// for efficient sorting and faceting. Values are stored in sorted order and
// deduplicated across documents.
func NewSortedDocValuesField(name string, value []byte) (*SortedDocValuesField, error) {
	field, err := NewField(name, value, SortedDocValuesFieldType)
	if err != nil {
		return nil, err
	}

	return &SortedDocValuesField{Field: field}, nil
}

// GetValue returns the binary value of this field.
func (f *SortedDocValuesField) GetValue() []byte {
	return f.BinaryValue()
}
