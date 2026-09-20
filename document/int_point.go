// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// IntPoint is an indexed int field for fast range filters. If you also need to store the value, you
// should add a separate StoredField instance.
//
// Finding all documents within an N-dimensional shape or range at search time is efficient.
// Multiple values for the same field in one document is allowed.
//
// This is the Go port of Lucene's org.apache.lucene.document.IntPoint.
type IntPoint struct {
	Field
}

func getIntPointType(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetDimensions(numDims, 4) // Integer.BYTES = 4
	ft.Freeze()
	return ft
}

// NewIntPoint creates a new IntPoint, indexing the provided N-dimensional int point.
func NewIntPoint(name string, point ...int32) *IntPoint {
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}
	ft := getIntPointType(len(point))
	val := packIntPoint(point...)
	f, _ := NewField(name, val, ft)
	return &IntPoint{Field: *f}
}

// SetIntValue sets the value of this field.
func (ip *IntPoint) SetIntValue(value int32) {
	ip.SetIntValues(value)
}

// SetIntValues changes the values of this field.
func (ip *IntPoint) SetIntValues(point ...int32) {
	if ip.ft.PointDimensionCount() != len(point) {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot change to (incoming) %d dimensions",
			ip.name, ip.ft.PointDimensionCount(), len(point)))
	}
	ip.value = binaryValue(packIntPoint(point...))
}

// SetBytesValue is not supported for IntPoint.
func (ip *IntPoint) SetBytesValue(bytes []byte) {
	panic("cannot change value type from int to BytesRef")
}

// NumericValue returns the numeric value of the field.
func (ip *IntPoint) NumericValue() interface{} {
	if ip.ft.PointDimensionCount() != 1 {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot convert to a single numeric value",
			ip.name, ip.ft.PointDimensionCount()))
	}
	bytes := ip.BinaryValue()
	if len(bytes) != 4 {
		panic("invalid binary value length for int point")
	}
	return decodeIntDimension(bytes, 0)
}

// packIntPoint packs an integer point into a byte slice.
func packIntPoint(point ...int32) []byte {
	if point == nil {
		panic("point must not be null")
	}
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}
	packed := make([]byte, len(point)*4)

	for dim := 0; dim < len(point); dim++ {
		encodeIntDimension(point[dim], packed, dim*4)
	}

	return packed
}

func (ip *IntPoint) String() string {
	var sb strings.Builder
	sb.WriteString("IntPoint <")
	sb.WriteString(ip.name)
	sb.WriteByte(':')

	bytes := ip.BinaryValue()
	for dim := 0; dim < ip.ft.PointDimensionCount(); dim++ {
		if dim > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%d", decodeIntDimension(bytes, dim*4)))
	}

	sb.WriteByte('>')
	return sb.String()
}

// encodeIntDimension encodes a single integer dimension.
func encodeIntDimension(value int32, dest []byte, offset int) {
	util.IntToSortableBytes(value, dest, offset)
}

// decodeIntDimension decodes a single integer dimension.
func decodeIntDimension(value []byte, offset int) int32 {
	return util.SortableBytesToInt(value, offset)
}
