// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
)

// IntRange is a field for indexing integer ranges.
// It stores min and max values and supports range queries.
type IntRange struct {
	Field
	min int32
	max int32
}

// NewIntRange creates a new IntRange field.
func NewIntRange(name string, min, max int32) *IntRange {
	// Encode range as binary data
	encoded := make([]byte, 8)
	// Store min in first 4 bytes
	encoded[0] = byte(min >> 24)
	encoded[1] = byte(min >> 16)
	encoded[2] = byte(min >> 8)
	encoded[3] = byte(min)
	// Store max in next 4 bytes
	encoded[4] = byte(max >> 24)
	encoded[5] = byte(max >> 16)
	encoded[6] = byte(max >> 8)
	encoded[7] = byte(max)

	ft := PointFieldType()
	ft.DimensionNumBytes = 4

	field, _ := NewField(name, encoded, ft)
	return &IntRange{
		Field: *field,
		min:   min,
		max:   max,
	}
}

// Min returns the minimum value.
func (r *IntRange) Min() int32 {
	return r.min
}

// Max returns the maximum value.
func (r *IntRange) Max() int32 {
	return r.max
}

// String returns a string representation.
func (r *IntRange) String() string {
	return fmt.Sprintf("IntRange(name=%s, min=%d, max=%d)", r.name, r.min, r.max)
}

// NOTE: The canonical LongRange (Lucene 10.4.0-compatible, N-dimensional,
// sortable-bytes encoded) lives in long_range.go. The legacy single-dim
// stub formerly defined here was removed by GOC-3219.

// FloatRange is a field for indexing float ranges.
type FloatRange struct {
	Field
	min float32
	max float32
}

// NewFloatRange creates a new FloatRange field.
func NewFloatRange(name string, min, max float32) *FloatRange {
	// Encode using PackFloat
	encoded := make([]byte, 8)
	copy(encoded[0:4], PackFloat(min))
	copy(encoded[4:8], PackFloat(max))

	ft := PointFieldType()
	ft.DimensionNumBytes = 4

	field, _ := NewField(name, encoded, ft)
	return &FloatRange{
		Field: *field,
		min:   min,
		max:   max,
	}
}

// Min returns the minimum value.
func (r *FloatRange) Min() float32 {
	return r.min
}

// Max returns the maximum value.
func (r *FloatRange) Max() float32 {
	return r.max
}

// String returns a string representation.
func (r *FloatRange) String() string {
	return fmt.Sprintf("FloatRange(name=%s, min=%f, max=%f)", r.name, r.min, r.max)
}

// NOTE: The canonical DoubleRange (Lucene 10.4.0-compatible, N-dimensional,
// sortable-bytes encoded) lives in double_range.go. The legacy single-dim
// stub formerly defined here was removed by GOC-3222.

// BinaryRange is a field for indexing binary ranges.
type BinaryRange struct {
	Field
	min []byte
	max []byte
}

// NewBinaryRange creates a new BinaryRange field.
func NewBinaryRange(name string, min, max []byte) *BinaryRange {
	// Concatenate min and max
	encoded := make([]byte, len(min)+len(max))
	copy(encoded, min)
	copy(encoded[len(min):], max)

	ft := PointFieldType()
	ft.DimensionNumBytes = len(min)

	field, _ := NewField(name, encoded, ft)
	return &BinaryRange{
		Field: *field,
		min:   min,
		max:   max,
	}
}

// Min returns the minimum value.
func (r *BinaryRange) Min() []byte {
	return r.min
}

// Max returns the maximum value.
func (r *BinaryRange) Max() []byte {
	return r.max
}

// String returns a string representation.
func (r *BinaryRange) String() string {
	return fmt.Sprintf("BinaryRange(name=%s, min=%d bytes, max=%d bytes)", r.name, len(r.min), len(r.max))
}
