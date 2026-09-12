// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DoublePoint is an indexed float64 field for fast range filters.
// If you also need to store the value, you should add a separate StoredField instance.
//
// Finding all documents within an N-dimensional shape or range at search time is efficient.
// Multiple values for the same field in one document is allowed.
//
// This is the Go port of Lucene's org.apache.lucene.document.DoublePoint.
type DoublePoint struct {
	*Field
}

// NextUp returns the least double that compares greater than d consistently with
// math.Float64bits. The only difference with math.Nextafter is that this method
// returns +0d when the argument is -0d.
func doubleNextUp(d float64) float64 {
	if math.Float64bits(d) == 0x8000000000000000 { // -0d
		return 0.0
	}
	return math.Nextafter(d, math.Inf(1))
}

// NextDown returns the greatest double that compares less than d consistently with
// math.Float64bits. The only difference with math.Nextafter is that this method
// returns -0d when the argument is +0d.
func doubleNextDown(d float64) float64 {
	if math.Float64bits(d) == 0 { // +0d
		return math.Float64frombits(0x8000000000000000) // -0d
	}
	return math.Nextafter(d, math.Inf(-1))
}

func getDoublePointType(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetDimensions(numDims, 8) // Double.BYTES = 8
	ft.Freeze()
	return ft
}

// NewDoublePoint creates a new DoublePoint, indexing the provided N-dimensional double point.
func NewDoublePoint(name string, point ...float64) *DoublePoint {
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}

	packed := packDoublePoint(point)
	ft := getDoublePointType(len(point))

	// Use NewField to initialize the base Field struct
	f, _ := NewField(name, packed.Bytes, ft)
	return &DoublePoint{f}
}

// SetDoubleValue sets the value of this field.
func (dp *DoublePoint) SetDoubleValue(value float64) {
	dp.SetDoubleValues(value)
}

// SetDoubleValues changes the values of this field.
func (dp *DoublePoint) SetDoubleValues(point ...float64) {
	if dp.ft.PointDimensionCount() != len(point) {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot change to %d dimensions",
			dp.name, dp.ft.PointDimensionCount(), len(point)))
	}
	dp.value = binaryValue(packDoublePoint(point).Bytes)
}

// SetBytesValue is not supported for DoublePoint.
func (dp *DoublePoint) SetBytesValue(bytes *util.BytesRef) {
	panic("cannot change value type from double to BytesRef")
}

// NumericValue returns the numeric value of the field.
// Panics if the field uses more than one dimension.
func (dp *DoublePoint) NumericValue() interface{} {
	if dp.ft.PointDimensionCount() != 1 {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot convert to a single numeric value",
			dp.name, dp.ft.PointDimensionCount()))
	}

	bytes, ok := dp.value.(binaryValue)
	if !ok {
		return nil
	}

	return decodeDoubleDimension(bytes, 0)
}

// Pack packs a double point into a BytesRef.
func packDoublePoint(point []float64) *util.BytesRef {
	if point == nil {
		panic("point must not be null")
	}
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}

	packed := make([]byte, len(point)*8)
	for dim := 0; dim < len(point); dim++ {
		encodeDoubleDimension(point[dim], packed, dim*8)
	}

	return util.NewBytesRef(packed)
}

// encodeDoubleDimension encodes a single double dimension.
func encodeDoubleDimension(value float64, dest []byte, offset int) {
	sortableLong := util.DoubleToSortableLong(value)
	util.LongToSortableBytes(sortableLong, dest, offset)
}

// decodeDoubleDimension decodes a single double dimension.
func decodeDoubleDimension(value []byte, offset int) float64 {
	sortableLong := util.SortableBytesToLong(value, offset)
	return util.SortableLongToDouble(sortableLong)
}

func (dp *DoublePoint) String() string {
	var sb strings.Builder
	sb.WriteString("DoublePoint <")
	sb.WriteString(dp.name)
	sb.WriteByte(':')

	bytes, ok := dp.value.(binaryValue)
	if !ok {
		sb.WriteString("nil")
	} else {
		for dim := 0; dim < dp.ft.PointDimensionCount(); dim++ {
			if dim > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(fmt.Sprintf("%g", decodeDoubleDimension(bytes, dim*8)))
		}
	}

	sb.WriteByte('>')
	return sb.String()
}
