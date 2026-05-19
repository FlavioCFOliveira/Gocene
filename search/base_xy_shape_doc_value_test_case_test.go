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
// lucene/core/src/test/org/apache/lucene/document/BaseXYShapeDocValueTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class
// BaseXYShapeDocValueTestCase extends BaseXYShapeTestCase` and
// exposes only five `@Override protected` factory hooks plus a
// `getSupportedQueryRelations()` override; it declares no `@Test`
// methods of its own. All test bodies are inherited from
// BaseXYShapeTestCase -> BaseSpatialTestCase. Concrete subclasses
// (e.g. TestXYShapeDocValues) plug in the doc-values-flavoured
// shape queries via these overrides.
//
// The doc-values flavour differs from BaseXYShapeTestCase in three
// observable ways, all preserved in the Go port for byte-level
// compatibility once the document.XYShape surface lands:
//
//  1. getSupportedQueryRelations() returns only
//     {INTERSECTS, WITHIN, DISJOINT} — CONTAINS is excluded because
//     the doc-values implementation cannot evaluate it efficiently.
//  2. newRectQuery dispatches to
//     XYShape.newSlowDocValuesBoxQuery (the doc-values box variant)
//     rather than XYShape.newBoxQuery.
//  3. newLineQuery / newPolygonQuery / newPointsQuery all funnel
//     through ShapeDocValuesField.newGeometryQuery instead of the
//     XYShape per-shape factories.
//
// Note the cartesian quirk reproduced verbatim from the Java
// reference (line 72): `newDistanceQuery` on the doc-values XY
// class still calls `LatLonShape.newDistanceQuery(...)`, not
// `XYShape.newDistanceQuery`. This is an upstream oddity (almost
// certainly a copy-paste from BaseLatLonShapeDocValueTestCase) and
// is preserved here to keep byte-level parity; the future activation
// task should mirror it exactly. See [[project-gocene]] context: the
// port aims at byte-for-byte parity, so quirks are reproduced, not
// corrected.
//
// Per the Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - the helpers below are typed and constructible but never
//     invoke the (still-missing) XYShape / LatLonShape /
//     ShapeDocValuesField surfaces;
//   - there is one compile-only Test* sentinel that opens with
//     t.Skip so `go test -v` records the work without ever touching
//     the non-existent doc-values query layer.
//
// Activation cost when the document.XYShape, document.LatLonShape
// and document.ShapeDocValuesField surfaces ship: populate
// newBaseXYShapeDocValueFactories, drop the t.Skip in the sentinel,
// and route concrete sub-tests through the bundle.

// ---------------------------------------------------------------------
// getSupportedQueryRelations() override (Java lines 28-35).
// ---------------------------------------------------------------------
//
// The Java method returns a fresh ShapeField.QueryRelation[] each
// call; the Go port mirrors the contract with a package-private
// constructor so the slice is never aliased across callers.

// baseXYShapeDocValueSupportedQueryRelations returns the three
// QueryRelation values the doc-values variant supports. Mirrors the
// Java getSupportedQueryRelations() override on
// BaseXYShapeDocValueTestCase (lines 28-35 of the reference).
//
// CONTAINS is intentionally absent: the Java doc-values
// implementation does not support it (see the parent
// BaseXYShapeTestCase / BaseSpatialTestCase, which filters
// CONTAINS-only test paths based on the override's return value).
func baseXYShapeDocValueSupportedQueryRelations() []document.QueryRelation {
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
// Query` methods on BaseXYShapeDocValueTestCase (lines 37-73 of the
// source). The Go port models them as constructor closures so a
// future concrete sub-test can swap the implementations without
// inheritance — exactly the role java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped sentinel never
// invokes them, and the eventual unblocking task will populate the
// closures once document.XYShape, document.LatLonShape and
// document.ShapeDocValuesField ship.

// xyDocValueShapeRectQueryFactory mirrors `protected Query
// newRectQuery(String, QueryRelation, double minX, double maxX,
// double minY, double maxY)` on the Java doc-values base class.
// Argument order matches the Java signature byte-for-byte. The body
// in Java casts each coordinate to float and dispatches to
// `XYShape.newSlowDocValuesBoxQuery(field, queryRelation, (float)
// minX, (float) maxX, (float) minY, (float) maxY)` — coordinate
// order is preserved end-to-end (no swap, unlike the LatLon sibling).
//
// This differs from xyShapeRectQueryFactory in
// base_xy_shape_test_case_test.go by routing through the doc-values
// box variant rather than the points-index box variant.
type xyDocValueShapeRectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minX, maxX, minY, maxY float64,
) Query

// xyDocValueShapeLineQueryFactory mirrors `protected Query
// newLineQuery(String, QueryRelation, Object... lines)`; the
// variadic `Object[]` becomes `...geo.XYLine` because Gocene types
// these strongly upstream. The Java implementation funnels through
// `Arrays.stream(lines).toArray(XYLine[]::new)` before calling
// `ShapeDocValuesField.newGeometryQuery` (not XYShape).
type xyDocValueShapeLineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.XYLine,
) Query

// xyDocValueShapePolygonQueryFactory mirrors `protected Query
// newPolygonQuery(String, QueryRelation, Object... polygons)`. The
// Java side streams through `XYPolygon[]::new` before reaching
// `ShapeDocValuesField.newGeometryQuery`.
type xyDocValueShapePolygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.XYPolygon,
) Query

