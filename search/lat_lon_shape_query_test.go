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
)

// testLatLonRect builds a rectangle covering [minLat..maxLat] ×
// [minLon..maxLon]; it panics on validation error so tests stay
// terse.
func testLatLonRect(t *testing.T, minLat, maxLat, minLon, maxLon float64) geo.Rectangle {
	t.Helper()
	r, err := geo.NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		t.Fatalf("geo.NewRectangle: %v", err)
	}
	return r
}

// testLatLonLine builds a Line from parallel lats/lons; panics on
// validation error.
func testLatLonLine(t *testing.T, lats, lons []float64) geo.Line {
	t.Helper()
	l, err := geo.NewLine(lats, lons)
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	return l
}

// TestNewLatLonShapeQuery_BasicConstruction confirms the happy path
// for every non-Line geometry: builds the parent SpatialQuery,
// stores the field/relation/geometry triple, and exposes the
// queryComponent2D.
func TestNewLatLonShapeQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -20, 20)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
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

// TestNewLatLonShapeQuery_RejectsWithinLine confirms that
// constructing a WITHIN query over a Line geometry surfaces
// ErrLatLonShapeQueryWithinLine, mirroring the Java reference's
// IllegalArgumentException.
func TestNewLatLonShapeQuery_RejectsWithinLine(t *testing.T) {
	t.Parallel()
	line := testLatLonLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	if _, err := NewLatLonShapeQuery(
		"shape",
		document.QueryRelationWithin,
		line,
	); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
}

// TestNewLatLonShapeQuery_RejectsWithinLinePtr mirrors the
// value-form test for a pointer-to-Line input, covering the
// type-assertion branch that handles *geo.Line.
func TestNewLatLonShapeQuery_RejectsWithinLinePtr(t *testing.T) {
	t.Parallel()
	line := testLatLonLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	if _, err := NewLatLonShapeQuery(
		"shape",
		document.QueryRelationWithin,
		&line,
	); !errors.Is(err, ErrLatLonShapeQueryWithinLine) {
		t.Fatalf("WITHIN+*Line: expected ErrLatLonShapeQueryWithinLine, got %v", err)
	}
}

// TestNewLatLonShapeQuery_AllowsLineForOtherRelations confirms that
// non-WITHIN relations accept Line geometries — only WITHIN is
// blacklisted.
func TestNewLatLonShapeQuery_AllowsLineForOtherRelations(t *testing.T) {
	t.Parallel()
	line := testLatLonLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewLatLonShapeQuery("shape", rel, line); err != nil {
				t.Fatalf("%v + Line: unexpected error %v", rel, err)
			}
		})
	}
}

// TestNewLatLonShapeQuery_RejectsEmptyField confirms the parent's
// empty-field guard surfaces through the LatLonShape constructor.
func TestNewLatLonShapeQuery_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, 0, 1, 0, 1)
	if _, err := NewLatLonShapeQuery("", document.QueryRelationIntersects, rect); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestNewLatLonShapeQuery_RejectsEmptyGeometries confirms that an
// empty geometries slice surfaces through the Component2D builder
// (the Java reference also rejects empty input via
// LatLonGeometry.create).
func TestNewLatLonShapeQuery_RejectsEmptyGeometries(t *testing.T) {
	t.Parallel()
	if _, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}

// TestLatLonShapeQuery_Equals_SameInputs asserts that two
// LatLonShapeQuery values built from the same field, relation, and
// geometry list compare equal under the parent's Equals contract.
func TestLatLonShapeQuery_Equals_SameInputs(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery a: %v", err)
	}
	b, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery b: %v", err)
	}
	if !a.Equals(b.SpatialQuery) {
		t.Fatalf("equal queries should compare equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal queries should hash to the same value: %d vs %d",
			a.HashCode(), b.HashCode())
	}
}

// TestLatLonShapeQuery_Equals_DiffersByField confirms that
// distinct field names break equality even for otherwise identical
// queries.
func TestLatLonShapeQuery_Equals_DiffersByField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeQuery("shape_a", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery a: %v", err)
	}
	b, err := NewLatLonShapeQuery("shape_b", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestLatLonShapeQuery_Equals_DiffersByRelation confirms that
// different query relations break equality.
func TestLatLonShapeQuery_Equals_DiffersByRelation(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery a: %v", err)
	}
	b, err := NewLatLonShapeQuery("shape", document.QueryRelationContains, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different relations should not compare equal")
	}
}

