// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPointShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPointShapeDVQueries (GOC-3991).
//
// The Java class is a thin subclass of BaseLatLonShapeDocValueTestCase that:
//   - selects ShapeType.POINT,
//   - delegates indexable-field creation to LatLonShape.createDocValueField, and
//   - reuses TestLatLonPointShapeQueries.PointValidator.
//
// Gocene verifies that NewLatLonShapeDocValuesQuery accepts a Point geometry
// and validates basic construction properties (field, relation, Component2D).
// The full random-test harness (RandomIndexWriter, GeoTestUtil, CheckHits,
// QueryUtils) is not yet ported; this test exercises the constructor surface
// only.
func TestLatLonPointShapeDVQueries(t *testing.T) {
	t.Parallel()
	pt, err := geo.NewPoint(10.0, 20.0)
	// INTERSECTS with Point via DocValues query.
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(INTERSECTS, point): %v", err)
	}
	if q.GetField() != "shape" {
		t.Fatalf("GetField: got %q, want %q", q.GetField(), "shape")
	}
	if q.GetQueryRelation() != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", q.GetQueryRelation())
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil")
	}
	// DISJOINT with Point.
	q2, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationDisjoint, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(DISJOINT, point): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN with Point.
	q3, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationWithin, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(WITHIN, point): %v", err)
	}
	if q3.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q3.GetQueryRelation())
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
