// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonDocValuesBoxQuery is a data carrier for a doc-values-backed
// lat/lon bounding-box query. It mirrors the package-private class
// org.apache.lucene.document.LatLonDocValuesBoxQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name and the bounding-box parameters
// (min/max latitude and longitude) that the search-layer implementation
// consumes. The coordinates are validated at construction and the
// encoded int32 forms are stored alongside the raw degree values.
type LatLonDocValuesBoxQuery struct {
	field          string
	minLatitude    float64
	maxLatitude    float64
	minLongitude   float64
	maxLongitude   float64
	crossesDateline bool
	minLatEncoded  int32
	maxLatEncoded  int32
	minLonEncoded  int32
	maxLonEncoded  int32
}

// NewLatLonDocValuesBoxQuery constructs a LatLonDocValuesBoxQuery data
// carrier. Bounds are validated via geo.CheckLatitude / CheckLongitude.
// When minLongitude > maxLongitude the query crosses the dateline.
func NewLatLonDocValuesBoxQuery(field string, minLatitude, maxLatitude, minLongitude, maxLongitude float64) (*LatLonDocValuesBoxQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if err := geo.CheckLatitude(minLatitude); err != nil {
		return nil, err
	}
	if err := geo.CheckLatitude(maxLatitude); err != nil {
		return nil, err
	}
	if err := geo.CheckLongitude(minLongitude); err != nil {
		return nil, err
	}
	if err := geo.CheckLongitude(maxLongitude); err != nil {
		return nil, err
	}
	crossesDateline := minLongitude > maxLongitude
	return &LatLonDocValuesBoxQuery{
		field:            field,
		minLatitude:      minLatitude,
		maxLatitude:      maxLatitude,
		minLongitude:     minLongitude,
		maxLongitude:     maxLongitude,
		crossesDateline:  crossesDateline,
		minLatEncoded:    geo.EncodeLatitudeCeil(minLatitude),
		maxLatEncoded:    geo.EncodeLatitude(maxLatitude),
		minLonEncoded:    geo.EncodeLongitudeCeil(minLongitude),
		maxLonEncoded:    geo.EncodeLongitude(maxLongitude),
	}, nil
}

// Field returns the target field name.
func (q *LatLonDocValuesBoxQuery) Field() string { return q.field }

// MinLatitude returns the minimum latitude in decimal degrees.
func (q *LatLonDocValuesBoxQuery) MinLatitude() float64 { return q.minLatitude }

// MaxLatitude returns the maximum latitude in decimal degrees.
func (q *LatLonDocValuesBoxQuery) MaxLatitude() float64 { return q.maxLatitude }

// MinLongitude returns the minimum longitude in decimal degrees.
func (q *LatLonDocValuesBoxQuery) MinLongitude() float64 { return q.minLongitude }

// MaxLongitude returns the maximum longitude in decimal degrees.
func (q *LatLonDocValuesBoxQuery) MaxLongitude() float64 { return q.maxLongitude }

// CrossesDateline reports whether the bounding box crosses the
// antimeridian (i.e. minLongitude > maxLongitude).
func (q *LatLonDocValuesBoxQuery) CrossesDateline() bool { return q.crossesDateline }

// MinLatEncoded returns the encoded (quantised int32) minimum latitude.
func (q *LatLonDocValuesBoxQuery) MinLatEncoded() int32 { return q.minLatEncoded }

// MaxLatEncoded returns the encoded (quantised int32) maximum latitude.
func (q *LatLonDocValuesBoxQuery) MaxLatEncoded() int32 { return q.maxLatEncoded }

// MinLonEncoded returns the encoded (quantised int32) minimum longitude.
func (q *LatLonDocValuesBoxQuery) MinLonEncoded() int32 { return q.minLonEncoded }

// MaxLonEncoded returns the encoded (quantised int32) maximum longitude.
func (q *LatLonDocValuesBoxQuery) MaxLonEncoded() int32 { return q.maxLonEncoded }

// String returns a human-readable representation.
func (q *LatLonDocValuesBoxQuery) String() string {
	return fmt.Sprintf("LatLonDocValuesBoxQuery(field=%s, minLat=%f, maxLat=%f, minLon=%f, maxLon=%f)",
		q.field, q.minLatitude, q.maxLatitude, q.minLongitude, q.maxLongitude)
}

// Equals reports whether two LatLonDocValuesBoxQuery carriers are equal.
func (q *LatLonDocValuesBoxQuery) Equals(other *LatLonDocValuesBoxQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.field == other.field &&
		q.crossesDateline == other.crossesDateline &&
		q.minLatEncoded == other.minLatEncoded &&
		q.maxLatEncoded == other.maxLatEncoded &&
		q.minLonEncoded == other.minLonEncoded &&
		q.maxLonEncoded == other.maxLonEncoded
}