// TestLatLonShapeQuery_String_DefaultField asserts the toString
// output prefixes "LatLonShapeQuery:" when the default field
// matches the query's field, mirroring the Java reference's
// "<ClassName>:[geom,]" layout.
func TestLatLonShapeQuery_String_DefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	got := q.String("shape")
	if !strings.HasPrefix(got, "LatLonShapeQuery:") {
		t.Fatalf("String(default field): prefix mismatch: %q", got)
	}
	if strings.Contains(got, "field=") {
		t.Fatalf("String(default field): should not emit field=: %q", got)
	}
}

// TestLatLonShapeQuery_String_DiffersFromDefaultField checks that
// the toString output prepends "field=<field>:" when the supplied
// default field differs from the query's field, mirroring the Java
// reference's SpatialQuery.toString(String).
func TestLatLonShapeQuery_String_DiffersFromDefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	got := q.String("other")
	if !strings.Contains(got, "field=shape:") {
		t.Fatalf("String(non-default field): missing field= clause: %q", got)
	}
}

// TestLatLonShapeQuery_Visit_AcceptedField asserts that Visit
// invokes VisitLeaf when the QueryVisitor accepts the bound field.
func TestLatLonShapeQuery_Visit_AcceptedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestLatLonShapeQuery_Visit_RejectedField asserts that Visit
// suppresses the VisitLeaf call when the visitor rejects the
// field.
func TestLatLonShapeQuery_Visit_RejectedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called when field is rejected")
	}
}

// recordingShapeVisitor is the minimal QueryVisitor stub used by
// the Visit tests above. It records whether VisitLeaf was called
// and routes every other hook to a no-op.
type recordingShapeVisitor struct {
	accept     bool
	leafCalled bool
}

func (v *recordingShapeVisitor) AcceptField(_ string) bool                   { return v.accept }
func (v *recordingShapeVisitor) VisitLeaf(_ Query)                           { v.leafCalled = true }
func (v *recordingShapeVisitor) GetSubVisitor(_ Occur, _ Query) QueryVisitor { return v }
func (v *recordingShapeVisitor) ConsumeTerms(_ Query, _ ...*index.Term)      {}
func (v *recordingShapeVisitor) ConsumeTermsMatching(_ Query, _ string, _ func() ByteRunAutomaton) {
}

// TestLatLonShapeQuery_SpatialVisitor_Relate confirms the Relate
// hook decodes the 4-corner cell layout correctly and forwards to
// the underlying Component2D. An "inside" cell exercises the
// CellInsideQuery branch; a disjoint cell exercises
// CellOutsideQuery; a crossing cell exercises CellCrossesQuery.
func TestLatLonShapeQuery_SpatialVisitor_Relate(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	// A cell strictly inside the 20°×20° query rectangle.
	insideMin, insideMax := encodeCellBounds(t, -1, 1, -1, 1)
	if got := visitor.Relate(insideMin, insideMax); got != spatialCellInsideQuery {
		t.Fatalf("Relate inside-cell: got %v, want CELL_INSIDE_QUERY", got)
	}
	// A cell entirely north of the query rectangle.
	outsideMin, outsideMax := encodeCellBounds(t, 50, 60, -1, 1)
	if got := visitor.Relate(outsideMin, outsideMax); got != spatialCellOutsideQuery {
		t.Fatalf("Relate outside-cell: got %v, want CELL_OUTSIDE_QUERY", got)
	}
	// A cell that straddles the query rectangle's eastern boundary.
	crossMin, crossMax := encodeCellBounds(t, -1, 1, 5, 15)
	if got := visitor.Relate(crossMin, crossMax); got != spatialCellCrossesQuery {
		t.Fatalf("Relate crossing-cell: got %v, want CELL_CROSSES_QUERY", got)
	}
}

