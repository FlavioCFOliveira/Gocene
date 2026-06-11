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

// MultiValuedLongFieldSource obtains long field values from
// SortedNumericDocValues, selecting a single value per document.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiValuedLongFieldSource.
type MultiValuedLongFieldSource struct {
	function.BaseValueSource
	field    string
	selector search.SortedNumericSelectorType
}

// NewMultiValuedLongFieldSource creates a MultiValuedLongFieldSource.
func NewMultiValuedLongFieldSource(field string, selector search.SortedNumericSelectorType) *MultiValuedLongFieldSource {
	return &MultiValuedLongFieldSource{field: field, selector: selector}
}

// Description returns "long(<field>,<selector>)".
func (s *MultiValuedLongFieldSource) Description() string {
	return fmt.Sprintf("long(%s,%s)", s.field, s.selector)
}

// GetField returns the field name.
func (s *MultiValuedLongFieldSource) GetField() string { return s.field }

// GetValues returns FunctionValues backed by SortedNumericDocValues.
func (s *MultiValuedLongFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	sndv, err := getSortedNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if sndv == nil {
		return &multiLongMissingValues{description: s.Description()}, nil
	}

	wrapped := wrapSortedNumericDocValues(sndv, s.selector, search.SortFieldTypeLong)
	v := &multiLongFunctionValues{
		LongDocValues: *docvalues.NewLongDocValues(s, func(doc int) (int64, error) {
			if docFieldExists(wrapped, doc) {
				return wrapped.LongValue()
			}
			return 0, nil
		}),
		arr: wrapped,
	}
	return v, nil
}

// Equals reports value equality.
func (s *MultiValuedLongFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiValuedLongFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field && s.selector == o.selector
}

// HashCode returns a stable hash.
func (s *MultiValuedLongFieldSource) HashCode() int32 {
	return hashString("mlong") + hashString(s.field) + int32(s.selector)
}

type multiLongFunctionValues struct {
	docvalues.LongDocValues
	arr index.NumericDocValues
}

func (v *multiLongFunctionValues) Exists(doc int) (bool, error) { return docFieldExists(v.arr, doc), nil }

type multiLongMissingValues struct {
	missingValuesBase
	description string
}

func (v *multiLongMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *multiLongMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*MultiValuedLongFieldSource)(nil)
