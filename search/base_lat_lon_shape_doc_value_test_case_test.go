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
// lucene/core/src/test/org/apache/lucene/document/BaseLatLonShapeDocValueTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class
// BaseLatLonShapeDocValueTestCase extends BaseLatLonSpatialTestCase`
// and exposes only five `@Override protected` factory hooks plus a
// `getSupportedQueryRelations()` override; it declares no `@Test`
// methods of its own. All test bodies are inherited from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase. Concrete
// subclasses (e.g. TestLatLonShapeDocValues) plug in the
// doc-values-flavoured shape queries via these overrides.
//
// The doc-values flavour differs from BaseLatLonShapeTestCase in
// three observable ways, all preserved in the Go port for byte-level
// compatibility once the document.LatLonShape surface lands:
//
//  1. getSupportedQueryRelations() returns only
//     {INTERSECTS, WITHIN, DISJOINT} — CONTAINS is excluded because
//     the doc-values implementation cannot evaluate it efficiently.
//  2. newRectQuery dispatches to
//     LatLonShape.newSlowDocValuesBoxQuery (the doc-values box
//     variant) rather than LatLonShape.newBoxQuery.
//  3. newLineQuery / newPolygonQuery / newPointsQuery all funnel
//     through ShapeDocValuesField.newGeometryQuery instead of the
//     LatLonShape per-shape factories.
//
// Per the Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - the helpers below are typed and constructible but never
//     invoke the (still-missing) LatLonShape / ShapeDocValuesField
//     surfaces;
//   - there is one compile-only Test* sentinel that opens with
//     t.Skip so `go test -v` records the work without ever touching
//     the non-existent doc-values query layer.
//
// Activation cost when the document.LatLonShape and
// ShapeDocValuesField surfaces ship: populate
// newBaseLatLonShapeDocValueFactories, drop the t.Skip in the
// sentinel, and route concrete sub-tests through the bundle.

// ---------------------------------------------------------------------
// getSupportedQueryRelations() override (Java lines 28-35).
// ---------------------------------------------------------------------
//
// The Java method returns a fresh ShapeField.QueryRelation[] each
// call; the Go port mirrors the contract with a package-private
// constructor so the slice is never aliased across callers.

// baseLatLonShapeDocValueSupportedQueryRelations returns the three
// QueryRelation values the doc-values variant supports. Mirrors the
// Java getSupportedQueryRelations() override on
// BaseLatLonShapeDocValueTestCase (lines 28-35 of the reference).
//
// CONTAINS is intentionally absent: the Java doc-values
// implementation does not support it (see the parent
// BaseLatLonSpatialTestCase, which filters CONTAINS-only test paths
// based on the override's return value).
func baseLatLonShapeDocValueSupportedQueryRelations() []document.QueryRelation {
	return []document.QueryRelation{
		document.QueryRelationIntersects,
		document.QueryRelationWithin,
		document.QueryRelationDisjoint,
	}
}

// ---------------------------------------------------------------------
// Helpers (Java abstract hooks -> Go function-typed factories).
// ---------------------------------------------------------------------
//
// In the Java reference these are the five `@Override protected
// Query` methods on BaseLatLonShapeDocValueTestCase (lines 37-74 of
// the source). The Go port models them as constructor closures so a
// future concrete sub-test can swap the implementations without
// inheritance — exactly the role java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped sentinel never
// invokes them, and the eventual unblocking task will populate the
// closures once document.LatLonShape and ShapeDocValuesField ship.

// docValueShapeRectQueryFactory mirrors `protected Query
// newRectQuery(String, QueryRelation, double minLon, double maxLon,
// double minLat, double maxLat)` on the Java doc-values base class.
// Argument order matches the Java signature byte-for-byte. The body
// in Java dispatches to
// `LatLonShape.newSlowDocValuesBoxQuery(field, queryRelation,
// minLat, maxLat, minLon, maxLon)` — note the swap from
// (minLon,maxLon,minLat,maxLat) to (minLat,maxLat,minLon,maxLon)
// inside the factory; preserved here for the future wiring.
//
// This differs from shapeRectQueryFactory in
// base_lat_lon_shape_test_case_test.go by routing through the
// doc-values box variant rather than the points-index box variant.
type docValueShapeRectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minLon, maxLon, minLat, maxLat float64,
) Query

// docValueShapeLineQueryFactory mirrors `protected Query
// newLineQuery(String, QueryRelation, Object... lines)`; the
// variadic `Object[]` becomes `...geo.Line` because Gocene types
// these strongly upstream. The Java implementation funnels through
// `Arrays.stream(lines).toArray(Line[]::new)` before calling
// `ShapeDocValuesField.newGeometryQuery` (not LatLonShape).
type docValueShapeLineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.Line,
) Query

// docValueShapePolygonQueryFactory mirrors `protected Query
// newPolygonQuery(String, QueryRelation, Object... polygons)`. The
// Java side streams through `Polygon[]::new` before reaching
// `ShapeDocValuesField.newGeometryQuery`.
type docValueShapePolygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.Polygon,
) Query

// docValueShapePointsQueryFactory mirrors `protected Query
// newPointsQuery(String, QueryRelation, Object... points)`. The
// Java side hides a `double[]{lat, lon}` payload behind each
// `Object` and dispatches to `ShapeDocValuesField.newGeometryQuery`
// with a `double[][]::new` array. Go exposes the strongly-typed
// geo.Point directly, which keeps the helper allocation-free at the
// call site.
type docValueShapePointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.Point,
) Query

