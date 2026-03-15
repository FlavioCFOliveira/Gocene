// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// BinaryPoint is an indexed binary point field for range queries.
// This is used for multi-dimensional point data in BKD trees.
type BinaryPoint struct {
	*Field
}

// NewBinaryPoint creates a new BinaryPoint with a single value.
func NewBinaryPoint(name string, value []byte) (*BinaryPoint, error) {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.Freeze()

	field, err := NewField(name, string(value), ft)
	if err != nil {
		return nil, err
	}

	return &BinaryPoint{Field: field}, nil
}

// NewBinaryPointMulti creates a new BinaryPoint with multiple dimensions.
// The values are concatenated into a single packed byte array.
func NewBinaryPointMulti(name string, values [][]byte) *Field {
	// Flatten multi-dimensional values
	totalLen := 0
	for _, v := range values {
		totalLen += len(v)
	}
	packed := make([]byte, 0, totalLen)
	for _, v := range values {
		packed = append(packed, v...)
	}

	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.Freeze()

	field, _ := NewField(name, string(packed), ft)
	return field
}

// Value returns the binary value of this point.
func (bp *BinaryPoint) Value() []byte {
	return bp.Field.BinaryValue()
}
