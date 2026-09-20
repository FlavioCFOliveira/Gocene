// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
)

// FloatRangeDocValuesField stores an N-dimensional float range. This is a single
// valued field per document due to being an extension of BinaryRangeDocValuesField.
//
// Go port of Lucene 10.5.0's org.apache.lucene.document.FloatRangeDocValuesField.
type FloatRangeDocValuesField struct {
	*BinaryRangeDocValuesField
	min []float32
	max []float32
}

// NewFloatRangeDocValuesField constructs a FloatRangeDocValuesField for the
// given field name and min/max range values.
func NewFloatRangeDocValuesField(field string, min, max []float32) (*FloatRangeDocValuesField, error) {
	if err := checkFloatRangeDocValuesArgs(min, max); err != nil {
		return nil, err
	}

	encoded, err := EncodeFloatRangeLucene(min, max)
	if err != nil {
		return nil, err
	}

	// Java: super(field, FloatRange.encode(min, max), min.length, FloatRange.BYTES)
	b, err := NewBinaryRangeDocValuesField(field, encoded, len(min), 4)
	if err != nil {
		return nil, err
	}

	dupMin := make([]float32, len(min))
	dupMax := make([]float32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)

	return &FloatRangeDocValuesField{
		BinaryRangeDocValuesField: b,
		min:                       dupMin,
		max:                       dupMax,
	}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *FloatRangeDocValuesField) GetMin(dimension int) float32 {
	if dimension > 4 || dimension >= len(f.min) {
		panic("Dimension out of valid range")
	}
	return f.min[dimension]
}

// GetMax returns the maximum value for the given dimension.
func (f *FloatRangeDocValuesField) GetMax(dimension int) float32 {
	if dimension > 4 || dimension >= len(f.max) {
		panic("Dimension out of valid range")
	}
	return f.max[dimension]
}

func checkFloatRangeDocValuesArgs(min, max []float32) error {
	if len(min) == 0 || len(max) == 0 {
		return fmt.Errorf("min/max range values cannot be null or empty")
	}
	if len(min) != len(max) {
		return fmt.Errorf("min/max ranges must agree")
	}
	for i := 0; i < len(min); i++ {
		if min[i] > max[i] {
			return fmt.Errorf("min should be less than max")
		}
	}
	return nil
}
