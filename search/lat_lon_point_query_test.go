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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testLatLonPointGeo returns a geo.Point with the given (lat, lon),
// panicking on validation failure so tests stay terse.
func testLatLonPointGeo(t *testing.T, lat, lon float64) geo.Point {
	t.Helper()
	p, err := geo.NewPoint(lat, lon)
	if err != nil {
		t.Fatalf("geo.NewPoint(%v, %v): %v", lat, lon, err)
	}
	return p
}

// testLatLonPointRect returns a geo.Rectangle covering the supplied
// lat/lon range; panics on validation failure.
func testLatLonPointRect(t *testing.T, minLat, maxLat, minLon, maxLon float64) geo.Rectangle {
	t.Helper()
	r, err := geo.NewRectangle(minLat, maxLat, minLon, maxLon)
	if err != nil {
		t.Fatalf("geo.NewRectangle: %v", err)
	}
	return r
}

// testLatLonPointLine returns a geo.Line built from the supplied
// parallel lats/lons; panics on validation failure.
func testLatLonPointLine(t *testing.T, lats, lons []float64) geo.Line {
	t.Helper()
	l, err := geo.NewLine(lats, lons)
	if err != nil {
		t.Fatalf("geo.NewLine: %v", err)
	}
	return l
}

// TestNewLatLonPointQuery_BasicConstruction covers the happy path
// for a single rectangle geometry: the parent SpatialQuery is built
// and the field/relation/geometry triple is exposed unchanged.
func TestNewLatLonPointQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -20, 20)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	if got := q.GetField(); got != "point" {
		t.Fatalf("GetField: got %q, want %q", got, "point")
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

// TestNewLatLonPointQuery_RejectsWithinLine confirms that a WITHIN
// query over a Line geometry surfaces ErrLatLonPointQueryWithinLine,
// mirroring the Java reference's IllegalArgumentException for
// (WITHIN, Line).
func TestNewLatLonPointQuery_RejectsWithinLine(t *testing.T) {
	t.Parallel()
	line := testLatLonPointLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationWithin,
		line,
	); !errors.Is(err, ErrLatLonPointQueryWithinLine) {
		t.Fatalf("WITHIN+Line: expected ErrLatLonPointQueryWithinLine, got %v", err)
	}
}

// TestNewLatLonPointQuery_RejectsWithinLinePtr mirrors the value
// test for *geo.Line, covering the second type assertion branch.
func TestNewLatLonPointQuery_RejectsWithinLinePtr(t *testing.T) {
	t.Parallel()
	line := testLatLonPointLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationWithin,
		&line,
	); !errors.Is(err, ErrLatLonPointQueryWithinLine) {
		t.Fatalf("WITHIN+*Line: expected ErrLatLonPointQueryWithinLine, got %v", err)
	}
}

// TestNewLatLonPointQuery_RejectsContainsRect confirms that a
// CONTAINS query over a non-Point geometry (here a Rectangle)
// surfaces ErrLatLonPointQueryContainsNonPoint, mirroring the Java
// reference's IllegalArgumentException for the same case.
func TestNewLatLonPointQuery_RejectsContainsRect(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationContains,
		rect,
	); !errors.Is(err, ErrLatLonPointQueryContainsNonPoint) {
		t.Fatalf("CONTAINS+Rect: expected ErrLatLonPointQueryContainsNonPoint, got %v", err)
	}
}

// TestNewLatLonPointQuery_RejectsContainsLine covers the second
// non-Point geometry kind (Line) for the CONTAINS relation, again
// mirroring the Java reference's validateGeometry branch.
func TestNewLatLonPointQuery_RejectsContainsLine(t *testing.T) {
	t.Parallel()
	line := testLatLonPointLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationContains,
		line,
	); !errors.Is(err, ErrLatLonPointQueryContainsNonPoint) {
		t.Fatalf("CONTAINS+Line: expected ErrLatLonPointQueryContainsNonPoint, got %v", err)
	}
}

