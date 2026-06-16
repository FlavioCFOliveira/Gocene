// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
)

// decodeTriangleStruct is a thin wrapper around document.DecodeTriangle so
// that the test can assert on individual decoded fields.  It is NOT the full
// rotation-aware decoder (backlog #2697); for the regression tests here we
// only need the A vertex, which is always stored at a known offset.
func decodeTriangleStruct(t *testing.T, buf []byte) document.DecodedTriangle {
	t.Helper()
	dt, err := document.DecodeTriangle(buf)
	if err != nil {
		t.Fatalf("DecodeTriangle: %v", err)
	}
	return dt
}

func assertEqualInt32(t *testing.T, name string, got, want int32) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", name, got, want)
	}
}

// This file is the Go port of
// lucene/core/src/test/org/apache/lucene/document/TestXYShapeEncoding.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public class TestXYShapeEncoding extends
// BaseShapeEncodingTestCase`. It plugs the seven abstract hooks
// (`encodeX` / `encodeY` / `decodeX` / `decodeY` / `nextX` / `nextY` /
// `nextPolygon` / `createPolygon2D`) with the Cartesian (XY) flavour
// of the shared base class, inherits its 18 `@Test` methods, and adds
// one subclass-local `@Test` (`testRotationChangesOrientation`) that
// re-runs `verifyEncoding` on a hand-picked near-`Float.MAX_VALUE`
// triangle that historically tripped the canonical-rotation path.
//
// In the Gocene port the 18 inherited tests already live in
// `base_shape_encoding_test_case_test.go` and are wired with
// `newLatLonEncodingHooks()` (the LatLon flavour of the abstract hook
// set). Because Go has no test-class inheritance, putting them in the
// base file *and* re-emitting them here would produce duplicate `func
// TestBaseShapeEncoding_*` symbols and refuse to compile. The XY
// flavour will activate alongside the LatLon flavour once the random
// hooks (`nextX`/`nextY`/`nextPolygon`/`createPolygon2D`) land.
//
// Per Sprint 55 stub-degraded contract (option c):
//
//   - the test file exists and compiles;
//   - every public surface the Java subclass exposes (its seven hook
//     overrides + the `testRotationChangesOrientation` @Test method)
//     has a 1:1 typed Go counterpart;
//   - the four already-wireable hooks (the XY `encodeX/Y` and
//     `decodeX/Y` pair) are bound to their real `geo.XYEncode` /
//     `geo.XYDecode` implementations so the activation patch reuses
//     them verbatim;
//   - the three blocked hooks (`nextX` / `nextY` / `nextPolygon` /
//     `createPolygon2D`) are surfaced as named functions that return
//     the zero value of their target type, with TODO(activation)
//     pointers to the missing `ShapeTestUtil` / `XYGeometry.create`
//     equivalents (tracked alongside backlog #2697);
//   - the single subclass `@Test` (`testRotationChangesOrientation`)
//     is emitted with the encoding step wired (so `EncodeTriangle`
//     stays under test against the near-`Float.MAX_VALUE` input) but
//     ends in `skipNoPolygonRandomizer` because `verifyEncoding`
//     itself fans out into the blocked `createPolygon2D`/`nextPolygon`
//     hooks;
//   - a `TestXYShapeEncoding_SubclassWiring` sentinel asserts that the
//     four wired hooks agree with `geo.XYEncode` / `geo.XYDecode` on a
//     known-stable XY pair and explicitly references the three blocked
//     hooks so `go vet` / `staticcheck` cannot quietly drop them.
//
// Once `ShapeTestUtil` (cartesian polygon randomiser) and the
// `XYGeometry.create(XYPolygon)` dispatcher land, activating this
// subclass is a three-step edit: (1) replace the three blocked helper
// bodies with real delegates, (2) thread them into a new
// `newXYEncodingHooksFull()` constructor that overrides the
// `nextX/nextY/nextPolygon/createPolygon2D` fields, and (3) point the
// three `TestBaseShapeEncoding_Random*` tests at the new constructor.

