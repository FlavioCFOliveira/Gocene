// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestLatLonPolygonShapeQueries exercises NewLatLonShapeQuery with a
// single Polygon geometry, verifying construction and basic query
// properties.
func TestLatLonPolygonShapeQueries(t *testing.T) {
	t.Parallel()
	poly, err := geo.NewPolygon(
		[]float64{0, 10, 10, 0, 0},
		[]float64{0, 0, 10, 10, 0},
	)
	if err != nil {
		t.Fatalf("geo.NewPolygon: %v", err)
	}
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(polygon): %v", err)
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
	if len(q.GetGeometries()) != 1 {
		t.Fatalf("geometries length: got %d, want 1", len(q.GetGeometries()))
	}
	// Test WITHIN relation works for polygon.
	q2, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(WITHIN, polygon): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q2.GetQueryRelation())
	}
	// Test CONTAINS relation works for polygon.
	q3, err := NewLatLonShapeQuery("shape", document.QueryRelationContains, poly)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(CONTAINS, polygon): %v", err)
	}
	if q3.GetQueryRelation() != document.QueryRelationContains {
		t.Fatalf("GetQueryRelation: got %v, want CONTAINS", q3.GetQueryRelation())
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
	// Verify WITHIN+Line rejection.
	line, err := geo.NewLine([]float64{0, 1}, []float64{0, 1})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, line); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
}