// TestNewLatLonPointQuery_AcceptsContainsPoint confirms the
// CONTAINS branch is allowed when every geometry is a Point — both
// value and pointer shapes round-trip through validateGeometry.
func TestNewLatLonPointQuery_AcceptsContainsPoint(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 1, 1)
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationContains,
		pt,
	); err != nil {
		t.Fatalf("CONTAINS+Point (value): unexpected error %v", err)
	}
	if _, err := NewLatLonPointQuery(
		"point",
		document.QueryRelationContains,
		&pt,
	); err != nil {
		t.Fatalf("CONTAINS+*Point: unexpected error %v", err)
	}
}

// TestNewLatLonPointQuery_AllowsLineForNonWithin confirms that
// non-WITHIN relations accept Line geometries (mirroring the
// Java reference's per-relation gating).
func TestNewLatLonPointQuery_AllowsLineForNonWithin(t *testing.T) {
	t.Parallel()
	line := testLatLonPointLine(t, []float64{0, 1, 2}, []float64{0, 1, 2})
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewLatLonPointQuery("point", rel, line); err != nil {
				t.Fatalf("%v + Line: unexpected error %v", rel, err)
			}
		})
	}
}

// TestNewLatLonPointQuery_RejectsEmptyField confirms the empty
// field guard inherited from NewSpatialQuery surfaces unchanged.
func TestNewLatLonPointQuery_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, 0, 1, 0, 1)
	if _, err := NewLatLonPointQuery("", document.QueryRelationIntersects, rect); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestNewLatLonPointQuery_RejectsEmptyGeometries confirms an empty
// geometries slice surfaces as an error from
// geo.CreateLatLonGeometry.
func TestNewLatLonPointQuery_RejectsEmptyGeometries(t *testing.T) {
	t.Parallel()
	if _, err := NewLatLonPointQuery("point", document.QueryRelationIntersects); err == nil {
		t.Fatalf("expected error on empty geometries")
	}
}

// TestLatLonPointQuery_Equals_SameInputs asserts queries built from
// the same field/relation/geometry triple compare equal and hash
// to the same value.
func TestLatLonPointQuery_Equals_SameInputs(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -2, 2)
	a, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
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

// TestLatLonPointQuery_Equals_DiffersByField confirms a different
// field name breaks equality.
func TestLatLonPointQuery_Equals_DiffersByField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -2, 2)
	a, err := NewLatLonPointQuery("point_a", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point_b", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestLatLonPointQuery_Equals_DiffersByRelation confirms a different
// relation breaks equality.
func TestLatLonPointQuery_Equals_DiffersByRelation(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -2, 2)
	a, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("point", document.QueryRelationDisjoint, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if a.Equals(b.SpatialQuery) {
		t.Fatalf("queries with different relations should not compare equal")
	}
}

// TestLatLonPointQuery_String_DefaultField confirms the toString
// output prefixes "LatLonPointQuery:" with no "field=" segment
// when the supplied default field matches the query's field.
func TestLatLonPointQuery_String_DefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -2, 2)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	got := q.String("point")
	if !strings.HasPrefix(got, "LatLonPointQuery:") {
		t.Fatalf("String(default field): prefix mismatch: %q", got)
	}
	if strings.Contains(got, "field=") {
		t.Fatalf("String(default field): should not emit field=: %q", got)
	}
}

// TestLatLonPointQuery_String_DiffersFromDefaultField confirms the
// toString output prepends "field=<field>:" when the supplied
// default field differs from the query's field.
func TestLatLonPointQuery_String_DiffersFromDefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -2, 2)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	got := q.String("other")
	if !strings.Contains(got, "field=point:") {
		t.Fatalf("String(non-default field): missing field= clause: %q", got)
	}
}

// TestLatLonPointQuery_Visit_AcceptedField confirms Visit invokes
// VisitLeaf when the QueryVisitor accepts the bound field.
func TestLatLonPointQuery_Visit_AcceptedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestLatLonPointQuery_Visit_RejectedField confirms Visit suppresses
// VisitLeaf when the visitor rejects the field.
func TestLatLonPointQuery_Visit_RejectedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called for rejected field")
	}
}

