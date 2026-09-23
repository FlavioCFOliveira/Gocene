// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
package document

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"strconv"
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
	ft.SetIndexOptions(spi.IndexOptionsDocs)
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
	// Encode with Lucene's two-stage sortable encoding
	// (NumericUtils.floatToSortableInt then intToSortableBytes), the on-disk
	// BKD point format Apache Lucene 10.4.0 produces and consumes. A raw IEEE
	// big-endian layout would mis-order negative floats and break binary
	// compatibility with Lucene's FloatPoint.
	encoded := PackFloatsLucene(values...)
	ft := PointFieldType()
	ft.SetDimensions(1, 4)
	point, _ := NewPoint(name, ft, encoded, len(values), 4)
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

// FloatValues returns all float values, decoding the Lucene sortable
// encoding used by NewFloatPoints.
func (fp *FloatPoint) FloatValues() []float32 {
	packed := fp.PointValues()
	if len(packed)%4 != 0 {
		return nil
	}
	out := make([]float32, len(packed)/4)
	for i := range out {
		out[i] = DecodeDimensionFloatLucene(packed, i*4)
	}
	return out
}

// SetFloatValue renders FloatPoint.setFloatValue(float): change the value of
// this field.
func (fp *FloatPoint) SetFloatValue(value float32) {
	fp.SetFloatValues(value)
}

// SetFloatValues renders FloatPoint.setFloatValues(float...): change the
// values of this field.
func (fp *FloatPoint) SetFloatValues(point ...float32) {
	if fp.ft.PointDimensionCount() != len(point) {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot change to (incoming) %d dimensions",
			fp.name, fp.ft.PointDimensionCount(), len(point)))
	}
	fp.value = binaryValue(PackFloatsLucene(point...))
}
