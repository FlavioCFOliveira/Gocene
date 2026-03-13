// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SortedNumericDocValuesField is a field that stores multiple numeric values as sorted DocValues.
// Each document can have multiple values, which are stored in a sorted columnar structure.
//
// This is the Go port of Lucene's org.apache.lucene.document.SortedNumericDocValuesField.
type SortedNumericDocValuesField struct {
	*Field
	values []int64
}

var (
	// SortedNumericDocValuesFieldType is the FieldType for a SortedNumericDocValuesField.
	// The field is not indexed or stored, but has sorted numeric DocValues.
	SortedNumericDocValuesFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	SortedNumericDocValuesFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeSortedNumeric)
	SortedNumericDocValuesFieldType.Freeze()
}

// NewSortedNumericDocValuesField creates a new SortedNumericDocValuesField with the given name and values.
// The field values are not indexed or stored, but are available via the DocValues API
// for efficient sorting and faceting. Values are stored in sorted order.
func NewSortedNumericDocValuesField(name string, values []int64) (*SortedNumericDocValuesField, error) {
	// Use the first value for the base field if available
	var baseValue int64
	if len(values) > 0 {
		baseValue = values[0]
	}

	field, err := NewField(name, baseValue, SortedNumericDocValuesFieldType)
	if err != nil {
		return nil, err
	}

	return &SortedNumericDocValuesField{
		Field:  field,
		values: values,
	}, nil
}

// GetValues returns all numeric values of this field.
func (f *SortedNumericDocValuesField) GetValues() []int64 {
	return f.values
}

// AddValue adds a value to this field's sorted numeric set.
func (f *SortedNumericDocValuesField) AddValue(value int64) {
	f.values = append(f.values, value)
}

// ValueCount returns the number of values in this field.
func (f *SortedNumericDocValuesField) ValueCount() int {
	return len(f.values)
}