// TestLatLonPointQuery_SpatialVisitor_Relate exercises the three
// Relate branches: a cell fully inside the query rectangle, a cell
// disjoint from it, and a cell that crosses the eastern boundary.
//
// Cells are encoded using the same 8-byte LatLonPoint layout the
// Java reference's anonymous relate() consumes:
//
//	[0..4)  → sortable-bytes int32 latitude
//	[4..8)  → sortable-bytes int32 longitude
func TestLatLonPointQuery_SpatialVisitor_Relate(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	insideMin, insideMax := encodeLatLonCellBounds(t, -1, 1, -1, 1)
	if got := visitor.Relate(insideMin, insideMax); got != spatialCellInsideQuery {
		t.Fatalf("Relate inside: got %v, want CELL_INSIDE_QUERY", got)
	}
	outsideMin, outsideMax := encodeLatLonCellBounds(t, 50, 60, -1, 1)
	if got := visitor.Relate(outsideMin, outsideMax); got != spatialCellOutsideQuery {
		t.Fatalf("Relate outside (bbox-short-circuit): got %v, want CELL_OUTSIDE_QUERY", got)
	}
	crossMin, crossMax := encodeLatLonCellBounds(t, -1, 1, 5, 15)
	if got := visitor.Relate(crossMin, crossMax); got != spatialCellCrossesQuery {
		t.Fatalf("Relate crossing: got %v, want CELL_CROSSES_QUERY", got)
	}
}

// TestLatLonPointQuery_SpatialVisitor_Relate_LongitudeShortCircuit
// drives the second bbox short-circuit branch (longitude). Latitudes
// stay inside the query band; longitudes are fully east of the
// query rectangle. Mirrors the second outside-bbox check in the
// Java reference's anonymous relate().
func TestLatLonPointQuery_SpatialVisitor_Relate_LongitudeShortCircuit(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	minBuf, maxBuf := encodeLatLonCellBounds(t, -1, 1, 50, 60)
	if got := visitor.Relate(minBuf, maxBuf); got != spatialCellOutsideQuery {
		t.Fatalf("Relate (lon outside): got %v, want CELL_OUTSIDE_QUERY", got)
	}
}

// TestLatLonPointQuery_SpatialVisitor_Relate_RejectsShortPayload
// confirms a malformed (under-length) packed value is rejected as
// CELL_OUTSIDE_QUERY rather than panicking.
func TestLatLonPointQuery_SpatialVisitor_Relate_RejectsShortPayload(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()
	if got := visitor.Relate([]byte{0x01, 0x02}, []byte{0x03, 0x04}); got != spatialCellOutsideQuery {
		t.Fatalf("Relate short payload: got %v, want CELL_OUTSIDE_QUERY", got)
	}
}

// TestLatLonPointQuery_SpatialVisitor_Intersects_Inside packs a
// single (lat, lon) inside the query rectangle and asserts the
// Intersects predicate returns true. Mirrors the Java reference's
// anonymous intersects().
func TestLatLonPointQuery_SpatialVisitor_Intersects_Inside(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()
	packed := document.EncodeLatLon(1, 1)
	if !visitor.Intersects()(packed) {
		t.Fatalf("Intersects: point inside query rectangle should match")
	}
}

// TestLatLonPointQuery_SpatialVisitor_Intersects_Outside confirms
// the predicate rejects a point outside the query rectangle.
func TestLatLonPointQuery_SpatialVisitor_Intersects_Outside(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()
	packed := document.EncodeLatLon(50, 50)
	if visitor.Intersects()(packed) {
		t.Fatalf("Intersects: point outside query rectangle should not match")
	}
}

// TestLatLonPointQuery_SpatialVisitor_Within_MatchesIntersects
// confirms the Within hook produces the same boolean decision as
// Intersects for a point-shaped indexed value — there is no
// "partial" containment to distinguish, exactly as the Java
// reference collapses both predicates onto the same closure.
func TestLatLonPointQuery_SpatialVisitor_Within_MatchesIntersects(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -10, 10, -10, 10)
	q, err := NewLatLonPointQuery("point", document.QueryRelationWithin, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()
	inside := document.EncodeLatLon(1, 1)
	outside := document.EncodeLatLon(50, 50)
	if !visitor.Within()(inside) {
		t.Fatalf("Within: point inside query rectangle should match")
	}
	if visitor.Within()(outside) {
		t.Fatalf("Within: point outside query rectangle should not match")
	}
}

