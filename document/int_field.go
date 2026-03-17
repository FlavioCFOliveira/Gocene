// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// IntField is a field for indexing int values.
type IntField struct {
	*Field
}

// NewIntField creates a new IntField.
func NewIntField(name string, value int, store bool) (*IntField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.Itoa(value), ft)
	if err != nil {
		return nil, err
	}

	return &IntField{Field: field}, nil
}

// encodeInt32Legacy encodes an int to a 4-byte representation for legacy IntField.
func encodeInt32Legacy(v int) []byte {
	buf := make([]byte, 4)
	// Flip sign bit for correct ordering
	x := uint32(v)
	x ^= 0x80000000 // Flip sign bit
	buf[0] = byte(x >> 24)
	buf[1] = byte(x >> 16)
	buf[2] = byte(x >> 8)
	buf[3] = byte(x)
	return buf
}

// decodeInt32Legacy decodes a 4-byte representation back to int for legacy IntField.
func decodeInt32Legacy(buf []byte) int {
	if len(buf) < 4 {
		return 0
	}
	x := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	x ^= 0x80000000 // Flip sign bit back
	return int(int32(x))
}

// IntPoint is an indexed int32 point field for range queries using the Point API.
type IntPoint struct {
	Point
}

// NewIntPoint creates a new IntPoint with a single value.
func NewIntPoint(name string, value int32) *IntPoint {
	return NewIntPoints(name, value)
}

// NewIntPoints creates a new IntPoint with multiple values.
func NewIntPoints(name string, values ...int32) *IntPoint {
	if len(values) == 0 {
		return nil
	}

	encoded := PackInts(values)
	ft := PointFieldType()
	ft.DimensionNumBytes = 4

	point, _ := NewPoint(name, ft, encoded, 1, 4)
	return &IntPoint{Point: *point}
}

// IntValue returns the first int value.
func (ip *IntPoint) IntValue() int32 {
	values := ip.IntValues()
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

// IntValues returns all int values.
func (ip *IntPoint) IntValues() []int32 {
	return UnpackInts(ip.PointValues())
}
