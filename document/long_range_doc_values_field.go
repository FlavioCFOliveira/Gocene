// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
)

// LongRangeDocValuesField is the Go port of org.apache.lucene.document.LongRangeDocValuesField.
// It extends BinaryRangeDocValuesField to provide a specialized field for N-dimensional
// long ranges.
type LongRangeDocValuesField struct {
	*BinaryRangeDocValuesField
	min []int64
	max []int64
}

// NewLongRangeDocValuesField constructs a LongRangeDocValuesField for the given field
// name and min/max range values.
func NewLongRangeDocValuesField(field string, min, max []int64) (*LongRangeDocValuesField, error) {
	checkLongRangeArgs(min, max)

	packed := EncodeLongRange(min, max)

	b, err := NewBinaryRangeDocValuesField(field, packed, len(min), LongRangeBytes)
	if err != nil {
		return nil, err
	}

	// Defensive copies to ensure that modifications to the input slices
	// do not affect the field's internal state.
	dupMin := make([]int64, len(min))
	copy(dupMin, min)
	dupMax := make([]int64, len(max))
	copy(dupMax, max)

	return &LongRangeDocValuesField{
		BinaryRangeDocValuesField: b,
		min:                       dupMin,
		max:                       dupMax,
	}, nil
}

// GetMin returns the minimum value for the given dimension.
// Returns an error if the dimension is out of the valid range (0..numDims-1).
func (f *LongRangeDocValuesField) GetMin(dimension int) (int64, error) {
	if dimension < 0 || dimension >= len(f.min) {
		return 0, fmt.Errorf("dimension %d out of valid range", dimension)
	}
	return f.min[dimension], nil
}

// GetMax returns the maximum value for the given dimension.
// Returns an error if the dimension is out of the valid range (0..numDims-1).
func (f *LongRangeDocValuesField) GetMax(dimension int) (int64, error) {
	if dimension < 0 || dimension >= len(f.max) {
		return 0, fmt.Errorf("dimension %d out of valid range", dimension)
	}
	return f.max[dimension], nil
}

// NewSlowIntersectsQuery constructs a data carrier for a slow intersects query.
// This matches the Java LongRangeDocValuesField.newSlowIntersectsQuery factory.
func newLongSlowIntersectsQuery(field string, min, max []int64) (*LongRangeSlowRangeQuery, error) {
	return NewLongRangeSlowRangeQuery(field, min, max, RangeFieldQueryTypeIntersects)
}
