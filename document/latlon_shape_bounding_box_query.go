// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonShapeBoundingBoxQuery is a data carrier for the parameters of a
// LatLonShape bounding-box query. It mirrors the package-private class
// org.apache.lucene.document.LatLonShapeBoundingBoxQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the spatial relation and the bounding
// rectangle that the search-layer implementation consumes.
type LatLonShapeBoundingBoxQuery struct {
	*SpatialQuery
	rectangle geo.Rectangle
}

// NewLatLonShapeBoundingBoxQuery constructs a LatLonShapeBoundingBoxQuery
// data carrier for the given field, spatial relation and bounding box.
//
// The rectangle is encoded and stored; the query matches indexed shapes
// that satisfy the requested relation with the bounding box.
func NewLatLonShapeBoundingBoxQuery(field string, queryRelation QueryRelation, rectangle geo.Rectangle) (*LatLonShapeBoundingBoxQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	parent, err := NewSpatialQuery(field, queryRelation, rectangle)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeBoundingBoxQuery{
		SpatialQuery: parent,
		rectangle:    rectangle,
	}, nil
}

// Rectangle returns the bounding rectangle used by this query.
func (q *LatLonShapeBoundingBoxQuery) Rectangle() geo.Rectangle { return q.rectangle }

// String returns a human-readable representation.
func (q *LatLonShapeBoundingBoxQuery) String() string {
	return fmt.Sprintf("LatLonShapeBoundingBoxQuery(field=%s, relation=%s, rectangle=%s)", q.Field(), q.QueryRelation(), q.rectangle.String())
}

// Equals reports whether two LatLonShapeBoundingBoxQuery carriers are equal.
func (q *LatLonShapeBoundingBoxQuery) Equals(other *LatLonShapeBoundingBoxQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.SpatialQuery.Equals(other.SpatialQuery) && q.rectangle.Equals(other.rectangle)
}
