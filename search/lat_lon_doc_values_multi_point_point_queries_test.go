// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// TestLatLonDocValuesMultiPointPoint_BasicConstruction verifies that
// NewLatLonDocValuesQuery with multiple Point geometries builds a
// query with the correct field, relation, and component2D via the
// concrete type accessors.
func TestLatLonDocValuesMultiPointPoint_BasicConstruction(t *testing.T) {
	t.Parallel()
	p1 := testLatLonGeoPoint(t, 10, 20)
	p2 := testLatLonGeoPoint(t, -10, -20)
	q, err := NewLatLonDocValuesQuery("location", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(multi-point): %v", err)
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
	if len(concrete.GetGeometries()) != 2 {
		t.Fatalf("GetGeometries: got %d, want 2", len(concrete.GetGeometries()))
	}
}

// TestLatLonDocValuesMultiPointPoint_Equals verifies that two
// identically-constructed multi-point queries compare equal and have
// the same hash code.
func TestLatLonDocValuesMultiPointPoint_Equals(t *testing.T) {
	t.Parallel()
	p1 := testLatLonGeoPoint(t, 10, 20)
	p2 := testLatLonGeoPoint(t, -10, -20)
	a, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, p1, p2)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery a: %v", err)
	}
	b, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, p1, p2)
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

// TestLatLonDocValuesMultiPointPoint_DifferentField verifies that
// queries with different fields do not compare equal.
func TestLatLonDocValuesMultiPointPoint_DifferentField(t *testing.T) {
	t.Parallel()
	p1 := testLatLonGeoPoint(t, 10, 20)
	a, err := NewLatLonDocValuesQuery("loc_a", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery a: %v", err)
	}
	b, err := NewLatLonDocValuesQuery("loc_b", document.QueryRelationIntersects, p1)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery b: %v", err)
	}
	if a.Equals(b) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestLatLonDocValuesMultiPointPoint_String verifies the string
// representation includes the class prefix.
func TestLatLonDocValuesMultiPointPoint_String(t *testing.T) {
	t.Parallel()
	p1 := testLatLonGeoPoint(t, 10, 20)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, p1)
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
}

// TestLatLonDocValuesMultiPointPoint_RejectsEmptyField verifies
// empty field rejection.
func TestLatLonDocValuesMultiPointPoint_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	p1 := testLatLonGeoPoint(t, 10, 20)
	if _, err := NewLatLonDocValuesQuery("", document.QueryRelationIntersects, p1); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestLatLonDocValuesMultiPointPoint_RejectsWithinLine verifies
// WITHIN+Line rejection.
func TestLatLonDocValuesMultiPointPoint_RejectsWithinLine(t *testing.T) {
	t.Parallel()
	line := testLatLonGeoLine(t, []float64{0, 1}, []float64{0, 1})
	if _, err := NewLatLonDocValuesQuery("loc", document.QueryRelationWithin, line); err == nil {
		t.Fatalf("expected error for WITHIN+Line")
	}
}

// TestLatLonDocValuesMultiPointPoint_AcceptsContainsPoint verifies
// CONTAINS is accepted for Point geometry.
func TestLatLonDocValuesMultiPointPoint_AcceptsContainsPoint(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 10, 20)
	if _, err := NewLatLonDocValuesQuery("loc", document.QueryRelationContains, pt); err != nil {
		t.Fatalf("NewLatLonDocValuesQuery(CONTAINS, point): %v", err)
	}
}

// TestLatLonDocValuesMultiPointPoint_RejectsContainsNonPoint verifies
// CONTAINS rejects non-Point geometry.
func TestLatLonDocValuesMultiPointPoint_RejectsContainsNonPoint(t *testing.T) {
	t.Parallel()
	rect := testLatLonGeoRect(t, -10, 10, -20, 20)
	if _, err := NewLatLonDocValuesQuery("loc", document.QueryRelationContains, rect); err == nil {
		t.Fatalf("expected error for CONTAINS+non-Point")
	}
}

// TestLatLonDocValuesMultiPointPoint_Visit exercises the QueryVisitor
// path on a doc-values point query.
func TestLatLonDocValuesMultiPointPoint_Visit(t *testing.T) {
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

// TestLatLonDocValuesMultiPointPoint_HashCodeFoldsField verifies that
// switching the field produces a different hash code.
func TestLatLonDocValuesMultiPointPoint_HashCodeFoldsField(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 10, 20)
	a, err := NewLatLonDocValuesQuery("alpha", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery a: %v", err)
	}
	b, err := NewLatLonDocValuesQuery("beta", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery b: %v", err)
	}
	if a.HashCode() == b.HashCode() {
		t.Fatalf("HashCode collision across different fields: %d", a.HashCode())
	}

// TestLatLonDocValuesMultiPointPoint_CloneNonNil verifies that Clone
// returns a non-nil query.
func TestLatLonDocValuesMultiPointPoint_CloneNonNil(t *testing.T) {
	t.Parallel()
	pt := testLatLonGeoPoint(t, 10, 20)
	q, err := NewLatLonDocValuesQuery("loc", document.QueryRelationIntersects, pt)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesQuery: %v", err)
	}
	if c := q.Clone(); c == nil {
		t.Fatalf("Clone must not return nil")
	}
}