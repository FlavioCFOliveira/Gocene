// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FloatField is a field for indexing float32 values.
type FloatField struct {
	*Field
}

// NewFloatField creates a new FloatField.
func NewFloatField(name string, value float32, store bool) (*FloatField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatFloat(float64(value), 'f', -1, 32), ft)
	if err != nil {
		return nil, err
	}

	return &FloatField{Field: field}, nil
}

// FloatValue returns the float32 value.
func (f *FloatField) FloatValue() float32 {
	val, _ := strconv.ParseFloat(f.StringValue(), 32)
	return float32(val)
}

// FloatPoint is an indexed float32 point field for range queries.
type FloatPoint struct {
	*Field
}

// NewFloatPoint creates a new FloatPoint.
func NewFloatPoint(name string, value float32) (*FloatPoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	// Encode float32 as sortable int32 for BKD tree
	encoded := encodeFloat32(value)
	field, err := NewField(name, encoded, ft)
	if err != nil {
		return nil, err
	}

	return &FloatPoint{Field: field}, nil
}

// encodeFloat32 encodes a float32 to a sortable byte representation.
// Uses IEEE 754 float32 bits with sign flip for correct ordering.
func encodeFloat32(f float32) []byte {
	bits := math.Float32bits(f)
	// Flip sign bit for correct ordering (negative < positive)
	if bits&0x80000000 != 0 {
		bits = ^bits // Flip all bits for negative
	} else {
		bits |= 0x80000000 // Set sign bit for positive
	}

	buf := make([]byte, 4)
	buf[0] = byte(bits >> 24)
	buf[1] = byte(bits >> 16)
	buf[2] = byte(bits >> 8)
	buf[3] = byte(bits)
	return buf
}

// decodeFloat32 decodes a byte representation back to float32.
func decodeFloat32(buf []byte) float32 {
	if len(buf) < 4 {
		return 0
	}
	bits := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])

	// Reverse the encoding
	if bits&0x80000000 != 0 {
		bits &= 0x7FFFFFFF // Clear sign bit
	} else {
		bits = ^bits // Flip all bits
	}

	return math.Float32frombits(bits)
}
