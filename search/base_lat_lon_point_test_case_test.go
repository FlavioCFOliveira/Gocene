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
// lucene/core/src/test/org/apache/lucene/document/BaseLatLonPointTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseLatLonPointTestCase
// extends BaseLatLonSpatialTestCase`. Concrete subclasses (e.g.
// TestLatLonPoint) plug their LatLonPoint-specific implementations of
// the `newRectQuery` / `newLineQuery` / `newPolygonQuery` /
// `newDistanceQuery` / `newPointsQuery` factory hooks and inherit the
// `@Test` methods defined here plus the bulk of the spatial test
// matrix defined on the parent.
//
// Gocene currently lacks the test infrastructure the parent test
// case relies on (RandomIndexWriter, GeoTestUtil, QueryUtils, the
// LuceneTestCase Directory/Searcher helpers) and the concrete
// query factory the body calls — document.NewGeometryQuery is a
// placeholder that returns nil pending the ShapeDocValuesQuery port
// (see document/shape_doc_values.go and task GOC-4532+). Both
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
//     invoke NewGeometryQuery — the skip happens before any helper
//     is exercised.

// ---------------------------------------------------------------------
// Helpers (Java abstract hooks → Go function-typed factories).
// ---------------------------------------------------------------------
//
// In the Java reference these are `protected Query` overrides on
// `BaseLatLonPointTestCase`. The Go port models them as
// constructor closures so a future concrete sub-test can swap the
// implementations without inheritance — exactly the role
// java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped tests never
// invoke them, and the eventual unblocking task will populate the
// closures once document.NewGeometryQuery returns a real
// search.Query (currently it returns `nil` per the
// shape_doc_values.go TODO).

// rectQueryFactory mirrors `protected Query newRectQuery(String,
// QueryRelation, double minLon, double maxLon, double minLat,
// double maxLat)` on the Java base class. Argument order matches
// the Java signature byte-for-byte.
type rectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minLon, maxLon, minLat, maxLat float64,
) Query

// lineQueryFactory mirrors `protected Query newLineQuery(String,
// QueryRelation, Object... lines)`; the variadic `Object[]` becomes
// `...geo.Line` because Gocene types these strongly upstream.
type lineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.Line,
) Query

// polygonQueryFactory mirrors `protected Query newPolygonQuery(
// String, QueryRelation, Object... polygons)`.
type polygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.Polygon,
) Query

// distanceQueryFactory mirrors `protected Query newDistanceQuery(
// String, QueryRelation, Object circle)`. Single value (not
// variadic) to match the Java signature.
type distanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.Circle,
) Query

// pointsQueryFactory mirrors `protected Query newPointsQuery(
// String, QueryRelation, Object... points)`. The Java side hides a
// `double[]{lat, lon}` payload behind each `Object`; Go exposes the
// strongly-typed geo.Point directly, which keeps the helper
// allocation-free at the call site.
type pointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.Point,
) Query

// baseLatLonPointFactories bundles the five hooks the parent
// `@Test` body would dispatch to. The struct exists so the future
// activation work can populate it in one place; today every field
// is left nil because no test reaches it before t.Skip fires.
type baseLatLonPointFactories struct {
	rect     rectQueryFactory
	line     lineQueryFactory
	polygon  polygonQueryFactory
	distance distanceQueryFactory
	points   pointsQueryFactory
}

// newBaseLatLonPointFactories returns the canonical factory bundle
// the Java reference's overrides would produce: every hook routes
// through `document.NewGeometryQuery`. The functions are wired but
// must NOT be invoked until that helper stops returning `nil`; the
// surrounding tests gate every call site behind t.Skip.
//
// We do not call NewGeometryQuery here because the helper is
// currently typed `func(...) interface{}` and returns nil; making
// the closures unreachable rather than wired-then-failing keeps the
// stub honest until the real query type lands.
func newBaseLatLonPointFactories() baseLatLonPointFactories {
	return baseLatLonPointFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseLatLonPointFieldName mirrors `BaseSpatialTestCase.FIELD_NAME`
// ("shape" in the Java reference). Kept here so the activated
// tests can reference it identically.
const baseLatLonPointFieldName = "shape"

// ---------------------------------------------------------------------
// Ported @Test methods.
// ---------------------------------------------------------------------

// TestBaseLatLonPoint_BoundingBoxQueriesEquivalence ports
// `BaseLatLonPointTestCase#testBoundingBoxQueriesEquivalence`.
//
// The Java body indexes ~20 random shapes via RandomIndexWriter,
// optionally force-merges, opens a searcher, draws a random
// Rectangle through GeoTestUtil.nextBox(), and asserts that
// `LatLonPoint.newBoxQuery(...)` and `new LatLonPointQuery(field,
// INTERSECTS, box)` return the same hit count.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextBox helper yet)
//   - document.LatLonPoint.NewBoxQuery (deferred — see
//     document/latlon_point.go header)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO
//     GOC-4532+).
func TestBaseLatLonPoint_BoundingBoxQueriesEquivalence(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewBoxQuery/document.NewGeometryQuery; remove this Skip when fixed")

	// Reserved factories: the future implementation reads from this
	// bundle. Touching it here keeps the symbol live for static
	// analysis without invoking the unbuilt query layer.
	_ = newBaseLatLonPointFactories()
	_ = baseLatLonPointFieldName
}

// TestBaseLatLonPoint_QueryEqualsAndHashcode ports
// `BaseLatLonPointTestCase#testQueryEqualsAndHashcode`.
//
// The Java body builds two polygon queries with identical
// arguments and asserts equality; then varies the field name, the
// relation, and the polygon to probe inequality. It relies on
// `QueryUtils.checkEqual` / `checkUnequal` and on
// `RandomPicks.randomFrom` to draw a `QueryRelation` value at
// random.
//
// Blocked by:
//   - tests/queryutils            (no QueryUtils.checkEqual helper yet)
//   - tests/geotestutil           (no GeoTestUtil.nextPolygon helper yet)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO
//     GOC-4532+) so the polygon helper has no real Query to
//     hand back.
func TestBaseLatLonPoint_QueryEqualsAndHashcode(t *testing.T) {
	t.Skip("blocked by QueryUtils/GeoTestUtil/document.NewGeometryQuery; remove this Skip when fixed")

	// Reserved factories: as above, kept reachable but unused.
	_ = newBaseLatLonPointFactories()
	_ = baseLatLonPointFieldName
}
