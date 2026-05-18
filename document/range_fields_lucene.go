// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file adds Lucene 10.4.0-compatible multi-dimensional Range field
// constructors. The pre-existing single-dim IntRange/LongRange/FloatRange/
// DoubleRange/BinaryRange types in range_field.go are preserved.
//
// Layout: each Range field packs N dimensions of [min, max] pairs. The
// FieldType dimensionCount is 2*N and the bytes-per-dimension matches the
// primitive width (Integer.BYTES=4 etc.). All numeric values use Lucene's
// sortable-bytes encoding (sign-flip), matching the JVM-produced byte
// streams exactly.
//
// Static query factories (NewIntersectsQuery / NewContainsQuery /
// NewWithinQuery / NewCrossesQuery) are deferred — they depend on
// search.RangeFieldQuery. See backlog #2695 for similar deferrals.

// rangeFieldType builds a FieldType for a Range field of N dimensions and
// numBytes per dimension. Each Range packs N pairs of [min, max], so the
// effective dimensionCount is 2*N.
func rangeFieldType(numDims, numBytes int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(2*numDims, numBytes)
	ft.Freeze()
	return ft
}

// IntRangeLucene is the Lucene 10.4.0 IntRange field — variadic dimensions.
type IntRangeLucene struct {
	*Field
	numDims int
	min     []int32
	max     []int32
}

// NewIntRangeLucene creates a new IntRangeLucene with N-dimensional
// [min, max] pairs. len(min) must equal len(max). Each min[i] must be
// <= max[i] (panics otherwise, matching Lucene's IllegalArgumentException).
func NewIntRangeLucene(name string, min, max []int32) (*IntRangeLucene, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeIntRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, rangeFieldType(len(min), 4))
	if err != nil {
		return nil, err
	}
	dupMin := make([]int32, len(min))
	dupMax := make([]int32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &IntRangeLucene{Field: field, numDims: len(min), min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (r *IntRangeLucene) GetMin(dim int) int32 { return r.min[dim] }

// GetMax returns the maximum value for the given dimension.
func (r *IntRangeLucene) GetMax(dim int) int32 { return r.max[dim] }

// EncodeIntRangeLucene packs N-dimensional min/max into Lucene's
// sortable-byte layout: all min values first, all max values second.
func EncodeIntRangeLucene(min, max []int32) ([]byte, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	n := len(min)
	out := make([]byte, 2*n*4)
	for i := 0; i < n; i++ {
		if min[i] > max[i] {
			return nil, fmt.Errorf("dim %d: min %d > max %d", i, min[i], max[i])
		}
		util.IntToSortableBytes(min[i], out, i*4)
		util.IntToSortableBytes(max[i], out, n*4+i*4)
	}
	return out, nil
}

// Deprecated: use [LongRange] / [NewLongRange] / [EncodeLongRange] (Lucene
// canonical names). Retained for backward compatibility (GOC-3219). The
// canonical implementation lives in long_range.go.
type LongRangeLucene = LongRange

// Deprecated: use [NewLongRange]. Retained for backward compatibility (GOC-3219).
var NewLongRangeLucene = NewLongRange

// Deprecated: use [EncodeLongRange]. Retained for backward compatibility (GOC-3219).
var EncodeLongRangeLucene = EncodeLongRange

// FloatRangeLucene is the Lucene 10.4.0 FloatRange field.
type FloatRangeLucene struct {
	*Field
	numDims int
	min     []float32
	max     []float32
}

// NewFloatRangeLucene creates a new FloatRangeLucene with N-dimensional ranges.
func NewFloatRangeLucene(name string, min, max []float32) (*FloatRangeLucene, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeFloatRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, rangeFieldType(len(min), 4))
	if err != nil {
		return nil, err
	}
	dupMin := make([]float32, len(min))
	dupMax := make([]float32, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &FloatRangeLucene{Field: field, numDims: len(min), min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (r *FloatRangeLucene) GetMin(dim int) float32 { return r.min[dim] }

// GetMax returns the maximum value for the given dimension.
func (r *FloatRangeLucene) GetMax(dim int) float32 { return r.max[dim] }

// EncodeFloatRangeLucene packs N-dimensional float ranges with Lucene
// sortable-bytes (FloatToSortableInt + IntToSortableBytes).
func EncodeFloatRangeLucene(min, max []float32) ([]byte, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	n := len(min)
	out := make([]byte, 2*n*4)
	for i := 0; i < n; i++ {
		if min[i] > max[i] {
			return nil, fmt.Errorf("dim %d: min %v > max %v", i, min[i], max[i])
		}
		util.IntToSortableBytes(util.FloatToSortableInt(min[i]), out, i*4)
		util.IntToSortableBytes(util.FloatToSortableInt(max[i]), out, n*4+i*4)
	}
	return out, nil
}

// DoubleRangeLucene is the Lucene 10.4.0 DoubleRange field.
type DoubleRangeLucene struct {
	*Field
	numDims int
	min     []float64
	max     []float64
}

// NewDoubleRangeLucene creates a new DoubleRangeLucene with N-dimensional ranges.
func NewDoubleRangeLucene(name string, min, max []float64) (*DoubleRangeLucene, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	encoded, err := EncodeDoubleRangeLucene(min, max)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, rangeFieldType(len(min), 8))
	if err != nil {
		return nil, err
	}
	dupMin := make([]float64, len(min))
	dupMax := make([]float64, len(max))
	copy(dupMin, min)
	copy(dupMax, max)
	return &DoubleRangeLucene{Field: field, numDims: len(min), min: dupMin, max: dupMax}, nil
}

// GetMin returns the minimum value for the given dimension.
func (r *DoubleRangeLucene) GetMin(dim int) float64 { return r.min[dim] }

// GetMax returns the maximum value for the given dimension.
func (r *DoubleRangeLucene) GetMax(dim int) float64 { return r.max[dim] }

// EncodeDoubleRangeLucene packs N-dimensional double ranges with Lucene
// sortable-bytes (DoubleToSortableLong + LongToSortableBytes).
func EncodeDoubleRangeLucene(min, max []float64) ([]byte, error) {
	if err := validateRangePairs(len(min), len(max)); err != nil {
		return nil, err
	}
	n := len(min)
	out := make([]byte, 2*n*8)
	for i := 0; i < n; i++ {
		if min[i] > max[i] {
			return nil, fmt.Errorf("dim %d: min %v > max %v", i, min[i], max[i])
		}
		util.LongToSortableBytes(util.DoubleToSortableLong(min[i]), out, i*8)
		util.LongToSortableBytes(util.DoubleToSortableLong(max[i]), out, n*8+i*8)
	}
	return out, nil
}

func validateRangePairs(nMin, nMax int) error {
	if nMin == 0 || nMax == 0 {
		return fmt.Errorf("range field requires at least one dimension")
	}
	if nMin != nMax {
		return fmt.Errorf("range field min/max dimension count mismatch: %d vs %d", nMin, nMax)
	}
	return nil
}
