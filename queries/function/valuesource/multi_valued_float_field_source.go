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

// MultiValuedFloatFieldSource obtains float field values from
// SortedNumericDocValues, selecting a single value per document via the
// configured selector.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiValuedFloatFieldSource.
type MultiValuedFloatFieldSource struct {
	function.BaseValueSource
	field    string
	selector search.SortedNumericSelectorType
}

// NewMultiValuedFloatFieldSource creates a MultiValuedFloatFieldSource.
func NewMultiValuedFloatFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedFloatFieldSource {
	return &MultiValuedFloatFieldSource{field: field, selector: selector}
}

// Description returns "float(<field>,<selector>)".
func (s *MultiValuedFloatFieldSource) Description() string {
	return fmt.Sprintf("float(%s,%s)", s.field, s.selector)
}

// GetField returns the field name.
func (s *MultiValuedFloatFieldSource) GetField() string { return s.field }

// GetValues returns FunctionValues backed by SortedNumericDocValues.
func (s *MultiValuedFloatFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	sndv, err := getSortedNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if sndv == nil {
		return &multiFloatMissingValues{description: s.Description()}, nil
	}

	wrapped := wrapSortedNumericDocValues(sndv, s.selector, search.SortFieldTypeFloat)
	v := &multiValuedFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			if docFieldExists(wrapped, doc) {
				raw, err := wrapped.LongValue()
				if err != nil {
					return 0, err
				}
				return floatBitsToFloat(raw), nil
			}
			return 0, nil
		}),
		arr: wrapped,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *MultiValuedFloatFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedFloatFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field && s.selector == o.selector
}

// HashCode returns a stable hash.
func (s *MultiValuedFloatFieldSource) HashCode() int32 {
	return hashString("mfloat") + hashString(s.field) + int32(s.selector)
}

type multiValuedFloatFunctionValues struct {
	docvalues.FloatDocValues
	arr index.NumericDocValues
}

func (v *multiValuedFloatFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

type multiFloatMissingValues struct {
	missingValuesBase
	description string
}

func (v *multiFloatMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *multiFloatMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*MultiValuedFloatFieldSource)(nil)
