// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestNewLatLonShapeBoundingBoxQuery_BasicConstruction confirms the
// happy-path for a non-wrapping rectangle: builds the parent
// SpatialQuery, stores the rectangle, and exposes the
// queryComponent2D the parent uses for identity.
func TestNewLatLonShapeBoundingBoxQuery_BasicConstruction(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -20, 20)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
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
	if q.GetRectangle() != rect {
		t.Fatalf("GetRectangle: got %v, want %v", q.GetRectangle(), rect)
	}
}

// TestNewLatLonShapeBoundingBoxQuery_RejectsEmptyField confirms the
// parent's empty-field guard surfaces through the bounding-box
// constructor.
func TestNewLatLonShapeBoundingBoxQuery_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, 0, 1, 0, 1)
	if _, err := NewLatLonShapeBoundingBoxQuery("", document.QueryRelationIntersects, rect); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestNewLatLonShapeBoundingBoxQuery_AllRelations confirms the
// constructor accepts every QueryRelation value (no per-relation
// restrictions, unlike LatLonShapeQuery's WITHIN+Line rejection).
func TestNewLatLonShapeBoundingBoxQuery_AllRelations(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -1, 1)
	for _, rel := range []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationContains,
		document.QueryRelationDisjoint,
	} {
		rel := rel
		t.Run(rel.String(), func(t *testing.T) {
			t.Parallel()
			if _, err := NewLatLonShapeBoundingBoxQuery("shape", rel, rect); err != nil {
				t.Fatalf("relation %v: unexpected error %v", rel, err)
			}
		})
	}
}

// TestLatLonShapeBoundingBoxQuery_Equals_SameInputs asserts that two
// queries built from the same field/relation/rectangle compare equal
// and hash to the same value.
func TestLatLonShapeBoundingBoxQuery_Equals_SameInputs(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery a: %v", err)
	}
	b, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery b: %v", err)
	}
	if !a.Equals(b) {
		t.Fatalf("equal queries should compare equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal queries should hash to the same value: %d vs %d",
			a.HashCode(), b.HashCode())
	}
}

// TestLatLonShapeBoundingBoxQuery_Equals_DiffersByField confirms
// that distinct field names break equality.
func TestLatLonShapeBoundingBoxQuery_Equals_DiffersByField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeBoundingBoxQuery("shape_a", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery a: %v", err)
	}
	b, err := NewLatLonShapeBoundingBoxQuery("shape_b", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery b: %v", err)
	}
	if a.Equals(b) {
		t.Fatalf("queries with different fields should not compare equal")
	}
}

// TestLatLonShapeBoundingBoxQuery_Equals_DiffersByRelation confirms
// that different query relations break equality.
func TestLatLonShapeBoundingBoxQuery_Equals_DiffersByRelation(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	a, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery a: %v", err)
	}
	b, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationContains, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery b: %v", err)
	}
	if a.Equals(b) {
		t.Fatalf("queries with different relations should not compare equal")
	}
}

// TestLatLonShapeBoundingBoxQuery_Equals_DiffersByRectangle confirms
// that distinct rectangle bounds break equality.
func TestLatLonShapeBoundingBoxQuery_Equals_DiffersByRectangle(t *testing.T) {
	t.Parallel()
	a, err := NewLatLonShapeBoundingBoxQuery(
		"shape", document.QueryRelationIntersects, testLatLonRect(t, -1, 1, -2, 2),
	)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery a: %v", err)
	}
	b, err := NewLatLonShapeBoundingBoxQuery(
		"shape", document.QueryRelationIntersects, testLatLonRect(t, -3, 3, -2, 2),
	)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery b: %v", err)
	}
	if a.Equals(b) {
		t.Fatalf("queries with different rectangles should not compare equal")
	}
}

