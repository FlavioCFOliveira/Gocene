// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestLatLonDocValuesPointPoint_BasicConstruction verifies that
// NewLatLonDocValuesQuery with a single Point geometry builds a
// query with the correct field, relation, and geometries.
func TestLatLonDocValuesPointPoint_BasicConstruction(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	q, err := NewLatLonDocValuesQuery("location", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(point): %v", err)
	}
	concrete, ok := q.(*latLonDocValuesQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesQuery, got %T", q)
	}
	if got := concrete.GetField(); got != "location" {
		t.Fatalf("GetField: got %q, want %q", got, "location")
	}
	if got := concrete.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v, want INTERSECTS", got)
	}
	if len(concrete.GetGeometries()) != 1 {
		t.Fatalf("GetGeometries: got %d, want 1", len(concrete.GetGeometries()))
	}
}

// TestLatLonDocValuesPointPoint_Equals verifies that two
// identically-constructed single-point queries compare equal.
func TestLatLonDocValuesPointPoint_Equals(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	a, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery a: %v", err)
	}
	b, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery b: %v", err)
	}
	if !a.Equals(b) {
		t.Fatalf("equal queries should compare equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal queries should hash equal: %d vs %d", a.HashCode(), b.HashCode())
	}
}

// TestLatLonDocValuesPointPoint_DifferentRelation verifies that
// queries with different relations do not compare equal.
func TestLatLonDocValuesPointPoint_DifferentRelation(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	a, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery a: %v", err)
	}
	b, err := NewLatLonDocValuesQuery("loc", document.QueryRelationDisjoint, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery b: %v", err)
	}
	if a.Equals(b) {
		t.Fatalf("queries with different relations should not compare equal")
	}
}

// TestLatLonDocValuesPointPoint_String verifies the string
// representation includes the relation and geometries.
func TestLatLonDocValuesPointPoint_String(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery: %v", err)
	}
	concrete, ok := q.(*latLonDocValuesQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesQuery, got %T", q)
	}
	s := concrete.String("loc")
	if !strings.Contains(s, "INTERSECTS") || !strings.Contains(s, "geometries") {
		t.Fatalf("String: expected INTERSECTS:geometries(...), got %q", s)
	}
	// Verify that passing a non-default field emits the "field:" prefix.
	s2 := concrete.String("other")
	if !strings.Contains(s2, "loc:") {
		t.Fatalf("String(other): should contain field prefix 'loc:', got %q", s2)
	}
}

// TestLatLonDocValuesPointPoint_RejectsEmptyField verifies empty field
// rejection.
func TestLatLonDocValuesPointPoint_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	if _, err := NewLatLonDocValuesQuery("", document.QueryRelationIntersects, pt); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestLatLonDocValuesPointPoint_RejectsInvalidRelation verifies an
// out-of-range relation value is rejected.
func TestLatLonDocValuesPointPoint_RejectsInvalidRelation(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 37, -122)
	if _, err := NewLatLonDocValuesQuery("loc", document.QueryRelation(99), pt); err == nil {
		t.Fatalf("expected error on invalid relation")
	}
}

// TestLatLonDocValuesPointPoint_Visit exercises the QueryVisitor path.
func TestLatLonDocValuesPointPoint_Visit(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 0, 0)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery: %v", err)
	}
	concrete, ok := q.(*latLonDocValuesQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesQuery, got %T", q)
	}
	v := &recordingShapeVisitor{accept: true}
	concrete.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestLatLonDocValuesPointPoint_VisitRejectedField verifies Visit
// suppresses VisitLeaf when the visitor rejects the field.
func TestLatLonDocValuesPointPoint_VisitRejectedField(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 0, 0)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery: %v", err)
	}
	concrete, ok := q.(*latLonDocValuesQuery)
	if !ok {
		t.Fatalf("expected *latLonDocValuesQuery, got %T", q)
	}
	v := &recordingShapeVisitor{accept: false}
	concrete.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called for rejected field")
	}
}
