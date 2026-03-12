// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SortedSetDocValuesField is a field that stores multiple binary values as sorted DocValues.
// Each document can have multiple values, which are stored in a sorted, deduplicated
// columnar structure for efficient lookups and sorting.
//
// This is the Go port of Lucene's org.apache.lucene.document.SortedSetDocValuesField.
type SortedSetDocValuesField struct {
	*Field
	values [][]byte
}

var (
	// SortedSetDocValuesFieldType is the FieldType for a SortedSetDocValuesField.
	// The field is not indexed or stored, but has sorted set DocValues.
	SortedSetDocValuesFieldType *FieldType
)

func init() {
	// Initialize the FieldType
	SortedSetDocValuesFieldType = NewFieldType().
		SetIndexed(false).
		SetStored(false).
		SetDocValuesType(index.DocValuesTypeSortedSet)
	SortedSetDocValuesFieldType.Freeze()
}

// NewSortedSetDocValuesField creates a new SortedSetDocValuesField with the given name and values.
// The field values are not indexed or stored, but are available via the DocValues API
// for efficient sorting and faceting. Values are stored in sorted order and
// deduplicated across documents.
func NewSortedSetDocValuesField(name string, values [][]byte) (*SortedSetDocValuesField, error) {
	// Use the first value for the base field if available
	var baseValue []byte
	if len(values) > 0 {
		baseValue = values[0]
	}

	field, err := NewField(name, baseValue, SortedSetDocValuesFieldType)
	if err != nil {
		return nil, err
	}

	return &SortedSetDocValuesField{
		Field:  field,
		values: values,
	}, nil
}

// GetValues returns all binary values of this field.
func (f *SortedSetDocValuesField) GetValues() [][]byte {
	return f.values
}

// AddValue adds a value to this field's sorted set.
func (f *SortedSetDocValuesField) AddValue(value []byte) {
	f.values = append(f.values, value)
}

// ValueCount returns the number of values in this field.
func (f *SortedSetDocValuesField) ValueCount() int {
	return len(f.values)
}
