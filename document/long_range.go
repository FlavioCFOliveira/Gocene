// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LongRange is the Lucene 10.4.0-compatible indexed Long Range field.
//
// It indexes dimensional ranges defined as min/max pairs, supporting up to
// 4 dimensions (indexed as 8 numeric values). With 1 dimension representing
// a single long range, 2 dimensions a bounding box, 3 dimensions a bounding
// cube, and 4 dimensions a tesseract.
//
// Open-ended ranges are expressed with math.MinInt64 / math.MaxInt64.
//
// This is the canonical Gocene port of org.apache.lucene.document.LongRange.
//
// Layout: dimensionCount = 2 * numDims, bytes-per-dim = [LongRangeBytes].
// Values are encoded with Lucene's sortable-bytes scheme
// ([util.LongToSortableBytes]) so that unsigned byte-order matches numeric
// order. Bytes are packed as:
//
//	minD0 ... minD{N-1} | maxD0 ... maxD{N-1}
//
// matching the JVM-produced byte stream exactly.
//
// Static query factories (NewIntersectsQuery / NewContainsQuery /
// NewWithinQuery / NewCrossesQuery) are deferred — they depend on
// search.RangeFieldQuery. See backlog #2695.
type LongRange struct {
	*Field
	numDims int
}

// LongRangeBytes is the byte width of a single encoded long value
// (mirrors org.apache.lucene.document.LongRange.BYTES = Long.BYTES = 8).
const LongRangeBytes = 8

// longRangeMaxDims is the maximum number of dimensions allowed by Lucene
// (mirrors the IllegalArgumentException thrown in LongRange.checkArgs / getType).
const longRangeMaxDims = 4

// NewLongRange creates a new LongRange with N-dimensional [min, max] pairs.
//
// Constraints (matching Lucene):
//   - len(min) == len(max), both in 1..4.
//   - For every dimension i, min[i] <= max[i].
//
// Validation errors mirror Lucene's IllegalArgumentException semantics.
func NewLongRange(name string, min, max []int64) (*LongRange, error) {
	if err := checkLongRangeArgs(min, max); err != nil {
		return nil, err
	}
	encoded, err := EncodeLongRange(min, max)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, longRangeFieldType(len(min)))
	if err != nil {
		return nil, err
	}
	return &LongRange{Field: field, numDims: len(min)}, nil
}

// NumDimensions returns the dimension count (1..4).
func (r *LongRange) NumDimensions() int { return r.numDims }

// GetMin returns the decoded minimum value for the given dimension.
// Panics if dim is out of [0, NumDimensions()).
func (r *LongRange) GetMin(dim int) int64 {
	r.checkDim(dim)
	return decodeLongRangeMin(r.BinaryValue(), dim)
}

// GetMax returns the decoded maximum value for the given dimension.
// Panics if dim is out of [0, NumDimensions()).
func (r *LongRange) GetMax(dim int) int64 {
	r.checkDim(dim)
	return decodeLongRangeMax(r.BinaryValue(), dim)
}

// String returns the Lucene-compatible representation, e.g.
// "LongRange <foo: [1 : 2] [11 : 12] [21 : 22] [31 : 32]>".
func (r *LongRange) String() string {
	b := r.BinaryValue()
	var sb strings.Builder
	sb.WriteString("LongRange <")
	sb.WriteString(r.Name())
	sb.WriteByte(':')
	for d := 0; d < r.numDims; d++ {
		sb.WriteByte(' ')
		sb.WriteString(formatLongRangeDim(b, d))
	}
	sb.WriteByte('>')
	return sb.String()
}

// EncodeLongRange packs N-dimensional long min/max pairs into Lucene's
// sortable-byte layout. Validates dimension count and per-dimension order.
func EncodeLongRange(min, max []int64) ([]byte, error) {
	if err := checkLongRangeArgs(min, max); err != nil {
		return nil, err
	}
	n := len(min)
	out := make([]byte, 2*n*LongRangeBytes)
	for i := 0; i < n; i++ {
		if min[i] > max[i] {
			return nil, fmt.Errorf("min value (%d) is greater than max value (%d)", min[i], max[i])
		}
		util.LongToSortableBytes(min[i], out, i*LongRangeBytes)
		util.LongToSortableBytes(max[i], out, n*LongRangeBytes+i*LongRangeBytes)
	}
	return out, nil
}

// longRangeFieldType builds the FieldType for a LongRange with the given
// number of dimensions; dimensionCount = 2 * numDims.
func longRangeFieldType(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(2*numDims, LongRangeBytes)
	ft.Freeze()
	return ft
}

// checkLongRangeArgs mirrors Lucene's LongRange.checkArgs.
func checkLongRangeArgs(min, max []int64) error {
	if len(min) == 0 || len(max) == 0 {
		return fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return fmt.Errorf("min/max ranges must agree")
	}
	if len(min) > longRangeMaxDims {
		return fmt.Errorf("LongRange does not support greater than %d dimensions", longRangeMaxDims)
	}
	return nil
}

// checkDim mirrors Objects.checkIndex(dim, numDims) in Lucene's getMin/getMax.
func (r *LongRange) checkDim(dim int) {
	if dim < 0 || dim >= r.numDims {
		panic(fmt.Sprintf("dimension %d out of range [0, %d)", dim, r.numDims))
	}
}

// decodeLongRangeMin reads the min value at the given dimension from the
// packed Lucene layout.
func decodeLongRangeMin(b []byte, dim int) int64 {
	return util.SortableBytesToLong(b, dim*LongRangeBytes)
}

// decodeLongRangeMax reads the max value at the given dimension from the
// packed Lucene layout.
func decodeLongRangeMax(b []byte, dim int) int64 {
	return util.SortableBytesToLong(b, len(b)/2+dim*LongRangeBytes)
}

// formatLongRangeDim renders one dimension as "[min : max]".
func formatLongRangeDim(b []byte, dim int) string {
	return fmt.Sprintf("[%d : %d]", decodeLongRangeMin(b, dim), decodeLongRangeMax(b, dim))
}