// docValueShapeDistanceQueryFactory mirrors `protected Query
// newDistanceQuery(String, QueryRelation, Object circle)`. Single
// value (not variadic) to match the Java signature; the Java side
// casts the `Object` to `(Circle)` before calling
// `LatLonShape.newDistanceQuery` (the only override that still
// targets LatLonShape rather than ShapeDocValuesField).
type docValueShapeDistanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.Circle,
) Query

// baseLatLonShapeDocValueFactories bundles the five hooks the
// inherited `@Test` bodies would dispatch to. The struct exists so
// the future activation work can populate it in one place; today
// every field is left nil because no test reaches it before t.Skip
// fires.
type baseLatLonShapeDocValueFactories struct {
	rect     docValueShapeRectQueryFactory
	line     docValueShapeLineQueryFactory
	polygon  docValueShapePolygonQueryFactory
	points   docValueShapePointsQueryFactory
	distance docValueShapeDistanceQueryFactory
}

// newBaseLatLonShapeDocValueFactories returns the canonical factory
// bundle the Java reference's overrides would produce: rect routes
// through document.LatLonShape.NewSlowDocValuesBoxQuery, the three
// geometry hooks route through document.ShapeDocValuesField.
// NewGeometryQuery, and distance routes through
// document.LatLonShape.NewDistanceQuery. The functions are wired
// but must NOT be invoked until those surfaces land; the
// surrounding sentinel gates every call site behind t.Skip.
//
// We do not call the (still-missing) constructors here because they
// are not yet declared; making the closures unreachable rather than
// wired-then-failing keeps the stub honest until the real query
// types ship.
func newBaseLatLonShapeDocValueFactories() baseLatLonShapeDocValueFactories {
	return baseLatLonShapeDocValueFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseLatLonShapeDocValueFieldName mirrors `BaseSpatialTestCase.
// FIELD_NAME` ("shape" in the Java reference, inherited via
// BaseLatLonSpatialTestCase). Kept here so the activated tests can
// reference it identically.
const baseLatLonShapeDocValueFieldName = "shape"

// ---------------------------------------------------------------------
// Sentinel @Test (compile-only).
// ---------------------------------------------------------------------
//
// The Java BaseLatLonShapeDocValueTestCase declares no `@Test`
// methods of its own — every test is inherited from
// BaseLatLonSpatialTestCase. We expose a single skipped Go sentinel
// so `go test -v` lists the file in its run output and so any
// future activation task has a clear, named anchor to drop the
// `t.Skip` from.

// TestBaseLatLonShapeDocValue_StubAlive is a compile-only sentinel
// that mirrors the Sprint 55 stub-degraded contract: the helpers
// above are constructed (proving the surface compiles) and the test
// immediately skips, preserving the file in the binary without
// touching the unbuilt doc-values query layer.
//
// The Java reference declares no `@Test` methods on this class, so
// there is no per-method port to mirror; the sentinel exists purely
// to register the stub with `go test` and to make activation a
// one-line `t.Skip` removal once document.LatLonShape and
// document.ShapeDocValuesField ship.
//
// Blocked by:
//   - document.LatLonShape.NewSlowDocValuesBoxQuery (deferred — see
//     document/shape_field.go header)
//   - document.LatLonShape.NewDistanceQuery (deferred — see same)
//   - document.ShapeDocValuesField.NewGeometryQuery (currently a
//     `nil`-returning placeholder in document/shape_doc_values.go;
//     full wiring tracked under GOC-4532+)
//   - inherited `@Test` bodies on BaseLatLonSpatialTestCase /
//     BaseSpatialTestCase (also stubbed)
func TestBaseLatLonShapeDocValue_StubAlive(t *testing.T) {
	// Verify the factory constructor returns a correctly-typed bundle.
	factories := newBaseLatLonShapeDocValueFactories()
	if want := "shape"; baseLatLonShapeDocValueFieldName != want {
		t.Fatalf("field name: got %q, want %q", baseLatLonShapeDocValueFieldName, want)
	}

	// Verify all five factory types are constructible (interface compliance).
	_ = (docValueShapeRectQueryFactory)(factories.rect)
	_ = (docValueShapeLineQueryFactory)(factories.line)
	_ = (docValueShapePolygonQueryFactory)(factories.polygon)
	_ = (docValueShapePointsQueryFactory)(factories.points)
	_ = (docValueShapeDistanceQueryFactory)(factories.distance)

	// Verify the supported query relations are correct (CONTAINS excluded).
	relations := baseLatLonShapeDocValueSupportedQueryRelations()
	if len(relations) != 3 {
		t.Fatalf("supported query relations: got %d, want 3", len(relations))
	}
	if relations[0] != document.QueryRelationIntersects {
		t.Fatalf("relations[0]: got %v, want INTERSECTS", relations[0])
	}
	if relations[1] != document.QueryRelationWithin {
		t.Fatalf("relations[1]: got %v, want WITHIN", relations[1])
	}
	if relations[2] != document.QueryRelationDisjoint {
		t.Fatalf("relations[2]: got %v, want DISJOINT", relations[2])
	}
}
