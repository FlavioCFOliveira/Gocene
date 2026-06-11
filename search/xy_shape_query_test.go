// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testXYRect builds a rectangle covering [minX..maxX] × [minY..maxY];
// panics on validation error so tests stay terse.
func testXYRect(t *testing.T, minX, maxX, minY, maxY float32) geo.XYRectangle {
	t.Helper()
	r, err := geo.NewXYRectangle(minX, maxX, minY, maxY)
	if err != nil {
		t.Fatalf("geo.NewXYRectangle: %v", err)
	}
	return r
}

// testXYLine builds an XYLine from parallel xs/ys; panics on
// validation error.
func testXYLine(t *testing.T, xs, ys []float32) geo.XYLine {
	t.Helper()
	l, err := geo.NewXYLine(xs, ys)
	if err != nil {
		t.Fatalf("geo.NewXYLine: %v", err)
	}
	return l
}

// TestNewXYShapeQuery_BasicConstruction confirms the happy path for
// every non-Line geometry: builds the parent SpatialQuery, stores
// the field/relation/geometry triple, and exposes the
// queryComponent2D.
func TestNewXYShapeQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -20, 20, -10, 10)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	if got := q.GetField(); got != "shape" {
		t.Fatalf("GetField: got %q, want %q", got, "shape")
	}
	if got := q.GetQueryRelation(); got != document.QueryRelationIntersects {
		t.Fatalf("GetQueryRelation: got %v", got)
	}
	if q.GetQueryComponent2D() == nil {
		t.Fatalf("queryComponent2D must not be nil")
	}
	if len(q.GetGeometries()) != 1 {
		t.Fatalf("geometries length: got %d, want 1", len(q.GetGeometries()))
	}
}

// TestNewXYShapeQuery_RejectsWithinLine confirms that constructing a
// WITHIN query over an XYLine geometry surfaces
// ErrXYShapeQueryWithinLine, mirroring the Java reference's
// IllegalArgumentException.
func TestNewXYShapeQuery_RejectsWithinLine(t *testing.T) {
	t.Parallel()
	line := testXYLine(t, []float32{0, 1, 2}, []float32{0, 1, 2})
	if _, err := NewXYShapeQuery(
		"shape",
		document.QueryRelationWithin,
		line,
	); !errors.Is(err, ErrXYShapeQueryWithinLine) {
		t.Fatalf("WITHIN+XYLine: expected ErrXYShapeQueryWithinLine, got %v", err)
	}
}

// TestNewXYShapeQuery_AllowsLineForOtherRelations confirms that
// non-WITHIN relations accept XYLine geometries — only WITHIN is
// blacklisted.
func TestNewXYShapeQuery_AllowsLineForOtherRelations(t *testing.T) {
	t.Parallel()
	line := testXYLine(t, []float32{0, 1, 2}, []float32{0, 1, 2})
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewXYShapeQuery("shape", rel, line); err != nil {
				t.Fatalf("%v + XYLine: unexpected error %v", rel, err)
			}
		})
	}
}

// TestNewXYShapeQuery_RejectsEmptyField confirms the parent's
// empty-field guard surfaces through the XYShape constructor.
func TestNewXYShapeQuery_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, 0, 1, 0, 1)
	if _, err := NewXYShapeQuery("", document.QueryRelationIntersects, rect); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestNewXYShapeQuery_RejectsEmptyGeometries confirms that an empty
// geometries slice surfaces through the Component2D builder (the
// Java reference also rejects empty input via XYGeometry.create).
func TestNewXYShapeQuery_RejectsEmptyGeometries(t *testing.T) {
	t.Parallel()
	if _, err := NewXYShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}

// TestXYShapeQuery_Equals_SameInputs asserts that two XYShapeQuery
// values built from the same field, relation, and geometry list
// compare equal under the parent's Equals contract.
func TestXYShapeQuery_Equals_SameInputs(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	a, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery a: %v", err)
	}
	b, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery b: %v", err)
	}
	if !a.Equals(b.SpatialQuery) {
		t.Fatalf("equal queries should compare equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal queries should hash to the same value: %d vs %d",
			a.HashCode(), b.HashCode())
	}
}

// TestXYShapeQuery_Equals_DiffersByField confirms that distinct
// field names break equality even for otherwise identical queries.
func TestXYShapeQuery_Equals_DiffersByField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	a, err := NewXYShapeQuery("shape_a", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery a: %v", err)
	}
	b, err := NewXYShapeQuery("shape_b", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestXYShapeQuery_Equals_DiffersByRelation confirms that different
// query relations break equality.
func TestXYShapeQuery_Equals_DiffersByRelation(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	a, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery a: %v", err)
	}
	b, err := NewXYShapeQuery("shape", document.QueryRelationContains, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different relations should not compare equal")
	}
}

