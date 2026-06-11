// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongFieldSource obtains long field values from NumericDocValues and makes
// them available as other numeric types, casting as needed.
//
// Go port of org.apache.lucene.queries.function.valuesource.LongFieldSource.
type LongFieldSource struct {
	function.BaseValueSource
	field string
	// ExternalToLongFunc lets subclasses (e.g. DateFieldSource) translate
	// textual representations to int64s. Defaults to strconv.ParseInt(s, 10, 64).
	ExternalToLongFunc func(extVal string) (int64, error)
}

// NewLongFieldSource creates a LongFieldSource for the given field.
func NewLongFieldSource(field string) *LongFieldSource {
	return &LongFieldSource{
		field:              field,
		ExternalToLongFunc: defaultExternalToLong,
	}
}

func defaultExternalToLong(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// Description returns "long(<field>)".
func (s *LongFieldSource) Description() string { return fmt.Sprintf("long(%s)", s.field) }

// GetField returns the field name.
func (s *LongFieldSource) GetField() string { return s.field }

// GetSortField returns a SortField suitable for sorting by this source's values.
func (s *LongFieldSource) GetSortField(reverse bool) *search.SortField {
	return search.NewSortField(s.field, search.SortFieldTypeLong)
}

// ExternalToLong converts an external string representation to int64.
func (s *LongFieldSource) ExternalToLong(extVal string) (int64, error) {
	if s.ExternalToLongFunc != nil {
		return s.ExternalToLongFunc(extVal)
	}
	return defaultExternalToLong(extVal)
}

// LongToObject converts a long value to its object representation.
// Subclasses may override this to return dates, etc.
func (s *LongFieldSource) LongToObject(val int64) any {
	return val
}

// LongToString converts a long value to its string representation.
func (s *LongFieldSource) LongToString(val int64) string {
	return strconv.FormatInt(val, 10)
}

// GetValues returns FunctionValues backed by NumericDocValues for this field.
func (s *LongFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	arr, err := getNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &longFieldMissingValues{description: s.Description()}, nil
	}

	v := &longFieldFunctionValues{
		LongDocValues: *docvalues.NewLongDocValues(s, func(doc int) (int64, error) {
			if docFieldExists(arr, doc) {
				return arr.LongValue()
			}
			return 0, nil
		}),
		arr: arr,
		vs:  s,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *LongFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*LongFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}

// HashCode returns a stable hash.
func (s *LongFieldSource) HashCode() int32 {
	h := hashString("long")
	h += hashString(s.field)
	return h
}

type longFieldFunctionValues struct {
	docvalues.LongDocValues
	arr index.NumericDocValues
	vs  *LongFieldSource
}

func (v *longFieldFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

func (v *longFieldFunctionValues) ObjectVal(doc int) (any, error) {
	if docFieldExists(v.arr, doc) {
		val, err := v.LongFunc(doc)
		if err != nil {
			return nil, err
		}
		return v.vs.LongToObject(val), nil
	}
	return nil, nil
}

func (v *longFieldFunctionValues) StrVal(doc int) (string, error) {
	if docFieldExists(v.arr, doc) {
		val, err := v.LongFunc(doc)
		if err != nil {
			return "", err
		}
		return v.vs.LongToString(val), nil
	}
	return "", nil
}

type longFieldMissingValues struct {
	missingValuesBase
	description string
}

func (v *longFieldMissingValues) ToString(doc int) (string, error) { return v.description + "=0", nil }
func (v *longFieldMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*LongFieldSource)(nil)
