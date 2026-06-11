// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// LatLonDocValuesQuery is a data carrier for a doc-values-backed geo
// spatial query. It mirrors the class (package-private in Java)
// org.apache.lucene.document.LatLonDocValuesQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the spatial relation and the geometry
// operands that the search-layer implementation consumes.
//
// LatLonDocValuesQuery validates geometry constraints:
//   - WITHIN is not supported with Line geometries.
//   - CONTAINS is only supported with Point geometries.
type LatLonDocValuesQuery struct {
	field          string
	queryRelation  QueryRelation
	geometries     []geo.LatLonGeometry
}

// NewLatLonDocValuesQuery constructs a LatLonDocValuesQuery data carrier.
// field must be non-empty, queryRelation must be provided, and
// geometries must be non-empty.
//
// Returns an error if constraints are violated (see type doc).
func NewLatLonDocValuesQuery(field string, queryRelation QueryRelation, geometries ...geo.LatLonGeometry) (*LatLonDocValuesQuery, error) {
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
	if queryRelation == QueryRelationWithin {
		for _, g := range geometries {
			if _, ok := g.(geo.Line); ok {
				return nil, fmt.Errorf("LatLonDocValuesQuery does not support WITHIN queries with line geometries")
			}
		}
	}
	if queryRelation == QueryRelationContains {
		for _, g := range geometries {
			if _, ok := g.(geo.Point); !ok {
				return nil, fmt.Errorf("LatLonDocValuesQuery does not support CONTAINS with non-point geometries")
			}
		}
	}
	dup := make([]geo.LatLonGeometry, len(geometries))
	copy(dup, geometries)
	return &LatLonDocValuesQuery{
		field:         field,
		queryRelation: queryRelation,
		geometries:    dup,
	}, nil
}

// Field returns the target field name.
func (q *LatLonDocValuesQuery) Field() string { return q.field }

// QueryRelation returns the spatial relation for this query.
func (q *LatLonDocValuesQuery) QueryRelation() QueryRelation { return q.queryRelation }

// Geometries returns a defensive copy of the geometry list.
func (q *LatLonDocValuesQuery) Geometries() []geo.LatLonGeometry {
	out := make([]geo.LatLonGeometry, len(q.geometries))
	copy(out, q.geometries)
	return out
}

// String returns a human-readable representation.
func (q *LatLonDocValuesQuery) String() string {
	return fmt.Sprintf("LatLonDocValuesQuery(field=%s, relation=%s, geometries=%d)", q.field, q.queryRelation, len(q.geometries))
}

// Equals reports whether two LatLonDocValuesQuery carriers are equal.
func (q *LatLonDocValuesQuery) Equals(other *LatLonDocValuesQuery) bool {
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
		if fmt.Sprintf("%v", q.geometries[i]) != fmt.Sprintf("%v", other.geometries[i]) {
			return false
		}
	}
	return true
}
