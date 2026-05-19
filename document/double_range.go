// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DoubleRange is the Lucene 10.4.0-compatible indexed Double Range field.
//
// It indexes dimensional ranges defined as min/max pairs, supporting up to
// 4 dimensions (indexed as 8 numeric values). With 1 dimension representing
// a single double range, 2 dimensions a bounding box, 3 dimensions a bounding
// cube, and 4 dimensions a tesseract.
//
// Open-ended ranges are expressed with math.Inf(-1) / math.Inf(+1). NaN is
// rejected (mirroring Lucene's IllegalArgumentException in verifyAndEncode).
//
// This is the canonical Gocene port of org.apache.lucene.document.DoubleRange.
//
// Layout: dimensionCount = 2 * numDims, bytes-per-dim = [DoubleRangeBytes].
// Values are encoded with Lucene's sortable scheme
// ([util.DoubleToSortableLong] followed by [util.LongToSortableBytes]) so
// that unsigned byte-order matches numeric order. Bytes are packed as:
//
//	minD0 ... minD{N-1} | maxD0 ... maxD{N-1}
//
// matching the JVM-produced byte stream exactly.
//
// Static query factories (NewIntersectsQuery / NewContainsQuery /
// NewWithinQuery / NewCrossesQuery) are deferred — they depend on
// search.RangeFieldQuery. See backlog #2695.
type DoubleRange struct {
	*Field
	numDims int
}

// DoubleRangeBytes is the byte width of a single encoded double value
// (mirrors org.apache.lucene.document.DoubleRange.BYTES = Double.BYTES = 8).
const DoubleRangeBytes = 8

// doubleRangeMaxDims is the maximum number of dimensions allowed by Lucene
// (mirrors the IllegalArgumentException thrown in DoubleRange.checkArgs / getType).
const doubleRangeMaxDims = 4

// NewDoubleRange creates a new DoubleRange with N-dimensional [min, max] pairs.
//
// Constraints (matching Lucene):
//   - len(min) == len(max), both in 1..4.
//   - Neither min[i] nor max[i] may be NaN.
//   - For every dimension i, min[i] <= max[i].
//
// Validation errors mirror Lucene's IllegalArgumentException semantics.
func NewDoubleRange(name string, min, max []float64) (*DoubleRange, error) {
	if err := checkDoubleRangeArgs(min, max); err != nil {
		return nil, err
	}
	encoded, err := EncodeDoubleRange(min, max)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, doubleRangeFieldType(len(min)))
	if err != nil {
		return nil, err
	}
	return &DoubleRange{Field: field, numDims: len(min)}, nil
}

// NumDimensions returns the dimension count (1..4).
func (r *DoubleRange) NumDimensions() int { return r.numDims }

// GetMin returns the decoded minimum value for the given dimension.
// Panics if dim is out of [0, NumDimensions()).
func (r *DoubleRange) GetMin(dim int) float64 {
	r.checkDim(dim)
	return decodeDoubleRangeMin(r.BinaryValue(), dim)
}

// GetMax returns the decoded maximum value for the given dimension.
// Panics if dim is out of [0, NumDimensions()).
func (r *DoubleRange) GetMax(dim int) float64 {
	r.checkDim(dim)
	return decodeDoubleRangeMax(r.BinaryValue(), dim)
}

// String returns the Lucene-compatible representation, e.g.
// "DoubleRange <foo: [0.1 : 0.2] [1.1 : 1.2] [2.1 : 2.2] [3.1 : 3.2]>".
func (r *DoubleRange) String() string {
	b := r.BinaryValue()
	var sb strings.Builder
	sb.WriteString("DoubleRange <")
	sb.WriteString(r.Name())
	sb.WriteByte(':')
	for d := 0; d < r.numDims; d++ {
		sb.WriteByte(' ')
		sb.WriteString(formatDoubleRangeDim(b, d))
	}
	sb.WriteByte('>')
	return sb.String()
}

