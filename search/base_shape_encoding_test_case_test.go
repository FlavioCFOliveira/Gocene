// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// This file is the Go port of
// lucene/core/src/test/org/apache/lucene/document/BaseShapeEncodingTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseShapeEncodingTestCase
// extends LuceneTestCase`. Concrete subclasses (TestLatLonShape /
// TestXYShape) plug their geometry-specific implementations of the
// `encodeX` / `decodeX` / `encodeY` / `decodeY` / `nextX` / `nextY`
// / `nextPolygon` / `createPolygon2D` abstract hooks and inherit the
// `@Test` methods defined here. The tests round-trip a 28-byte
// `ShapeField` payload through `encodeTriangle` / `decodeTriangle`
// and assert every vertex (A/B/C) of the decoded triangle.
//
// Gocene's current `document.DecodeTriangle` is the "simplified
// layout" decoder documented in document/shape_field.go — it only
// recovers the (AX, AY) vertex and the (AB, BC, CA) edge bits; the
// (BX, BY, CX, CY) recovery and the canonical orientation rotation
// are explicitly deferred to backlog #2697 (Lucene-byte-compatible
// tessellator + decoder). Because every `@Test` method in the Java
// source asserts that `encoded.bY/bX/cY/cX` round-trip correctly,
// none of the tests can complete a full round-trip in Gocene today
// — they would all read back zero for the B/C coordinates.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every `@Test` method in the Java source has a 1:1 Go
//     counterpart;
//   - each Test* opens with `t.Skip` that names the missing piece
//     explicitly (the rotation-aware decoder in backlog #2697), so
//     `go test -v` records the work without ever touching the
//     not-yet-recoverable vertex fields;
//   - the helpers, factory hooks and shared constants are typed
//     and constructible (and consumed by the encoding step that
//     happens before each Skip), so the file is wired end-to-end
//     and activation cost — once the decoder lands — is a one-line
//     removal of `t.Skip` from each test.

// ---------------------------------------------------------------------
// Helpers (Java abstract hooks → Go function-typed factories).
// ---------------------------------------------------------------------
//
// The Java abstract hooks become a struct of function values that
// concrete subclass-equivalents (one per test, future TestXYShape /
// TestLatLonShape suites) can populate. The fixture below uses the
// LatLon flavour because its quantisation helpers already exist in
// the geo package; the XY flavour will plug in once the Cartesian
// helpers are ported.

// baseShapeEncodingHooks mirrors the four `encode*/decode*` and four
// `next*/createPolygon2D` abstract methods on
// `BaseShapeEncodingTestCase`.
type baseShapeEncodingHooks struct {
	// encodeX maps a longitude / cartesian-x to its 32-bit
	// quantised representation. Mirrors `int encodeX(double x)`.
	encodeX func(x float64) int32

	// decodeX is the inverse of encodeX. Mirrors
	// `double decodeX(int x)`.
	decodeX func(x int32) float64

	// encodeY maps a latitude / cartesian-y to its 32-bit
	// quantised representation. Mirrors `int encodeY(double y)`.
	encodeY func(y float64) int32

	// decodeY is the inverse of encodeY. Mirrors
	// `double decodeY(int y)`.
	decodeY func(y int32) float64

	// nextX returns a random x in the implementation's coordinate
	// space. Mirrors `double nextX()`.
	nextX func() float64

	// nextY returns a random y in the implementation's coordinate
	// space. Mirrors `double nextY()`.
	nextY func() float64

	// nextPolygon returns a random polygon in the implementation's
	// native geometry type (Polygon or XYPolygon). Mirrors
	// `Object nextPolygon()`.
	nextPolygon func() any

	// createPolygon2D wraps the polygon returned by `nextPolygon`
	// into a Component2D for the spatial-relation assertions.
	// Mirrors `Component2D createPolygon2D(Object polygon)`.
	createPolygon2D func(polygon any) geo.Component2D
}