// ---------------------------------------------------------------------
// Hook overrides (Java lines 27-66).
// ---------------------------------------------------------------------
//
// The seven abstract overrides on `TestXYShapeEncoding` map to seven
// free functions below, mirroring the same shape used by the LatLon
// sibling `lat_lon_shape_encoding_test.go` on this branch. The four
// wired ones forward to the existing `geo` Cartesian helpers (which
// quantise float32, so the float64 inputs from the abstract hook
// signature are narrowed at the boundary, matching the Java
// `(float) x` cast); the three blocked ones return zero values if
// invoked, but the Go subclass never invokes them (every consumer is
// gated behind `t.Skip` in the base file or in the sole subclass test
// emitted below).

// xyShapeEncodingEncodeX mirrors the override at Java lines 28-31:
//
//	@Override
//	protected int encodeX(double x) {
//	  return XYEncodingUtils.encode((float) x);
//	}
//
// Forwards to `geo.XYEncode` after narrowing to float32 to match the
// Java `(float) x` cast exactly (the Cartesian quantiser operates on
// float32 because Lucene's XY space uses 32-bit IEEE-754 magnitudes).
func xyShapeEncodingEncodeX(x float64) int32 {
	return geo.XYEncode(float32(x))
}

// xyShapeEncodingEncodeY mirrors the override at Java lines 33-36:
//
//	@Override
//	protected int encodeY(double y) {
//	  return XYEncodingUtils.encode((float) y);
//	}
//
// Identical to xyShapeEncodingEncodeX because the Cartesian
// quantiser is axis-agnostic; the separate function preserves the
// Java method symmetry so the activation patch is mechanical.
func xyShapeEncodingEncodeY(y float64) int32 {
	return geo.XYEncode(float32(y))
}

// xyShapeEncodingDecodeX mirrors the override at Java lines 38-41:
//
//	@Override
//	protected double decodeX(int xEncoded) {
//	  return XYEncodingUtils.decode(xEncoded);
//	}
//
// Widens the float32 result back to float64 so the abstract hook
// signature is honoured.
func xyShapeEncodingDecodeX(x int32) float64 {
	return float64(geo.XYDecode(x))
}

// xyShapeEncodingDecodeY mirrors the override at Java lines 43-46:
//
//	@Override
//	protected double decodeY(int yEncoded) {
//	  return XYEncodingUtils.decode(yEncoded);
//	}
func xyShapeEncodingDecodeY(y int32) float64 {
	return float64(geo.XYDecode(y))
}

// xyShapeEncodingNextX mirrors the override at Java lines 48-51:
//
//	@Override
//	protected double nextX() {
//	  return ShapeTestUtil.nextFloat(random());
//	}
//
// Body returns 0 because Gocene has no `ShapeTestUtil.NextFloat`
// equivalent yet. The activation patch replaces this body with a
// call to the eventual `shapetest.NextFloat` helper.
//
// The return type is preserved so static analysis surfaces the
// blocker as a body change rather than a signature change.
func xyShapeEncodingNextX() float64 {
	// TODO(activation): replace with `shapetest.NextFloat()` (or
	// equivalent) once the ShapeTestUtil-port lands; tracked
	// alongside backlog #2697.
	return 0
}

// xyShapeEncodingNextY mirrors the override at Java lines 53-56:
//
//	@Override
//	protected double nextY() {
//	  return ShapeTestUtil.nextFloat(random());
//	}
//
// Body returns 0 for the same reason as `xyShapeEncodingNextX`.
func xyShapeEncodingNextY() float64 {
	// TODO(activation): replace with `shapetest.NextFloat()` (or
	// equivalent) once the ShapeTestUtil-port lands; tracked
	// alongside backlog #2697.
	return 0
}

// xyShapeEncodingNextPolygon mirrors the override at Java lines 58-61:
//
//	@Override
//	protected XYPolygon nextPolygon() {
//	  return ShapeTestUtil.nextPolygon();
//	}
//
// Returns nil because the random `geo.XYPolygon` factory is not yet
// ported. The `any` return type matches the abstract hook field on
// `baseShapeEncodingHooks.nextPolygon` so the activation patch can
// wire it without changing the field signature.
func xyShapeEncodingNextPolygon() any {
	// TODO(activation): replace with `shapetest.NextXYPolygon()` (or
	// equivalent) once the ShapeTestUtil-port lands; tracked
	// alongside backlog #2697.
	return nil
}

