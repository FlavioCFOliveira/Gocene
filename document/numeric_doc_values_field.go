// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// NumericDocValuesField is a field that stores a single int64 value as DocValues.
// This field is not indexed or stored, but the value can be used for sorting
// and faceting via the DocValues API.
//
// This is the Go port of Lucene's org.apache.lucene.document.NumericDocValuesField.
type NumericDocValuesField struct {
	*Field
}

var (
	// NumericDocValuesFieldType is the FieldType for a NumericDocValuesField.
	// The field is not indexed or stored, but has numeric DocValues.
	NumericDocValuesFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	NumericDocValuesFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeNumeric)
	NumericDocValuesFieldType.Freeze()
}

// NewNumericDocValuesField creates a new NumericDocValuesField with the given name and value.
// The field value is not indexed or stored, but is available via the DocValues API
// for efficient sorting and faceting.
func NewNumericDocValuesField(name string, value int64) (*NumericDocValuesField, error) {
	field, err := NewField(name, value, NumericDocValuesFieldType)
	if err != nil {
		return nil, err
	}

	return &NumericDocValuesField{Field: field}, nil
}

// GetValue returns the int64 value of this field.
func (f *NumericDocValuesField) GetValue() int64 {
	if v := f.NumericValue(); v != nil {
		if iv, ok := v.(int64); ok {
			return iv
		}
	}
	return 0
}
