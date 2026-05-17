// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// XYPointField is an indexed 2-dimensional Cartesian point with x and y
// encoded as Lucene-compatible sortable int32 dimensions.
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.XYPointField.
//
// Static query factories (NewBoxQuery / NewDistanceQuery / NewPolygonQuery
// / NewGeometryQuery) deferred — backlog #2697.
type XYPointField struct {
	*Field
}

var (
	// XYPointFieldType is the FieldType for an XYPointField (dim=2, bytes=4).
	XYPointFieldType *FieldType

	// XYPointFieldTYPE is the Lucene-canonical alias.
	XYPointFieldTYPE *FieldType
)

func init() {
	XYPointFieldType = NewFieldType()
	XYPointFieldType.SetIndexed(true)
	XYPointFieldType.SetDimensions(2, 4)
	XYPointFieldType.Freeze()
	XYPointFieldTYPE = XYPointFieldType
}

// NewXYPointField creates a new XYPointField at (x, y).
func NewXYPointField(name string, x, y float32) (*XYPointField, error) {
	encoded, err := EncodeXY(x, y)
	if err != nil {
		return nil, err
	}
	field, err := NewField(name, encoded, XYPointFieldType)
	if err != nil {
		return nil, err
	}
	return &XYPointField{Field: field}, nil
}

// EncodeXY packs (x, y) into the 8-byte XY wire format: 4-byte
// sortable-bytes encoded x followed by 4-byte sortable-bytes encoded y.
func EncodeXY(x, y float32) ([]byte, error) {
	if _, err := geo.XYCheckVal(x); err != nil {
		return nil, fmt.Errorf("invalid x: %w", err)
	}
	if _, err := geo.XYCheckVal(y); err != nil {
		return nil, fmt.Errorf("invalid y: %w", err)
	}
	out := make([]byte, 8)
	util.IntToSortableBytes(geo.XYEncode(x), out, 0)
	util.IntToSortableBytes(geo.XYEncode(y), out, 4)
	return out, nil
}

// DecodeXY unpacks an 8-byte XYPointField encoding back to (x, y).
func DecodeXY(encoded []byte) (float32, float32, error) {
	if len(encoded) != 8 {
		return 0, 0, fmt.Errorf("XYPointField encoding must be 8 bytes; got %d", len(encoded))
	}
	x := geo.XYDecode(util.SortableBytesToInt(encoded, 0))
	y := geo.XYDecode(util.SortableBytesToInt(encoded, 4))
	return x, y, nil
}
