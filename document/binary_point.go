// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"sort"
)

// BinaryPoint is an indexed binary field for fast range filters. If you also need to store the value, you should
// add a separate StoredField instance.
//
// Finding all documents within an N-dimensional shape or range at search time is efficient.
// Multiple values for the same field in one document is allowed.
//
// This is the Go port of Lucene's org.apache.lucene.document.BinaryPoint.
type BinaryPoint struct {
	*Field
}

func getType(point [][]byte) *FieldType {
	if point == nil {
		panic("point must not be null")
	}
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}
	bytesPerDim := -1
	for i := 0; i < len(point); i++ {
		oneDim := point[i]
		if oneDim == nil {
			panic("point must not have null values")
		}
		if len(oneDim) == 0 {
			panic("point must not have 0-length values")
		}
		if bytesPerDim == -1 {
			bytesPerDim = len(oneDim)
		} else if bytesPerDim != len(oneDim) {
			panic(fmt.Sprintf("all dimensions must have same bytes length; got %d and %d", bytesPerDim, len(oneDim)))
		}
	}
	return getTypeFixed(len(point), bytesPerDim)
}

func getTypeFixed(numDims, bytesPerDim int) *FieldType {
	ft := NewFieldType()
	ft.SetDimensions(numDims, bytesPerDim)
	ft.Freeze()
	return ft
}

func pack(point ...[]byte) []byte {
	if point == nil {
		panic("point must not be null")
	}
	if len(point) == 0 {
		panic("point must not be 0 dimensions")
	}
	if len(point) == 1 {
		return point[0]
	}
	bytesPerDim := -1
	for _, dim := range point {
		if dim == nil {
			panic("point must not have null values")
		}
		if bytesPerDim == -1 {
			if len(dim) == 0 {
				panic("point must not have 0-length values")
			}
			bytesPerDim = len(dim)
		} else if len(dim) != bytesPerDim {
			panic(fmt.Sprintf("all dimensions must have same bytes length; got %d and %d", bytesPerDim, len(dim)))
		}
	}
	packed := make([]byte, bytesPerDim*len(point))
	for i := 0; i < len(point); i++ {
		copy(packed[i*bytesPerDim:], point[i])
	}
	return packed
}

// NewBinaryPoint creates a new BinaryPoint, indexing the provided N-dimensional binary point.
func NewBinaryPoint(name string, point ...[]byte) *BinaryPoint {
	packed := pack(point...)
	ft := getType(point)
	f, _ := NewField(name, packed, ft)
	return &BinaryPoint{Field: f}
}

// NewBinaryPointExpert creates a new BinaryPoint using a packed point and a field type.
func NewBinaryPointExpert(name string, packedPoint []byte, ft *FieldType) *BinaryPoint {
	if len(packedPoint) != ft.PointDimensionCount()*ft.PointNumBytes() {
		panic(fmt.Sprintf("packedPoint is length=%d but type.pointDimensionCount()=%d and type.pointNumBytes()=%d",
			len(packedPoint), ft.PointDimensionCount(), ft.PointNumBytes()))
	}
	f, _ := NewField(name, packedPoint, ft)
	return &BinaryPoint{Field: f}
}
