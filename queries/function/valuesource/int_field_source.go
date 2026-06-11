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

// IntFieldSource obtains int field values from NumericDocValues and makes
// them available as other numeric types, casting as needed.
//
// Go port of org.apache.lucene.queries.function.valuesource.IntFieldSource.
type IntFieldSource struct {
	function.BaseValueSource
	field string
}

// NewIntFieldSource creates an IntFieldSource for the given field.
func NewIntFieldSource(field string) *IntFieldSource {
	return &IntFieldSource{field: field}
}

// Description returns "int(<field>)".
func (s *IntFieldSource) Description() string { return fmt.Sprintf("int(%s)", s.field) }

// GetField returns the field name.
func (s *IntFieldSource) GetField() string { return s.field }

// GetSortField returns a SortField suitable for sorting by this source's values.
func (s *IntFieldSource) GetSortField(reverse bool) *search.SortField {
	return search.NewSortField(s.field, search.SortFieldTypeInt)
}

// GetValues returns FunctionValues backed by NumericDocValues for this field.
func (s *IntFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	arr, err := getNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &intFieldMissingValues{description: s.Description()}, nil
	}

	v := &intFieldFunctionValues{
		IntDocValues: *docvalues.NewIntDocValues(s, func(doc int) (int32, error) {
			if docFieldExists(arr, doc) {
				raw, err := arr.LongValue()
				if err != nil {
					return 0, err
				}
				return int32(raw), nil
			}
			return 0, nil
		}),
		arr: arr,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *IntFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*IntFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}

// HashCode returns a stable hash.
func (s *IntFieldSource) HashCode() int32 {
	return hashString("int") + hashString(s.field)
}

type intFieldFunctionValues struct {
	docvalues.IntDocValues
	arr index.NumericDocValues
}

func (v *intFieldFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

type intFieldMissingValues struct {
	missingValuesBase
	description string
}

func (v *intFieldMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *intFieldMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*IntFieldSource)(nil)
