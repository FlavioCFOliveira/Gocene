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

// LongRange is a field for indexing long ranges.
type LongRange struct {
	Field
	min int64
	max int64
}

// NewLongRange creates a new LongRange field.
func NewLongRange(name string, min, max int64) *LongRange {
	// Encode range as binary data
	encoded := make([]byte, 16)
	// Store min in first 8 bytes
	encoded[0] = byte(min >> 56)
	encoded[1] = byte(min >> 48)
	encoded[2] = byte(min >> 40)
	encoded[3] = byte(min >> 32)
	encoded[4] = byte(min >> 24)
	encoded[5] = byte(min >> 16)
	encoded[6] = byte(min >> 8)
	encoded[7] = byte(min)
	// Store max in next 8 bytes
	encoded[8] = byte(max >> 56)
	encoded[9] = byte(max >> 48)
	encoded[10] = byte(max >> 40)
	encoded[11] = byte(max >> 32)
	encoded[12] = byte(max >> 24)
	encoded[13] = byte(max >> 16)
	encoded[14] = byte(max >> 8)
	encoded[15] = byte(max)

	ft := PointFieldType()
	ft.DimensionNumBytes = 8

	field, _ := NewField(name, encoded, ft)
	return &LongRange{
		Field: *field,
		min:   min,
		max:   max,
	}
}

// Min returns the minimum value.
func (r *LongRange) Min() int64 {
	return r.min
}

// Max returns the maximum value.
func (r *LongRange) Max() int64 {
	return r.max
}

// String returns a string representation.
func (r *LongRange) String() string {
	return fmt.Sprintf("LongRange(name=%s, min=%d, max=%d)", r.name, r.min, r.max)
}

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

// DoubleRange is a field for indexing double ranges.
type DoubleRange struct {
	Field
	min float64
	max float64
}

// NewDoubleRange creates a new DoubleRange field.
func NewDoubleRange(name string, min, max float64) *DoubleRange {
	// Encode using PackDouble
	encoded := make([]byte, 16)
	copy(encoded[0:8], PackDouble(min))
	copy(encoded[8:16], PackDouble(max))

	ft := PointFieldType()
	ft.DimensionNumBytes = 8

	field, _ := NewField(name, encoded, ft)
	return &DoubleRange{
		Field: *field,
		min:   min,
		max:   max,
	}
}

// Min returns the minimum value.
func (r *DoubleRange) Min() float64 {
	return r.min
}

// Max returns the maximum value.
func (r *DoubleRange) Max() float64 {
	return r.max
}

// String returns a string representation.
func (r *DoubleRange) String() string {
	return fmt.Sprintf("DoubleRange(name=%s, min=%f, max=%f)", r.name, r.min, r.max)
}

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
