// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// BinaryPoint is an indexed binary point field for range queries.
// This is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.BinaryPoint, which underpins multi-dimensional
// binary point data in BKD trees.
//
// Divergences from Java:
//   - Lucene's variadic byte[]... constructor maps to NewBinaryPointMulti
//     (which already existed). The single-value NewBinaryPoint(name, []byte)
//     was pre-shipped in Gocene and now sets pointDimensionCount/numBytes
//     correctly from the provided value length.
//   - Static query factory methods (newExactQuery, newRangeQuery,
//     newSetQuery in Java) live in package search to avoid the
//     document/ -> search/ cycle Gocene's package layout would otherwise
//     close (search/ already imports document/ for FieldType). Callers
//     write search.NewBinaryPointExactQuery / NewBinaryPointRangeQuery /
//     NewBinaryPointMultiDimRangeQuery / NewBinaryPointSetQuery instead
//     of the Java BinaryPoint.* static methods. See
//     search/binary_point_queries.go for the rationale and the wire-format
//     contract — the helpers are thin wrappers over the existing
//     PointRangeQuery and PointInSetQuery types and do not introduce any
//     new on-disk layout.
type BinaryPoint struct {
	*Field
}

// NewBinaryPoint creates a new BinaryPoint with a single 1-dimensional
// binary value. The dimension count and per-dimension byte width are
// derived from the value itself (dimensionCount=1, numBytes=len(value)).
func NewBinaryPoint(name string, value []byte) (*BinaryPoint, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("BinaryPoint value cannot be empty")
	}
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(1, len(value))
	ft.Freeze()

	field, err := NewField(name, string(value), ft)
	if err != nil {
		return nil, err
	}
	return &BinaryPoint{Field: field}, nil
}

// NewBinaryPointMulti creates a new BinaryPoint with multiple dimensions.
// The values are concatenated into a single packed byte array.
// All dimensions must have the same byte length, matching Lucene's
// IllegalArgumentException behaviour.
func NewBinaryPointMulti(name string, values [][]byte) *Field {
	if len(values) == 0 {
		panic("BinaryPoint requires at least one dimension value")
	}
	dimNumBytes := len(values[0])
	for i, v := range values {
		if len(v) != dimNumBytes {
			panic(fmt.Sprintf("dimension %d has length %d, expected %d (all dimensions must share the same length)", i, len(v), dimNumBytes))
		}
	}

	totalLen := dimNumBytes * len(values)
	packed := make([]byte, 0, totalLen)
	for _, v := range values {
		packed = append(packed, v...)
	}

	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(len(values), dimNumBytes)
	ft.Freeze()

	field, err := NewField(name, string(packed), ft)
	if err != nil {
		panic(err)
	}
	return field
}

// NewBinaryPointPacked is the expert API mirroring Lucene's
// BinaryPoint(String, byte[] packedPoint, IndexableFieldType type).
// It validates that packedPoint length equals
// pointDimensionCount * pointNumBytes.
func NewBinaryPointPacked(name string, packedPoint []byte, ft *FieldType) (*BinaryPoint, error) {
	if ft == nil {
		return nil, fmt.Errorf("FieldType cannot be nil")
	}
	expect := ft.PointDimensionCount() * ft.PointNumBytes()
	if expect == 0 {
		return nil, fmt.Errorf("FieldType does not declare any point dimensions")
	}
	if len(packedPoint) != expect {
		return nil, fmt.Errorf("packedPoint length %d != pointDimensionCount * pointNumBytes (%d)", len(packedPoint), expect)
	}
	field, err := NewField(name, string(packedPoint), ft)
	if err != nil {
		return nil, err
	}
	return &BinaryPoint{Field: field}, nil
}

// Value returns the binary value of this point.
func (bp *BinaryPoint) Value() []byte {
	return bp.Field.BinaryValue()
}
