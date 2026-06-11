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

// TestLatLonMultiPolygonShapeQueries exercises NewLatLonShapeQuery with
// multiple Polygon geometries (multi-polygon shape), verifying
// construction and basic query properties.
func TestLatLonMultiPolygonShapeQueries(t *testing.T) {
	t.Parallel()
	p1, err := geo.NewPolygon(
		[]float64{0, 5, 5, 0, 0},
		[]float64{0, 0, 5, 5, 0},
	)
	if err != nil {
		t.Fatalf("geo.NewPolygon: %v", err)
	}
	p2, err := geo.NewPolygon(
		[]float64{10, 15, 15, 10, 10},
		[]float64{10, 10, 15, 15, 10},
	)
	if err != nil {
		t.Fatalf("geo.NewPolygon: %v", err)
	}
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(multi-polygon): %v", err)
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
	if len(q.GetGeometries()) != 2 {
		t.Fatalf("geometries length: got %d, want 2", len(q.GetGeometries()))
	}
	// Test WITHIN for multi-polygon.
	q2, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(WITHIN, multi-polygon): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q2.GetQueryRelation())
	}
	// Test CONTAINS for multi-polygon.
	_, err = NewLatLonShapeQuery("shape", document.QueryRelationContains, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(CONTAINS, multi-polygon): %v", err)
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
