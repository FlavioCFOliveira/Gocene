// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// SpatialQuery is the base data carrier for every spatial query in the
// shape family: LatLonShapeQuery, LatLonShapeBoundingBoxQuery and the
// doc-values variants all share the same parameter surface.
//
// It mirrors the package-private abstract class
// org.apache.lucene.document.SpatialQuery (Lucene 10.4.0), but in its
// role as a data holder only — the actual CreateWeight / ScorerSupplier
// logic lives in the search package.
//
// Gocene splits the Java pattern: the document package carries the
// query parameters (field name, spatial relation, geometry list) and
// the search package implements the Query / Weight / Scorer machinery
// using those same parameters. This keeps the document → search import
// direction unidirectional.
type SpatialQuery struct {
	field          string
	queryRelation  QueryRelation
	geometries     []geo.LatLonGeometry
}

// NewSpatialQuery constructs a SpatialQuery data carrier.
// field must be non-empty, queryRelation must be provided, and
// geometries must be non-empty.
func NewSpatialQuery(field string, queryRelation QueryRelation, geometries ...geo.LatLonGeometry) (*SpatialQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	if len(geometries) == 0 {
		return nil, fmt.Errorf("geometries must not be empty")
	}
	for i, g := range geometries {
		if g == nil {
			return nil, fmt.Errorf("geometries[%d] must not be null", i)
		}
	}
	dup := make([]geo.LatLonGeometry, len(geometries))
	copy(dup, geometries)
	return &SpatialQuery{
		field:         field,
		queryRelation: queryRelation,
		geometries:    dup,
	}, nil
}

// Field returns the target field name.
func (q *SpatialQuery) Field() string { return q.field }

// QueryRelation returns the spatial relation for this query.
func (q *SpatialQuery) QueryRelation() QueryRelation { return q.queryRelation }

// Geometries returns a defensive copy of the geometry list.
func (q *SpatialQuery) Geometries() []geo.LatLonGeometry {
	out := make([]geo.LatLonGeometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// String returns a human-readable representation.
func (q *SpatialQuery) String() string {
	return fmt.Sprintf("SpatialQuery(field=%s, relation=%s, geometries=%d)", q.field, q.queryRelation, len(q.geometries))
}

// Equals reports whether two SpatialQuery carriers have the same field,
// query relation and geometry list.
func (q *SpatialQuery) Equals(other *SpatialQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	if q.field != other.field || q.queryRelation != other.queryRelation {
		return false
	}
	if len(q.geometries) != len(other.geometries) {
		return false
	}
	for i := range q.geometries {
		// LatLonGeometry is a sealed interface so we compare by
		// string representation for now — concrete geometry types
		// define their own Equals but the interface does not
		// expose it.
		if fmt.Sprintf("%v", q.geometries[i]) != fmt.Sprintf("%v", other.geometries[i]) {
			return false
		}
	}
	return true
}

// TransposeRelation mirrors the Java helper: CELL_INSIDE_QUERY becomes
// CELL_OUTSIDE_QUERY, CELL_OUTSIDE_QUERY becomes CELL_INSIDE_QUERY,
// and CELL_CROSSES_QUERY stays unchanged. It is used by DISJOINT queries.
func TransposeRelation(r geo.Relation) geo.Relation {
	switch r {
	case geo.CellInsideQuery:
		return geo.CellOutsideQuery
	case geo.CellOutsideQuery:
		return geo.CellInsideQuery
	default:
		return geo.CellCrossesQuery
	}
}
