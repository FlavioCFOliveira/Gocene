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

// decodeTriangleStruct is a thin wrapper around document.DecodeTriangle
// that mirrors the Java idiom of allocating a `DecodedTriangle` and
// having `decodeTriangle(b, encoded)` populate it. Failing here would
// indicate a length mismatch on the input buffer, which the encoder
// rules out.
func decodeTriangleStruct(t *testing.T, buf []byte) document.DecodedTriangle {
	t.Helper()
	out, err := document.DecodeTriangle(buf)
	if err != nil {
		t.Fatalf("DecodeTriangle: %v", err)
	}
	return out
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

// skipNoDecoder is the canonical Skip used by every Test* method in
// this file. Centralising the gap message keeps the rationale uniform
// and makes the eventual unblock (delete this helper + each call)
// trivially grep-able.
func skipNoDecoder(t *testing.T) {
	t.Helper()
	t.Skip("blocked: document.DecodeTriangle does not yet recover the (BX, BY, CX, CY) vertices nor apply the canonical orientation rotation; deferred to backlog #2697 (Lucene-byte-compatible tessellator + decoder).")
}

// skipNoPolygonRandomizer is the gap message for the three random
// Test* methods (testRandomPointEncoding / testRandomLineEncoding /
// testRandomPolygonEncoding). They additionally require the
// not-yet-ported GeoTestUtil randomiser and a Polygon2D builder,
// referenced through the `nextX`, `nextY`, `nextPolygon`,
// `createPolygon2D` hooks.
func skipNoPolygonRandomizer(t *testing.T) {
	t.Helper()
	t.Skip("blocked: requires (1) the rotation-aware ShapeField decoder (backlog #2697) and (2) the GeoTestUtil-equivalent polygon randomiser (nextX/nextY/nextPolygon/createPolygon2D hooks).")
}

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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, ayEnc)
	assertEqualInt32(t, "aX", enc.AX, axEnc)
	assertEqualInt32(t, "bY", enc.BY, byEnc)
	assertEqualInt32(t, "bX", enc.BX, bxEnc)
	assertEqualInt32(t, "cY", enc.CY, cyEnc)
	assertEqualInt32(t, "cX", enc.CX, cxEnc)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", enc.AY, latEnc)
	assertEqualInt32(t, "aX", enc.AX, lonEnc)
	assertEqualInt32(t, "bY", enc.BY, latEnc)
	assertEqualInt32(t, "bX", enc.BX, lonEnc)
	assertEqualInt32(t, "cY", enc.CY, latEnc)
	assertEqualInt32(t, "cX", enc.CX, lonEnc)
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

	skipNoDecoder(t)
	for _, buf := range [][]byte{buf1, buf2, buf3} {
		enc := decodeTriangleStruct(t, buf)
		assertEqualInt32(t, "aY", enc.AY, latEnc)
		assertEqualInt32(t, "aX", enc.AX, axEnc)
		assertEqualInt32(t, "bY", enc.BY, latEnc)
		assertEqualInt32(t, "bX", enc.BX, bxEnc)
		assertEqualInt32(t, "cY", enc.CY, latEnc)
		assertEqualInt32(t, "cX", enc.CX, axEnc)
	}
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

	skipNoDecoder(t)
	for _, buf := range [][]byte{buf1, buf2, buf3} {
		enc := decodeTriangleStruct(t, buf)
		assertEqualInt32(t, "aY", enc.AY, ayEnc)
		assertEqualInt32(t, "aX", enc.AX, lonEnc)
		assertEqualInt32(t, "bY", enc.BY, byEnc)
		assertEqualInt32(t, "bX", enc.BX, lonEnc)
		assertEqualInt32(t, "cY", enc.CY, ayEnc)
		assertEqualInt32(t, "cX", enc.CX, lonEnc)
	}
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

	skipNoDecoder(t)
	for _, buf := range [][]byte{buf1, buf2, buf3} {
		enc := decodeTriangleStruct(t, buf)
		assertEqualInt32(t, "aY", enc.AY, ayEnc)
		assertEqualInt32(t, "aX", enc.AX, axEnc)
		assertEqualInt32(t, "bY", enc.BY, byEnc)
		assertEqualInt32(t, "bX", enc.BX, bxEnc)
		assertEqualInt32(t, "cY", enc.CY, ayEnc)
		assertEqualInt32(t, "cX", enc.CX, axEnc)
	}
}