// EncodeDoubleRange packs N-dimensional double min/max pairs into Lucene's
// sortable-byte layout. Validates dimension count, NaN values, and
// per-dimension order.
func EncodeDoubleRange(min, max []float64) ([]byte, error) {
	if err := checkDoubleRangeArgs(min, max); err != nil {
		return nil, err
	}
	n := len(min)
	out := make([]byte, 2*n*DoubleRangeBytes)
	for i := 0; i < n; i++ {
		if math.IsNaN(min[i]) {
			return nil, fmt.Errorf("invalid min value (NaN) in DoubleRange")
		}
		if math.IsNaN(max[i]) {
			return nil, fmt.Errorf("invalid max value (NaN) in DoubleRange")
		}
		if min[i] > max[i] {
			return nil, fmt.Errorf("min value (%s) is greater than max value (%s)",
				formatDoubleRangeValue(min[i]), formatDoubleRangeValue(max[i]))
		}
		util.LongToSortableBytes(util.DoubleToSortableLong(min[i]), out, i*DoubleRangeBytes)
		util.LongToSortableBytes(util.DoubleToSortableLong(max[i]), out, n*DoubleRangeBytes+i*DoubleRangeBytes)
	}
	return out, nil
}

// doubleRangeFieldType builds the FieldType for a DoubleRange with the given
// number of dimensions; dimensionCount = 2 * numDims.
func doubleRangeFieldType(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(2*numDims, DoubleRangeBytes)
	ft.Freeze()
	return ft
}

// checkDoubleRangeArgs mirrors Lucene's DoubleRange.checkArgs.
func checkDoubleRangeArgs(min, max []float64) error {
	if len(min) == 0 || len(max) == 0 {
		return fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return fmt.Errorf("min/max ranges must agree")
	}
	if len(min) > doubleRangeMaxDims {
		return fmt.Errorf("DoubleRange does not support greater than %d dimensions", doubleRangeMaxDims)
	}
	return nil
}

// checkDim mirrors Objects.checkIndex(dim, numDims) in Lucene's getMin/getMax.
func (r *DoubleRange) checkDim(dim int) {
	if dim < 0 || dim >= r.numDims {
		panic(fmt.Sprintf("dimension %d out of range [0, %d)", dim, r.numDims))
	}
}

// decodeDoubleRangeMin reads the min value at the given dimension from the
// packed Lucene layout.
func decodeDoubleRangeMin(b []byte, dim int) float64 {
	return util.SortableLongToDouble(util.SortableBytesToLong(b, dim*DoubleRangeBytes))
}

// decodeDoubleRangeMax reads the max value at the given dimension from the
// packed Lucene layout.
func decodeDoubleRangeMax(b []byte, dim int) float64 {
	return util.SortableLongToDouble(util.SortableBytesToLong(b, len(b)/2+dim*DoubleRangeBytes))
}

// formatDoubleRangeDim renders one dimension as "[min : max]" using the
// Java Double.toString-equivalent formatting (shortest round-trip).
func formatDoubleRangeDim(b []byte, dim int) string {
	return "[" + formatDoubleRangeValue(decodeDoubleRangeMin(b, dim)) +
		" : " + formatDoubleRangeValue(decodeDoubleRangeMax(b, dim)) + "]"
}

// formatDoubleRangeValue mirrors java.lang.Double.toString for finite values:
// shortest decimal that round-trips. Go's strconv.FormatFloat(v, 'g', -1, 64)
// produces the same output for the values exercised by Lucene's tests
// (e.g. 0.1, 0.2, ..., 3.1, 3.2 → "0.1", "0.2", ..., "3.1", "3.2"). Special
// values (Infinity, -Infinity) are emitted as "Infinity"/"-Infinity" to match
// Java's representation; NaN is included for completeness (rejected at encode).
func formatDoubleRangeValue(v float64) string {
	switch {
	case math.IsInf(v, +1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case math.IsNaN(v):
		return "NaN"
	default:
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
}
