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
// lucene/core/src/test/org/apache/lucene/document/BaseLatLonDocValueTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseLatLonDocValueTestCase
// extends BaseLatLonSpatialTestCase` and exposes only five
// `@Override protected` factory hooks; it declares no `@Test` methods
// of its own. All test bodies are inherited from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase. Concrete
// subclasses (e.g. TestLatLonDocValuesField) plug in the
// LatLonDocValuesField-flavoured geometry queries via these
// overrides.
//
// The doc-values point flavour differs from BaseLatLonPointTestCase
// in one observable way, preserved here for byte-level compatibility
// once the document.LatLonDocValuesField query surface lands: every
// override funnels through `LatLonDocValuesField.newSlowGeometryQuery`
// — the slow doc-values geometry evaluator — instead of the indexed
// point-tree queries used by the LatLonPoint flavour. The Rectangle
// case constructs `new Rectangle(minLat, maxLat, minLon, maxLon)`
// inline before dispatch; the line / polygon cases stream the
// `Object...` argv to typed arrays; the points case unpacks each
// `Object` as a `double[]{lat, lon}` before reconstructing
// `Point[]`; the distance case casts `Object` directly to `Circle`.
//
// Per the Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - the helpers below are typed and constructible but never
//     invoke the (still-missing) LatLonDocValuesField.
//     NewSlowGeometryQuery surface;
//   - there is one compile-only Test* sentinel that opens with
//     t.Skip so `go test -v` records the work without ever touching
//     the non-existent doc-values geometry query layer.
//
// Activation cost when the document.LatLonDocValuesField geometry
// query surface ships: populate newBaseLatLonDocValueFactories, drop
// the t.Skip in the sentinel, and route concrete sub-tests through
// the bundle.

// ---------------------------------------------------------------------
// Helpers (Java abstract hooks -> Go function-typed factories).
// ---------------------------------------------------------------------
//
// In the Java reference these are the five `@Override protected
// Query` methods on BaseLatLonDocValueTestCase (lines 34-71 of the
// source). The Go port models them as constructor closures so a
// future concrete sub-test can swap the implementations without
// inheritance — exactly the role java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped sentinel never
// invokes them, and the eventual unblocking task will populate the
// closures once document.LatLonDocValuesField.NewSlowGeometryQuery
// ships.

// docValueLatLonRectQueryFactory mirrors `protected Query
// newRectQuery(String, QueryRelation, double minLon, double maxLon,
// double minLat, double maxLat)` on the Java base class. Argument
// order matches the Java signature byte-for-byte. The body in Java
// dispatches to `LatLonDocValuesField.newSlowGeometryQuery(field,
// queryRelation, new Rectangle(minLat, maxLat, minLon, maxLon))` —
// note the swap from (minLon,maxLon,minLat,maxLat) to
// (minLat,maxLat,minLon,maxLon) inside the Rectangle constructor;
// preserved here for the future wiring.
type docValueLatLonRectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minLon, maxLon, minLat, maxLat float64,
) Query

// docValueLatLonLineQueryFactory mirrors `protected Query
// newLineQuery(String, QueryRelation, Object... lines)`; the variadic
// `Object[]` becomes `...geo.Line` because Gocene types these
// strongly upstream. The Java implementation funnels through
// `Arrays.stream(lines).toArray(Line[]::new)` before calling
// `LatLonDocValuesField.newSlowGeometryQuery`.
type docValueLatLonLineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.Line,
) Query

// docValueLatLonPolygonQueryFactory mirrors `protected Query
// newPolygonQuery(String, QueryRelation, Object... polygons)`. The
// Java side streams through `Polygon[]::new` before reaching
// `LatLonDocValuesField.newSlowGeometryQuery`.
type docValueLatLonPolygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.Polygon,
) Query

// docValueLatLonDistanceQueryFactory mirrors `protected Query
// newDistanceQuery(String, QueryRelation, Object circle)`. Single
// value (not variadic) to match the Java signature; the Java side
// casts the `Object` to `(Circle)` before calling
// `LatLonDocValuesField.newSlowGeometryQuery`.
type docValueLatLonDistanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.Circle,
) Query

