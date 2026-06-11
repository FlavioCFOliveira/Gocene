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

// TestLatLonLineShapeQueries exercises NewLatLonShapeQuery with a
// single Line geometry, verifying construction and basic query
// properties including the WITHIN+Line rejection.
func TestLatLonLineShapeQueries(t *testing.T) {
	t.Parallel()
	line, err := geo.NewLine([]float64{0, 1, 2}, []float64{0, 1, 2})
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	// INTERSECTS with Line.
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, line)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(INTERSECTS, line): %v", err)
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
	q2, err := NewLatLonShapeQuery("shape", document.QueryRelationDisjoint, line)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery(DISJOINT, line): %v", err)
	}
	if q2.GetQueryRelation() != document.QueryRelationDisjoint {
		t.Fatalf("GetQueryRelation: got %v, want DISJOINT", q2.GetQueryRelation())
	}
	// WITHIN+Line must be rejected.
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationWithin, line); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
	// Verify empty geometries rejection.
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error for empty geometries")
	}
}