// TestXYShapeQuery_String_DefaultField asserts the toString output
// prefixes "XYShapeQuery:" when the default field matches the
// query's field, mirroring the Java reference's
// "<ClassName>:[geom,]" layout.
func TestXYShapeQuery_String_DefaultField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	got := q.String("shape")
	if !strings.HasPrefix(got, "XYShapeQuery:") {
		t.Fatalf("String(default field): prefix mismatch: %q", got)
	}
	if strings.Contains(got, "field=") {
		t.Fatalf("String(default field): should not emit field=: %q", got)
	}
}

// TestXYShapeQuery_String_DiffersFromDefaultField checks that the
// toString output prepends "field=<field>:" when the supplied default
// field differs from the query's field, mirroring the Java
// reference's SpatialQuery.toString(String).
func TestXYShapeQuery_String_DiffersFromDefaultField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	got := q.String("other")
	if !strings.Contains(got, "field=shape:") {
		t.Fatalf("String(non-default field): missing field= clause: %q", got)
	}
}

// TestXYShapeQuery_Visit_AcceptedField asserts that Visit invokes
// VisitLeaf when the QueryVisitor accepts the bound field.
func TestXYShapeQuery_Visit_AcceptedField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	v := &recordingXYShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestXYShapeQuery_Visit_RejectedField asserts that Visit suppresses
// the VisitLeaf call when the visitor rejects the field.
func TestXYShapeQuery_Visit_RejectedField(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -2, 2, -1, 1)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	v := &recordingXYShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called when field is rejected")
	}
}

// recordingXYShapeVisitor is the minimal QueryVisitor stub used by
// the Visit tests above. It records whether VisitLeaf was called
// and routes every other hook to a no-op.
type recordingXYShapeVisitor struct {
	accept     bool
	leafCalled bool
}

func (v *recordingXYShapeVisitor) AcceptField(_ string) bool                   { return v.accept }
func (v *recordingXYShapeVisitor) VisitLeaf(_ Query)                           { v.leafCalled = true }
func (v *recordingXYShapeVisitor) GetSubVisitor(_ Occur, _ Query) QueryVisitor { return v }
func (v *recordingXYShapeVisitor) ConsumeTerms(_ Query, _ ...*index.Term)      {}
func (v *recordingXYShapeVisitor) ConsumeTermsMatching(_ Query, _ string, _ func() ByteRunAutomaton) {
}

// TestXYShapeQuery_SpatialVisitor_Relate confirms the Relate hook
// decodes the 4-corner cell layout correctly and forwards to the
// underlying Component2D. An "inside" cell exercises the
// CellInsideQuery branch; a disjoint cell exercises
// CellOutsideQuery; a crossing cell exercises CellCrossesQuery.
func TestXYShapeQuery_SpatialVisitor_Relate(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -10, 10, -10, 10)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	// A cell strictly inside the 20×20 query rectangle.
	insideMin, insideMax := encodeXYCellBounds(t, -1, 1, -1, 1)
	if got := visitor.Relate(insideMin, insideMax); got != spatialCellInsideQuery {
		t.Fatalf("Relate inside-cell: got %v, want CELL_INSIDE_QUERY", got)
	}
	// A cell entirely above the query rectangle (Y range outside).
	outsideMin, outsideMax := encodeXYCellBounds(t, -1, 1, 50, 60)
	if got := visitor.Relate(outsideMin, outsideMax); got != spatialCellOutsideQuery {
		t.Fatalf("Relate outside-cell: got %v, want CELL_OUTSIDE_QUERY", got)
	}
	// A cell that straddles the query rectangle's eastern boundary.
	crossMin, crossMax := encodeXYCellBounds(t, 5, 15, -1, 1)
	if got := visitor.Relate(crossMin, crossMax); got != spatialCellCrossesQuery {
		t.Fatalf("Relate crossing-cell: got %v, want CELL_CROSSES_QUERY", got)
	}
}

// TestXYShapeQuery_SpatialVisitor_TriangleBranchInside drives the
// visitor's TRIANGLE branch against a payload whose A-vertex (the
// only vertex the current simplified decoder recovers) lies inside
// the query rectangle. The remaining vertices decode to the origin
// (0, 0) under the current Gocene decoder; the chosen rectangle
// covers the origin so the test stays decoder-agnostic.
//
// Intersects/Within/Contains all exercise the same TRIANGLE branch
// because document.DecodeTriangle currently classifies every
// payload as TRIANGLE-kind (the rotation-aware POINT/LINE
// classification is deferred to backlog #2697).
func TestXYShapeQuery_SpatialVisitor_TriangleBranchInside(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -10, 10, -10, 10)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeXYTriangleAVertex(t, 0, 0)

	if !visitor.Intersects()(packed) {
		t.Fatalf("Intersects: triangle inside query rectangle should match")
	}
	if !visitor.Within()(packed) {
		t.Fatalf("Within: triangle inside query rectangle should match")
	}
	if got := visitor.Contains()(packed); got == geo.WithinDisjoint {
		t.Fatalf("Contains: triangle inside query rectangle should not be DISJOINT, got %v", got)
	}
}

