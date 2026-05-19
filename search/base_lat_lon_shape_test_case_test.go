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
// lucene/core/src/test/org/apache/lucene/document/BaseLatLonShapeTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseLatLonShapeTestCase
// extends BaseLatLonSpatialTestCase`. Concrete subclasses (e.g.
// TestLatLonShape) plug their LatLonShape-specific implementations
// of the `newRectQuery` / `newLineQuery` / `newPolygonQuery` /
// `newPointsQuery` / `newDistanceQuery` factory hooks and inherit
// the `@Test` methods defined here plus the bulk of the spatial
// test matrix defined on the parent
// (BaseLatLonSpatialTestCase -> BaseSpatialTestCase).
//
// Gocene currently lacks the test infrastructure the parent test
// case relies on (RandomIndexWriter, GeoTestUtil, QueryUtils, the
// LuceneTestCase Directory/Searcher helpers) and the concrete
// query factories the bodies call — `document.LatLonShape` is not
// yet wired and `document.NewGeometryQuery` is a placeholder that
// returns nil pending the ShapeDocValuesQuery port (see
// document/shape_doc_values.go and task GOC-4532+). All three
// `@Test` methods are therefore staged as skipped Go stubs that
// preserve the test names and structure so the activation cost,
// once the infra arrives, is a one-line removal of t.Skip.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every `@Test` method in the Java source has a 1:1 Go
//     counterpart;
//   - each Test* opens with t.Skip naming the missing piece
//     explicitly, so `go test -v` records the work without ever
//     touching the non-existent surfaces;
//   - the helpers below are typed and constructible but never
//     invoke LatLonShape — the skip happens before any helper
//     is exercised.

// ---------------------------------------------------------------------
// Helpers (Java abstract hooks -> Go function-typed factories).
// ---------------------------------------------------------------------
//
// In the Java reference these are the five `@Override protected
// Query` methods on `BaseLatLonShapeTestCase` (lines 40-72 of the
// source). The Go port models them as constructor closures so a
// future concrete sub-test can swap the implementations without
// inheritance — exactly the role java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped tests never
// invoke them, and the eventual unblocking task will populate the
// closures once document.LatLonShape (and the inner
// LatLonShapeQuery / newGeometryQuery counterparts) ship.

// shapeRectQueryFactory mirrors `protected Query newRectQuery(
// String, QueryRelation, double minLon, double maxLon,
// double minLat, double maxLat)` on the Java base class. Argument
// order matches the Java signature byte-for-byte. The body in Java
// dispatches to `LatLonShape.newBoxQuery(field, queryRelation,
// minLat, maxLat, minLon, maxLon)` — note the swap from
// (minLon,maxLon,minLat,maxLat) to (minLat,maxLat,minLon,maxLon)
// inside the factory; preserved here for the future wiring.
type shapeRectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minLon, maxLon, minLat, maxLat float64,
) Query

// shapeLineQueryFactory mirrors `protected Query newLineQuery(
// String, QueryRelation, Object... lines)`; the variadic `Object[]`
// becomes `...geo.Line` because Gocene types these strongly
// upstream. The Java implementation funnels through
// `Arrays.stream(lines).toArray(Line[]::new)` before calling
// `LatLonShape.newLineQuery`.
type shapeLineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.Line,
) Query

// shapePolygonQueryFactory mirrors `protected Query newPolygonQuery(
// String, QueryRelation, Object... polygons)`. The Java side
// streams through `Polygon[]::new` before reaching
// `LatLonShape.newPolygonQuery`.
type shapePolygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.Polygon,
) Query

// shapePointsQueryFactory mirrors `protected Query newPointsQuery(
// String, QueryRelation, Object... points)`. The Java side hides a
// `double[]{lat, lon}` payload behind each `Object` and dispatches
// to `LatLonShape.newPointQuery(field, queryRelation,
// double[][]::new)`. Go exposes the strongly-typed geo.Point
// directly, which keeps the helper allocation-free at the call
// site.
type shapePointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.Point,
) Query

// shapeDistanceQueryFactory mirrors `protected Query
// newDistanceQuery(String, QueryRelation, Object circle)`. Single
// value (not variadic) to match the Java signature; the Java side
// casts the `Object` to `(Circle)` before calling
// `LatLonShape.newDistanceQuery`.
type shapeDistanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.Circle,
) Query

// baseLatLonShapeFactories bundles the five hooks the parent
// `@Test` body would dispatch to. The struct exists so the future
// activation work can populate it in one place; today every field
// is left nil because no test reaches it before t.Skip fires.
type baseLatLonShapeFactories struct {
	rect     shapeRectQueryFactory
	line     shapeLineQueryFactory
	polygon  shapePolygonQueryFactory
	points   shapePointsQueryFactory
	distance shapeDistanceQueryFactory
}