// xyDocValueShapePointsQueryFactory mirrors `protected Query
// newPointsQuery(String, QueryRelation, Object... points)`. The
// Java side hides a `float[]{x, y}` payload behind each `Object`
// and dispatches to `ShapeDocValuesField.newGeometryQuery` with a
// `float[][]::new` array. Go exposes the strongly-typed geo.XYPoint
// directly, which keeps the helper allocation-free at the call site.
type xyDocValueShapePointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.XYPoint,
) Query

// xyDocValueShapeDistanceQueryFactory mirrors `protected Query
// newDistanceQuery(String, QueryRelation, Object circle)`. Single
// value (not variadic) to match the Java signature; the Java side
// casts the `Object` to `(Circle)` (note: java.lang.Circle is the
// geographic Circle type, NOT XYCircle) and calls
// `LatLonShape.newDistanceQuery` — see the file-level note above on
// this upstream quirk. The Go port keeps geo.Circle here (the
// geographic variant) so the signature mirrors the Java reference
// byte-for-byte; the future activation task will dispatch to
// document.LatLonShape.NewDistanceQuery exactly as the Java does.
type xyDocValueShapeDistanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.Circle,
) Query

// baseXYShapeDocValueFactories bundles the five hooks the inherited
// `@Test` bodies would dispatch to. The struct exists so the future
// activation work can populate it in one place; today every field
// is left nil because no test reaches it before t.Skip fires.
type baseXYShapeDocValueFactories struct {
	rect     xyDocValueShapeRectQueryFactory
	line     xyDocValueShapeLineQueryFactory
	polygon  xyDocValueShapePolygonQueryFactory
	points   xyDocValueShapePointsQueryFactory
	distance xyDocValueShapeDistanceQueryFactory
}

// newBaseXYShapeDocValueFactories returns the canonical factory
// bundle the Java reference's overrides would produce: rect routes
// through document.XYShape.NewSlowDocValuesBoxQuery, the three
// geometry hooks route through document.ShapeDocValuesField.
// NewGeometryQuery, and distance routes through
// document.LatLonShape.NewDistanceQuery (the upstream quirk; see
// the file-level comment). The functions are wired but must NOT be
// invoked until those surfaces land; the surrounding sentinel gates
// every call site behind t.Skip.
//
// We do not call the (still-missing) constructors here because they
// are not yet declared; making the closures unreachable rather than
// wired-then-failing keeps the stub honest until the real query
// types ship.
func newBaseXYShapeDocValueFactories() baseXYShapeDocValueFactories {
	return baseXYShapeDocValueFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseXYShapeDocValueFieldName mirrors `BaseSpatialTestCase.
// FIELD_NAME` ("shape" in the Java reference, inherited via
// BaseXYShapeTestCase). Kept here so the activated tests can
// reference it identically.
const baseXYShapeDocValueFieldName = "shape"

// ---------------------------------------------------------------------
// Sentinel @Test (compile-only).
// ---------------------------------------------------------------------
//
// The Java BaseXYShapeDocValueTestCase declares no `@Test` methods
// of its own — every test is inherited from BaseXYShapeTestCase. We
// expose a single skipped Go sentinel so `go test -v` lists the
// file in its run output and so any future activation task has a
// clear, named anchor to drop the `t.Skip` from.

// TestBaseXYShapeDocValue_StubAlive is a compile-only sentinel that
// mirrors the Sprint 55 stub-degraded contract: the helpers above
// are constructed (proving the surface compiles) and the test
// immediately skips, preserving the file in the binary without
// touching the unbuilt doc-values query layer.
//
// The Java reference declares no `@Test` methods on this class, so
// there is no per-method port to mirror; the sentinel exists purely
// to register the stub with `go test` and to make activation a
// one-line `t.Skip` removal once document.XYShape,
// document.LatLonShape and document.ShapeDocValuesField ship.
//
// Blocked by:
//   - document.XYShape.NewSlowDocValuesBoxQuery (deferred — see
//     document/shape_field.go header)
//   - document.LatLonShape.NewDistanceQuery (deferred — see same;
//     called by the cartesian distance hook per the upstream quirk)
//   - document.ShapeDocValuesField.NewGeometryQuery (currently a
//     `nil`-returning placeholder in document/shape_doc_values.go;
//     full wiring tracked under GOC-4532+)
//   - inherited `@Test` bodies on BaseXYShapeTestCase /
//     BaseSpatialTestCase (also stubbed)
func TestBaseXYShapeDocValue_StubAlive(t *testing.T) {
	t.Skip("blocked by XYShape.NewSlowDocValuesBoxQuery/LatLonShape.NewDistanceQuery/ShapeDocValuesField.NewGeometryQuery and inherited BaseXYShapeTestCase bodies; remove this Skip when fixed")

	// Reserved factories and constants: the future implementation
	// reads from these. Touching them here keeps the symbols live
	// for static analysis without invoking the unbuilt query layer.
	_ = newBaseXYShapeDocValueFactories()
	_ = baseXYShapeDocValueFieldName
	_ = baseXYShapeDocValueSupportedQueryRelations()
}
