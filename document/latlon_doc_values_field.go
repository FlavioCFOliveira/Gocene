// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// LatLonDocValuesField stores a geographic point as numeric doc-values.
// The encoded value is a 64-bit long: upper 32 bits = encoded latitude,
// lower 32 bits = encoded longitude.
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.LatLonDocValuesField.
type LatLonDocValuesField struct {
	*Field
}

var (
	// LatLonDocValuesFieldType is the FieldType for a LatLonDocValuesField.
	LatLonDocValuesFieldType *FieldType

	// LatLonDocValuesFieldTYPE is the Lucene-canonical alias.
	LatLonDocValuesFieldTYPE *FieldType
)

func init() {
	LatLonDocValuesFieldType = NewFieldType()
	LatLonDocValuesFieldType.SetDocValuesType(index.DocValuesTypeSortedNumeric)
	LatLonDocValuesFieldType.Freeze()
	LatLonDocValuesFieldTYPE = LatLonDocValuesFieldType
}

// NewLatLonDocValuesField creates a new LatLonDocValuesField.
func NewLatLonDocValuesField(name string, latitude, longitude float64) (*LatLonDocValuesField, error) {
	if err := validateLatLon(latitude, longitude); err != nil {
		return nil, err
	}
	value := EncodeLatLonAsLong(latitude, longitude)
	field, err := NewField(name, value, LatLonDocValuesFieldType)
	if err != nil {
		return nil, err
	}
	return &LatLonDocValuesField{Field: field}, nil
}

// EncodeLatLonAsLong combines latitude and longitude into a single int64:
// upper 32 bits = encoded latitude, lower 32 bits = encoded longitude.
// Mirrors Lucene's setLocationValue bit layout.
func EncodeLatLonAsLong(latitude, longitude float64) int64 {
	lat := int64(geo.EncodeLatitude(latitude)) & 0xFFFFFFFF
	lon := int64(geo.EncodeLongitude(longitude)) & 0xFFFFFFFF
	return (lat << 32) | lon
}

// DecodeLatLonFromLong reverses EncodeLatLonAsLong.
func DecodeLatLonFromLong(encoded int64) (float64, float64) {
	lat := int32(encoded >> 32)
	lon := int32(encoded)
	return geo.DecodeLatitude(lat), geo.DecodeLongitude(lon)
}
