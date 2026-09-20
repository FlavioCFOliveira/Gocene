// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
)

// IntRangeDocValuesField is a DocValues field for IntRange.
// This is a single valued field per document due to being an
// extension of BinaryRangeDocValuesField.
type IntRangeDocValuesField struct {
	*BinaryRangeDocValuesField
	min []int32
	max []int32
}

// NewIntRangeDocValuesField is the sole constructor.
func NewIntRangeDocValuesField(field string, min, max []int32) (*IntRangeDocValuesField, error) {
	if err := checkIntRangeDocValuesArgs(min, max); err != nil {
		return nil, err
	}
	encoded, err := EncodeIntRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryRangeDocValuesField(field, encoded, len(min), 4)
	if err != nil {
		return nil, err
	}
	dupMin := make([]int32, len(min))
	dupMax := make([]int32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &IntRangeDocValuesField{
		BinaryRangeDocValuesField: b,
		min:                       dupMin,
		max:                       dupMax,
	}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *IntRangeDocValuesField) GetMin(dimension int) (int32, error) {
	if dimension > 4 || dimension >= len(f.min) || dimension < 0 {
		return 0, fmt.Errorf("dimension %d out of valid range", dimension)
	}
	return f.min[dimension], nil
}

// GetMax returns the maximum value for the given dimension.
func (f *IntRangeDocValuesField) GetMax(dimension int) (int32, error) {
	if dimension > 4 || dimension >= len(f.max) || dimension < 0 {
		return 0, fmt.Errorf("dimension %d out of valid range", dimension)
	}
	return f.max[dimension], nil
}

// NewSlowIntersectsQuery creates a new range query that finds all ranges that intersect using doc values.
// NOTE: This doesn't leverage indexing and may be slow.
func newIntSlowIntersectsQuery(field string, min, max []int32) (*IntRangeSlowRangeQuery, error) {
	return newSlowRangeQuery(field, min, max, RangeFieldQueryTypeIntersects)
}

func newSlowRangeQuery(field string, min, max []int32, queryType RangeFieldQueryType) (*IntRangeSlowRangeQuery, error) {
	if err := checkIntRangeDocValuesArgs(min, max); err != nil {
		return nil, err
	}
	return NewIntRangeSlowRangeQuery(field, min, max, queryType)
}

func checkIntRangeDocValuesArgs(min, max []int32) error {
	if len(min) == 0 || len(max) == 0 {
		return fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return fmt.Errorf("min/max ranges must agree")
	}
	for i := 0; i < len(min); i++ {
		if min[i] > max[i] {
			return fmt.Errorf("min should be less than max but min = %d and max = %d", min[i], max[i])
		}
	}
	return nil
}
