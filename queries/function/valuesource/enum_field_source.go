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
)

// EnumFieldSource obtains int field values from NumericDocValues. strVal
// is the mapped enum string (displayed) value.
//
// Go port of org.apache.lucene.queries.function.valuesource.EnumFieldSource.
type EnumFieldSource struct {
	function.BaseValueSource
	field              string
	enumIntToStringMap map[int32]string
	enumStringToIntMap map[string]int32
}

// NewEnumFieldSource creates an EnumFieldSource with field name and mapping.
func NewEnumFieldSource(
	field string,
	enumIntToStringMap map[int32]string,
	enumStringToIntMap map[string]int32,
) *EnumFieldSource {
	return &EnumFieldSource{
		field:              field,
		enumIntToStringMap: enumIntToStringMap,
		enumStringToIntMap: enumStringToIntMap,
	}
}

// Description returns "enum(<field>)".
func (s *EnumFieldSource) Description() string { return fmt.Sprintf("enum(%s)", s.field) }

// GetField returns the field name.
func (s *EnumFieldSource) GetField() string { return s.field }

func (s *EnumFieldSource) intValueToStringValue(intVal int32) string {
	if enumString, ok := s.enumIntToStringMap[intVal]; ok {
		return enumString
	}
	return "-1"
}

func (s *EnumFieldSource) stringValueToIntValue(stringVal string) int32 {
	if stringVal == "" {
		return 0
	}
	if enumInt, ok := s.enumStringToIntMap[stringVal]; ok {
		return enumInt
	}
	intValue, err := strconv.ParseInt(stringVal, 10, 32)
	if err != nil {
		return -1
	}
	iv := int32(intValue)
	if _, ok := s.enumIntToStringMap[iv]; ok {
		return iv
	}
	return -1
}

// GetValues returns FunctionValues backed by NumericDocValues with enum mapping.
func (s *EnumFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	arr, err := getNumericDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &enumFieldMissingValues{description: s.Description()}, nil
	}

	v := &enumFieldFunctionValues{
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
		vs:  s,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality including map contents.
func (s *EnumFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*EnumFieldSource)
	if !ok || o == nil {
		return false
	}
	if s.field != o.field {
		return false
	}
	if len(s.enumIntToStringMap) != len(o.enumIntToStringMap) {
		return false
	}
	for k, v := range s.enumIntToStringMap {
		if ov, ok := o.enumIntToStringMap[k]; !ok || ov != v {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash incorporating maps.
func (s *EnumFieldSource) HashCode() int32 {
	h := hashString(s.field)
	for k, v := range s.enumIntToStringMap {
		h ^= k
		h += hashString(v)
	}
	return h
}

type enumFieldFunctionValues struct {
	docvalues.IntDocValues
	arr index.NumericDocValues
	vs  *EnumFieldSource
}

func (v *enumFieldFunctionValues) Exists(doc int) (bool, error) {
	return docFieldExists(v.arr, doc), nil
}

func (v *enumFieldFunctionValues) StrVal(doc int) (string, error) {
	intValue, err := v.IntFunc(doc)
	if err != nil {
		return "", err
	}
	return v.vs.intValueToStringValue(intValue), nil
}

func (v *enumFieldFunctionValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return v.vs.Description() + "=" + s, nil
}

type enumFieldMissingValues struct {
	missingValuesBase
	description string
}

func (v *enumFieldMissingValues) ToString(doc int) (string, error) { return v.description + "=", nil }
func (v *enumFieldMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*EnumFieldSource)(nil)