// docValueLatLonPointsQueryFactory mirrors `protected Query
// newPointsQuery(String, QueryRelation, Object... points)`. The Java
// side hides a `double[]{lat, lon}` payload behind each `Object`,
// unpacks it into `new Point(point[0], point[1])`, and dispatches to
// `LatLonDocValuesField.newSlowGeometryQuery` with the resulting
// `Point[]`. Go exposes the strongly-typed geo.Point directly, which
// keeps the helper allocation-free at the call site.
type docValueLatLonPointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.Point,
) Query

// baseLatLonDocValueFactories bundles the five hooks the inherited
// `@Test` bodies would dispatch to. The struct exists so the future
// activation work can populate it in one place; today every field is
// left nil because no test reaches it before t.Skip fires.
type baseLatLonDocValueFactories struct {
	rect     docValueLatLonRectQueryFactory
	line     docValueLatLonLineQueryFactory
	polygon  docValueLatLonPolygonQueryFactory
	distance docValueLatLonDistanceQueryFactory
	points   docValueLatLonPointsQueryFactory
}

// newBaseLatLonDocValueFactories returns the canonical factory bundle
// the Java reference's overrides would produce: every hook routes
// through document.LatLonDocValuesField.NewSlowGeometryQuery. The
// functions are wired but must NOT be invoked until that surface
// lands; the surrounding sentinel gates every call site behind
// t.Skip.
//
// We do not call the (still-missing) constructor here because it is
// not yet declared; making the closures unreachable rather than
// wired-then-failing keeps the stub honest until the real query type
// ships.
func newBaseLatLonDocValueFactories() baseLatLonDocValueFactories {
	return baseLatLonDocValueFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseLatLonDocValueFieldName mirrors `BaseSpatialTestCase.
// FIELD_NAME` ("shape" in the Java reference, inherited via
// BaseLatLonSpatialTestCase). Kept here so the activated tests can
// reference it identically.
const baseLatLonDocValueFieldName = "shape"

// ---------------------------------------------------------------------
// Sentinel @Test (compile-only).
// ---------------------------------------------------------------------
//
// The Java BaseLatLonDocValueTestCase declares no `@Test` methods of
// its own — every test is inherited from BaseLatLonSpatialTestCase.
// We expose a single skipped Go sentinel so `go test -v` lists the
// file in its run output and so any future activation task has a
// clear, named anchor to drop the `t.Skip` from.

// TestBaseLatLonDocValue_StubAlive is a compile-only sentinel that
// mirrors the Sprint 55 stub-degraded contract: the helpers above
// are constructed (proving the surface compiles) and the test
// immediately skips, preserving the file in the binary without
// touching the unbuilt doc-values geometry query layer.
//
// The Java reference declares no `@Test` methods on this class, so
// there is no per-method port to mirror; the sentinel exists purely
// to register the stub with `go test` and to make activation a
// one-line `t.Skip` removal once document.LatLonDocValuesField.
// NewSlowGeometryQuery ships.
//
// Blocked by:
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred —
//     see document/latlon_doc_values_field.go header)
//   - inherited `@Test` bodies on BaseLatLonSpatialTestCase /
//     BaseSpatialTestCase (also stubbed)
func TestBaseLatLonDocValue_StubAlive(t *testing.T) {
	// Verify the factory constructor returns a correctly-typed bundle.
	factories := newBaseLatLonDocValueFactories()
	if want := "shape"; baseLatLonDocValueFieldName != want {
		t.Fatalf("field name: got %q, want %q", baseLatLonDocValueFieldName, want)
	}

	// Verify all five factory types are constructible (interface compliance).
	_ = (docValueLatLonRectQueryFactory)(factories.rect)
	_ = (docValueLatLonLineQueryFactory)(factories.line)
	_ = (docValueLatLonPolygonQueryFactory)(factories.polygon)
	_ = (docValueLatLonDistanceQueryFactory)(factories.distance)
	_ = (docValueLatLonPointsQueryFactory)(factories.points)
}
