// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPolygonShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPolygonShapeDVQueries (GOC-3993).
//
// The Java class is a thin subclass of BaseLatLonShapeDocValueTestCase that:
//   - selects ShapeType.POLYGON,
//   - delegates indexable-field creation to LatLonShape.createDocValueField, and
//   - reuses TestLatLonPolygonShapeQueries.PolygonValidator.
//
// Gocene verifies that NewLatLonShapeDocValuesQuery accepts a Polygon geometry
// and validates basic construction properties (field, relation, Component2D).
// The full random-test harness (RandomIndexWriter, GeoTestUtil, CheckHits,
// QueryUtils) is not yet ported; this test exercises the constructor surface
// only.
func TestLatLonPolygonShapeDVQueries(t *testing.T) {
	t.Parallel()
	// Build a minimal triangle polygon.
	poly, err := geo.NewPolygon(
		[]float64{0, 1, 0, 0},
		[]float64{0, 0, 1, 0},
	)
	if err != nil {
		t.Fatalf("geo.NewPolygon: %v", err)
	}
	// INTERSECTS with Polygon via DocValues query.
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(INTERSECTS, polygon): %v", err)
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
	// DISJOINT with Polygon.
	q2, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationDisjoint, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(DISJOINT, polygon): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN with Polygon.
	q3, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationWithin, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(WITHIN, polygon): %v", err)
	}
	if q3.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q3.GetQueryRelation())
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
