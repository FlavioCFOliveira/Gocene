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

// MultiValuedDoubleFieldSource obtains double field values from
// SortedNumericDocValues, selecting a single value per document.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiValuedDoubleFieldSource.
type MultiValuedDoubleFieldSource struct {
	function.BaseValueSource
	field    string
	selector search.SortedNumericSelectorType
}

// NewMultiValuedDoubleFieldSource creates a MultiValuedDoubleFieldSource.
func NewMultiValuedDoubleFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedDoubleFieldSource {
	return &MultiValuedDoubleFieldSource{field: field, selector: selector}
}

// Description returns "double(<field>,<selector>)".
func (s *MultiValuedDoubleFieldSource) Description() string {
	return fmt.Sprintf("double(%s,%s)", s.field, s.selector)
}

// GetField returns the field name.
func (s *MultiValuedDoubleFieldSource) GetField() string { return s.field }

// GetValues returns FunctionValues backed by SortedNumericDocValues.
func (s *MultiValuedDoubleFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	sndv, err := getSortedNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if sndv == nil {
		return &multiDoubleMissingValues{description: s.Description()}, nil
	}

	wrapped := wrapSortedNumericDocValues(sndv, s.selector, search.SortFieldTypeDouble)
	v := &multiDoubleFunctionValues{
		DoubleDocValues: *docvalues.NewDoubleDocValues(s, func(doc int) (float64, error) {
			if docFieldExists(wrapped, doc) {
				raw, err := wrapped.LongValue()
				if err != nil {
					return 0, err
				}
				return doubleBitsToDouble(raw), nil
			}
			return 0, nil
		}),
		arr: wrapped,
	}
	return v, nil
}

// Equals reports value equality.
func (s *MultiValuedDoubleFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedDoubleFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field && s.selector == o.selector
}

// HashCode returns a stable hash.
func (s *MultiValuedDoubleFieldSource) HashCode() int32 {
	return hashString("mdouble") + hashString(s.field) + int32(s.selector)
}

type multiDoubleFunctionValues struct {
	docvalues.DoubleDocValues
	arr index.NumericDocValues
}

func (v *multiDoubleFunctionValues) Exists(doc int) (bool, error) { return docFieldExists(v.arr, doc), nil }

type multiDoubleMissingValues struct {
	missingValuesBase
	description string
}

func (v *multiDoubleMissingValues) ToString(doc int) (string, error) { return v.description + "=0.0", nil }
func (v *multiDoubleMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*MultiValuedDoubleFieldSource)(nil)
