// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DoubleFieldSource obtains double field values from NumericDocValues and
// makes them available as other numeric types, casting as needed.
//
// Go port of org.apache.lucene.queries.function.valuesource.DoubleFieldSource.
type DoubleFieldSource struct {
	function.BaseValueSource
	field string
}

// NewDoubleFieldSource creates a DoubleFieldSource for the given field.
func NewDoubleFieldSource(field string) *DoubleFieldSource {
	return &DoubleFieldSource{field: field}
}

// Description returns "double(<field>)".
func (s *DoubleFieldSource) Description() string { return fmt.Sprintf("double(%s)", s.field) }

// GetField returns the field name.
func (s *DoubleFieldSource) GetField() string { return s.field }

// GetSortField returns a SortField suitable for sorting by this source's values.
func (s *DoubleFieldSource) GetSortField(reverse bool) *search.SortField {
	return search.NewSortField(s.field, search.SortFieldTypeDouble)
}

// GetValues returns FunctionValues backed by NumericDocValues for this field.
func (s *DoubleFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	arr, err := getNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &doubleFieldMissingValues{description: s.Description()}, nil
	}

	v := &doubleFieldFunctionValues{
		DoubleDocValues: *docvalues.NewDoubleDocValues(s, func(doc int) (float64, error) {
			if docFieldExists(arr, doc) {
				raw, err := arr.LongValue()
				if err != nil {
					return 0, err
				}
				return doubleBitsToDouble(raw), nil
			}
			return 0, nil
		}),
		arr: arr,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *DoubleFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*DoubleFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}

// HashCode returns a stable hash.
func (s *DoubleFieldSource) HashCode() int32 {
	return hashFloat64(0) + hashString(s.field)
}

type doubleFieldFunctionValues struct {
	docvalues.DoubleDocValues
	arr index.NumericDocValues
}

func (v *doubleFieldFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

type doubleFieldMissingValues struct {
	missingValuesBase
	description string
}

func (v *doubleFieldMissingValues) ToString(doc int) (string, error) { return v.description + "=0.0", nil }
func (v *doubleFieldMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*DoubleFieldSource)(nil)
