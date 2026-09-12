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

// FloatRange is an indexed Float Range field.
//
// This field indexes dimensional ranges defined as min/max pairs. It supports up to a maximum of
// 4 dimensions (indexed as 8 numeric values). With 1 dimension representing a single float range,
// 2 dimensions representing a bounding box, 3 dimensions a bounding cube, and 4 dimensions a
// tesseract.
//
// Multiple values for the same field in one document is supported, and open ended ranges can be
// defined using math.Inf(-1) and math.Inf(1).
//
// This is the Go port of Lucene's org.apache.lucene.document.FloatRange.
type FloatRange struct {
	Field
}

// BYTES stores float values so number of bytes is 4.
const BYTES = 4

// NewFloatRange creates a new FloatRange type from min/max parallel arrays.
//
// min range min values; each entry is the min value for the dimension.
// max range max values; each entry is the max value for the dimension.
func NewFloatRange(name string, min, max []float32) (*FloatRange, error) {
	if err := checkFloatRangeArgs(min, max); err != nil {
		return nil, err
	}

	ft := getFloatRangeType(len(min))
	f, err := NewField(name, nil, ft)
	if err != nil {
		return nil, err
	}

	fr := &FloatRange{Field: *f}
	if err := fr.SetRangeValues(min, max); err != nil {
		return nil, err
	}

	return fr, nil
}

func getFloatRangeType(dimensions int) *FieldType {
	if dimensions > 4 {
		panic("FloatRange does not support greater than 4 dimensions")
	}

	ft := NewFieldType()
	// dimensions is set as 2*dimension size (min/max per dimension)
	ft.SetDimensions(dimensions*2, BYTES)
	ft.Freeze()
	return ft
}

// SetRangeValues changes the values of the field.
//
// min array of min values. (accepts math.Inf(-1))
// max array of max values. (accepts math.Inf(1))
func (fr *FloatRange) SetRangeValues(min, max []float32) error {
	if err := checkFloatRangeArgs(min, max); err != nil {
		return err
	}

	if len(min)*2 != fr.ft.PointDimensionCount() || len(max)*2 != fr.ft.PointDimensionCount() {
		return fmt.Errorf("field (name=%s) uses %d dimensions; cannot change to (incoming) %d dimensions",
			fr.name, fr.ft.PointDimensionCount()/2, len(min))
	}

	var bytes []byte
	if fr.value == nil {
		bytes = make([]byte, BYTES*2*len(min))
		fr.value = binaryValue(bytes)
	} else {
		bytes = []byte(fr.value.(binaryValue))
	}

	return verifyAndEncode(min, max, bytes)
}

func checkFloatRangeArgs(min, max []float32) error {
	if min == nil || max == nil || len(min) == 0 || len(max) == 0 {
		return fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return fmt.Errorf("min/max ranges must agree")
	}
	if len(min) > 4 {
		return fmt.Errorf("FloatRange does not support greater than 4 dimensions")
	}
	return nil
}

// Encode encodes the min, max ranges into a byte array.
func Encode(min, max []float32) ([]byte, error) {
	if err := checkFloatRangeArgs(min, max); err != nil {
		return nil, err
	}
	b := make([]byte, BYTES*2*len(min))
	if err := verifyAndEncode(min, max, b); err != nil {
		return nil, err
	}
	return b, nil
}

func verifyAndEncode(min, max []float32, bytes []byte) error {
	for d := 0; d < len(min); d++ {
		i := d * BYTES
		j := len(min)*BYTES + (d * BYTES)

		if math.IsNaN(float64(min[d])) {
			return fmt.Errorf("invalid min value (%f) in FloatRange", min[d])
		}
		if math.IsNaN(float64(max[d])) {
			return fmt.Errorf("invalid max value (%f) in FloatRange", max[d])
		}
		if min[d] > max[d] {
			return fmt.Errorf("min value (%f) is greater than max value (%f)", min[d], max[d])
		}
		encodeValue(min[d], bytes, i)
		encodeValue(max[d], bytes, j)
	}
	return nil
}

