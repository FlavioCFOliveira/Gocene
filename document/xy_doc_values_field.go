// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// XYDocValuesField stores a Cartesian point as numeric doc-values: a
// 64-bit value with upper 32 bits = encoded x, lower 32 bits = encoded y.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.XYDocValuesField.
type XYDocValuesField struct {
	*Field
}

var (
	// XYDocValuesFieldType is the FieldType for an XYDocValuesField.
	XYDocValuesFieldType *FieldType

	// XYDocValuesFieldTYPE is the Lucene-canonical alias.
	XYDocValuesFieldTYPE *FieldType
)

func init() {
	XYDocValuesFieldType = NewFieldType()
	XYDocValuesFieldType.SetDocValuesType(index.DocValuesTypeSortedNumeric)
	XYDocValuesFieldType.Freeze()
	XYDocValuesFieldTYPE = XYDocValuesFieldType
}

// NewXYDocValuesField creates a new XYDocValuesField.
func NewXYDocValuesField(name string, x, y float32) (*XYDocValuesField, error) {
	if _, err := geo.XYCheckVal(x); err != nil {
		return nil, fmt.Errorf("invalid x: %w", err)
	}
	if _, err := geo.XYCheckVal(y); err != nil {
		return nil, fmt.Errorf("invalid y: %w", err)
	}
	encoded := EncodeXYAsLong(x, y)
	field, err := NewField(name, encoded, XYDocValuesFieldType)
	if err != nil {
		return nil, err
	}
	return &XYDocValuesField{Field: field}, nil
}

// EncodeXYAsLong packs (x, y) into a single int64: upper 32 bits = encoded
// x, lower 32 bits = encoded y. Matches Lucene's setLocationValue layout.
func EncodeXYAsLong(x, y float32) int64 {
	xi := int64(geo.XYEncode(x)) & 0xFFFFFFFF
	yi := int64(geo.XYEncode(y)) & 0xFFFFFFFF
	return (xi << 32) | yi
}

// DecodeXYFromLong reverses EncodeXYAsLong.
func DecodeXYFromLong(encoded int64) (float32, float32) {
	x := int32(encoded >> 32)
	y := int32(encoded)
	return geo.XYDecode(x), geo.XYDecode(y)
}