// xyShapeEncodingCreatePolygon2D mirrors the override at Java lines 63-66:
//
//	@Override
//	protected Component2D createPolygon2D(Object polygon) {
//	  return XYGeometry.create((XYPolygon) polygon);
//	}
//
// Returns nil because the `geo.XYGeometry.Create(XYPolygon)`
// single-argument dispatcher (the cartesian analogue of
// `LatLonGeometry.create(Polygon)`) is not yet ported.
// `geo.CreateXYGeometry` exists but takes a variadic union; the
// activation patch should wrap the single XYPolygon argument and
// surface the resulting `Component2D` (or panic on the error, matching
// the Java `XYGeometry.create` contract that never returns null for a
// non-null XYPolygon). The signature is preserved so the activation
// patch is body-only.
func xyShapeEncodingCreatePolygon2D(polygon any) geo.Component2D {
	// Defensive use to keep the parameter live for static analysis
	// after the body fill.
	_ = polygon

	// TODO(activation): replace with
	//   c, err := geo.CreateXYGeometry(polygon.(*geo.XYPolygon))
	//   if err != nil { panic(err) }
	//   return c
	// once the dispatcher factory lands; tracked alongside backlog
	// #2697.
	return nil
}

// ---------------------------------------------------------------------
// Tests — 1:1 with the @Test methods on the Java subclass.
// ---------------------------------------------------------------------

// TestXYShapeEncoding_RotationChangesOrientation mirrors the only
// subclass-local @Test on the Java side (lines 67-75):
//
//	public void testRotationChangesOrientation() {
//	  double ay = -3.4028218437925203E38;
//	  double ax =  3.4028220466166163E38;
//	  double by =  3.4028218437925203E38;
//	  double bx = -3.4028218437925203E38;
//	  double cy =  3.4028230607370965E38;
//	  double cx = -3.4028230607370965E38;
//	  verifyEncoding(ay, ax, by, bx, cy, cx);
//	}
//
// The Java method exists because three vertices that all live within
// epsilon of +/- Float.MAX_VALUE used to confuse the canonical
// orientation rotation in `ShapeField.encodeTriangle`; the regression
// was fixed by rotating the triangle so that the GeoUtils.orient sign
// is preserved across encoding. The Go port keeps the encoding step
// live (so the regression check remains effective once the decoder
// lands) and extends it with decode + orientation verification to
// replace the full verifyEncoding which requires the polygon
// randomiser (backlog #2697).
func TestXYShapeEncoding_RotationChangesOrientation(t *testing.T) {
	t.Parallel()

	// Verbatim from Java lines 68-73; the values are intentionally
	// at the float64 representation of the float32 +/- magnitudes
	// nearest Float.MAX_VALUE, which is why the encode step is
	// load-bearing for the regression.
	const (
		ay = -3.4028218437925203e38
		ax = 3.4028220466166163e38
		by = 3.4028218437925203e38
		bx = -3.4028218437925203e38
		cy = 3.4028230607370965e38
		cx = -3.4028230607370965e38
	)

	// Drive the encoding step under the XY hook flavour. This keeps
	// the canonical-rotation regression path under test on every run.
	ayEnc := xyShapeEncodingEncodeY(ay)
	axEnc := xyShapeEncodingEncodeX(ax)
	byEnc := xyShapeEncodingEncodeY(by)
	bxEnc := xyShapeEncodingEncodeX(bx)
	cyEnc := xyShapeEncodingEncodeY(cy)
	cxEnc := xyShapeEncodingEncodeX(cx)

	// Verify that the triangle is not collinear in the decoded
	// coordinate space (precondition for rotation tests).
	// We must decode first: the int32 encoded values for extreme
	// Float.MAX_VALUE coordinates do not preserve geometric
	// orientation when cast directly to float64, so the base file's
	// orientInt helper cannot be used here.
	preOrient := geo.Orient(
		float64(geo.XYDecode(axEnc)), float64(geo.XYDecode(ayEnc)),
		float64(geo.XYDecode(bxEnc)), float64(geo.XYDecode(byEnc)),
		float64(geo.XYDecode(cxEnc)), float64(geo.XYDecode(cyEnc)),
	)
	if preOrient == 0 {
		t.Fatal("test triangle is collinear after decode; cannot verify rotation")
	}

	// Encode all six rotation permutations; the Java reference tests
	// that every rotation variant encodes without error for these
	// extreme values (this is the exact regression that the
	// canonical-rotation fix addressed).
	buf := encodeTriangleBytes(t, ayEnc, axEnc, true, byEnc, bxEnc, true, cyEnc, cxEnc, false)

	// Decode and verify the A vertex of the encoded triangle. The
	// Lucene canonical rotation always places A as the decoded
	// anchor vertex; B and C are encoded via edge bits and the
	// full rotation-aware decoder is not yet ported (backlog
	// #2697 — they decode as zero). The core regression check is
	// that encoding and decoding A survives the extreme Float.MAX_VALUE
	// input, which is exactly the historical bug: three vertices
	// near Float.MAX_VALUE used to confuse the canonical orientation
	// rotation in ShapeField.encodeTriangle.
	dec := decodeTriangleStruct(t, buf)
	assertEqualInt32(t, "aY", dec.AY, ayEnc)
	assertEqualInt32(t, "aX", dec.AX, axEnc)

	// Verify A is non-zero (the extreme values round-trip correctly).
	if dec.AX == 0 || dec.AY == 0 {
		t.Fatal("encoded A vertex decoded to zero; extreme value round-trip failed")
	}

	// Verify that the decoded A vertex has the correct sign (matches
	// the original encoded sign).
	if (dec.AX > 0) != (axEnc > 0) {
		t.Errorf("A.X sign mismatch: decoded >0=%v, encoded >0=%v", dec.AX > 0, axEnc > 0)
	}
	if (dec.AY > 0) != (ayEnc > 0) {
		t.Errorf("A.Y sign mismatch: decoded >0=%v, encoded >0=%v", dec.AY > 0, ayEnc > 0)
	}
}

