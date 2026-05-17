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

// ErrInvalidLongBound is returned when a textual bound supplied to
// [LongDocValues.GetRangeScorer] cannot be parsed as int64 (via
// LongDocValues.ExternalToLong by default, but custom parsers can be set
// in LongDocValues.ExternalToLongFunc).
var ErrInvalidLongBound = errors.New("docvalues: invalid long range bound")

// LongDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.LongDocValues.
type LongDocValues struct {
	function.BaseFunctionValues
	VS       function.ValueSource
	LongFunc func(doc int) (int64, error)
	// ExternalToLongFunc lets subclasses (e.g. DateFieldSource) translate
	// textual bounds like "2026-05-17" into int64s. Defaults to
	// strconv.ParseInt(s, 10, 64).
	ExternalToLongFunc func(extVal string) (int64, error)
}

// NewLongDocValues wires vs + longFunc into a ready-to-use LongDocValues.
func NewLongDocValues(vs function.ValueSource, longFunc func(doc int) (int64, error)) *LongDocValues {
	v := &LongDocValues{VS: vs, LongFunc: longFunc, ExternalToLongFunc: defaultExternalToLong}
	v.SetSelf(v)
	return v
}

func defaultExternalToLong(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, ErrInvalidLongBound
	}
	return v, nil
}

// LongVal delegates to the embedded function.
func (l *LongDocValues) LongVal(doc int) (int64, error) { return l.LongFunc(doc) }

// ByteVal returns the truncated int8 of LongVal.
func (l *LongDocValues) ByteVal(doc int) (int8, error) {
	v, err := l.LongFunc(doc)
	return int8(v), err
}

// ShortVal returns the truncated int16 of LongVal.
func (l *LongDocValues) ShortVal(doc int) (int16, error) {
	v, err := l.LongFunc(doc)
	return int16(v), err
}

// FloatVal widens LongVal to float32.
func (l *LongDocValues) FloatVal(doc int) (float32, error) {
	v, err := l.LongFunc(doc)
	return float32(v), err
}

// IntVal narrows LongVal to int32 (Java cast semantics).
func (l *LongDocValues) IntVal(doc int) (int32, error) {
	v, err := l.LongFunc(doc)
	return int32(v), err
}

// DoubleVal widens LongVal to float64.
func (l *LongDocValues) DoubleVal(doc int) (float64, error) {
	v, err := l.LongFunc(doc)
	return float64(v), err
}

// BoolVal reports LongVal != 0.
func (l *LongDocValues) BoolVal(doc int) (bool, error) {
	v, err := l.LongFunc(doc)
	return v != 0, err
}

// StrVal renders LongVal as a base-10 string.
func (l *LongDocValues) StrVal(doc int) (string, error) {
	v, err := l.LongFunc(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(v, 10), nil
}

// ObjectVal returns the int64 when the doc has a value.
func (l *LongDocValues) ObjectVal(doc int) (any, error) {
	ok, err := l.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return l.LongFunc(doc)
}

// ToString renders "<vs.description>=<value>".
func (l *LongDocValues) ToString(doc int) (string, error) {
	s, err := l.StrVal(doc)
	if err != nil {
		return "", err
	}
	return l.VS.Description() + "=" + s, nil
}

// GetRangeScorer adjusts the bounds inclusively in advance, then matches
// against LongVal directly.
func (l *LongDocValues) GetRangeScorer(
	readerContext *index.LeafReaderContext,
	lowerVal, upperVal string,
	includeLower, includeUpper bool,
) (function.ValueSourceScorer, error) {
	convert := l.ExternalToLongFunc
	if convert == nil {
		convert = defaultExternalToLong
	}
	lo, err := parseLongBoundLower(lowerVal, includeLower, convert)
	if err != nil {
		return nil, err
	}
	hi, err := parseLongBoundUpper(upperVal, includeUpper, convert)
	if err != nil {
		return nil, err
	}
	return &longRangeValueSourceScorer{readerContext: readerContext, values: l, lo: lo, hi: hi}, nil
}

func parseLongBoundLower(bound string, includeLower bool, convert func(string) (int64, error)) (int64, error) {
	if bound == "" {
		return math.MinInt64, nil
	}
	v, err := convert(bound)
	if err != nil {
		return 0, err
	}
	if !includeLower && v < math.MaxInt64 {
		v++
	}
	return v, nil
}

func parseLongBoundUpper(bound string, includeUpper bool, convert func(string) (int64, error)) (int64, error) {
	if bound == "" {
		return math.MaxInt64, nil
	}
	v, err := convert(bound)
	if err != nil {
		return 0, err
	}
	if !includeUpper && v > math.MinInt64 {
		v--
	}
	return v, nil
}

type longRangeValueSourceScorer struct {
	readerContext *index.LeafReaderContext
	values        *LongDocValues
	lo, hi        int64
}

func (s *longRangeValueSourceScorer) Values() function.FunctionValues       { return s.values }
func (s *longRangeValueSourceScorer) LeafContext() *index.LeafReaderContext { return s.readerContext }
func (s *longRangeValueSourceScorer) MaxDoc() int {
	if s.readerContext == nil {
		return 0
	}
	leaf := s.readerContext.LeafReader()
	if leaf == nil {
		return 0
	}
	return leaf.MaxDoc()
}
func (s *longRangeValueSourceScorer) MatchCost() float32 {
	return function.DefaultMatchCost + s.values.Cost()
}
func (s *longRangeValueSourceScorer) Score(doc int) (float32, error) {
	return s.values.FloatVal(doc)
}
func (s *longRangeValueSourceScorer) MaxScore(_ int) float32 { return float32(math.Inf(1)) }

func (s *longRangeValueSourceScorer) Matches(doc int) (bool, error) {
	exists, err := s.values.Exists(doc)
	if err != nil || !exists {
		return false, err
	}
	v, err := s.values.LongFunc(doc)
	if err != nil {
		return false, err
	}
	return v >= s.lo && v <= s.hi, nil
}
