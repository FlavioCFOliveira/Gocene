// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LongPoint is an indexed long field for fast range filters. If you also need to store the value, you
// should add a separate StoredField instance.
//
// Finding all documents within an N-dimensional shape or range at search time is efficient.
// Multiple values for the same field in one document is allowed.
//
// This field defines static factory methods for creating common queries:
//
//   - NewExactQuery(string, int64) for matching an exact 1D point.
//   - NewSetQuery(string, ...int64) for matching a set of 1D values.
//   - NewRangeQuery(string, int64, int64) for matching a 1D range.
//   - NewRangeQueryND(string, []int64, []int64) for matching points/ranges in n-dimensional space.
//
// This is the Go port of Lucene's org.apache.lucene.document.LongPoint.
type LongPoint struct {
	Field
}

func getType(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetDimensions(numDims, 8) // Long.BYTES = 8
	ft.Freeze()
	return ft
}

// SetLongValue sets a single long value.
func (lp *LongPoint) SetLongValue(value int64) {
	lp.SetLongValues(value)
}

// SetLongValues changes the values of this field.
func (lp *LongPoint) SetLongValues(point ...int64) {
	if lp.ft.PointDimensionCount() != len(point) {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot change to (incoming) %d dimensions",
			lp.name, lp.ft.PointDimensionCount(), len(point)))
	}
	lp.value = binaryValue(Pack(point))
}

// SetBytesValue is not supported for LongPoint.
func (lp *LongPoint) SetBytesValue(bytes *util.BytesRef) {
	panic("cannot change value type from long to BytesRef")
}

// NumericValue returns the numeric value of the field.
// Only supported for 1D points.
func (lp *LongPoint) NumericValue() interface{} {
	if lp.ft.PointDimensionCount() != 1 {
		panic(fmt.Sprintf("this field (name=%s) uses %d dimensions; cannot convert to a single numeric value",
			lp.name, lp.ft.PointDimensionCount()))
	}
	bytes := lp.value.(binaryValue)
	return DecodeDimension(bytes, 0)
}

// Pack packs a long point into a byte slice.
//
// Returns an error if the point is nil or of zero length.
func Pack(point ...int64) []byte {
	if point == nil {
		panic("point must not be null")
	}
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}
	packed := make([]byte, len(point)*8)

	for dim := 0; dim < len(point); dim++ {
		EncodeDimension(point[dim], packed, dim*8)
	}

	return packed
}

// Unpack unpacks a BytesRef into a long point.
func Unpack(bytesRef *util.BytesRef, start int, buf []int64) {
	if bytesRef == nil || buf == nil {
		panic("bytesRef and buf must not be null")
	}

	for i, offset := 0, start; i < len(buf); i, offset = i+1, offset+8 {
		buf[i] = DecodeDimension(bytesRef.ValidBytes(), offset)
	}
}

// NewLongPoint creates a new LongPoint, indexing the provided N-dimensional long point.
func NewLongPoint(name string, point ...int64) *LongPoint {
	ft := getType(len(point))
	f, _ := NewField(name, binaryValue(Pack(point)), ft)
	return &LongPoint{Field: *f}
}

func (lp *LongPoint) String() string {
	var sb strings.Builder
	sb.WriteString("LongPoint <")
	sb.WriteString(lp.name)
	sb.WriteByte(':')

	bytes := lp.value.(binaryValue)
	for dim := 0; dim < lp.ft.PointDimensionCount(); dim++ {
		if dim > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatInt(DecodeDimension(bytes, dim*8), 10))
	}

	sb.WriteByte('>')
	return sb.String()
}

// EncodeDimension encodes a single long dimension into sortable bytes.
func EncodeDimension(value int64, dest []byte, offset int) {
	util.LongToSortableBytes(value, dest, offset)
}

// DecodeDimension decodes a single long dimension from sortable bytes.
func DecodeDimension(value []byte, offset int) int64 {
	return util.SortableBytesToLong(value, offset)
}
