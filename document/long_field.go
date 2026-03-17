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

	encoded := PackLongs(values)
	ft := PointFieldType()
	ft.DimensionNumBytes = 8

	point, _ := NewPoint(name, ft, encoded, 1, 8)
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

// LongValues returns all long values.
func (lp *LongPoint) LongValues() []int64 {
	return UnpackLongs(lp.PointValues())
}