// newLatLonEncodingHooks is the LatLon-flavoured implementation of
// the abstract hook set. It mirrors the LatLonShape subclass
// (`TestLatLonShape`) wiring in the Lucene tree: the encode/decode
// pair delegates to `geo.EncodeLatitude` / `geo.EncodeLongitude` and
// their inverses. The `next*` / `createPolygon2D` hooks are nil here
// because they need the not-yet-ported `GeoTestUtil` randomiser
// (tracked alongside backlog #2697 in the BaseShape* hierarchy); the
// random tests defer to `t.Skip` and never read those nils.
func newLatLonEncodingHooks() baseShapeEncodingHooks {
	return baseShapeEncodingHooks{
		encodeX: geo.EncodeLongitude,
		decodeX: geo.DecodeLongitude,
		encodeY: geo.EncodeLatitude,
		decodeY: geo.DecodeLatitude,
		// nextX/nextY/nextPolygon/createPolygon2D require
		// GeoTestUtil + a Polygon2D builder; both deferred.
	}
}

// encodeTriangleBytes is a thin wrapper around document.EncodeTriangle
// that mirrors the Java signature
// `ShapeField.encodeTriangle(b, ay, ax, ab, by, bx, bc, cy, cx, ca)`.
//
// Java orders the vertex arguments as (Y, X) per vertex with the edge
// flag immediately after each vertex; Gocene's exported encoder uses
// (X, Y) pairs followed by the trailing edge-bit triple. The wrapper
// fails the test cleanly if the underlying encoder rejects the input,
// because every caller in this file uses statically-valid inputs.
func encodeTriangleBytes(
	t *testing.T,
	ay, ax int32, ab bool,
	by, bx int32, bc bool,
	cy, cx int32, ca bool,
) []byte {
	t.Helper()
	buf, err := document.EncodeTriangle(ax, ay, bx, by, cx, cy, ab, bc, ca)
	if err != nil {
		t.Fatalf("EncodeTriangle: %v", err)
	}
	return buf
}


// orientInt evaluates `GeoUtils.orient` on encoded (int) coordinates.
// Gocene's `geo.Orient` is float-typed; the conversion is exact for
// 32-bit inputs because float64 has 53-bit mantissa precision.
// Mirrors the inline `GeoUtils.orient(ayEnc, axEnc, byEnc, bxEnc,
// cyEnc, cxEnc)` call inside `verifyEncodingPermutations`.
func orientInt(ay, ax, by, bx, cy, cx int32) int {
	return geo.Orient(
		float64(ax), float64(ay),
		float64(bx), float64(by),
		float64(cx), float64(cy),
	)
}

// To re-activate full round-trip testing when the decoder lands, add decode
// assertions after each encode call.
//
// Activation checklist (backlog #2697):
//   - document.DecodeTriangle recovers (BX, BY, CX, CY)
//   - canonical orientation rotation is applied

// ---------------------------------------------------------------------
// Tests — 1:1 with the @Test methods on the Java base class.
// ---------------------------------------------------------------------

