// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// This file ports the Range*DocValuesField family from Lucene 10.4.0:
// IntRangeDocValuesField, LongRangeDocValuesField, FloatRangeDocValuesField,
// DoubleRangeDocValuesField. Each is a BinaryDocValuesField wrapper whose
// payload is the Lucene-encoded packed range bytes (see EncodeXxxRangeLucene
// in range_fields_lucene.go).
//
// NewSlowIntersectsQuery factories are deferred — depend on
// search.RangeFieldQuery (task 249). See backlog #2695.

// IntRangeDocValuesField stores an N-dimensional int range as binary
// doc-values.
type IntRangeDocValuesField struct {
	*BinaryDocValuesField
	min []int32
	max []int32
}

// NewIntRangeDocValuesField creates a new IntRangeDocValuesField.
// min and max must have the same length (1..4) and min[i] <= max[i].
func NewIntRangeDocValuesField(name string, min, max []int32) (*IntRangeDocValuesField, error) {
	if err := checkRangeDocValuesArgs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeIntRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, encoded)
	if err != nil {
		return nil, err
	}
	dupMin := make([]int32, len(min))
	dupMax := make([]int32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &IntRangeDocValuesField{BinaryDocValuesField: b, min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *IntRangeDocValuesField) GetMin(dim int) int32 {
	mustDim(dim, len(f.min))
	return f.min[dim]
}

// GetMax returns the maximum value for the given dimension.
func (f *IntRangeDocValuesField) GetMax(dim int) int32 {
	mustDim(dim, len(f.max))
	return f.max[dim]
}

// LongRangeDocValuesField stores an N-dimensional long range.
type LongRangeDocValuesField struct {
	*BinaryDocValuesField
	min []int64
	max []int64
}

// NewLongRangeDocValuesField creates a new LongRangeDocValuesField.
func NewLongRangeDocValuesField(name string, min, max []int64) (*LongRangeDocValuesField, error) {
	if err := checkRangeDocValuesArgs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeLongRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, encoded)
	if err != nil {
		return nil, err
	}
	dupMin := make([]int64, len(min))
	dupMax := make([]int64, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &LongRangeDocValuesField{BinaryDocValuesField: b, min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *LongRangeDocValuesField) GetMin(dim int) int64 {
	mustDim(dim, len(f.min))
	return f.min[dim]
}

// GetMax returns the maximum value for the given dimension.
func (f *LongRangeDocValuesField) GetMax(dim int) int64 {
	mustDim(dim, len(f.max))
	return f.max[dim]
}

// FloatRangeDocValuesField stores an N-dimensional float range.
type FloatRangeDocValuesField struct {
	*BinaryDocValuesField
	min []float32
	max []float32
}

// NewFloatRangeDocValuesField creates a new FloatRangeDocValuesField.
func NewFloatRangeDocValuesField(name string, min, max []float32) (*FloatRangeDocValuesField, error) {
	if err := checkRangeDocValuesArgs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeFloatRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, encoded)
	if err != nil {
		return nil, err
	}
	dupMin := make([]float32, len(min))
	dupMax := make([]float32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &FloatRangeDocValuesField{BinaryDocValuesField: b, min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *FloatRangeDocValuesField) GetMin(dim int) float32 {
	mustDim(dim, len(f.min))
	return f.min[dim]
}

// GetMax returns the maximum value for the given dimension.
func (f *FloatRangeDocValuesField) GetMax(dim int) float32 {
	mustDim(dim, len(f.max))
	return f.max[dim]
}

// DoubleRangeDocValuesField stores an N-dimensional double range.
type DoubleRangeDocValuesField struct {
	*BinaryDocValuesField
	min []float64
	max []float64
}

// NewDoubleRangeDocValuesField creates a new DoubleRangeDocValuesField.
func NewDoubleRangeDocValuesField(name string, min, max []float64) (*DoubleRangeDocValuesField, error) {
	if err := checkRangeDocValuesArgs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeDoubleRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, encoded)
	if err != nil {
		return nil, err
	}
	dupMin := make([]float64, len(min))
	dupMax := make([]float64, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &DoubleRangeDocValuesField{BinaryDocValuesField: b, min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (f *DoubleRangeDocValuesField) GetMin(dim int) float64 {
	mustDim(dim, len(f.min))
	return f.min[dim]
}

// GetMax returns the maximum value for the given dimension.
func (f *DoubleRangeDocValuesField) GetMax(dim int) float64 {
	mustDim(dim, len(f.max))
	return f.max[dim]
}

func checkRangeDocValuesArgs(nMin, nMax int) error {
	if nMin == 0 || nMax == 0 {
		return fmt.Errorf("range doc-values field requires at least one dimension")
	}
	if nMin > 4 {
		return fmt.Errorf("range doc-values field supports at most 4 dimensions; got %d", nMin)
	}
	if nMin != nMax {
		return fmt.Errorf("min/max dimension count mismatch: %d vs %d", nMin, nMax)
	}
	return nil
}

func mustDim(dim, max int) {
	if dim < 0 || dim >= max {
		panic(fmt.Sprintf("dimension %d out of range [0, %d)", dim, max))
	}
}
