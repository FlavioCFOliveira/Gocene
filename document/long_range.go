// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

const bytesPerValue = 8

// LongRangeBytes is the width of a single long value in bytes.
const LongRangeBytes = bytesPerValue

// LongRange is an indexed Long Range field.
//
// This field indexes dimensional ranges defined as min/max pairs. It supports up to a maximum of
// 4 dimensions (indexed as 8 numeric values). With 1 dimension representing a single long range, 2
// dimensions representing a bounding box, 3 dimensions a bounding cube, and 4 dimensions a
// tesseract.
//
// Multiple values for the same field in one document is supported, and open ended ranges can be
// defined using math.MinInt64 and math.MaxInt64.
//
// This is the Go port of Lucene's org.apache.lucene.document.LongRange.
type LongRange struct {
	Field
}

// NewLongRange creates a new LongRange type, from min/max parallel arrays.
//
// name: field name. must not be empty.
// min: range min values; each entry is the min value for the dimension.
// max: range max values; each entry is the max value for the dimension.
func NewLongRange(name string, min, max []int64) *LongRange {
	dims := len(min)
	ft := getLongRangeType(dims)

	f, err := NewField(name, nil, ft)
	if err != nil {
		// In the original Java code, NewField (via Field constructor) does not throw
		// for these arguments, but Gocene's NewField does. Since this is a constructor
		// that should be safe if args are valid, we handle the error.
		panic(err)
	}

	lr := &LongRange{Field: *f}
	lr.SetRangeValues(min, max)
	return lr
}

func getLongRangeType(dimensions int) *FieldType {
	if dimensions > 4 {
		panic("LongRange does not support greater than 4 dimensions")
	}

	ft := NewFieldType()
	// dimensions is set as 2*dimension size (min/max per dimension)
	ft.SetDimensions(dimensions*2, bytesPerValue)
	ft.Freeze()
	return ft
}

// SetRangeValues changes the values of the field.
//
// min: array of min values. (accepts math.MinInt64)
// max: array of max values. (accepts math.MaxInt64)
//
// It panics if min or max is invalid.
func (lr *LongRange) SetRangeValues(min, max []int64) {
	checkLongRangeArgs(min, max)
	if len(min)*2 != lr.ft.PointDimensionCount() || len(max)*2 != lr.ft.PointDimensionCount() {
		panic(fmt.Sprintf("field (name=%s) uses %d dimensions; cannot change to (incoming) %d dimensions",
			lr.name, lr.ft.PointDimensionCount()/2, len(min)))
	}

	var bytes []byte
	if lr.value == nil {
		bytes = make([]byte, bytesPerValue*2*len(min))
		lr.value = binaryValue(bytes)
	} else {
		if bv, ok := lr.value.(binaryValue); ok {
			bytes = []byte(bv)
		} else {
			// Fallback if value was set to something else
			bytes = make([]byte, bytesPerValue*2*len(min))
			lr.value = binaryValue(bytes)
		}
	}
	verifyAndEncodeLongRange(min, max, bytes)
}

func checkLongRangeArgs(min, max []int64) {
	if min == nil || max == nil || len(min) == 0 || len(max) == 0 {
		panic("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		panic("min/max ranges must agree")
	}
	if len(min) > 4 {
		panic("LongRange does not support greater than 4 dimensions")
	}
}

// EncodeLongRange encodes the min, max ranges into a byte array.
func EncodeLongRange(min, max []int64) []byte {
	checkLongRangeArgs(min, max)
	b := make([]byte, bytesPerValue*2*len(min))
	verifyAndEncodeLongRange(min, max, b)
	return b
}

func verifyAndEncodeLongRange(min, max []int64, bytes []byte) {
	for d := 0; d < len(min); d++ {
		i := d * bytesPerValue
		j := len(min)*bytesPerValue + i
		if min[d] > max[d] {
			panic(fmt.Sprintf("min value (%d) is greater than max value (%d)", min[d], max[d]))
		}
		encode(min[d], bytes, i)
		encode(max[d], bytes, j)
	}
}

func encode(val int64, bytes []byte, offset int) {
	util.LongToSortableBytes(val, bytes, offset)
}

// GetMin returns the min value for the given dimension.
//
// dimension: the dimension, always positive.
// It panics if the dimension is out of bounds.
func (lr *LongRange) GetMin(dimension int) int64 {
	if dimension < 0 || dimension >= lr.ft.PointDimensionCount()/2 {
		panic(fmt.Sprintf("dimension %d out of bounds [0, %d)", dimension, lr.ft.PointDimensionCount()/2))
	}
	return decodeMin(lr.binaryValue(), dimension)
}

// GetMax returns the max value for the given dimension.
//
// dimension: the dimension, always positive.
// It panics if the dimension is out of bounds.
func (lr *LongRange) GetMax(dimension int) int64 {
	if dimension < 0 || dimension >= lr.ft.PointDimensionCount()/2 {
		panic(fmt.Sprintf("dimension %d out of bounds [0, %d)", dimension, lr.ft.PointDimensionCount()/2))
	}
	return decodeMax(lr.binaryValue(), dimension)
}

func decodeMin(b []byte, dimension int) int64 {
	offset := dimension * bytesPerValue
	return util.SortableBytesToLong(b, offset)
}

func decodeMax(b []byte, dimension int) int64 {
	offset := len(b)/2 + dimension*bytesPerValue
	return util.SortableBytesToLong(b, offset)
}

func (lr *LongRange) binaryValue() []byte {
	if lr.value == nil {
		return nil
	}
	if bv, ok := lr.value.(binaryValue); ok {
		return []byte(bv)
	}
	return nil
}

// NewIntersectsQuery creates a query for matching indexed ranges that intersect the defined range.
func NewIntersectsQuery(field string, min, max []int64) (*RangeFieldQuery, error) {
	return newRelationQuery(field, min, max, RangeFieldQueryTypeIntersects)
}

// NewContainsQuery creates a query for matching indexed ranges that contain the defined range.
func NewContainsQuery(field string, min, max []int64) (*RangeFieldQuery, error) {
	return newRelationQuery(field, min, max, RangeFieldQueryTypeContains)
}

// NewWithinQuery creates a query for matching indexed ranges that are within the defined range.
func NewWithinQuery(field string, min, max []int64) (*RangeFieldQuery, error) {
	return newRelationQuery(field, min, max, RangeFieldQueryTypeWithin)
}

// NewCrossesQuery creates a query for matching indexed ranges that cross the defined range.
func NewCrossesQuery(field string, min, max []int64) (*RangeFieldQuery, error) {
	return newRelationQuery(field, min, max, RangeFieldQueryTypeCrosses)
}

func newRelationQuery(field string, min, max []int64, relation RangeFieldQueryType) (*RangeFieldQuery, error) {
	checkLongRangeArgs(min, max)
	return NewRangeFieldQuery(field, EncodeLongRange(min, max), len(min), relation)
}

// String returns the string representation of the LongRange field.
func (lr *LongRange) String() string {
	var sb strings.Builder
	sb.WriteString("LongRange <")
	sb.WriteString(lr.name)
	sb.WriteByte(':')

	b := lr.binaryValue()
	if b == nil {
		sb.WriteString("nil")
	} else {
		for d := 0; d < lr.ft.PointDimensionCount()/2; d++ {
			sb.WriteByte(' ')
			sb.WriteString(formatRange(b, d))
		}
	}
	sb.WriteByte('>')

	return sb.String()
}

func formatRange(ranges []byte, dimension int) string {
	return fmt.Sprintf("[%d : %d]", decodeMin(ranges, dimension), decodeMax(ranges, dimension))
}
