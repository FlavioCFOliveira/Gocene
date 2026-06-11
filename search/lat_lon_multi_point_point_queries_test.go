// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestLatLonMultiPointPoint_BasicConstruction verifies that
// NewLatLonPointQuery with multiple Point geometries builds a query
// with the correct field, relation, and component2D.
func TestLatLonMultiPointPoint_BasicConstruction(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 10, 20)
	p2 := testLatLonPointGeo(t, -10, -20)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery(multi-point): %v", err)
	}
	if q.GetField() != "point" {
		t.Fatalf("GetField: got %q, want %q", q.GetField(), "point")
	}
	if q.GetQueryRelation() != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", q.GetQueryRelation())
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil")
	}
	if len(q.GetGeometries()) != 2 {
		t.Fatalf("GetGeometries: got %d, want 2", len(q.GetGeometries()))
	}
}

// TestLatLonMultiPointPoint_Equals verifies that two identically-
// constructed multi-point queries compare equal.
func TestLatLonMultiPointPoint_Equals(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 10, 20)
	p2 := testLatLonPointGeo(t, -10, -20)
	a, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if !a.Equals(b.SpatialQuery) {
		t.Fatalf("equal queries should compare equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal queries should hash equal: %d vs %d", a.HashCode(), b.HashCode())
	}
}

// TestLatLonMultiPointPoint_DifferentField verifies that queries with
// different fields do not compare equal.
func TestLatLonMultiPointPoint_DifferentField(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 10, 20)
	a, err := NewLatLonPointQuery("point_a", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point_b", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestLatLonMultiPointPoint_String verifies the string representation.
func TestLatLonMultiPointPoint_String(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 10, 20)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	s := q.String("point")
	if !strings.HasPrefix(s, "LatLonPointQuery:") {
		t.Fatalf("String: expected LatLonPointQuery prefix, got %q", s)
	}
}

// TestLatLonMultiPointPoint_RejectsEmptyField verifies empty field
// rejection.
func TestLatLonMultiPointPoint_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 10, 20)
	if _, err := NewLatLonPointQuery("", document.QueryRelationIntersects, p1); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestLatLonMultiPointPoint_RejectsEmptyGeometries verifies that an
// empty geometries slice is rejected.
func TestLatLonMultiPointPoint_RejectsEmptyGeometries(t *testing.T) {
	t.Parallel()
	if _, err := NewLatLonPointQuery("point", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}

// TestLatLonMultiPointPoint_Visit exercises the QueryVisitor path.
func TestLatLonMultiPointPoint_Visit(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 0, 0)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestLatLonMultiPointPoint_VisitRejectedField verifies Visit
// suppresses VisitLeaf when the visitor rejects the field.
func TestLatLonMultiPointPoint_VisitRejectedField(t *testing.T) {
	t.Parallel()
	p1 := testLatLonPointGeo(t, 0, 0)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called for rejected field")
	}
}
