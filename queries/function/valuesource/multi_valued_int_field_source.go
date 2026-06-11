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

// MultiValuedIntFieldSource obtains int field values from
// SortedNumericDocValues, selecting a single value per document.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiValuedIntFieldSource.
type MultiValuedIntFieldSource struct {
	function.BaseValueSource
	field    string
	selector search.SortedNumericSelectorType
}

// NewMultiValuedIntFieldSource creates a MultiValuedIntFieldSource.
func NewMultiValuedIntFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedIntFieldSource {
	return &MultiValuedIntFieldSource{field: field, selector: selector}
}

// Description returns "int(<field>,<selector>)".
func (s *MultiValuedIntFieldSource) Description() string {
	return fmt.Sprintf("int(%s,%s)", s.field, s.selector)
}

// GetField returns the field name.
func (s *MultiValuedIntFieldSource) GetField() string { return s.field }

// GetValues returns FunctionValues backed by SortedNumericDocValues.
func (s *MultiValuedIntFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	sndv, err := getSortedNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if sndv == nil {
		return &multiIntMissingValues{description: s.Description()}, nil
	}

	wrapped := wrapSortedNumericDocValues(sndv, s.selector, search.SortFieldTypeInt)
	v := &multiIntFunctionValues{
		IntDocValues: *docvalues.NewIntDocValues(s, func(doc int) (int32, error) {
			if docFieldExists(wrapped, doc) {
				raw, err := wrapped.LongValue()
				if err != nil {
					return 0, err
				}
				return int32(raw), nil
			}
			return 0, nil
		}),
		arr: wrapped,
	}
	return v, nil
}

// Equals reports value equality.
func (s *MultiValuedIntFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedIntFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field && s.selector == o.selector
}

// HashCode returns a stable hash.
func (s *MultiValuedIntFieldSource) HashCode() int32 {
	return hashString("mint") + hashString(s.field) + int32(s.selector)
}

type multiIntFunctionValues struct {
	docvalues.IntDocValues
	arr index.NumericDocValues
}

func (v *multiIntFunctionValues) Exists(doc int) (bool, error) { return docFieldExists(v.arr, doc), nil }

type multiIntMissingValues struct {
	missingValuesBase
	description string
}

func (v *multiIntMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *multiIntMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*MultiValuedIntFieldSource)(nil)
