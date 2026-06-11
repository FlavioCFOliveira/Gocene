// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPolygonShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeDVQueries (GOC-3995).
//
// The Java class is a thin subclass of BaseXYShapeDocValueTestCase that:
//   - selects ShapeType.POLYGON,
//   - delegates indexable-field creation to XYShape.createDocValueField, and
//   - reuses TestXYPolygonShapeQueries.PolygonValidator.
//
// Gocene verifies that NewXYShapeDocValuesQuery accepts an XYPolygon geometry
// and validates basic construction properties (field, relation, Component2D).
// The full random-test harness (RandomIndexWriter, ShapeTestUtil, CheckHits,
// QueryUtils) is not yet ported; this test exercises the constructor surface
// only.
func TestXYPolygonShapeDVQueries(t *testing.T) {
	t.Parallel()
	// Build a minimal triangle polygon in Cartesian space.
	poly, err := geo.NewXYPolygon(
		[]float32{0, 1, 0, 0},
		[]float32{0, 0, 1, 0},
	)
	if err != nil {
		t.Fatalf("geo.NewXYPolygon: %v", err)
	}
	// INTERSECTS with XYPolygon via DocValues query.
	q, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects, poly)
	if err != nil {
		t.Fatalf("NewXYShapeDocValuesQuery(INTERSECTS, polygon): %v", err)
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
	// DISJOINT with XYPolygon.
	q2, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationDisjoint, poly)
	if err != nil {
		t.Fatalf("NewXYShapeDocValuesQuery(DISJOINT, polygon): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN with XYPolygon.
	q3, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationWithin, poly)
	if err != nil {
		t.Fatalf("NewXYShapeDocValuesQuery(WITHIN, polygon): %v", err)
	}
	if q3.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q3.GetQueryRelation())
	}
	// Verify empty geometries rejection.
	if _, err := NewXYShapeDocValuesQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
