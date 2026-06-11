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

// TestLatLonPointShapeQueries exercises NewLatLonShapeQuery with a
// single Point geometry, verifying construction and basic query
// properties.
func TestLatLonPointShapeQueries(t *testing.T) {
	t.Parallel()
	pt, err := geo.NewPoint(10, 20)
	if err != nil {
		t.Fatalf("geo.NewPoint: %v", err)
	}
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(point): %v", err)
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
	// Test WITHIN relation for point.
	q2, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(WITHIN, point): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want WITHIN", q2.GetQueryRelation())
	}
	// Test CONTAINS relation for point.
	q3, err := NewLatLonShapeQuery("shape", document.QueryRelationContains, pt)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(CONTAINS, point): %v", err)
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
