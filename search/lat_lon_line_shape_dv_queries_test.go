// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonLineShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonLineShapeDVQueries (GOC-4011).
//
// The Java class is a thin subclass of BaseLatLonShapeDocValueTestCase that:
//   - selects ShapeType.LINE,
//   - delegates indexable-field creation to LatLonShape.createDocValueField, and
//   - reuses TestLatLonLineShapeQueries.LineValidator.
//
// Gocene verifies that NewLatLonShapeDocValuesQuery accepts a Line geometry
// and validates basic construction properties (field, relation, Component2D).
// The full random-test harness (RandomIndexWriter, GeoTestUtil, CheckHits,
// QueryUtils) is not yet ported; this test exercises the constructor surface
// only.
func TestLatLonLineShapeDVQueries(t *testing.T) {
	t.Parallel()
	line, err := geo.NewLine([]float64{0, 1, 2}, []float64{0, 1, 2})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	// INTERSECTS with Line via DocValues query.
	q, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects, line)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(INTERSECTS, line): %v", err)
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
	// DISJOINT with Line.
	q2, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationDisjoint, line)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(DISJOINT, line): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN with Line is NOT rejected on the DocValues path (unlike the
	// BKD-driven LatLonShapeQuery).
	q3, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationWithin, line)
	if err != nil {
		t.Fatalf("NewLatLonShapeDocValuesQuery(WITHIN, line): %v", err)
	}
	if q3.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q3.GetQueryRelation())
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeDocValuesQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