// TestXYShapeQuery_SpatialVisitor_TriangleBranchOutside confirms the
// visitor's Intersects branch rejects a TRIANGLE-kind payload whose
// A-vertex lies outside the query rectangle. The current simplified
// decoder does not recover B/C vertices, so a rectangle that excludes
// only the A-vertex is a faithful proxy for the per-doc rejection
// path.
func TestXYShapeQuery_SpatialVisitor_TriangleBranchOutside(t *testing.T) {
	t.Parallel()
	// Restrict the query rectangle to a slice that does NOT cover
	// the origin (which is where B/C decode to under the simplified
	// decoder). Both A and B/C must miss for the triangle predicates
	// to return false reliably.
	rect := testXYRect(t, 20, 30, 20, 30)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeXYTriangleAVertex(t, 0, 0)

	if visitor.Intersects()(packed) {
		t.Fatalf("Intersects: triangle outside query rectangle should not match")
	}
}

// TestXYShapeQuery_SpatialVisitor_DecodeError surfaces the visitor's
// resilience to a malformed packed payload (wrong length). The
// Intersects, Within, and Contains predicates must return false (or
// WithinDisjoint for Contains) rather than panic.
func TestXYShapeQuery_SpatialVisitor_DecodeError(t *testing.T) {
	t.Parallel()
	rect := testXYRect(t, -1, 1, -1, 1)
	q, err := NewXYShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewXYShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	junk := []byte{0x01, 0x02, 0x03}
	if visitor.Intersects()(junk) {
		t.Fatalf("Intersects: malformed payload should not match")
	}
	if visitor.Within()(junk) {
		t.Fatalf("Within: malformed payload should not match")
	}
	if got := visitor.Contains()(junk); got != geo.WithinDisjoint {
		t.Fatalf("Contains: malformed payload should be DISJOINT, got %v", got)
	}

// encodeXYTriangleAVertex builds a 28-byte ShapeField payload whose
// A-vertex encodes the supplied (x, y) in the sortable-int wire
// format. The current simplified decoder only recovers A; B and C
// decode to the origin (0, 0) regardless of the encoded values.
// Tests that exercise the visitor's TRIANGLE branch should choose
// query rectangles that either cover the origin (positive hit) or
// exclude both the A-vertex and the origin (negative hit).
}
func encodeXYTriangleAVertex(t *testing.T, x, y float32) []byte {
	t.Helper()
	ay := geo.XYEncode(y)
	ax := geo.XYEncode(x)
	buf, err := document.EncodeTriangle(ax, ay, ax, ay, ax, ay, true, true, true)
	if err != nil {
		t.Fatalf("EncodeTriangle: %v", err)
	}
	return buf
}

// encodeXYCellBounds builds the (minPackedTriangle, maxPackedTriangle)
// pair the Relate hook expects, for the XY (cartesian) layout. Only
// four of the seven dimensions matter for relate; the remaining
// three carry edge data that is ignored by the cell-relate code
// path. Y comes first to mirror Lucene's XYShape BKD ordering.
func encodeXYCellBounds(t *testing.T, minX, maxX, minY, maxY float32) ([]byte, []byte) {
	t.Helper()
	const stride = document.ShapeFieldBytes / 7
	minBuf := make([]byte, document.ShapeFieldBytes)
	maxBuf := make([]byte, document.ShapeFieldBytes)

	writeSortableInt32BE(minBuf, 0, util.FloatToSortableInt(minY))
	writeSortableInt32BE(minBuf, stride, util.FloatToSortableInt(minX))
	writeSortableInt32BE(maxBuf, 2*stride, util.FloatToSortableInt(maxY))
	writeSortableInt32BE(maxBuf, 3*stride, util.FloatToSortableInt(maxX))

	// Sanity guard: minBuf and maxBuf must differ for the chosen
	// dimensions or the test would silently pass on a stub.
	if bytes.Equal(minBuf[:2*stride], maxBuf[:2*stride]) {
		t.Fatalf("encodeXYCellBounds: min and max share the lower-bound prefix")
	}
	return minBuf, maxBuf
}