// TestBaseShapeEncoding_RandomPointEncoding mirrors
// `testRandomPointEncoding`: a single random point round-tripped
// through `verifyEncoding`, which additionally checks spatial
// containment against 100 random polygons. Blocked on both the
// decoder and the polygon randomiser.
func TestBaseShapeEncoding_RandomPointEncoding(t *testing.T) {
	t.Parallel()
	skipNoPolygonRandomizer(t)

	hooks := newLatLonEncodingHooks()
	ay := hooks.nextY()
	ax := hooks.nextX()
	verifyEncoding(t, hooks, ay, ax, ay, ax, ay, ax)
}

// TestBaseShapeEncoding_RandomLineEncoding mirrors
// `testRandomLineEncoding`.
func TestBaseShapeEncoding_RandomLineEncoding(t *testing.T) {
	t.Parallel()
	skipNoPolygonRandomizer(t)

	hooks := newLatLonEncodingHooks()
	ay := hooks.nextY()
	ax := hooks.nextX()
	by := hooks.nextY()
	bx := hooks.nextX()
	verifyEncoding(t, hooks, ay, ax, by, bx, ay, ax)
}

// TestBaseShapeEncoding_RandomPolygonEncoding mirrors
// `testRandomPolygonEncoding`.
func TestBaseShapeEncoding_RandomPolygonEncoding(t *testing.T) {
	t.Parallel()
	skipNoPolygonRandomizer(t)

	hooks := newLatLonEncodingHooks()
	ay := hooks.nextY()
	ax := hooks.nextX()
	by := hooks.nextY()
	bx := hooks.nextX()
	cy := hooks.nextY()
	cx := hooks.nextX()
	verifyEncoding(t, hooks, ay, ax, by, bx, cy, cx)
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

	skipNoDecoder(t)
	enc := decodeTriangleStruct(t, buf)
	// Java asserts the rotation: A → B, B → C, C → A.
	assertEqualInt32(t, "aY", enc.AY, byEnc)
	assertEqualInt32(t, "aX", enc.AX, bxEnc)
	assertEqualInt32(t, "bY", enc.BY, cyEnc)
	assertEqualInt32(t, "bX", enc.BX, cxEnc)
	assertEqualInt32(t, "cY", enc.CY, ayEnc)
	assertEqualInt32(t, "cX", enc.CX, axEnc)
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

// verifyEncoding mirrors the Java helper of the same name. It
// encodes a triangle, decodes it, quantises both the original and
// the decoded vertices, and asserts that 100 random polygons relate
// to both quantisations identically (same intersects + contains
// result). Blocked on both the rotation-aware decoder and the
// polygon randomiser; this function is therefore wired but never
// reached after the Skip in each random Test*.
func verifyEncoding(
	t *testing.T,
	hooks baseShapeEncodingHooks,
	ay, ax, by, bx, cy, cx float64,
) {
	t.Helper()

	original := [6]int32{
		hooks.encodeX(ax), hooks.encodeY(ay),
		hooks.encodeX(bx), hooks.encodeY(by),
		hooks.encodeX(cx), hooks.encodeY(cy),
	}
	buf := encodeTriangleBytes(
		t,
		original[1], original[0], true,
		original[3], original[2], true,
		original[5], original[4], true,
	)
	enc := decodeTriangleStruct(t, buf)

	encodedQuantize := [6]float64{
		hooks.decodeX(enc.AX), hooks.decodeY(enc.AY),
		hooks.decodeX(enc.BX), hooks.decodeY(enc.BY),
		hooks.decodeX(enc.CX), hooks.decodeY(enc.CY),
	}
	originalQuantize := orderTriangle(
		hooks,
		original[0], original[1], original[2], original[3], original[4], original[5],
	)

	for i := 0; i < 100; i++ {
		polygon2D := hooks.createPolygon2D(hooks.nextPolygon())
		var originalIntersects, encodedIntersects bool
		var originalContains, encodedContains bool
		switch enc.Kind {
		case document.DecodedTriangleTypePoint:
			originalIntersects = polygon2D.Contains(originalQuantize[0], originalQuantize[1])
			encodedIntersects = polygon2D.Contains(encodedQuantize[0], encodedQuantize[1])
			originalContains = polygon2D.Contains(originalQuantize[0], originalQuantize[1])
			encodedContains = polygon2D.Contains(encodedQuantize[0], encodedQuantize[1])
		case document.DecodedTriangleTypeLine:
			originalIntersects = geo.IntersectsLineDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
			)
			encodedIntersects = geo.IntersectsLineDefault(
				polygon2D,
				encodedQuantize[0], encodedQuantize[1],
				encodedQuantize[2], encodedQuantize[3],
			)
			originalContains = geo.ContainsLineDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
			)
			encodedContains = geo.ContainsLineDefault(
				polygon2D,
				encodedQuantize[0], encodedQuantize[1],
				encodedQuantize[2], encodedQuantize[3],
			)
		case document.DecodedTriangleTypeTriangle:
			// Note: the Java original re-uses `originalQuantize`
			// (not `encodedQuantize`) for both branches on the
			// triangle path — that quirk is preserved here.
			originalIntersects = geo.IntersectsTriangleDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
				originalQuantize[4], originalQuantize[5],
			)
			encodedIntersects = geo.IntersectsTriangleDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
				originalQuantize[4], originalQuantize[5],
			)
			originalContains = geo.ContainsTriangleDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
				originalQuantize[4], originalQuantize[5],
			)
			encodedContains = geo.ContainsTriangleDefault(
				polygon2D,
				originalQuantize[0], originalQuantize[1],
				originalQuantize[2], originalQuantize[3],
				originalQuantize[4], originalQuantize[5],
			)
		}
		if originalIntersects != encodedIntersects {
			t.Fatalf("iteration %d: intersects mismatch (orig=%v enc=%v)", i, originalIntersects, encodedIntersects)
		}
		if originalContains != encodedContains {
			t.Fatalf("iteration %d: contains mismatch (orig=%v enc=%v)", i, originalContains, encodedContains)
		}
	}
}

