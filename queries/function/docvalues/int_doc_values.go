// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"errors"
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// ErrInvalidIntBound is returned when a textual bound supplied to
// [IntDocValues.GetRangeScorer] cannot be parsed as int32.
var ErrInvalidIntBound = errors.New("docvalues: invalid int range bound")

// IntDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.IntDocValues.
type IntDocValues struct {
	function.BaseFunctionValues
	VS      function.ValueSource
	IntFunc func(doc int) (int32, error)
}

// NewIntDocValues wires vs + intFunc into a ready-to-use IntDocValues.
func NewIntDocValues(vs function.ValueSource, intFunc func(doc int) (int32, error)) *IntDocValues {
	v := &IntDocValues{VS: vs, IntFunc: intFunc}
	v.SetSelf(v)
	return v
}

// IntVal delegates to the embedded function.
func (i *IntDocValues) IntVal(doc int) (int32, error) { return i.IntFunc(doc) }

// ByteVal returns the truncated int8 of IntVal.
func (i *IntDocValues) ByteVal(doc int) (int8, error) {
	v, err := i.IntFunc(doc)
	return int8(v), err
}

// ShortVal returns the truncated int16 of IntVal.
func (i *IntDocValues) ShortVal(doc int) (int16, error) {
	v, err := i.IntFunc(doc)
	return int16(v), err
}

// FloatVal widens IntVal to float32.
func (i *IntDocValues) FloatVal(doc int) (float32, error) {
	v, err := i.IntFunc(doc)
	return float32(v), err
}

// LongVal widens IntVal to int64.
func (i *IntDocValues) LongVal(doc int) (int64, error) {
	v, err := i.IntFunc(doc)
	return int64(v), err
}

// DoubleVal widens IntVal to float64.
func (i *IntDocValues) DoubleVal(doc int) (float64, error) {
	v, err := i.IntFunc(doc)
	return float64(v), err
}

// StrVal renders IntVal as a base-10 string.
func (i *IntDocValues) StrVal(doc int) (string, error) {
	v, err := i.IntFunc(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(v), 10), nil
}

// ObjectVal returns the int32 when the doc has a value.
func (i *IntDocValues) ObjectVal(doc int) (any, error) {
	ok, err := i.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return i.IntFunc(doc)
}

// ToString renders "<vs.description>=<value>".
func (i *IntDocValues) ToString(doc int) (string, error) {
	s, err := i.StrVal(doc)
	if err != nil {
		return "", err
	}
	return i.VS.Description() + "=" + s, nil
}

// GetRangeScorer adjusts the bounds inclusively in advance, then matches
// against IntVal directly. Mirrors Lucene's "instead of using separate
// comparison functions, adjust the endpoints" trick.
func (i *IntDocValues) GetRangeScorer(
	readerContext *index.LeafReaderContext,
	lowerVal, upperVal string,
	includeLower, includeUpper bool,
) (function.ValueSourceScorer, error) {
	lo, err := parseIntBoundLower(lowerVal, includeLower)
	if err != nil {
		return nil, err
	}
	hi, err := parseIntBoundUpper(upperVal, includeUpper)
	if err != nil {
		return nil, err
	}
	return &intRangeValueSourceScorer{readerContext: readerContext, values: i, lo: lo, hi: hi}, nil
}

func parseIntBoundLower(bound string, includeLower bool) (int32, error) {
	if bound == "" {
		return math.MinInt32, nil
	}
	v, err := strconv.ParseInt(bound, 10, 32)
	if err != nil {
		return 0, ErrInvalidIntBound
	}
	r := int32(v)
	if !includeLower && r < math.MaxInt32 {
		r++
	}
	return r, nil
}

func parseIntBoundUpper(bound string, includeUpper bool) (int32, error) {
	if bound == "" {
		return math.MaxInt32, nil
	}
	v, err := strconv.ParseInt(bound, 10, 32)
	if err != nil {
		return 0, ErrInvalidIntBound
	}
	r := int32(v)
	if !includeUpper && r > math.MinInt32 {
		r--
	}
	return r, nil
}

type intRangeValueSourceScorer struct {
	readerContext *index.LeafReaderContext
	values        *IntDocValues
	lo, hi        int32
}

func (s *intRangeValueSourceScorer) Values() function.FunctionValues       { return s.values }
func (s *intRangeValueSourceScorer) LeafContext() *index.LeafReaderContext { return s.readerContext }
func (s *intRangeValueSourceScorer) MaxDoc() int {
	if s.readerContext == nil {
		return 0
	}
	leaf := s.readerContext.LeafReader()
	if leaf == nil {
		return 0
	}
	return leaf.MaxDoc()
}
func (s *intRangeValueSourceScorer) MatchCost() float32 {
	return function.DefaultMatchCost + s.values.Cost()
}
func (s *intRangeValueSourceScorer) Score(doc int) (float32, error) {
	return s.values.FloatVal(doc)
}
func (s *intRangeValueSourceScorer) MaxScore(_ int) float32 { return float32(math.Inf(1)) }

func (s *intRangeValueSourceScorer) Matches(doc int) (bool, error) {
	exists, err := s.values.Exists(doc)
	if err != nil || !exists {
		return false, err
	}
	v, err := s.values.IntFunc(doc)
	if err != nil {
		return false, err
	}
	return v >= s.lo && v <= s.hi, nil
}
