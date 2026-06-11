// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonShapeQuery is a data carrier for the parameters of a
// LatLonShape spatial query. It mirrors the package-private class
// org.apache.lucene.document.LatLonShapeQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the spatial relation and the geometry
// operands that the search-layer implementation consumes.
//
// LatLonShapeQuery validates that line geometries are not used with
// the WITHIN relation (Lucene does not support that combination).
type LatLonShapeQuery struct {
	*SpatialQuery
}

// NewLatLonShapeQuery constructs a LatLonShapeQuery data carrier.
// field must be non-empty, queryRelation must be provided, and
// geometries must be non-empty LatLonGeometry values.
//
// Returns an error if any geometry is a Line and queryRelation is
// WITHIN — Lucene does not support that combination.
func NewLatLonShapeQuery(field string, queryRelation QueryRelation, geometries ...geo.LatLonGeometry) (*LatLonShapeQuery, error) {
	if queryRelation == QueryRelationWithin {
		for _, g := range geometries {
			if _, ok := g.(geo.Line); ok {
				return nil, fmt.Errorf("LatLonShapeQuery does not support WITHIN queries with line geometries")
			}
		}
	}
	parent, err := NewSpatialQuery(field, queryRelation, geometries...)
	if err != nil {
		return nil, err
	}
	return &LatLonShapeQuery{SpatialQuery: parent}, nil
}

// String returns a human-readable representation.
func (q *LatLonShapeQuery) String() string {
	return fmt.Sprintf("LatLonShapeQuery(field=%s, relation=%s, geometries=%d)", q.Field(), q.QueryRelation(), len(q.Geometries()))
}

// Equals reports whether two LatLonShapeQuery carriers are equal.
func (q *LatLonShapeQuery) Equals(other *LatLonShapeQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.SpatialQuery.Equals(other.SpatialQuery)
}
