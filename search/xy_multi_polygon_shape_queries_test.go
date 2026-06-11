// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYMultiPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiPolygonShapeQueries (GOC-4017).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that emits
// 1..4 random XYPolygons per shape. This test verifies production
// XYShapeQuery construction and validation for multiple XYPolygon geometries.
//
// Covers: basic construction with multiple polygons, GetField, GetQueryRelation,
// GetQueryComponent2D, WITHIN relation, empty-field guard.
func TestXYMultiPolygonShapeQueries(t *testing.T) {
	t.Parallel()

	polyA, err := geo.NewXYPolygon(
		[]float32{0, 10, 10, 0, 0},
		[]float32{0, 0, 10, 10, 0},
	)
	if err != nil {
		t.Fatalf("NewXYPolygon A: %v", err)
	}
	polyB, err := geo.NewXYPolygon(
		[]float32{20, 30, 30, 20, 20},
		[]float32{20, 20, 30, 30, 20},
	)
	if err != nil {
		t.Fatalf("NewXYPolygon B: %v", err)
	}

	// Basic construction with INTERSECTS and multiple polygons.
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, polyA, polyB)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	if got := q.GetField(); got != "shape" {
		t.Fatalf("GetField: got %q, want %q", got, "shape")
	}
	if got := q.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want %v", got, document.QueryRelationIntersects)
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil")
	}
	if len(q.GetGeometries()) != 2 {
		t.Fatalf("geometries length: got %d, want 2", len(q.GetGeometries()))
	}

	// WITHIN relation with polygon.
	qw, err := NewXYShapeQuery("shape", document.QueryRelationWithin, polyA)
	if err != nil {
		t.Fatalf("WITHIN+XYPolygon: unexpected error %v", err)
	}
	if got := qw.GetQueryRelation(); got != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want %v", got, document.QueryRelationWithin)
	}

	// CONTAINS and DISJOINT work with polygons.
	for _, rel := range []document.QueryRelation{
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewXYShapeQuery("shape", rel, polyA); err != nil {
				t.Fatalf("%v + XYPolygon: unexpected error %v", rel, err)
			}
		})
	}

	// Empty field guard.
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, polyA); err == nil {
		t.Fatalf("expected error on empty field")
	}

	// Empty geometries guard.
	if _, err := NewXYShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}
