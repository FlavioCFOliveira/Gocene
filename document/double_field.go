// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DoubleField is a field for indexing float64 values.
type DoubleField struct {
	*Field
}

// NewDoubleField creates a new DoubleField.
func NewDoubleField(name string, value float64, store bool) (*DoubleField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	field, err := NewField(name, strconv.FormatFloat(value, 'f', -1, 64), ft)
	if err != nil {
		return nil, err
	}

	return &DoubleField{Field: field}, nil
}

// DoubleValue returns the float64 value.
func (f *DoubleField) DoubleValue() float64 {
	val, _ := strconv.ParseFloat(f.StringValue(), 64)
	return val
}

// DoublePoint is an indexed float64 point field for range queries.
type DoublePoint struct {
	*Field
}

// NewDoublePoint creates a new DoublePoint.
func NewDoublePoint(name string, value float64) (*DoublePoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()

	// Encode float64 as sortable int64 for BKD tree
	encoded := encodeFloat64(value)
	field, err := NewField(name, encoded, ft)
	if err != nil {
		return nil, err
	}

	return &DoublePoint{Field: field}, nil
}

// encodeFloat64 encodes a float64 to a sortable byte representation.
// Uses IEEE 754 float64 bits with sign flip for correct ordering.
func encodeFloat64(f float64) []byte {
	bits := math.Float64bits(f)
	// Flip sign bit for correct ordering (negative < positive)
	if bits&0x8000000000000000 != 0 {
		bits = ^bits // Flip all bits for negative
	} else {
		bits |= 0x8000000000000000 // Set sign bit for positive
	}

	buf := make([]byte, 8)
	buf[0] = byte(bits >> 56)
	buf[1] = byte(bits >> 48)
	buf[2] = byte(bits >> 40)
	buf[3] = byte(bits >> 32)
	buf[4] = byte(bits >> 24)
	buf[5] = byte(bits >> 16)
	buf[6] = byte(bits >> 8)
	buf[7] = byte(bits)
	return buf
}

// decodeFloat64 decodes a byte representation back to float64.
func decodeFloat64(buf []byte) float64 {
	if len(buf) < 8 {
		return 0
	}
	bits := uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
		uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])

	// Reverse the encoding
	if bits&0x8000000000000000 != 0 {
		bits &= 0x7FFFFFFFFFFFFFFF // Clear sign bit
	} else {
		bits = ^bits // Flip all bits
	}

	return math.Float64frombits(bits)
}
