// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "strconv"

// IntField is a field for indexing int values.
type IntField struct {
	*Field
}

// NewIntField creates a new IntField.
func NewIntField(name string, value int, store bool) (*IntField, error) {
	ft := NewFieldType()
	ft.SetStored(store)
	ft.SetIndexed(true)
	ft.Freeze()

	field, err := NewField(name, strconv.Itoa(value), ft)
	if err != nil {
		return nil, err
	}

	return &IntField{Field: field}, nil
}

// IntPoint is an indexed int point field for range queries.
type IntPoint struct {
	*Field
}

// NewIntPoint creates a new IntPoint.
func NewIntPoint(name string, value int) (*IntPoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.Freeze()

	// Encode int as bytes for BKD tree (4 bytes, big-endian)
	encoded := encodeInt32(value)
	field, err := NewField(name, encoded, ft)
	if err != nil {
		return nil, err
	}

	return &IntPoint{Field: field}, nil
}

// encodeInt32 encodes an int to a 4-byte representation.
func encodeInt32(v int) []byte {
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

// decodeInt32 decodes a 4-byte representation back to int.
func decodeInt32(buf []byte) int {
	if len(buf) < 4 {
		return 0
	}
	x := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	x ^= 0x80000000 // Flip sign bit back
	return int(int32(x))
}
