// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DoubleField is a field for indexing float64 values.
type DoubleField struct {
	*Field
}

// NewDoubleField creates a new DoubleField.
func NewDoubleField(name string, value float64, store bool) (*DoubleField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatFloat(value, 'f', -1, 64), ft)
	if err != nil {
		return nil, err
	}

	return &DoubleField{Field: field}, nil
}

// DoubleValue returns the float64 value.
func (f *DoubleField) DoubleValue() float64 {
	val, _ := strconv.ParseFloat(f.StringValue(), 64)
	return val
}

// encodeFloat64Legacy encodes a float64 to a sortable byte representation.
func encodeFloat64Legacy(f float64) []byte {
	return PackDouble(f)
}

// decodeFloat64Legacy decodes a byte representation back to float64.
func decodeFloat64Legacy(buf []byte) float64 {
	return UnpackDouble(buf)
}

// DoublePoint is an indexed float64 point field for range queries using the Point API.
type DoublePoint struct {
	Point
}

// NewDoublePoint creates a new DoublePoint with a single value.
func NewDoublePoint(name string, value float64) *DoublePoint {
	return NewDoublePoints(name, value)
}

// NewDoublePoints creates a new DoublePoint with multiple values.
func NewDoublePoints(name string, values ...float64) *DoublePoint {
	if len(values) == 0 {
		return nil
	}

	encoded := PackDoubles(values)
	ft := PointFieldType()
	ft.DimensionNumBytes = 8

	point, _ := NewPoint(name, ft, encoded, 1, 8)
	return &DoublePoint{Point: *point}
}

// DoubleValue returns the first double value.
func (dp *DoublePoint) DoubleValue() float64 {
	values := dp.DoubleValues()
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

// DoubleValues returns all double values.
func (dp *DoublePoint) DoubleValues() []float64 {
	return UnpackDoubles(dp.PointValues())
}