// TestLatLonShapeBoundingBoxQuery_Equals_NotSameType confirms the
// Equals override rejects a non-bounding-box query, even when the
// underlying SpatialQuery would otherwise compare equal.
func TestLatLonShapeBoundingBoxQuery_Equals_NotSameType(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	bbox, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	shape, err := NewLatLonShapeQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeQuery: %v", err)
	}
	if bbox.Equals(shape) {
		t.Fatalf("bounding-box query should not equal a generic shape query")
	}
}

// TestLatLonShapeBoundingBoxQuery_String_DefaultField asserts the
// toString output prefixes "LatLonShapeBoundingBoxQuery:" when the
// default field matches the query's field, and renders the
// rectangle via its own String form.
func TestLatLonShapeBoundingBoxQuery_String_DefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	got := q.String("shape")
	if !strings.HasPrefix(got, "LatLonShapeBoundingBoxQuery:") {
		t.Fatalf("String(default field): prefix mismatch: %q", got)
	}
	if strings.Contains(got, "field=") {
		t.Fatalf("String(default field): should not emit field=: %q", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_String_DiffersFromDefaultField
// checks that the toString output prepends "field=<field>:" when the
// supplied default field differs from the query's field.
func TestLatLonShapeBoundingBoxQuery_String_DiffersFromDefaultField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	got := q.String("other")
	if !strings.Contains(got, "field=shape:") {
		t.Fatalf("String(non-default field): missing field= clause: %q", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_Visit_AcceptedField asserts that
// Visit invokes VisitLeaf when the QueryVisitor accepts the bound
// field.
func TestLatLonShapeBoundingBoxQuery_Visit_AcceptedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: true}
	q.Visit(v)
	if !v.leafCalled {
		t.Fatalf("Visit: VisitLeaf was not called for accepted field")
	}
}

// TestLatLonShapeBoundingBoxQuery_Visit_RejectedField asserts that
// Visit suppresses the VisitLeaf call when the visitor rejects the
// field.
func TestLatLonShapeBoundingBoxQuery_Visit_RejectedField(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -2, 2)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	v := &recordingShapeVisitor{accept: false}
	q.Visit(v)
	if v.leafCalled {
		t.Fatalf("Visit: VisitLeaf should not be called when field is rejected")
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Inside
// confirms the Relate hook classifies a fully-inside cell as
// CELL_INSIDE_QUERY. The cell is built with sortable bytes matching
// the BKD layout the production code path consumes.
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Inside(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	minBuf, maxBuf := encodeCellBounds(t, -1, 1, -1, 1)
	if got := visitor.Relate(minBuf, maxBuf); got != spatialCellInsideQuery {
		t.Fatalf("Relate inside-cell: got %v, want CELL_INSIDE_QUERY", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Outside
// confirms the Relate hook rejects a cell strictly north of the
// query rectangle.
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Outside(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	minBuf, maxBuf := encodeCellBounds(t, 50, 60, -1, 1)
	if got := visitor.Relate(minBuf, maxBuf); got != spatialCellOutsideQuery {
		t.Fatalf("Relate outside-cell: got %v, want CELL_OUTSIDE_QUERY", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Crosses
// confirms the Relate hook classifies a cell that straddles the
// eastern boundary as CELL_CROSSES_QUERY.
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Relate_Crosses(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	minBuf, maxBuf := encodeCellBounds(t, -1, 1, 5, 15)
	if got := visitor.Relate(minBuf, maxBuf); got != spatialCellCrossesQuery {
		t.Fatalf("Relate crossing-cell: got %v, want CELL_CROSSES_QUERY", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Intersects_Inside
// drives the visitor's TRIANGLE branch against a payload whose
// A-vertex (the only vertex the simplified decoder recovers) lies
// inside the query rectangle. The chosen rectangle also covers the
// origin so the (0, 0) decode of B/C does not cause a false negative.
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Intersects_Inside(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -10, 10, -10, 10)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeTriangleAVertex(t, 0, 0)

	if !visitor.Intersects()(packed) {
		t.Fatalf("Intersects: triangle inside rectangle should match")
	}
	if !visitor.Within()(packed) {
		t.Fatalf("Within: triangle inside rectangle should match")
	}
	if got := visitor.Contains()(packed); got == geo.WithinDisjoint {
		t.Fatalf("Contains: triangle inside rectangle should not be DISJOINT, got %v", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Intersects_Outside
// confirms the visitor rejects a TRIANGLE-kind payload whose
// A-vertex lies outside the query rectangle. Both A and the origin
// (which is where B/C decode to under the simplified decoder) must
// miss for the rejection to be reliable.
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_Intersects_Outside(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, 20, 30, 20, 30)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	packed := encodeTriangleAVertex(t, 0, 0)

	if visitor.Intersects()(packed) {
		t.Fatalf("Intersects: triangle outside rectangle should not match")
	}
}

// TestLatLonShapeBoundingBoxQuery_SpatialVisitor_DecodeError
// surfaces the visitor's resilience to a malformed packed payload
// (wrong length).
func TestLatLonShapeBoundingBoxQuery_SpatialVisitor_DecodeError(t *testing.T) {
	t.Parallel()
	rect := testLatLonRect(t, -1, 1, -1, 1)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationIntersects, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
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

// TestLatLonShapeBoundingBoxQuery_Contains_PanicsOnDatelineCross
// confirms the Contains() factory panics with
// ErrLatLonShapeBoundingBoxQueryContainsDateline when the underlying
// rectangle wraps the dateline. This mirrors the Java reference's
// eager IllegalArgumentException.
func TestLatLonShapeBoundingBoxQuery_Contains_PanicsOnDatelineCross(t *testing.T) {
	t.Parallel()
	// minLon=170 > maxLon=-170 wraps the dateline.
	rect := testLatLonRect(t, -10, 10, 170, -170)
	q, err := NewLatLonShapeBoundingBoxQuery("shape", document.QueryRelationContains, rect)
	if err != nil {
		t.Fatalf("NewLatLonShapeBoundingBoxQuery: %v", err)
	}
	visitor := q.GetSpatialVisitor()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Contains() on wrapping rectangle: expected panic")
		}
		err, ok := r.(error)
		if !ok || !errors.Is(err, ErrLatLonShapeBoundingBoxQueryContainsDateline) {
			t.Fatalf("Contains() panic: got %v, want ErrLatLonShapeBoundingBoxQueryContainsDateline", r)
		}
	}()
	_ = visitor.Contains()
}

// TestLatLonShapeBoundingBoxQuery_DatelineWrap_RectangleEncoding
// asserts that a dateline-crossing rectangle round-trips through the
// encoded form with two bbox payloads (east + west). This is a
// white-box check on the encoded rectangle plumbing.
func TestLatLonShapeBoundingBoxQuery_DatelineWrap_RectangleEncoding(t *testing.T) {
	t.Parallel()
	r := newEncodedLatLonRectangle(-10, 10, 170, -170)
	if !r.crossesDateline() {
		t.Fatalf("crossesDateline: expected true for wrapping rectangle")
	}
	if r.bbox == nil || r.west == nil {
		t.Fatalf("wrapping rectangle: expected both bbox and west to be populated, got bbox=%v west=%v",
			r.bbox != nil, r.west != nil)
	}
	if len(r.bbox) != 4*shapeFieldDimBytes || len(r.west) != 4*shapeFieldDimBytes {
		t.Fatalf("wrapping rectangle: bbox/west must be 16 bytes each, got %d/%d",
			len(r.bbox), len(r.west))
	}
}

// TestLatLonShapeBoundingBoxQuery_NonWrap_RectangleEncoding asserts
// that a non-wrapping rectangle exposes a single bbox payload and
// nil west.
func TestLatLonShapeBoundingBoxQuery_NonWrap_RectangleEncoding(t *testing.T) {
	t.Parallel()
	r := newEncodedLatLonRectangle(-10, 10, -20, 20)
	if r.crossesDateline() {
		t.Fatalf("crossesDateline: expected false for non-wrapping rectangle")
	}
	if r.bbox == nil {
		t.Fatalf("non-wrapping rectangle: bbox must be populated")
	}
	if r.west != nil {
		t.Fatalf("non-wrapping rectangle: west must be nil, got %v", r.west)
	}
}

// TestLatLonShapeBoundingBoxQuery_ValidateBoundingBoxMinLon covers
// the 180 == minLon special case mirrored from Java's validateMinLon.
func TestLatLonShapeBoundingBoxQuery_ValidateBoundingBoxMinLon(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		minLon float64
		maxLon float64
		want   float64
	}{
		{"non_wrap", -50.0, 50.0, -50.0},
		{"wrap_normal", 170.0, -170.0, 170.0},
		{"min_180_collapses_to_neg180", 180.0, -170.0, -180.0},
		{"min_180_not_collapsing", 180.0, 180.0, 180.0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := validateBoundingBoxMinLon(c.minLon, c.maxLon); got != c.want {
				t.Fatalf("validateBoundingBoxMinLon(%v, %v): got %v, want %v",
					c.minLon, c.maxLon, got, c.want)
			}
		})
	}
}

// TestLatLonShapeBoundingBoxQuery_RelationToSpatial confirms the
// exhaustive switch between pointRelation and spatialRelation.
func TestLatLonShapeBoundingBoxQuery_RelationToSpatial(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   pointRelation
		want spatialRelation
	}{
		{pointCellInsideQuery, spatialCellInsideQuery},
		{pointCellOutsideQuery, spatialCellOutsideQuery},
		{pointCellCrossesQuery, spatialCellCrossesQuery},
	}
	for _, c := range cases {
		if got := relationToSpatial(c.in); got != c.want {
			t.Fatalf("relationToSpatial(%v): got %v, want %v", c.in, got, c.want)
		}
	}
}

// TestLatLonShapeBoundingBoxQuery_CompareBBoxToRangeBBox covers the
// three classification branches (inside, outside, crosses) of the
// byte-level comparator. The bbox is encoded with the same
// IntToSortableBytes layout the production constructor uses.
func TestLatLonShapeBoundingBoxQuery_CompareBBoxToRangeBBox(t *testing.T) {
	t.Parallel()
	// Build a bbox covering [minY=-10, maxY=10] × [minX=-20, maxX=20]
	// in raw int32 (no encoding necessary — compareBBoxToRangeBBox
	// works on any sortable-bytes input).
	bbox := make([]byte, 4*shapeFieldDimBytes)
	encodeBoundingBoxBytes(-20, 20, -10, 10, bbox)

	inside := buildTriangleCell(t, -5, 5, -5, 5)
	outside := buildTriangleCell(t, 50, 60, -5, 5)
	crosses := buildTriangleCell(t, -5, 5, 15, 25)

	if got := compareBBoxToRangeBBox(
		bbox, shapeFieldDimBytes, 0, inside.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, inside.max,
	); got != pointCellInsideQuery {
		t.Fatalf("inside: got %v, want INSIDE", got)
	}
	if got := compareBBoxToRangeBBox(
		bbox, shapeFieldDimBytes, 0, outside.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, outside.max,
	); got != pointCellOutsideQuery {
		t.Fatalf("outside: got %v, want OUTSIDE", got)
	}
	if got := compareBBoxToRangeBBox(
		bbox, shapeFieldDimBytes, 0, crosses.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, crosses.max,
	); got != pointCellCrossesQuery {
		t.Fatalf("crosses: got %v, want CROSSES", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_IntersectBBoxWithRangeBBox covers
// the inside/outside paths of the looser INTERSECTS classifier. The
// branch parity with the strict comparator is exercised here for the
// disjoint and inside-on-inside cases; the extra fast-path
// classifications are not asserted because they are tied to the
// looser Java semantics the Gocene Relate path does not consume yet.
func TestLatLonShapeBoundingBoxQuery_IntersectBBoxWithRangeBBox(t *testing.T) {
	t.Parallel()
	bbox := make([]byte, 4*shapeFieldDimBytes)
	encodeBoundingBoxBytes(-20, 20, -10, 10, bbox)

	outside := buildTriangleCell(t, 50, 60, -5, 5)
	inside := buildTriangleCell(t, -5, 5, -5, 5)

	if got := intersectBBoxWithRangeBBox(
		bbox, shapeFieldDimBytes, 0, outside.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, outside.max,
	); got != pointCellOutsideQuery {
		t.Fatalf("outside: got %v, want OUTSIDE", got)
	}
	if got := intersectBBoxWithRangeBBox(
		bbox, shapeFieldDimBytes, 0, inside.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, inside.max,
	); got != pointCellInsideQuery {
		t.Fatalf("inside: got %v, want INSIDE", got)
	}
}

// TestLatLonShapeBoundingBoxQuery_RelateRangeBBox_WrapsDateline
// confirms that a wrapping rectangle falls back to the west bbox
// when the east bbox classifies the cell as OUTSIDE. The chosen
// cell sits firmly in the western half so the eastern half rejects
// it and the western half accepts it as INSIDE-or-CROSSES.
//
// The cell bounds are passed in degrees and encoded through
// geo.Encode{Latitude,Longitude} so they match the encoded space the
// rectangle's bbox lives in.
func TestLatLonShapeBoundingBoxQuery_RelateRangeBBox_WrapsDateline(t *testing.T) {
	t.Parallel()
	// Rectangle wraps from minLon=170 to maxLon=-170 (a 20-degree
	// slice centred on the antimeridian). The west bbox covers
	// [MinLonEncoded .. encode(-170)]; the east bbox covers
	// [encode(170) .. MaxLonEncoded].
	r := newEncodedLatLonRectangle(-10, 10, 170, -170)

	// Cell at lat [-1, 1], lon [-179, -175]: lies inside the
	// western half but outside the eastern half.
	cell := buildEncodedTriangleCell(t, -1, 1, -179, -175)
	got := r.relateRangeBBox(
		shapeFieldDimBytes, 0, cell.min,
		3*shapeFieldDimBytes, 2*shapeFieldDimBytes, cell.max,
	)
	if got != pointCellInsideQuery && got != pointCellCrossesQuery {
		t.Fatalf("wrapping rectangle, western cell: got %v, want INSIDE or CROSSES", got)
	}

// triangleCell is a holder for the (min, max) byte payloads of a
// triangle-range cell, used by the byte-comparator tests above.
type triangleCell struct {
	min []byte
	max []byte
}

// buildTriangleCell encodes the four bbox dims of a 7-dim
// triangle-range cell at the bbox offsets the visitor uses. The
// edge dims (4..6) are zeroed. The input values are taken as
// already-encoded int32 — useful for white-box tests that operate
// directly on the sortable-bytes layout.
}
func buildTriangleCell(t *testing.T, minLat, maxLat, minLon, maxLon int32) triangleCell {
	t.Helper()
	const stride = shapeFieldDimBytes
	minBuf := make([]byte, document.ShapeFieldBytes)
	maxBuf := make([]byte, document.ShapeFieldBytes)

	writeSortableInt32BE(minBuf, 0, minLat)        // dim 0 = minY
	writeSortableInt32BE(minBuf, stride, minLon)   // dim 1 = minX
	writeSortableInt32BE(maxBuf, 2*stride, maxLat) // dim 2 = maxY
	writeSortableInt32BE(maxBuf, 3*stride, maxLon) // dim 3 = maxX
	return triangleCell{min: minBuf, max: maxBuf}
}

// buildEncodedTriangleCell mirrors buildTriangleCell but takes
// degree-space coordinates and runs them through
// geo.Encode{Latitude,Longitude} first, so the produced bytes live
// in the same encoded space the production constructor uses for the
// rectangle bbox.
func buildEncodedTriangleCell(t *testing.T, minLat, maxLat, minLon, maxLon float64) triangleCell {
	t.Helper()
	return buildTriangleCell(
		t,
		geo.EncodeLatitudeCeil(minLat),
		geo.EncodeLatitude(maxLat),
		geo.EncodeLongitudeCeil(minLon),
		geo.EncodeLongitude(maxLon),
	)
}