// TestLatLonShapeQuery_SpatialVisitor_TriangleBranchInside drives
// the visitor's TRIANGLE branch against a payload whose A-vertex
// (the only vertex the current simplified decoder recovers) lies
// inside the query rectangle. The remaining vertices decode to the
// origin (0, 0) under the current Gocene decoder; the chosen
// rectangle covers the origin so the test stays decoder-agnostic.
//
// Intersects/Within/Contains all exercise the same TRIANGLE branch
// because document.DecodeTriangle currently classifies every
// payload as TRIANGLE-kind (the rotation-aware POINT/LINE
// classification is deferred to backlog #2697).
func TestLatLonShapeQuery_SpatialVisitor_TriangleBranchInside(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeTriangleAVertex(t, 0, 0)

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

// TestLatLonShapeQuery_SpatialVisitor_TriangleBranchOutside confirms
// the visitor's Intersects branch rejects a TRIANGLE-kind payload
// whose A-vertex lies outside the query rectangle. The current
// simplified decoder does not recover B/C vertices, so a rectangle
// that excludes only the A-vertex is a faithful proxy for the
// per-doc rejection path.
func TestLatLonShapeQuery_SpatialVisitor_TriangleBranchOutside(t *testing.T) {
	t.Parallel()
	// Restrict the query rectangle to a slice that does NOT cover
	// the origin (which is where B/C decode to under the simplified
	// decoder). Both A and B/C must miss for the triangle predicates
	// to return false reliably.
	rect := testLatLonRect(t, 20, 30, 20, 30)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeTriangleAVertex(t, 0, 0)

	if visitor.Intersects()(packed) {
		t.Fatalf("Intersects: triangle outside query rectangle should not match")
	}
}

// TestLatLonShapeQuery_SpatialVisitor_DecodeError surfaces the
// visitor's resilience to a malformed packed payload (wrong length).
// The Intersects, Within, and Contains predicates must return
// false (or WithinDisjoint for Contains) rather than panic.
func TestLatLonShapeQuery_SpatialVisitor_DecodeError(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -1, 1)
	q, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
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
}

// TestLatLonShapeQuery_GeoRelationToSpatial confirms the three
// in-range geo.Relation values map to the matching internal
// spatialRelation constants.
func TestLatLonShapeQuery_GeoRelationToSpatial(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   geo.Relation
		want spatialRelation
	}{
		{geo.CellInsideQuery, spatialCellInsideQuery},
		{geo.CellOutsideQuery, spatialCellOutsideQuery},
		{geo.CellCrossesQuery, spatialCellCrossesQuery},
	}
	for _, c := range cases {
		if got := geoRelationToSpatial(c.in); got != c.want {
			t.Fatalf("geoRelationToSpatial(%v): got %v, want %v", c.in, got, c.want)
		}
	}
}

// encodeTriangleAVertex builds a 28-byte ShapeField payload whose
// A-vertex encodes the supplied (lat, lon). The current
// simplified decoder only recovers A; B and C decode to the origin
// (0, 0) regardless of the encoded values. Tests that exercise the
// visitor's TRIANGLE branch should choose query rectangles that
// either cover the origin (when verifying a positive hit) or that
// exclude both the A-vertex and the origin (when verifying a
// negative hit).
func encodeTriangleAVertex(t *testing.T, lat, lon float64) []byte {
	t.Helper()
	ay := geo.EncodeLatitude(lat)
	ax := geo.EncodeLongitude(lon)
	buf, err := document.EncodeTriangle(ax, ay, ax, ay, ax, ay, true, true, true)
	if err != nil {
		t.Fatalf("EncodeTriangle: %v", err)
	}
	return buf
}

// encodeCellBounds builds the (minPackedTriangle, maxPackedTriangle)
// pair the Relate hook expects. Only four of the seven dimensions
// matter for relate; the remaining three carry edge data that is
// ignored by the cell-relate code path.
func encodeCellBounds(t *testing.T, minLat, maxLat, minLon, maxLon float64) ([]byte, []byte) {
	t.Helper()
	const stride = document.ShapeFieldBytes / 7
	minBuf := make([]byte, document.ShapeFieldBytes)
	maxBuf := make([]byte, document.ShapeFieldBytes)

	writeSortableInt32BE(minBuf, 0, geo.EncodeLatitude(minLat))
	writeSortableInt32BE(minBuf, stride, geo.EncodeLongitude(minLon))
	writeSortableInt32BE(maxBuf, 2*stride, geo.EncodeLatitude(maxLat))
	writeSortableInt32BE(maxBuf, 3*stride, geo.EncodeLongitude(maxLon))

	// Sanity guard: minBuf and maxBuf must differ for the chosen
	// dimensions or the test would silently pass on a stub.
	if bytes.Equal(minBuf[:2*stride], maxBuf[:2*stride]) {
		t.Fatalf("encodeCellBounds: min and max share the lower-bound prefix")
	}
	return minBuf, maxBuf
}

// writeSortableInt32BE writes value at buf[off..off+4] using the
// sortable big-endian transform NumericUtils.intToSortableBytes
// uses on the Java reference. The transform XORs the sign bit so
// negative values sort below positives in byte-lex order, matching
// what util.SortableBytesToInt undoes during decode.
func writeSortableInt32BE(buf []byte, off int, value int32) {
	u := uint32(value) ^ 0x80000000
	buf[off+0] = byte(u >> 24)
	buf[off+1] = byte(u >> 16)
	buf[off+2] = byte(u >> 8)
	buf[off+3] = byte(u)
}