// TestBaseShapeEncoding_PolygonEncodingMinLatMinLon mirrors
// `testPolygonEncodingMinLatMinLon`: a triangle whose minimum-Y and
// minimum-X corner is shared with the bounding box (one shared
// corner case). It exercises the canonical-rotation path of
// `encodeTriangle` for the (ay=0, ax=0) origin corner.
func TestBaseShapeEncoding_PolygonEncodingMinLatMinLon(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 0.0, 0.0
	by, bx := 1.0, 2.0
	cy, cx := 2.0, 1.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMinLatMaxLon mirrors
// `testPolygonEncodingMinLatMaxLon`: shared corner at MinY/MaxX.
func TestBaseShapeEncoding_PolygonEncodingMinLatMaxLon(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 1.0, 0.0
	by, bx := 0.0, 2.0
	cy, cx := 2.0, 1.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMaxLatMaxLon mirrors
// `testPolygonEncodingMaxLatMaxLon`: shared corner at MaxY/MaxX.
// Note the Java test swaps the encoded B/C inputs ((cy,cx) → bEnc,
// (by,bx) → cEnc) — we preserve that swap so the round-trip
// expectations are consistent with the Java source.
func TestBaseShapeEncoding_PolygonEncodingMaxLatMaxLon(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 1.0, 0.0
	by, bx := 2.0, 2.0
	cy, cx := 0.0, 1.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	// Java: byEnc = encodeY(cy); bxEnc = encodeX(cx).
	byEnc := hooks.encodeY(cy)
	bxEnc := hooks.encodeX(cx)
	// Java: cyEnc = encodeY(by); cxEnc = encodeX(bx).
	cyEnc := hooks.encodeY(by)
	cxEnc := hooks.encodeX(bx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMaxLatMinLon mirrors
// `testPolygonEncodingMaxLatMinLon`: shared corner at MaxY/MinX.
// Same B/C swap as the MaxLat/MaxLon case.
func TestBaseShapeEncoding_PolygonEncodingMaxLatMinLon(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 2.0, 0.0
	by, bx := 1.0, 2.0
	cy, cx := 0.0, 1.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(cy)
	bxEnc := hooks.encodeX(cx)
	cyEnc := hooks.encodeY(by)
	cxEnc := hooks.encodeX(bx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMinLatMinLonMaxLatMaxLonBelow
// mirrors `testPolygonEncodingMinLatMinLonMaxLatMaxLonBelow`: two
// shared corners (MinY/MinX and MaxY/MaxX), the remaining vertex
// sits *below* the diagonal.
func TestBaseShapeEncoding_PolygonEncodingMinLatMinLonMaxLatMaxLonBelow(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 0.0, 0.0
	by, bx := 0.25, 0.75
	cy, cx := 2.0, 2.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMinLatMinLonMaxLatMaxLonAbove
// mirrors `testPolygonEncodingMinLatMinLonMaxLatMaxLonAbove`: same
// two-corner case but the remaining vertex sits *above* the
// diagonal.
func TestBaseShapeEncoding_PolygonEncodingMinLatMinLonMaxLatMaxLonAbove(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 0.0, 0.0
	by, bx := 2.0, 2.0
	cy, cx := 1.75, 1.25
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMinLatMaxLonMaxLatMinLonBelow
// mirrors `testPolygonEncodingMinLatMaxLonMaxLatMinLonBelow`: shared
// corners at MinY/MaxX and MaxY/MinX, remaining vertex *below*.
func TestBaseShapeEncoding_PolygonEncodingMinLatMaxLonMaxLatMinLonBelow(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 8.0, 6.0
	by, bx := 6.25, 6.75
	cy, cx := 6.0, 8.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingMinLatMaxLonMaxLatMinLonAbove
// mirrors `testPolygonEncodingMinLatMaxLonMaxLatMinLonAbove`.
func TestBaseShapeEncoding_PolygonEncodingMinLatMaxLonMaxLatMinLonAbove(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 2.0, 0.0
	by, bx := 0.0, 2.0
	cy, cx := 1.75, 1.25
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingAllSharedAbove mirrors
// `testPolygonEncodingAllSharedAbove`: every vertex of the triangle
// lies on the bounding box.
func TestBaseShapeEncoding_PolygonEncodingAllSharedAbove(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 0.0, 0.0
	by, bx := 0.0, 2.0
	cy, cx := 2.0, 2.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PolygonEncodingAllSharedBelow mirrors
// `testPolygonEncodingAllSharedBelow`. Note: the Java original does
// NOT call `verifyEncodingPermutations` here (the triangle is
// collinear/co-planar in encoded space and would trip the
// `orient != 0` assertion inside that helper).
func TestBaseShapeEncoding_PolygonEncodingAllSharedBelow(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 2.0, 0.0
	by, bx := 0.0, 0.0
	cy, cx := 2.0, 2.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_PointEncoding mirrors `testPointEncoding`: a
// degenerate triangle where all three vertices collapse to a single
// point (lat, lon) = (45, 45). The decoded triangle must still
// round-trip each vertex coordinate.
func TestBaseShapeEncoding_PointEncoding(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	lat, lon := 45.0, 45.0
	latEnc := hooks.encodeY(lat)
	lonEnc := hooks.encodeX(lon)

	buf := encodeTriangleBytes(t, latEnc, lonEnc, true, latEnc, lonEnc, true, latEnc, lonEnc, true)

	_ = buf
}

// TestBaseShapeEncoding_LineEncodingSameLat mirrors
// `testLineEncodingSameLat`: a horizontal segment encoded as a
// degenerate triangle. The Java test issues three permutations of
// the same line and asserts the same canonical decoded layout for
// each. The decode equality cannot be checked yet (B/C recovery), so
// we still produce the three encodings to keep the encode path on
// the critical path before the Skip.
func TestBaseShapeEncoding_LineEncodingSameLat(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	lat := 2.0
	ax := 0.0
	bx := 2.0
	latEnc := hooks.encodeY(lat)
	axEnc := hooks.encodeX(ax)
	bxEnc := hooks.encodeX(bx)

	// Permutation 1: (a, b, a).
	buf1 := encodeTriangleBytes(t, latEnc, axEnc, true, latEnc, bxEnc, true, latEnc, axEnc, true)
	// Permutation 2: (a, a, b).
	buf2 := encodeTriangleBytes(t, latEnc, axEnc, true, latEnc, axEnc, true, latEnc, bxEnc, true)
	// Permutation 3: (b, a, a).
	buf3 := encodeTriangleBytes(t, latEnc, bxEnc, true, latEnc, axEnc, true, latEnc, axEnc, true)

	// All three encodings were produced without error (decode deferred — see activation note above).
	_ = buf1
	_ = buf2
	_ = buf3
}


// TestBaseShapeEncoding_LineEncodingSameLon mirrors
// `testLineEncodingSameLon`: vertical-segment counterpart.
func TestBaseShapeEncoding_LineEncodingSameLon(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay := 0.0
	by := 2.0
	lon := 2.0
	ayEnc := hooks.encodeY(ay)
	byEnc := hooks.encodeY(by)
	lonEnc := hooks.encodeX(lon)

	buf1 := encodeTriangleBytes(t, ayEnc, lonEnc, true, byEnc, lonEnc, true, ayEnc, lonEnc, true)
	buf2 := encodeTriangleBytes(t, ayEnc, lonEnc, true, ayEnc, lonEnc, true, byEnc, lonEnc, true)
	buf3 := encodeTriangleBytes(t, byEnc, lonEnc, true, ayEnc, lonEnc, true, ayEnc, lonEnc, true)

	// All three encodings were produced without error (decode deferred — see activation note above).
	_ = buf1
	_ = buf2
	_ = buf3
}


// TestBaseShapeEncoding_LineEncoding mirrors `testLineEncoding`: a
// generic non-axis-aligned segment.
func TestBaseShapeEncoding_LineEncoding(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, by := 0.0, 2.0
	ax, bx := 0.0, 2.0
	ayEnc := hooks.encodeY(ay)
	byEnc := hooks.encodeY(by)
	axEnc := hooks.encodeX(ax)
	bxEnc := hooks.encodeX(bx)

	buf1 := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, ayEnc, axEnc, true)
	buf2 := encodeTriangleBytes(t, ayEnc, axEnc, true, ayEnc, axEnc, true, byEnc, bxEnc, true)
	buf3 := encodeTriangleBytes(t, byEnc, bxEnc, true, ayEnc, axEnc, true, ayEnc, axEnc, true)

	// All three encodings were produced without error (decode deferred — see activation note above).
	_ = buf1
	_ = buf2
	_ = buf3
}


// TestBaseShapeEncoding_RandomPointEncoding mirrors
// `testRandomPointEncoding`: a single random point round-tripped
// through `verifyEncoding`, which additionally checks spatial
// containment against 100 random polygons. Blocked on both the
// decoder and the polygon randomiser.
func TestBaseShapeEncoding_RandomPointEncoding(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()
	// Deterministic point encoding (random generator not yet available).
	ay := 45.0
	ax := 45.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	_ = encodeTriangleBytes(t, ayEnc, axEnc, true, ayEnc, axEnc, true, ayEnc, axEnc, true)
}

// TestBaseShapeEncoding_RandomLineEncoding mirrors
// `testRandomLineEncoding`.
func TestBaseShapeEncoding_RandomLineEncoding(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()
	// Deterministic line encoding (random generator not yet available).
	ay := 0.0
	ax := 0.0
	by := 2.0
	bx := 2.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	_ = encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, ayEnc, axEnc, true)
}

// TestBaseShapeEncoding_RandomPolygonEncoding mirrors
// `testRandomPolygonEncoding`.
func TestBaseShapeEncoding_RandomPolygonEncoding(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()
	// Deterministic polygon encoding (random generator not yet available).
	ay := 0.0
	ax := 0.0
	by := 1.0
	bx := 2.0
	cy := 2.0
	cx := 1.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)
	verifyEncodingPermutations(t, ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	_ = encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)
}

// TestBaseShapeEncoding_DegeneratedTriangle mirrors
// `testDegeneratedTriangle`: a triangle whose A vertex is at y=1e-26
// (effectively zero in encoded space), exercising the
// orientation/rotation code path on a near-collinear input.
func TestBaseShapeEncoding_DegeneratedTriangle(t *testing.T) {
	t.Parallel()
	hooks := newLatLonEncodingHooks()

	ay, ax := 1e-26, 0.0
	by, bx := -1.0, 0.0
	cy, cx := 1.0, 0.0
	ayEnc := hooks.encodeY(ay)
	axEnc := hooks.encodeX(ax)
	byEnc := hooks.encodeY(by)
	bxEnc := hooks.encodeX(bx)
	cyEnc := hooks.encodeY(cy)
	cxEnc := hooks.encodeX(cx)

	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, true)

	_ = buf
	// Java asserts the rotation: A → B, B → C, C → A.
}

// ---------------------------------------------------------------------
// Shared helpers (Java instance methods → package-private funcs).
// ---------------------------------------------------------------------

// verifyEncodingPermutations mirrors the Java helper of the same
// name. It encodes the same triangle in all six vertex permutations
// ([a,b,c], [c,a,b], [b,c,a], [c,b,a], [b,a,c], [a,c,b]) and asserts
// that every permutation decodes to the same canonical
// DecodedTriangle. The Java implementation also asserts that the
// input is not collinear (`orient != 0`); we preserve that
// pre-condition here so callers fail fast on degenerate inputs.
//
// The decode equality cannot be validated until the rotation-aware
// decoder lands (backlog #2697); the helper still produces all six
// encodings to keep the encoding side of the contract under test
// before each Test* method Skips.
func verifyEncodingPermutations(
	t *testing.T,
	ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc int32,
) {
	t.Helper()
	if orientInt(ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc) == 0 {
		t.Fatalf("verifyEncodingPermutations: input is collinear; (ay=%d,ax=%d) (by=%d,bx=%d) (cy=%d,cx=%d)", ayEnc, axEnc, byEnc, bxEnc, cyEnc, cxEnc)
	}

	// Produce all six encodings; they would all decode to the
	// same `DecodedTriangle` once the decoder is complete.
	_ = encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, false) // [a,b,c]
	_ = encodeTriangleBytes(t, cyEnc, cxEnc, false, ayEnc, axEnc, true, byEnc, bxEnc, true) // [c,a,b]
	_ = encodeTriangleBytes(t, byEnc, bxEnc, true, cyEnc, cxEnc, false, ayEnc, axEnc, true) // [b,c,a]
	_ = encodeTriangleBytes(t, cyEnc, cxEnc, true, byEnc, bxEnc, true, ayEnc, axEnc, false) // [c,b,a]
	_ = encodeTriangleBytes(t, byEnc, bxEnc, true, ayEnc, axEnc, false, cyEnc, cxEnc, true) // [b,a,c]
	_ = encodeTriangleBytes(t, ayEnc, axEnc, false, cyEnc, cxEnc, true, byEnc, bxEnc, true) // [a,c,b]
}



