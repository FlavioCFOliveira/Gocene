// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BinaryDocValuesField is a field that stores a binary value as DocValues.
// This field is not indexed or stored, but the value can be used for sorting
// and faceting via the DocValues API.
//
// This is the Go port of Lucene's org.apache.lucene.document.BinaryDocValuesField.
type BinaryDocValuesField struct {
	*Field
}

var (
	// BinaryDocValuesFieldType is the FieldType for a BinaryDocValuesField.
	// The field is not indexed or stored, but has binary DocValues.
	BinaryDocValuesFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	BinaryDocValuesFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeBinary)
	BinaryDocValuesFieldType.Freeze()
}

// NewBinaryDocValuesField creates a new BinaryDocValuesField with the given name and value.
// The field value is not indexed or stored, but is available via the DocValues API
// for efficient sorting and faceting.
func NewBinaryDocValuesField(name string, value []byte) (*BinaryDocValuesField, error) {
	field, err := NewField(name, value, BinaryDocValuesFieldType)
	if err != nil {
		return nil, err
	}

	return &BinaryDocValuesField{Field: field}, nil
}

// GetValue returns the binary value of this field.
func (f *BinaryDocValuesField) GetValue() []byte {
	return f.BinaryValue()
}