// newBaseLatLonShapeFactories returns the canonical factory bundle
// the Java reference's overrides would produce: every hook routes
// through the `document.LatLonShape` surface. The functions are
// wired but must NOT be invoked until that surface lands; the
// surrounding tests gate every call site behind t.Skip.
//
// We do not call the (still-missing) LatLonShape constructors
// here because they are not yet declared; making the closures
// unreachable rather than wired-then-failing keeps the stub honest
// until the real query types ship.
func newBaseLatLonShapeFactories() baseLatLonShapeFactories {
	return baseLatLonShapeFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseLatLonShapeFieldName mirrors `BaseSpatialTestCase.FIELD_NAME`
// ("shape" in the Java reference, inherited via
// BaseLatLonSpatialTestCase). Kept here so the activated tests can
// reference it identically.
const baseLatLonShapeFieldName = "shape"

// ---------------------------------------------------------------------
// Ported @Test methods.
// ---------------------------------------------------------------------

// TestBaseLatLonShape_BoundingBoxQueriesEquivalence ports
// `BaseLatLonShapeTestCase#testBoundingBoxQueriesEquivalence`
// (lines 74-120 of the Java source).
//
// The Java body indexes `atLeast(20)` random shapes via
// RandomIndexWriter, optionally force-merges, opens a searcher,
// draws a random Rectangle through GeoTestUtil.nextBox(), and then
// asserts — for each of INTERSECTS / WITHIN / CONTAINS / DISJOINT
// — that `LatLonShape.newBoxQuery(...)` returns the same hit count
// as a hand-built `LatLonShapeQuery(FIELD_NAME, relation, box)`.
// For CONTAINS with a date-line-crossing box it falls back to
// `LatLonShape.newGeometryQuery`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextBox helper yet)
//   - document.LatLonShape.NewBoxQuery / LatLonShapeQuery
//     (deferred — see document/shape_field.go header)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO
//     GOC-4532+).
func TestBaseLatLonShape_BoundingBoxQueriesEquivalence(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonShape.NewBoxQuery/LatLonShapeQuery/document.NewGeometryQuery; remove this Skip when fixed")

	// Reserved factories: the future implementation reads from this
	// bundle. Touching it here keeps the symbol live for static
	// analysis without invoking the unbuilt query layer.
	_ = newBaseLatLonShapeFactories()
	_ = baseLatLonShapeFieldName
}

// TestBaseLatLonShape_BoxQueryEqualsAndHashcode ports
// `BaseLatLonShapeTestCase#testBoxQueryEqualsAndHashcode`
// (lines 122-183 of the Java source).
//
// The Java body draws a random Rectangle via GeoTestUtil.nextBox()
// and a random QueryRelation via RandomPicks.randomFrom, builds
// two `newRectQuery` instances with identical arguments and
// asserts equality via QueryUtils.checkEqual, then varies the
// field name, the relation, and the rectangle to probe inequality
// via QueryUtils.checkUnequal (with conditional re-equality when
// the random pick happens to repeat).
//
// Blocked by:
//   - tests/queryutils            (no QueryUtils.checkEqual helper yet)
//   - tests/geotestutil           (no GeoTestUtil.nextBox helper yet)
//   - document.LatLonShape.NewBoxQuery is still missing so the
//     rectQueryFactory cannot be populated.
func TestBaseLatLonShape_BoxQueryEqualsAndHashcode(t *testing.T) {
	t.Skip("blocked by QueryUtils/GeoTestUtil/LatLonShape.NewBoxQuery; remove this Skip when fixed")

	// Reserved factories: as above, kept reachable but unused.
	_ = newBaseLatLonShapeFactories()
	_ = baseLatLonShapeFieldName
}

// TestBaseLatLonShape_LineQueryEqualsAndHashcode ports
// `BaseLatLonShapeTestCase#testLineQueryEqualsAndHashcode`
// (lines 185-211 of the Java source).
//
// The Java body draws a Line via `nextLine()` (inherited from
// BaseLatLonSpatialTestCase) and a random QueryRelation restricted
// to POINT_LINE_RELATIONS (a Java-side static subset that excludes
// CONTAINS for line shapes), builds two `newLineQuery` instances
// and asserts QueryUtils.checkEqual, then varies the field name,
// the relation, and the line to probe checkUnequal.
//
// Blocked by:
//   - tests/queryutils            (no QueryUtils.checkEqual helper yet)
//   - tests/geotestutil           (no nextLine helper yet)
//   - POINT_LINE_RELATIONS subset (defined on the parent
//     BaseLatLonSpatialTestCase, also stubbed)
//   - document.LatLonShape.NewLineQuery is still missing so the
//     lineQueryFactory cannot be populated.
func TestBaseLatLonShape_LineQueryEqualsAndHashcode(t *testing.T) {
	t.Skip("blocked by QueryUtils/GeoTestUtil/POINT_LINE_RELATIONS/LatLonShape.NewLineQuery; remove this Skip when fixed")

	// Reserved factories: as above, kept reachable but unused.
	_ = newBaseLatLonShapeFactories()
	_ = baseLatLonShapeFieldName
}

// TestBaseLatLonShape_PolygonQueryEqualsAndHashcode ports
// `BaseLatLonShapeTestCase#testPolygonQueryEqualsAndHashcode`
// (lines 213-239 of the Java source).
//
// The Java body draws a Polygon via GeoTestUtil.nextPolygon() and
// a random QueryRelation via RandomPicks.randomFrom, builds two
// `newPolygonQuery` instances and asserts QueryUtils.checkEqual,
// then varies the field name, the relation, and the polygon to
// probe checkUnequal.
//
// Blocked by:
//   - tests/queryutils            (no QueryUtils.checkEqual helper yet)
//   - tests/geotestutil           (no GeoTestUtil.nextPolygon helper yet)
//   - document.LatLonShape.NewPolygonQuery is still missing so the
//     polygonQueryFactory cannot be populated.
func TestBaseLatLonShape_PolygonQueryEqualsAndHashcode(t *testing.T) {
	t.Skip("blocked by QueryUtils/GeoTestUtil/LatLonShape.NewPolygonQuery; remove this Skip when fixed")

	// Reserved factories: as above, kept reachable but unused.
	_ = newBaseLatLonShapeFactories()
	_ = baseLatLonShapeFieldName
}
