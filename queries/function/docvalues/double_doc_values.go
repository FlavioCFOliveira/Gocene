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

// ErrInvalidDoubleBound is returned when a textual bound supplied to
// [DoubleDocValues.GetRangeScorer] is not a valid float64.
var ErrInvalidDoubleBound = errors.New("docvalues: invalid double range bound")

// DoubleDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.DoubleDocValues. Concrete
// embedders supply DoubleFunc; every other typed accessor coerces from
// the float64 result.
type DoubleDocValues struct {
	function.BaseFunctionValues
	VS         function.ValueSource
	DoubleFunc func(doc int) (float64, error)
}

// NewDoubleDocValues wires vs + doubleFunc into a ready-to-use DoubleDocValues.
func NewDoubleDocValues(vs function.ValueSource, doubleFunc func(doc int) (float64, error)) *DoubleDocValues {
	v := &DoubleDocValues{VS: vs, DoubleFunc: doubleFunc}
	v.SetSelf(v)
	return v
}

// DoubleVal delegates to the embedded function.
func (d *DoubleDocValues) DoubleVal(doc int) (float64, error) { return d.DoubleFunc(doc) }

// ByteVal returns the truncated int8 of DoubleVal.
func (d *DoubleDocValues) ByteVal(doc int) (int8, error) {
	v, err := d.DoubleFunc(doc)
	return int8(v), err
}

// ShortVal returns the truncated int16 of DoubleVal.
func (d *DoubleDocValues) ShortVal(doc int) (int16, error) {
	v, err := d.DoubleFunc(doc)
	return int16(v), err
}

// FloatVal returns the float32 narrowing of DoubleVal.
func (d *DoubleDocValues) FloatVal(doc int) (float32, error) {
	v, err := d.DoubleFunc(doc)
	return float32(v), err
}

// IntVal returns the truncated int32 of DoubleVal.
func (d *DoubleDocValues) IntVal(doc int) (int32, error) {
	v, err := d.DoubleFunc(doc)
	return int32(v), err
}

// LongVal returns the truncated int64 of DoubleVal.
func (d *DoubleDocValues) LongVal(doc int) (int64, error) {
	v, err := d.DoubleFunc(doc)
	return int64(v), err
}

// BoolVal reports DoubleVal != 0.
func (d *DoubleDocValues) BoolVal(doc int) (bool, error) {
	v, err := d.DoubleFunc(doc)
	return v != 0, err
}

// StrVal renders DoubleVal as a base-10 string.
func (d *DoubleDocValues) StrVal(doc int) (string, error) {
	v, err := d.DoubleFunc(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(v, 'g', -1, 64), nil
}

// ObjectVal returns the float64 when the doc has a value.
func (d *DoubleDocValues) ObjectVal(doc int) (any, error) {
	ok, err := d.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return d.DoubleFunc(doc)
}

// ToString renders "<vs.description>=<value>".
func (d *DoubleDocValues) ToString(doc int) (string, error) {
	s, err := d.StrVal(doc)
	if err != nil {
		return "", err
	}
	return d.VS.Description() + "=" + s, nil
}

// GetRangeScorer overrides the FloatVal-based scorer in BaseFunctionValues
// with one that compares against DoubleVal directly.
func (d *DoubleDocValues) GetRangeScorer(
	readerContext *index.LeafReaderContext,
	lowerVal, upperVal string,
	includeLower, includeUpper bool,
) (function.ValueSourceScorer, error) {
	lo, err := parseDoubleBound(lowerVal, true)
	if err != nil {
		return nil, err
	}
	hi, err := parseDoubleBound(upperVal, false)
	if err != nil {
		return nil, err
	}
	return &doubleRangeValueSourceScorer{
		readerContext: readerContext,
		values:        d,
		lo:            lo,
		hi:            hi,
		incLo:         includeLower,
		incHi:         includeUpper,
	}, nil
}

// parseDoubleBound parses a textual double, treating "" as ±Inf.
func parseDoubleBound(bound string, isLower bool) (float64, error) {
	if bound == "" {
		if isLower {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil
	}
	v, err := strconv.ParseFloat(bound, 64)
	if err != nil {
		return 0, ErrInvalidDoubleBound
	}
	return v, nil
}

type doubleRangeValueSourceScorer struct {
	readerContext *index.LeafReaderContext
	values        *DoubleDocValues
	lo, hi        float64
	incLo, incHi  bool
}

func (s *doubleRangeValueSourceScorer) Values() function.FunctionValues       { return s.values }
func (s *doubleRangeValueSourceScorer) LeafContext() *index.LeafReaderContext { return s.readerContext }
func (s *doubleRangeValueSourceScorer) MaxDoc() int {
	if s.readerContext == nil {
		return 0
	}
	leaf := s.readerContext.LeafReader()
	if leaf == nil {
		return 0
	}
	return leaf.MaxDoc()
}
func (s *doubleRangeValueSourceScorer) MatchCost() float32 {
	return function.DefaultMatchCost + s.values.Cost()
}
func (s *doubleRangeValueSourceScorer) Score(doc int) (float32, error) {
	return s.values.FloatVal(doc)
}
func (s *doubleRangeValueSourceScorer) MaxScore(_ int) float32 { return float32(math.Inf(1)) }

func (s *doubleRangeValueSourceScorer) Matches(doc int) (bool, error) {
	exists, err := s.values.Exists(doc)
	if err != nil || !exists {
		return false, err
	}
	v, err := s.values.DoubleFunc(doc)
	if err != nil {
		return false, err
	}
	switch {
	case s.incLo && s.incHi:
		return v >= s.lo && v <= s.hi, nil
	case s.incLo && !s.incHi:
		return v >= s.lo && v < s.hi, nil
	case !s.incLo && s.incHi:
		return v > s.lo && v <= s.hi, nil
	default:
		return v > s.lo && v < s.hi, nil
	}
}