func encodeValue(val float32, bytes []byte, offset int) {
	util.IntToSortableBytes(util.FloatToSortableInt(val), bytes, offset)
}

// GetMin returns the min value for the given dimension.
func (fr *FloatRange) GetMin(dimension int) (float32, error) {
	if dimension < 0 || dimension >= fr.ft.PointDimensionCount()/2 {
		return 0, fmt.Errorf("dimension %d out of bounds [0, %d)", dimension, fr.ft.PointDimensionCount()/2)
	}
	return DecodeMin([]byte(fr.value.(binaryValue)), dimension), nil
}

// GetMax returns the max value for the given dimension.
func (fr *FloatRange) GetMax(dimension int) (float32, error) {
	if dimension < 0 || dimension >= fr.ft.PointDimensionCount()/2 {
		return 0, fmt.Errorf("dimension %d out of bounds [0, %d)", dimension, fr.ft.PointDimensionCount()/2)
	}
	return DecodeMax([]byte(fr.value.(binaryValue)), dimension), nil
}

// DecodeMin decodes the min value (for the defined dimension) from the encoded input byte array.
func DecodeMin(b []byte, dimension int) float32 {
	offset := dimension * BYTES
	return util.SortableIntToFloat(util.SortableBytesToInt(b, offset))
}

// DecodeMax decodes the max value (for the defined dimension) from the encoded input byte array.
func DecodeMax(b []byte, dimension int) float32 {
	offset := len(b)/2 + dimension*BYTES
	return util.SortableIntToFloat(util.SortableBytesToInt(b, offset))
}

// NewFloatRangeIntersectsQuery creates a query for matching indexed ranges that intersect the defined range.
func NewFloatRangeIntersectsQuery(field string, min, max []float32) (*RangeFieldQuery, error) {
	return newFloatRelationQuery(field, min, max, RangeFieldQueryTypeIntersects)
}

// NewFloatRangeContainsQuery creates a query for matching indexed float ranges that contain the defined range.
func NewFloatRangeContainsQuery(field string, min, max []float32) (*RangeFieldQuery, error) {
	return newFloatRelationQuery(field, min, max, RangeFieldQueryTypeContains)
}

// NewFloatRangeWithinQuery creates a query for matching indexed ranges that are within the defined range.
func NewFloatRangeWithinQuery(field string, min, max []float32) (*RangeFieldQuery, error) {
	return newFloatRelationQuery(field, min, max, RangeFieldQueryTypeWithin)
}

// NewFloatRangeCrossesQuery creates a query for matching indexed ranges that cross the defined range.
func NewFloatRangeCrossesQuery(field string, min, max []float32) (*RangeFieldQuery, error) {
	return newFloatRelationQuery(field, min, max, RangeFieldQueryTypeCrosses)
}

func newFloatRelationQuery(field string, min, max []float32, relation RangeFieldQueryType) (*RangeFieldQuery, error) {
	if err := checkFloatRangeArgs(min, max); err != nil {
		return nil, err
	}
	encoded, err := Encode(min, max)
	if err != nil {
		return nil, err
	}
	return NewRangeFieldQuery(field, encoded, len(min), relation)
}

// String returns a string representation of the FloatRange.
func (fr *FloatRange) String() string {
	var sb strings.Builder
	sb.WriteString("FloatRange <")
	sb.WriteString(fr.name)
	sb.WriteByte(':')

	b := []byte(fr.value.(binaryValue))
	for d := 0; d < fr.ft.PointDimensionCount()/2; d++ {
		if d > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(toStringRange(b, d))
	}
	sb.WriteByte('>')

	return sb.String()
}

func toStringRange(ranges []byte, dimension int) string {
	return fmt.Sprintf("[%f : %f]", DecodeMin(ranges, dimension), DecodeMax(ranges, dimension))
}
