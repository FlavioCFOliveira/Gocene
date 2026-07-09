// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LongField is a field for indexing int64 values.
type LongField struct {
	*Field
}

// NewLongField creates a new LongField.
func NewLongField(name string, value int64, store bool) (*LongField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatInt(value, 10), ft)
	if err != nil {
		return nil, err
	}

	return &LongField{Field: field}, nil
}

// LongPoint is an indexed int64 point field for range queries using the Point API.
type LongPoint struct {
	Point
}

// NewLongPoint creates a new LongPoint with a single value.
func NewLongPoint(name string, value int64) *LongPoint {
	return NewLongPoints(name, value)
}

// NewLongPoints creates a new LongPoint with multiple values.
func NewLongPoints(name string, values ...int64) *LongPoint {
	if len(values) == 0 {
		return nil
	}

	// Encode with Lucene's sign-flipped sortable-bytes encoding
	// (NumericUtils.longToSortableBytes), the on-disk BKD point format Apache
	// Lucene 10.4.0 produces and consumes. Plain big-endian would mis-order
	// negative values and break binary compatibility with Lucene's LongPoint.
	encoded := PackLongsLucene(values...)
	ft := PointFieldType()
	ft.DimensionNumBytes = 8

	point, _ := NewPoint(name, ft, encoded, len(values), 8)
	return &LongPoint{Point: *point}
}

// LongValue returns the first long value.
func (lp *LongPoint) LongValue() int64 {
	values := lp.LongValues()
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

// LongValues returns all long values, decoding the Lucene sortable-bytes
// encoding used by NewLongPoints.
func (lp *LongPoint) LongValues() []int64 {
	packed := lp.PointValues()
	if len(packed)%8 != 0 {
		return nil
	}
	out := make([]int64, len(packed)/8)
	for i := range out {
		out[i] = DecodeDimensionLongLucene(packed, i*8)
	}
	return out
}
