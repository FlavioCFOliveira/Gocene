// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FloatField is a field for indexing float32 values.
type FloatField struct {
	*Field
}

// NewFloatField creates a new FloatField.
func NewFloatField(name string, value float32, store bool) (*FloatField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatFloat(float64(value), 'f', -1, 32), ft)
	if err != nil {
		return nil, err
	}

	return &FloatField{Field: field}, nil
}

// FloatValue returns the float32 value.
func (f *FloatField) FloatValue() float32 {
	val, _ := strconv.ParseFloat(f.StringValue(), 32)
	return float32(val)
}

// encodeFloat32Legacy encodes a float32 to a sortable byte representation.
func encodeFloat32Legacy(f float32) []byte {
	return PackFloat(f)
}

// decodeFloat32Legacy decodes a byte representation back to float32.
func decodeFloat32Legacy(buf []byte) float32 {
	return UnpackFloat(buf)
}

// FloatPoint is an indexed float32 point field for range queries using the Point API.
type FloatPoint struct {
	Point
}

// NewFloatPoint creates a new FloatPoint with a single value.
func NewFloatPoint(name string, value float32) *FloatPoint {
	return NewFloatPoints(name, value)
}

// NewFloatPoints creates a new FloatPoint with multiple values.
func NewFloatPoints(name string, values ...float32) *FloatPoint {
	if len(values) == 0 {
		return nil
	}

	encoded := PackFloats(values)
	ft := PointFieldType()
	ft.DimensionNumBytes = 4

	point, _ := NewPoint(name, ft, encoded, 1, 4)
	return &FloatPoint{Point: *point}
}

// FloatValue returns the first float value.
func (fp *FloatPoint) FloatValue() float32 {
	values := fp.FloatValues()
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

// FloatValues returns all float values.
func (fp *FloatPoint) FloatValues() []float32 {
	return UnpackFloats(fp.PointValues())
}
