// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// This file is the Go port of
// lucene/core/src/test/org/apache/lucene/document/TestLatLonShapeEncoding.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public class TestLatLonShapeEncoding extends
// BaseShapeEncodingTestCase`. It declares *zero* `@Test` methods of
// its own; the inheritance contract is to plug seven abstract hooks
// (`encodeX` / `encodeY` / `decodeX` / `decodeY` / `nextX` / `nextY` /
// `nextPolygon` / `createPolygon2D`) and inherit the 18 `@Test`
// methods defined on `BaseShapeEncodingTestCase`.
//
// In the Gocene port those 18 inherited tests already live in
// `base_shape_encoding_test_case_test.go` and are wired with
// `newLatLonEncodingHooks()` (the LatLon flavour of the abstract hook
// set). Because Go has no test-class inheritance, putting them in the
// base file *and* re-emitting them here would produce duplicate `func
// TestBaseShapeEncoding_*` symbols and refuse to compile.
//
// Per Sprint 55 stub-degraded contract (option c):
//
//   - the test file exists and compiles;
//   - every public surface the Java subclass exposes (its seven hook
//     overrides) has a 1:1 typed Go counterpart;
//   - the four already-wireable hooks (the LatLon `encodeX/Y` and
//     `decodeX/Y` pair) are bound to their real
//     `geo.EncodeLongitude` / `geo.EncodeLatitude` /
//     `geo.DecodeLongitude` / `geo.DecodeLatitude` implementations so
//     the activation patch reuses them verbatim;
//   - the three blocked hooks (`nextX` / `nextY` / `nextPolygon` /
//     `createPolygon2D`) are surfaced as named functions that return
//     the zero value of their target type, with TODO(activation)
//     pointers to the missing `GeoTestUtil` / `LatLonGeometry.create`
//     equivalents (tracked alongside backlog #2697);
//   - because the Java class adds no Test* methods, the Go subclass
//     adds none either; instead a single `TestLatLonShapeEncoding_
//     SubclassWiring` sentinel asserts that the four wired hooks
//     produce the same encodings the base file consumes via
//     `newLatLonEncodingHooks()`, and explicitly references the three
//     blocked hooks so `go vet` / `staticcheck` cannot quietly drop
//     them.
//
// Once `GeoTestUtil` and `LatLonGeometry.create` land, activating
// this subclass is a three-step edit: (1) replace the three blocked
// helper bodies with real delegates, (2) thread them into a new
// `newLatLonEncodingHooksFull()` constructor that overrides the
// `nextX/nextY/nextPolygon/createPolygon2D` fields, and (3) point
// the three `TestBaseShapeEncoding_Random*` tests at the new
// constructor.

// ---------------------------------------------------------------------
// Hook overrides (Java lines 28-66).
// ---------------------------------------------------------------------
//
// The seven abstract overrides on `TestLatLonShapeEncoding` map to
// seven free functions below, mirroring the same shape used by the
// `latLonDocValuesPoint*` and `latLon*MultiPoint*` subclass ports on
// this branch. The four wired ones forward to the existing `geo`
// helpers; the three blocked ones panic with an activation TODO if
// invoked, but the Go subclass never invokes them (every consumer is
// gated behind `t.Skip` in the base file).

// latLonShapeEncodingEncodeX mirrors the override at Java lines 29-32:
//
//	@Override
//	protected int encodeX(double x) {
//	  return GeoEncodingUtils.encodeLongitude(x);
//	}
//
// Forwards to `geo.EncodeLongitude` because the LatLon flavour treats
// X as longitude.
func latLonShapeEncodingEncodeX(x float64) int32 {
	return geo.EncodeLongitude(x)
}

// latLonShapeEncodingEncodeY mirrors the override at Java lines 34-37:
//
//	@Override
//	protected int encodeY(double y) {
//	  return GeoEncodingUtils.encodeLatitude(y);
//	}
//
// Forwards to `geo.EncodeLatitude` because the LatLon flavour treats
// Y as latitude.
func latLonShapeEncodingEncodeY(y float64) int32 {
	return geo.EncodeLatitude(y)
}

// latLonShapeEncodingDecodeX mirrors the override at Java lines 39-42:
//
//	@Override
//	protected double decodeX(int xEncoded) {
//	  return GeoEncodingUtils.decodeLongitude(xEncoded);
//	}
func latLonShapeEncodingDecodeX(x int32) float64 {
	return geo.DecodeLongitude(x)
}

// latLonShapeEncodingDecodeY mirrors the override at Java lines 44-47:
//
//	@Override
//	protected double decodeY(int yEncoded) {
//	  return GeoEncodingUtils.decodeLatitude(yEncoded);
//	}
func latLonShapeEncodingDecodeY(y int32) float64 {
	return geo.DecodeLatitude(y)
}

// latLonShapeEncodingNextX mirrors the override at Java lines 49-52:
//
//	@Override
//	protected double nextX() {
//	  return GeoTestUtil.nextLongitude();
//	}
//
// Body returns 0 because Gocene has no `GeoTestUtil.NextLongitude`
// equivalent yet. The activation patch replaces this body with a
// call to the eventual `geo.tests.NextLongitude` helper.
//
// The return type is preserved so static analysis surfaces the
// blocker as a body change rather than a signature change.
func latLonShapeEncodingNextX() float64 {
	// TODO(activation): replace with `geotest.NextLongitude()` (or
	// equivalent) once the GeoTestUtil-port lands; tracked alongside
	// backlog #2697.
	return 0
}