// TestLatLonPointQuery_SpatialVisitor_Contains_PointGeometry exercises
// the CONTAINS branch against a Point query (the only geometry kind
// CONTAINS allows). When the indexed point coincides with the query
// point, the Component2D.WithinPoint hook returns CANDIDATE; for a
// disjoint indexed point it returns DISJOINT.
func TestLatLonPointQuery_SpatialVisitor_Contains_PointGeometry(t *testing.T) {
	t.Parallel()
	pt := testLatLonPointGeo(t, 0, 0)
	q, err := NewLatLonPointQuery("point", document.QueryRelationContains, pt)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	coincident := document.EncodeLatLon(0, 0)
	if got := visitor.Contains()(coincident); got == geo.WithinDisjoint {
		t.Fatalf("Contains: coincident point should not be DISJOINT, got %v", got)
	}

	disjoint := document.EncodeLatLon(45, 45)
	if got := visitor.Contains()(disjoint); got != geo.WithinDisjoint {
		t.Fatalf("Contains: disjoint point should be DISJOINT, got %v", got)
	}
}

// TestLatLonPointQuery_SpatialVisitor_DecodeError ensures the three
// per-doc predicates survive a malformed packed payload (wrong
// length) without panicking. Intersects/Within return false;
// Contains returns DISJOINT.
func TestLatLonPointQuery_SpatialVisitor_DecodeError(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
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

// TestLatLonPointQuery_HashCode_FoldsField confirms that switching
// the field invalidates the hash, exercising the parent's class +
// field fold.
func TestLatLonPointQuery_HashCode_FoldsField(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	a, err := NewLatLonPointQuery("alpha", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery a: %v", err)
	}
	b, err := NewLatLonPointQuery("beta", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery b: %v", err)
	}
	if a.HashCode() == b.HashCode() {
		t.Fatalf("HashCode collision across different fields: %d", a.HashCode())
	}
}

// TestLatLonPointQuery_QueryIsCacheable_DefaultsTrue confirms the
// parent's default cacheability (true for every leaf) is preserved
// when no override is installed via WithSpatialQueryCacheableHook.
func TestLatLonPointQuery_QueryIsCacheable_DefaultsTrue(t *testing.T) {
	t.Parallel()
	rect := testLatLonPointRect(t, -1, 1, -1, 1)
	q, err := NewLatLonPointQuery("point", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonPointQuery: %v", err)
	}
	// QueryIsCacheable accepts a nil ctx because the default hook
	// is the constant predicate "true"; the production hook never
	// runs for this assertion.
	if !q.QueryIsCacheable(nil) {
		t.Fatalf("QueryIsCacheable: default should be true")
	}

// encodeLatLonCellBounds builds the (minPackedValue, maxPackedValue)
// pair the Relate hook expects: 8 bytes per buffer, 4-byte sortable
// latitude followed by 4-byte sortable longitude. Matches the Java
// reference's PointValues cell encoding for LatLonPoint.
func encodeLatLonCellBounds(t *testing.T, minLat, maxLat, minLon, maxLon float64) ([]byte, []byte) {
	t.Helper()
	const total = 2 * latLonPointBytesPerDim
	minBuf := make([]byte, total)
	maxBuf := make([]byte, total)
	util.IntToSortableBytes(geo.EncodeLatitude(minLat), minBuf, 0)
	util.IntToSortableBytes(geo.EncodeLongitude(minLon), minBuf, latLonPointBytesPerDim)
	util.IntToSortableBytes(geo.EncodeLatitude(maxLat), maxBuf, 0)
	util.IntToSortableBytes(geo.EncodeLongitude(maxLon), maxBuf, latLonPointBytesPerDim)
	if bytes.Equal(minBuf, maxBuf) {
		t.Fatalf("encodeLatLonCellBounds: min and max payloads coincide")
	}
	return minBuf, maxBuf
}