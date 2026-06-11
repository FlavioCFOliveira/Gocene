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

// TestLatLonMultiLineShapeQueries exercises NewLatLonShapeQuery with
// multiple Line geometries (multi-line shape), verifying construction
// and basic query properties including the WITHIN+Line rejection.
func TestLatLonMultiLineShapeQueries(t *testing.T) {
	t.Parallel()
	l1, err := geo.NewLine([]float64{0, 1, 2}, []float64{0, 1, 2})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	l2, err := geo.NewLine([]float64{3, 4, 5}, []float64{3, 4, 5})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	// INTERSECTS with multi-line.
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, l1, l2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(INTERSECTS, multi-line): %v", err)
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
	// DISJOINT with multi-line.
	q2, err := NewLatLonShapeQuery("shape", document.QueryRelationDisjoint, l1, l2)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(DISJOINT, multi-line): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN+Line must be rejected (even for multi-line).
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, l1); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
