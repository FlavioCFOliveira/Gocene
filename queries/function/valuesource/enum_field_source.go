// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// EnumFieldSource obtains int field values from NumericDocValues and
// makes those values available as other numeric types, casting as needed.
// strVal of the value is not the int value, but its string (displayed) value.
type EnumFieldSource struct {
	FieldCacheSource
	EnumIntToStringMap map[int]string
	EnumStringToIntMap map[string]int
}

var defaultEnumValue = -1

func NewEnumFieldSource(
	field string,
	enumIntToStringMap map[int]string,
	enumStringToIntMap map[string]int,
) *EnumFieldSource {
	return &EnumFieldSource{
		FieldCacheSource:   FieldCacheSource{Field: field},
		EnumIntToStringMap: enumIntToStringMap,
		EnumStringToIntMap: enumStringToIntMap,
	}
}

func (f *EnumFieldSource) Description() string {
	return fmt.Sprintf("enum(%s)", f.Field)
}

func (f *EnumFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ndv, err := readerContext.LeafReader().GetNumericDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	fv := &enumDocValues{
		source: f,
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

type enumDocValues struct {
	function.BaseFunctionValues
	source    *EnumFieldSource
	ndv       index.NumericDocValues
	lastDocID int
}

func (f *enumDocValues) IntVal(doc int) (int32, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return 0, err
	}
	if exists {
		val, err := f.ndv.LongValue()
		if err != nil {
			return 0, err
		}
		return int32(val), nil
	}
	return 0, nil
}

func (f *enumDocValues) StrVal(doc int) (string, error) {
	val, err := f.IntVal(doc)
	if err != nil {
		return "", err
	}
	intVal := int(val)
	if enumStr, ok := f.source.EnumIntToStringMap[intVal]; ok {
		return enumStr, nil
	}
	return fmt.Sprintf("%d", defaultEnumValue), nil
}

func (f *enumDocValues) Exists(doc int) (bool, error) {
	if doc < f.lastDocID {
		return false, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", f.lastDocID, doc)
	}
	f.lastDocID = doc
	curDocID := f.ndv.DocID()
	if doc > curDocID {
		next, err := f.ndv.Advance(doc)
		if err != nil {
			return false, err
		}
		curDocID = next
	}
	return doc == curDocID, nil
}

func (f *enumDocValues) ToString(doc int) (string, error) {
	s, err := f.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("enum(%s)=%s", f.source.Field, s), nil
}

func (f *enumDocValues) GetRangeScorer(
	readerContext *index.LeafReaderContext,
	lowerVal, upperVal string,
	includeLower, includeUpper bool,
) (function.ValueSourceScorer, error) {
	lower := f.stringValueToIntValue(lowerVal)
	upper := f.stringValueToIntValue(upperVal)

	// Instead of using separate comparison functions, adjust the range
	// endpoints. Java boxes the bounds in Integer so that "absent" is null;
	// the Go port uses *int for the same purpose and unboxes here.
	var ll int
	if lower == nil {
		ll = math.MinInt32 // Integer.MIN_VALUE
	} else {
		ll = *lower
		if !includeLower && ll < math.MaxInt32 {
			ll++
		}
	}

	var uu int
	if upper == nil {
		uu = math.MaxInt32 // Integer.MAX_VALUE
	} else {
		uu = *upper
		if !includeUpper && uu > math.MinInt32 {
			uu--
		}
	}

	return &enumRangeScorer{
		readerContext: readerContext,
		source:        f,
		lower:         ll,
		upper:         uu,
	}, nil
}

func (f *enumDocValues) stringValueToIntValue(stringVal string) *int {
	if stringVal == "" {
		return nil
	}
	enumInt, ok := f.source.EnumStringToIntMap[stringVal]
	if ok {
		return &enumInt
	}
	intValue := tryParseInt(stringVal)
	if intValue == nil {
		intValue = &defaultEnumValue
	}
	enumString := f.source.EnumIntToStringMap[*intValue]
	if enumString != "" {
		return intValue
	}
	res := defaultEnumValue
	return &res
}

func tryParseInt(valueStr string) *int {
	var v int
	if _, err := fmt.Sscanf(valueStr, "%d", &v); err != nil {
		return nil
	}
	return &v
}

type enumRangeScorer struct {
	function.ValueSourceScorer
	readerContext *index.LeafReaderContext
	source        *enumDocValues
	lower         int
	upper         int
}

// Matches reports whether doc's enum value falls inside the adjusted range.
//
// Mirrors the anonymous ValueSourceScorer.matches(int) override in
// EnumFieldSource.getRangeScorer; the IOException Java propagates becomes the
// returned error.
func (s *enumRangeScorer) Matches(doc int) (bool, error) {
	exists, err := s.source.Exists(doc)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	val, err := s.source.IntVal(doc)
	if err != nil {
		return false, err
	}
	return int(val) >= s.lower && int(val) <= s.upper, nil
}