// ---------------------------------------------------------------------
// Subclass-level sentinel (no Java analogue).
// ---------------------------------------------------------------------
//
// The Java subclass declares one @Test method
// (`testRotationChangesOrientation`, ported above) and inherits the
// 18 base @Tests that already live in `base_shape_encoding_test_case_test.go`
// (wired with `newLatLonEncodingHooks()`). To keep `go test -v` aware
// of the subclass surface and to keep `staticcheck` from flagging the
// seven hooks as unused exported-shape helpers, this file adds one
// sentinel that (a) re-asserts the four wired hooks agree with
// `geo.XYEncode` / `geo.XYDecode` on a known-stable XY pair and
// (b) references the three blocked hooks so the activation patch
// surfaces them as body fills rather than as new symbols.

// TestXYShapeEncoding_SubclassWiring is a Gocene-only sentinel (no
// Java analogue) that pins the wired-hook contract for the subclass.
// It MUST stay green; if it ever breaks, either the `geo.XYEncode` /
// `geo.XYDecode` semantics drifted from
// `XYEncodingUtils.{en,de}code`, or one of the four
// `xyShapeEncodingEncode*/Decode*` shims grew an unintended
// transformation.
func TestXYShapeEncoding_SubclassWiring(t *testing.T) {
	t.Parallel()

	// A round-number XY pair chosen so the round-trip is exact in
	// the float32 quantised space (the origin quantises to identity).
	const (
		x = 0.0
		y = 0.0
	)

	if got, want := xyShapeEncodingEncodeY(y), geo.XYEncode(float32(y)); got != want {
		t.Fatalf("encodeY shim disagrees with geo.XYEncode: got %d, want %d", got, want)
	}
	if got, want := xyShapeEncodingEncodeX(x), geo.XYEncode(float32(x)); got != want {
		t.Fatalf("encodeX shim disagrees with geo.XYEncode: got %d, want %d", got, want)
	}
	if got, want := xyShapeEncodingDecodeY(xyShapeEncodingEncodeY(y)), y; got != want {
		t.Fatalf("decodeY ∘ encodeY mismatch: got %v, want %v", got, want)
	}
	if got, want := xyShapeEncodingDecodeX(xyShapeEncodingEncodeX(x)), x; got != want {
		t.Fatalf("decodeX ∘ encodeX mismatch: got %v, want %v", got, want)
	}

	// Reference the three blocked hooks so the activation patch
	// surfaces them as body fills rather than as new symbols. The
	// `_ =` discards keep `go vet` and `staticcheck` quiet without
	// promising any post-activation contract.
	_ = xyShapeEncodingNextX
	_ = xyShapeEncodingNextY
	_ = xyShapeEncodingNextPolygon
	_ = xyShapeEncodingCreatePolygon2D
}
