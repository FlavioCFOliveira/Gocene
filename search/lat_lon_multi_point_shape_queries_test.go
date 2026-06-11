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

// TestNewLatLonShapeQuery_MultiPoint exercises NewLatLonShapeQuery with
// multiple geo.Point geometries (multi-point shape). It verifies
// construction and basic query properties.
func TestLatLonMultiPointShapeQueries(t *testing.T) {
	t.Parallel()
	p1, err := geo.NewPoint(10, 20)
	if err != nil {
		t.Fatalf("geo.NewPoint: %v", err)
	}
	p2, err := geo.NewPoint(-10, -20)
	if err != nil {
		t.Fatalf("geo.NewPoint: %v", err)
	}
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(multi-point): %v", err)
	}
	if q.GetField() != "shape" {
		t.Fatalf("GetField: got %q, want %q", q.GetField(), "shape")
	}
	if q.GetQueryRelation() != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", q.GetQueryRelation())
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil for multi-point")
	}
	if len(q.GetGeometries()) != 2 {
		t.Fatalf("geometries length: got %d, want 2", len(q.GetGeometries()))
	}
	// Verify that WITHIN is accepted for multi-point (non-Line geometry).
	_, err = NewLatLonShapeQuery("shape", document.QueryRelationWithin, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(WITHIN, multi-point): %v", err)
	}
	// Verify that CONTAINS is accepted for multi-point.
	_, err = NewLatLonShapeQuery("shape", document.QueryRelationContains, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(CONTAINS, multi-point): %v", err)
	}
	// Verify that empty geometries is rejected.
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
	// Verify that WITHIN rejects Line.
	line, err := geo.NewLine([]float64{0, 1}, []float64{0, 1})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, line); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
}
