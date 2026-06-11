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

// FloatFieldSource obtains float field values from NumericDocValues and
// makes them available as other numeric types, casting as needed.
//
// Go port of org.apache.lucene.queries.function.valuesource.FloatFieldSource.
type FloatFieldSource struct {
	function.BaseValueSource
	field string
}

// NewFloatFieldSource creates a FloatFieldSource for the given field.
func NewFloatFieldSource(field string) *FloatFieldSource {
	return &FloatFieldSource{field: field}
}

// Description returns "float(<field>)".
func (s *FloatFieldSource) Description() string { return fmt.Sprintf("float(%s)", s.field) }

// GetField returns the field name.
func (s *FloatFieldSource) GetField() string { return s.field }

// GetSortField returns a SortField suitable for sorting by this source's values.
func (s *FloatFieldSource) GetSortField(reverse bool) *search.SortField {
	return search.NewSortField(s.field, search.SortFieldTypeFloat)
}

// GetValues returns FunctionValues backed by NumericDocValues for this field.
func (s *FloatFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	arr, err := getNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &floatFieldMissingValues{description: s.Description()}, nil
	}

	v := &floatFieldFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			if docFieldExists(arr, doc) {
				raw, err := arr.LongValue()
				if err != nil {
					return 0, err
				}
				return floatBitsToFloat(raw), nil
			}
			return 0, nil
		}),
		arr: arr,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *FloatFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*FloatFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}

// HashCode returns a stable hash.
func (s *FloatFieldSource) HashCode() int32 {
	return hashFloat32(0) + hashString(s.field)
}

type floatFieldFunctionValues struct {
	docvalues.FloatDocValues
	arr index.NumericDocValues
}

func (v *floatFieldFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

type floatFieldMissingValues struct {
	missingValuesBase
	description string
}

func (v *floatFieldMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *floatFieldMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*FloatFieldSource)(nil)