// orderTriangle is the private Java helper of the same name: it
// quantises the encoded (int) vertices back to floats and rotates
// them into the same canonical orientation the encoder would have
// produced. Returns a length-6 array laid out as
// [aX, aY, bX, bY, cX, cY].
func orderTriangle(
	hooks baseShapeEncodingHooks,
	aX, aY, bX, bY, cX, cY int32,
) [6]float64 {
	orientation := orientInt(aY, aX, bY, bX, cY, cX)

	switch {
	case orientation == -1:
		// Clockwise: flip orientation by swapping A and C.
		return [6]float64{
			hooks.decodeX(cX), hooks.decodeY(cY),
			hooks.decodeX(bX), hooks.decodeY(bY),
			hooks.decodeX(aX), hooks.decodeY(aY),
		}
	case aX == bX && aY == bY:
		if aX != cX || aY != cY {
			if aX < cX {
				return [6]float64{
					hooks.decodeX(aX), hooks.decodeY(aY),
					hooks.decodeX(cX), hooks.decodeY(cY),
					hooks.decodeX(aX), hooks.decodeY(aY),
				}
			}
			return [6]float64{
				hooks.decodeX(cX), hooks.decodeY(cY),
				hooks.decodeX(aX), hooks.decodeY(aY),
				hooks.decodeX(cX), hooks.decodeY(cY),
			}
		}
	case (aX == cX && aY == cY) || (bX == cX && bY == cY):
		if aX < bX {
			return [6]float64{
				hooks.decodeX(aX), hooks.decodeY(aY),
				hooks.decodeX(bX), hooks.decodeY(bY),
				hooks.decodeX(aX), hooks.decodeY(aY),
			}
		}
		return [6]float64{
			hooks.decodeX(bX), hooks.decodeY(bY),
			hooks.decodeX(aX), hooks.decodeY(aY),
			hooks.decodeX(bX), hooks.decodeY(bY),
		}
	}
	return [6]float64{
		hooks.decodeX(aX), hooks.decodeY(aY),
		hooks.decodeX(bX), hooks.decodeY(bY),
		hooks.decodeX(cX), hooks.decodeY(cY),
	}
}

// assertEqualInt32 is a small helper that mirrors JUnit's
// `assertEquals(expected, actual)` with a descriptive message.
// Kept local because the existing search-package test helpers don't
// expose a typed int32 comparator.
func assertEqualInt32(t *testing.T, name string, got, want int32) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d, want %d", name, got, want)
	}
}
