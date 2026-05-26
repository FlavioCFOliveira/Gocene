// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LatLonPoint is an indexed 2-dimensional geographic point with latitude
// and longitude encoded as Lucene-compatible sortable int32 dimensions
// (4 bytes each).
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.LatLonPoint.
//
// Static query factories are available in the search package to avoid the
// document→search import cycle (search imports document for field types):
//
//   - search.NewLatLonPointBoxQuery    — bounding box (Sprint 116)
//   - search.NewLatLonPointDistanceQuery  — disk/radius (Sprint 116)
//   - search.NewLatLonPointQuery       — polygon / geometry (Sprint 55)
//   - search.NewLatLonPointDistanceFeatureQuery — score-by-distance (Sprint 116)
//   - document.Nearest                 — KNN nearest-neighbour (Sprint 116)
//
// The NewGeometryQuery multi-shape factory that routes through
// LatLonPointQuery is tracked as backlog #2697 (requires the
// ShapeDocValuesQuery port, GOC-4532+).
type LatLonPoint struct {
	*Field
}

var (
	// LatLonPointType is the FieldType for a LatLonPoint:
	// dimensionCount=2, numBytes=4. Mirrors Lucene's static TYPE.
	LatLonPointType *FieldType

	// LatLonPointTYPE is the Lucene-canonical alias.
	LatLonPointTYPE *FieldType
)

func init() {
	LatLonPointType = NewFieldType()
	LatLonPointType.SetIndexed(true)
	LatLonPointType.SetDimensions(2, 4)
	LatLonPointType.Freeze()
	LatLonPointTYPE = LatLonPointType
}

// NewLatLonPoint creates a new LatLonPoint with the given lat/lon (degrees).
func NewLatLonPoint(name string, latitude, longitude float64) (*LatLonPoint, error) {
	if err := validateLatLon(latitude, longitude); err != nil {
		return nil, err
	}
	encoded := EncodeLatLon(latitude, longitude)
	field, err := NewField(name, encoded, LatLonPointType)
	if err != nil {
		return nil, err
	}
	return &LatLonPoint{Field: field}, nil
}

// EncodeLatLon packs latitude and longitude into the 8-byte Lucene wire
// format: 4-byte sortable-bytes latitude followed by 4-byte sortable-bytes
// longitude.
func EncodeLatLon(latitude, longitude float64) []byte {
	out := make([]byte, 8)
	util.IntToSortableBytes(geo.EncodeLatitude(latitude), out, 0)
	util.IntToSortableBytes(geo.EncodeLongitude(longitude), out, 4)
	return out
}

// EncodeLatLonCeil packs latitude and longitude using ceiling rounding.
func EncodeLatLonCeil(latitude, longitude float64) []byte {
	out := make([]byte, 8)
	util.IntToSortableBytes(geo.EncodeLatitudeCeil(latitude), out, 0)
	util.IntToSortableBytes(geo.EncodeLongitudeCeil(longitude), out, 4)
	return out
}

// DecodeLatLon unpacks the 8-byte wire format back to (lat, lon) degrees.
func DecodeLatLon(encoded []byte) (float64, float64, error) {
	if len(encoded) != 8 {
		return 0, 0, fmt.Errorf("LatLonPoint encoding must be 8 bytes; got %d", len(encoded))
	}
	lat := geo.DecodeLatitude(util.SortableBytesToInt(encoded, 0))
	lon := geo.DecodeLongitude(util.SortableBytesToInt(encoded, 4))
	return lat, lon, nil
}

func validateLatLon(lat, lon float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude %v out of range [-90, 90]", lat)
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude %v out of range [-180, 180]", lon)
	}
	return nil
}
