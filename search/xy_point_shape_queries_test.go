// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPointShapeQueries (GOC-4002).
//
// The Java class is a subclass of BaseXYShapeTestCase that drives a
// random-test harness Gocene lacks. This test verifies production
// XYShapeQuery and XYPointInGeometryQuery construction for XYPoint geometries.
//
// Covers: XYShapeQuery with XYPoint (various relations),
// XYPointInGeometryQuery with XYPoint, GetField, GetQueryRelation,
// queryComponent2D, empty-field guard, empty-geometries guard.
func TestXYPointShapeQueries(t *testing.T) {
	t.Parallel()

	pt, err := geo.NewXYPoint(1.5, 2.5)
	if err != nil {
		t.Fatalf("NewXYPoint: %v", err)
	}

	// XYShapeQuery with XYPoint — all relations.
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run("XYShape_"+rel.String(), func(t *testing.T) {
			t.Parallel()
			q, err := NewXYShapeQuery("shape", rel, pt)
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

	// XYPointInGeometryQuery with XYPoint.
	pq, err := NewXYPointInGeometryQuery("point", pt)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery: %v", err)
	}
	if pq.(*xyPointInGeometryQuery).Field() != "point" {
		t.Fatalf("Field: got %q, want %q", pq.(*xyPointInGeometryQuery).Field(), "point")
	}
	if got := pq.(*xyPointInGeometryQuery).Geometries(); len(got) != 1 {
		t.Fatalf("geometries length: got %d, want 1", len(got))
	}

	// Empty field guard.
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, pt); err == nil {
		t.Fatalf("expected error on empty field")
	}

	// Empty geometries guard.
	if _, err := NewXYShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}
