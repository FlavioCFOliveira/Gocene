// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestLatLonPointPoint_BasicConstruction verifies that
// NewLatLonPointQuery with a single Point geometry builds a query
// with the correct field, relation, and component2D.
func TestLatLonPointPoint_BasicConstruction(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 37, -122)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	if q.GetField() != "point" {
		t.Fatalf("GetField: got %q, want %q", q.GetField(), "point")
	}
	if q.GetQueryRelation() != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", q.GetQueryRelation())
	}
	if len(q.GetGeometries()) != 1 {
		t.Fatalf("GetGeometries: got %d, want 1", len(q.GetGeometries()))
	}
}

// TestLatLonPointPoint_BoundingBoxQueryConstruction verifies that
// NewLatLonPointBoxQuery builds a box query without error.
func TestLatLonPointPoint_BoundingBoxQueryConstruction(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointBoxQuery("point", -10, 10, -20, 20)
	if err != nil {
		t.Fatalf("NewLatLonPointBoxQuery: %v", err)
	}
	if q == nil {
		t.Fatalf("NewLatLonPointBoxQuery: returned nil query")
	}
	// Verify the query can be cloned without error.
	if c := q.Clone(); c == nil {
		t.Fatalf("Clone must not return nil")
	}
}

// TestLatLonPointPoint_EqualsSameInputs verifies two identically-
// constructed point queries compare equal.
func TestLatLonPointPoint_EqualsSameInputs(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 37, -122)
	a, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
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

// TestLatLonPointPoint_HashCodeFoldsField verifies that switching
// the field invalidates the hash.
func TestLatLonPointPoint_HashCodeFoldsField(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 37, -122)
	a, err := NewLatLonPointQuery("alpha", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("beta", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if a.HashCode() == b.HashCode() {
		t.Fatalf("HashCode collision across different fields: %d", a.HashCode())
	}
}

// TestLatLonPointPoint_String verifies the string representation.
func TestLatLonPointPoint_String(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 37, -122)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	s := q.String("point")
	if !strings.HasPrefix(s, "LatLonPointQuery:") {
		t.Fatalf("String: expected LatLonPointQuery prefix, got %q", s)
	}
}

// TestLatLonPointPoint_RejectsEmptyField verifies empty field
// rejection.
func TestLatLonPointPoint_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 37, -122)
	if _, err := NewLatLonPointQuery("", document.QueryRelationIntersects, pt); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestLatLonPointPoint_RejectsEmptyGeometries verifies empty
// geometries rejection.
func TestLatLonPointPoint_RejectsEmptyGeometries(t *testing.T) {
	t.Parallel()
	if _, err := NewLatLonPointQuery("point", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}

// TestLatLonPointPoint_Visit exercises the QueryVisitor path.
func TestLatLonPointPoint_Visit(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 0, 0)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}

// TestLatLonPointPoint_VisitRejectedField verifies Visit suppresses
// VisitLeaf when the visitor rejects the field.
func TestLatLonPointPoint_VisitRejectedField(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 0, 0)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called for rejected field")
	}
}