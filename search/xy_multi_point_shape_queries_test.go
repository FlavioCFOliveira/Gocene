// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestXYMultiPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiPointShapeQueries (GOC-3997).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that emits
// 1..4 random XYPoints per shape. This test verifies production
// XYShapeQuery and XYPointInGeometryQuery construction for XYPoint geometries.
//
// Covers: XYShapeQuery with XYPoint, XYPointInGeometryQuery with XYPoint,
// GetField, GetQueryRelation, queryComponent2D, multiple point geometries,
// empty-field guard, empty-geometries guard.
func TestXYMultiPointShapeQueries(t *testing.T) {
	t.Parallel()

	ptA, err := geo.NewXYPoint(1, 2)
	if err != nil {
		t.Fatalf("NewXYPoint: %v", err)
	}
	ptB, err := geo.NewXYPoint(3, 4)
	if err != nil {
		t.Fatalf("NewXYPoint: %v", err)
	}

	// Test XYShapeQuery with XYPoint.
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, ptA, ptB)
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

	// Test XYPointInGeometryQuery with XYPoint.
	pq, err := NewXYPointInGeometryQuery("point", ptA, ptB)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery: %v", err)
	}
	if pq.(*xyPointInGeometryQuery).Field() != "point" {
		t.Fatalf("Field: got %q, want %q", pq.(*xyPointInGeometryQuery).Field(), "point")
	}
	if got := pq.(*xyPointInGeometryQuery).Geometries(); len(got) != 2 {
		t.Fatalf("geometries length: got %d, want 2", len(got))
	}

	// Empty field guard.
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, ptA); err == nil {
		t.Fatalf("expected error on empty field")
	}

	// Empty geometries guard.
	if _, err := NewXYShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}

	// WITHIN relation should work with XYPoint (only XYLine is rejected).
	qw, err := NewXYShapeQuery("shape", document.QueryRelationWithin, ptA)
	if err != nil {
		t.Fatalf("WITHIN+XYPoint: unexpected error %v", err)
	}
	if got := qw.GetQueryRelation(); got != document.QueryRelationWithin {
		t.Fatalf("GetQueryRelation: got %v, want %v", got, document.QueryRelationWithin)
	}
}