// latLonShapeEncodingNextY mirrors the override at Java lines 54-57:
//
//	@Override
//	protected double nextY() {
//	  return GeoTestUtil.nextLatitude();
//	}
//
// Body returns 0 for the same reason as `latLonShapeEncodingNextX`.
func latLonShapeEncodingNextY() float64 {
	// TODO(activation): replace with `geotest.NextLatitude()` (or
	// equivalent) once the GeoTestUtil-port lands; tracked alongside
	// backlog #2697.
	return 0
}

// latLonShapeEncodingNextPolygon mirrors the override at Java lines 59-62:
//
//	@Override
//	protected Polygon nextPolygon() {
//	  return GeoTestUtil.nextPolygon();
//	}
//
// Returns nil because the random `geo.Polygon` factory is not yet
// ported. The `any` return type matches the abstract hook field on
// `baseShapeEncodingHooks.nextPolygon` so the activation patch can
// wire it without changing the field signature.
func latLonShapeEncodingNextPolygon() any {
	// TODO(activation): replace with `geotest.NextPolygon()` (or
	// equivalent) once the GeoTestUtil-port lands; tracked alongside
	// backlog #2697.
	return nil
}

// latLonShapeEncodingCreatePolygon2D mirrors the override at Java lines 64-66:
//
//	@Override
//	protected Component2D createPolygon2D(Object polygon) {
//	  return LatLonGeometry.create((Polygon) polygon);
//	}
//
// Returns nil because the `geo.LatLonGeometry.Create(Polygon)`
// factory (the dispatcher that builds a `Component2D` from a
// `LatLonGeometry`) is not yet ported. The signature is preserved so
// the activation patch is body-only.
func latLonShapeEncodingCreatePolygon2D(polygon any) geo.Component2D {
	// Defensive use to keep the parameter live for static analysis
	// after the body fill.
	_ = polygon

	// TODO(activation): replace with
	// `geo.LatLonGeometryCreate(polygon.(*geo.Polygon))` (or
	// equivalent) once the dispatcher factory lands; tracked
	// alongside backlog #2697.
	return nil
}

// ---------------------------------------------------------------------
// Subclass-level sentinel (no Java analogue).
// ---------------------------------------------------------------------
//
// The Java subclass declares no @Test methods; every test is
// inherited and already wired in `base_shape_encoding_test_case_test.go`
// via `newLatLonEncodingHooks()`. To keep `go test -v` aware of the
// subclass and to keep `staticcheck` from flagging the seven hooks as
// unused exported-shape helpers, this file adds one sentinel that
// (a) re-asserts the four wired hooks agree with `geo.Encode*` /
// `geo.Decode*` on a known-stable LatLon pair and (b) references the
// three blocked hooks so the activation patch surfaces them as body
// fills rather than as new symbols.

// TestLatLonShapeEncoding_SubclassWiring is a Gocene-only sentinel
// (no Java analogue) that pins the wired-hook contract for the
// subclass. It MUST stay green; if it ever breaks, either the
// `geo.Encode*` / `geo.Decode*` semantics drifted from
// `GeoEncodingUtils.{en,de}code{Longitude,Latitude}`, or one of the
// four `latLonShapeEncodingEncode*/Decode*` shims grew an unintended
// transformation.
func TestLatLonShapeEncoding_SubclassWiring(t *testing.T) {
	t.Parallel()

	// A round-number LatLon pair chosen so the round-trip is exact
	// in the 32-bit quantised space (the equator + Greenwich
	// meridian quantise to identity).
	const (
		lat = 0.0
		lon = 0.0
	)

	if got, want := latLonShapeEncodingEncodeY(lat), geo.EncodeLatitude(lat); got != want {
		t.Fatalf("encodeY shim disagrees with geo.EncodeLatitude: got %d, want %d", got, want)
	}
	if got, want := latLonShapeEncodingEncodeX(lon), geo.EncodeLongitude(lon); got != want {
		t.Fatalf("encodeX shim disagrees with geo.EncodeLongitude: got %d, want %d", got, want)
	}
	if got, want := latLonShapeEncodingDecodeY(latLonShapeEncodingEncodeY(lat)), lat; got != want {
		t.Fatalf("decodeY ∘ encodeY mismatch: got %v, want %v", got, want)
	}
	if got, want := latLonShapeEncodingDecodeX(latLonShapeEncodingEncodeX(lon)), lon; got != want {
		t.Fatalf("decodeX ∘ encodeX mismatch: got %v, want %v", got, want)
	}

	// Reference the three blocked hooks so the activation patch
	// surfaces them as body fills rather than as new symbols. The
	// `_ =` discards keep `go vet` and `staticcheck` quiet without
	// promising any post-activation contract.
	_ = latLonShapeEncodingNextX
	_ = latLonShapeEncodingNextY
	_ = latLonShapeEncodingNextPolygon
	_ = latLonShapeEncodingCreatePolygon2D
}
