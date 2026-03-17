// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Point is the base type for point fields.
// Point fields are indexed using a KD-tree (BKD tree) structure for efficient
// multi-dimensional range and set queries.
//
// This is the Go port of Lucene's org.apache.lucene.document.Point.
type Point struct {
	Field
	numDimensions int
	bytesPerDim   int
}

// NewPoint creates a new Point field with the given name, type, and values.
// numDimensions specifies how many dimensions each point has (usually 1 for single-value).
// bytesPerDim specifies the number of bytes per dimension.
func NewPoint(name string, ft *FieldType, values []byte, numDimensions, bytesPerDim int) (*Point, error) {
	if name == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if ft == nil {
		return nil, fmt.Errorf("field type cannot be nil")
	}
	if err := ft.Validate(); err != nil {
		return nil, err
	}

	p := &Point{
		Field: Field{
			name: name,
			ft:   ft,
		},
		numDimensions: numDimensions,
		bytesPerDim:   bytesPerDim,
	}
	p.value = binaryValue(values)

	return p, nil
}

// NumDimensions returns the number of dimensions per point value.
func (p *Point) NumDimensions() int {
	return p.numDimensions
}

// BytesPerDimension returns the number of bytes used to encode each dimension.
func (p *Point) BytesPerDimension() int {
	return p.bytesPerDim
}

// PointValues returns the encoded point values.
func (p *Point) PointValues() []byte {
	if p.value == nil {
		return nil
	}
	return p.value.Binary()
}

// PackInt packs a single int value into a byte slice.
// Uses big-endian encoding for Lucene compatibility.
func PackInt(value int32) []byte {
	packed := make([]byte, 4)
	binary.BigEndian.PutUint32(packed, uint32(value))
	return packed
}

// UnpackInt unpacks a byte slice into an int value.
// Uses big-endian decoding for Lucene compatibility.
func UnpackInt(packed []byte) int32 {
	if len(packed) < 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(packed))
}

// PackLong packs a single long value into a byte slice.
// Uses big-endian encoding for Lucene compatibility.
func PackLong(value int64) []byte {
	packed := make([]byte, 8)
	binary.BigEndian.PutUint64(packed, uint64(value))
	return packed
}

// UnpackLong unpacks a byte slice into a long value.
// Uses big-endian decoding for Lucene compatibility.
func UnpackLong(packed []byte) int64 {
	if len(packed) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(packed))
}

// PackFloat packs a single float value into a byte slice.
// Uses IEEE 754 float32 bits with sign flip for correct ordering.
func PackFloat(value float32) []byte {
	packed := make([]byte, 4)
	bits := math.Float32bits(value)
	// Flip sign bit for correct ordering (negative < positive)
	if bits&0x80000000 != 0 {
		bits = ^bits // Flip all bits for negative
	} else {
		bits |= 0x80000000 // Set sign bit for positive
	}
	packed[0] = byte(bits >> 24)
	packed[1] = byte(bits >> 16)
	packed[2] = byte(bits >> 8)
	packed[3] = byte(bits)
	return packed
}

// UnpackFloat unpacks a byte slice into a float value.
func UnpackFloat(packed []byte) float32 {
	if len(packed) < 4 {
		return 0
	}
	bits := uint32(packed[0])<<24 | uint32(packed[1])<<16 | uint32(packed[2])<<8 | uint32(packed[3])
	// Reverse the encoding
	if bits&0x80000000 != 0 {
		bits &= 0x7FFFFFFF // Clear sign bit
	} else {
		bits = ^bits // Flip all bits
	}
	return math.Float32frombits(bits)
}

// PackDouble packs a single double value into a byte slice.
// Uses IEEE 754 float64 bits with sign flip for correct ordering.
func PackDouble(value float64) []byte {
	packed := make([]byte, 8)
	bits := math.Float64bits(value)
	// Flip sign bit for correct ordering (negative < positive)
	if bits&0x8000000000000000 != 0 {
		bits = ^bits // Flip all bits for negative
	} else {
		bits |= 0x8000000000000000 // Set sign bit for positive
	}
	packed[0] = byte(bits >> 56)
	packed[1] = byte(bits >> 48)
	packed[2] = byte(bits >> 40)
	packed[3] = byte(bits >> 32)
	packed[4] = byte(bits >> 24)
	packed[5] = byte(bits >> 16)
	packed[6] = byte(bits >> 8)
	packed[7] = byte(bits)
	return packed
}

