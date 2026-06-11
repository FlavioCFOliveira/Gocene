// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeQueries (GOC-4009).
//
// The Java class is a subclass of BaseXYShapeTestCase that drives a
// random-test harness Gocene lacks. This test verifies production
// XYShapeQuery construction and validation for XYPolygon geometries.
//
// Covers: XYShapeQuery with XYPolygon (all relations), GetField,
// GetQueryRelation, queryComponent2D, empty-field guard,
// empty-geometries guard.
func TestXYPolygonShapeQueries(t *testing.T) {
	t.Parallel()

	poly, err := geo.NewXYPolygon(
		[]float32{0, 10, 10, 0, 0},
		[]float32{0, 0, 10, 10, 0},
	)
	if err != nil {
		t.Fatalf("NewXYPolygon: %v", err)
	}

	// XYShapeQuery with XYPolygon — all relations.
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run("XYShape_"+rel.String(), func(t *testing.T) {
			t.Parallel()
			q, err := NewXYShapeQuery("shape", rel, poly)
			if err != nil {
				t.Fatalf("NewXYShapeQuery(%v): %v", rel, err)
			}
			if got := q.GetField(); got != "shape" {
				t.Fatalf("GetField: got %q, want %q", got, "shape")
			}
			if got := q.GetQueryRelation(); got != rel {
				t.Fatalf("GetQueryRelation: got %v, want %v", got, rel)
			}
			if q.GetQueryComponent2D() == nil {
				t.Fatalf("queryComponent2D must not be nil")
			}
			if len(q.GetGeometries()) != 1 {
				t.Fatalf("geometries length: got %d, want 1", len(q.GetGeometries()))
			}
		})
	}

	// Empty field guard.
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, poly); err == nil {
		t.Fatalf("expected error on empty field")
	}

	// Empty geometries guard.
	if _, err := NewXYShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}