// UnpackDouble unpacks a byte slice into a double value.
func UnpackDouble(packed []byte) float64 {
	if len(packed) < 8 {
		return 0
	}
	bits := uint64(packed[0])<<56 | uint64(packed[1])<<48 | uint64(packed[2])<<40 | uint64(packed[3])<<32 |
		uint64(packed[4])<<24 | uint64(packed[5])<<16 | uint64(packed[6])<<8 | uint64(packed[7])
	// Reverse the encoding
	if bits&0x8000000000000000 != 0 {
		bits &= 0x7FFFFFFFFFFFFFFF // Clear sign bit
	} else {
		bits = ^bits // Flip all bits
	}
	return math.Float64frombits(bits)
}

// PackInts packs multiple int values into a byte slice.
func PackInts(values []int32) []byte {
	result := make([]byte, len(values)*4)
	for i, v := range values {
		binary.BigEndian.PutUint32(result[i*4:], uint32(v))
	}
	return result
}

// UnpackInts unpacks a byte slice into int values.
func UnpackInts(packed []byte) []int32 {
	if len(packed)%4 != 0 {
		return nil
	}
	count := len(packed) / 4
	result := make([]int32, count)
	for i := 0; i < count; i++ {
		result[i] = int32(binary.BigEndian.Uint32(packed[i*4:]))
	}
	return result
}

// PackLongs packs multiple long values into a byte slice.
func PackLongs(values []int64) []byte {
	result := make([]byte, len(values)*8)
	for i, v := range values {
		binary.BigEndian.PutUint64(result[i*8:], uint64(v))
	}
	return result
}

// UnpackLongs unpacks a byte slice into long values.
func UnpackLongs(packed []byte) []int64 {
	if len(packed)%8 != 0 {
		return nil
	}
	count := len(packed) / 8
	result := make([]int64, count)
	for i := 0; i < count; i++ {
		result[i] = int64(binary.BigEndian.Uint64(packed[i*8:]))
	}
	return result
}

// PackFloats packs multiple float values into a byte slice.
func PackFloats(values []float32) []byte {
	result := make([]byte, 0, len(values)*4)
	for _, v := range values {
		result = append(result, PackFloat(v)...)
	}
	return result
}

// UnpackFloats unpacks a byte slice into float values.
func UnpackFloats(packed []byte) []float32 {
	if len(packed)%4 != 0 {
		return nil
	}
	count := len(packed) / 4
	result := make([]float32, count)
	for i := 0; i < count; i++ {
		result[i] = UnpackFloat(packed[i*4:])
	}
	return result
}

// PackDoubles packs multiple double values into a byte slice.
func PackDoubles(values []float64) []byte {
	result := make([]byte, 0, len(values)*8)
	for _, v := range values {
		result = append(result, PackDouble(v)...)
	}
	return result
}

// UnpackDoubles unpacks a byte slice into double values.
func UnpackDoubles(packed []byte) []float64 {
	if len(packed)%8 != 0 {
		return nil
	}
	count := len(packed) / 8
	result := make([]float64, count)
	for i := 0; i < count; i++ {
		result[i] = UnpackDouble(packed[i*8:])
	}
	return result
}

// PointFieldType returns a FieldType suitable for point fields.
// Point fields are indexed but not tokenized.
func PointFieldType() *FieldType {
	ft := NewFieldType()
	ft.Indexed = true
	ft.Stored = false
	ft.Tokenized = false
	ft.OmitNorms = true
	ft.IndexOptions = index.IndexOptionsDocs
	ft.DocValuesType = index.DocValuesTypeNone
	ft.DimensionCount = 1
	ft.DimensionNumBytes = 4
	return ft
}

// Ensure Point implements IndexableField
var _ IndexableField = (*Point)(nil)
