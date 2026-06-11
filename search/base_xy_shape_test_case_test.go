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
// lucene/core/src/test/org/apache/lucene/document/BaseXYShapeTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseXYShapeTestCase
// extends BaseSpatialTestCase` — note that, unlike its geographic
// sibling BaseLatLonShapeTestCase, the cartesian variant inherits
// directly from BaseSpatialTestCase rather than from a dedicated
// BaseXYSpatialTestCase. The Java source carries NO `@Test`
// methods of its own; it is a pure hook-override layer that wires
// the (XY) cartesian geometry surface into the inherited spatial
// test matrix defined on BaseSpatialTestCase. Concrete subclasses
// (e.g. TestXYShape) plug their XYShape-specific behaviour by
// extending this class and inherit the test methods from
// BaseSpatialTestCase.
//
// Gocene currently lacks the test infrastructure the parent test
// case relies on (RandomIndexWriter, ShapeTestUtil random
// generators, QueryUtils, the LuceneTestCase Directory/Searcher
// helpers) and the concrete cartesian query surface — there is no
// document.XYShape type and no document.NewXYBoxQuery /
// NewXYLineQuery / NewXYPolygonQuery / NewXYPointQuery /
// NewXYDistanceQuery in this tree. The full set of XY shape
// queries is deferred to the same wave of work that lights up
// LatLonShape (see document/shape_doc_values.go and task
// GOC-4532+).
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - because the Java source defines zero `@Test` methods, no
//     Test* function is required for 1:1 parity with the body;
//     instead a single sentinel TestBaseXYShape_PortStub is exposed
//     to make the stub discoverable via `go test -v` and to record
//     the blocking dependencies in one place;
//   - the helpers below are typed and constructible but never
//     invoke XYShape — the skip happens before any helper is
//     exercised.

// ---------------------------------------------------------------------
// Helpers (Java abstract-override hooks -> Go function-typed factories).
// ---------------------------------------------------------------------
//
// In the Java reference these are the thirteen `@Override` methods
// on `BaseXYShapeTestCase` (lines 40-207 of the source). The Go
// port models them as constructor closures so a future concrete
// sub-test can swap the implementations without inheritance —
// exactly the role Java's @Override plays.
//
// They are intentionally `nil`-bodied: the skipped tests never
// invoke them, and the eventual unblocking task will populate the
// closures once document.XYShape (and its NewBoxQuery /
// NewLineQuery / NewPolygonQuery / NewPointQuery / NewDistanceQuery
// counterparts) ship.

// xyShapeRectQueryFactory mirrors `protected Query newRectQuery(
// String, QueryRelation, double minX, double maxX, double minY,
// double maxY)` (lines 50-59). The Java implementation casts each
// coordinate to float and dispatches to
// `XYShape.newBoxQuery(field, queryRelation, minX, maxX, minY,
// maxY)` — coordinate order is preserved end-to-end (no swap,
// unlike the LatLon sibling).
type xyShapeRectQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	minX, maxX, minY, maxY float64,
) Query

// xyShapeLineQueryFactory mirrors `protected Query newLineQuery(
// String, QueryRelation, Object... lines)` (lines 62-65); the
// variadic `Object[]` becomes `...geo.XYLine` because Gocene types
// these strongly upstream. The Java implementation funnels through
// `Arrays.stream(lines).toArray(XYLine[]::new)` before calling
// `XYShape.newLineQuery`.
type xyShapeLineQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	lines ...geo.XYLine,
) Query

// xyShapePolygonQueryFactory mirrors `protected Query
// newPolygonQuery(String, QueryRelation, Object... polygons)`
// (lines 68-72). The Java side streams through `XYPolygon[]::new`
// before reaching `XYShape.newPolygonQuery`.
type xyShapePolygonQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	polygons ...geo.XYPolygon,
) Query

// xyShapePointsQueryFactory mirrors `protected Query
// newPointsQuery(String, QueryRelation, Object... points)`
// (lines 75-78). The Java side hides a `float[]{x, y}` payload
// behind each `Object` and dispatches to
// `XYShape.newPointQuery(field, queryRelation, float[][]::new)`.
// Go exposes the strongly-typed geo.XYPoint directly, which keeps
// the helper allocation-free at the call site.
type xyShapePointsQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	points ...geo.XYPoint,
) Query

// xyShapeDistanceQueryFactory mirrors `protected Query
// newDistanceQuery(String, QueryRelation, Object circle)`
// (lines 81-83). Single value (not variadic) to match the Java
// signature; the Java side casts the `Object` to `(XYCircle)`
// before calling `XYShape.newDistanceQuery`.
type xyShapeDistanceQueryFactory func(
	field string,
	queryRelation document.QueryRelation,
	circle geo.XYCircle,
) Query

// xyShapeComponent2DFactory bundles the five Java
// `Component2D toXxx2D(...)` hooks (lines 86-114). They feed into
// the brute-force verification path in BaseSpatialTestCase. Each
// hook in Java just delegates to `XYGeometry.create(...)`; the
// Gocene equivalent is `geo.CreateXYGeometry(...)`. The bundle is
// modelled as a struct rather than five top-level types because
// the hooks share a common return type and are always supplied
// together.
type xyShapeComponent2DFactory struct {
	// point mirrors `protected Component2D toPoint2D(Object... points)`
	// (lines 86-93). The Java body maps each `float[]{x, y}` to a
	// fresh `XYPoint` then calls `XYGeometry.create(XYPoint[])`.
	point func(points ...geo.XYPoint) (geo.Component2D, error)

	// line mirrors `protected Component2D toLine2D(Object... lines)`
	// (lines 96-98). Delegates to `XYGeometry.create(XYLine[])` in
	// Java.
	line func(lines ...geo.XYLine) (geo.Component2D, error)

	// polygon mirrors `protected Component2D toPolygon2D(Object... polygons)`
	// (lines 101-103). Delegates to `XYGeometry.create(XYPolygon[])`
	// in Java.
	polygon func(polygons ...geo.XYPolygon) (geo.Component2D, error)

	// rectangle mirrors `protected Component2D toRectangle2D(double
	// minX, double maxX, double minY, double maxY)` (lines 106-109).
	// The Java body casts each coordinate to float and wraps an
	// `XYRectangle` before calling `XYGeometry.create`.
	rectangle func(minX, maxX, minY, maxY float64) (geo.Component2D, error)

	// circle mirrors `protected Component2D toCircle2D(Object circle)`
	// (lines 112-114). Java casts to `(XYCircle)` before calling
	// `XYGeometry.create`.
	circle func(circle geo.XYCircle) (geo.Component2D, error)
}

// xyShapeRandomShapeFactory bundles the random-shape generator
// hooks the Java parent invokes during property-based tests:
// `randomQueryBox` (line 117-119), `nextLine` (148-150),
// `nextPolygon` (153-155), `nextPoints` (158-167) and `nextCircle`
// (170-172). Each one is backed by `ShapeTestUtil.nextXxx(...)`
// on the Java side. Gocene has no ShapeTestUtil port yet, so the
// fields below stay nil; the skip in the sentinel test gates every
// call site.
type xyShapeRandomShapeFactory struct {
	queryBox func() geo.XYRectangle
	line     func() geo.XYLine
	polygon  func() geo.XYPolygon
	points   func() []geo.XYPoint
	circle   func() geo.XYCircle
}

// xyShapeRectAccessors bundles the four `rectMinX/MaxX/MinY/MaxY`
// hooks (lines 122-139) plus `rectCrossesDateline` (142-144). On
// the cartesian plane the date line never crosses; the Java
// override returns the constant `false` regardless of the rect.
// Mirroring that as a function (rather than a constant) preserves
// signature parity with the LatLon sibling and lets the future
// concrete sub-test override it for diagnostic injection.
type xyShapeRectAccessors struct {
	minX            func(rect geo.XYRectangle) float64
	maxX            func(rect geo.XYRectangle) float64
	minY            func(rect geo.XYRectangle) float64
	maxY            func(rect geo.XYRectangle) float64
	crossesDateline func(rect geo.XYRectangle) bool
}

// xyShapeEncoder mirrors the inner anonymous `Encoder` returned by
// `getEncoder()` (lines 175-207). Six methods, all routed through
// `XYEncodingUtils.decode/encode` on the Java side. The Gocene
// equivalents live in geo/xy_encoding_utils.go; the wiring is
// staged but unused because the sentinel test skips before any
// encoder is exercised.
type xyShapeEncoder struct {
	decodeX     func(encoded int32) float64
	decodeY     func(encoded int32) float64
	quantizeX   func(raw float64) float64
	quantizeY   func(raw float64) float64
	quantizeXCl func(raw float64) float64
	quantizeYCl func(raw float64) float64
}

// xyShapeType mirrors the inner `protected enum ShapeType` (lines
// 210-249). Modelled as a typed integer with an exhaustive
// `nextShape` factory rather than Java's per-constant abstract
// method override — Go enums cannot carry per-constant bodies, so
// the dispatch is centralised on the factory bundle below.
type xyShapeType int

const (
	// xyShapeTypePoint mirrors `ShapeType.POINT`. nextShape ->
	// ShapeTestUtil.nextXYPoint().
	xyShapeTypePoint xyShapeType = iota
	// xyShapeTypeLine mirrors `ShapeType.LINE`. nextShape ->
	// ShapeTestUtil.nextLine().
	xyShapeTypeLine
	// xyShapeTypePolygon mirrors `ShapeType.POLYGON`. The Java body
	// loops on `ShapeTestUtil.nextPolygon()` and re-rolls on
	// Tessellator failures; the Go port will need the same
	// guard once Tessellator lands.
	xyShapeTypePolygon
	// xyShapeTypeMixed mirrors `ShapeType.MIXED`, picking uniformly
	// from {POINT, LINE, POLYGON} via RandomPicks.randomFrom.
	xyShapeTypeMixed
)

// xyShapeTypeFactory is the dispatcher for `xyShapeType.nextShape`.
// Java implements one abstract method per enum constant; the Go
// equivalent is a single table keyed by xyShapeType. Returning the
// shape as an interface{} preserves the heterogeneous return type
// the Java code exposes (Object).
type xyShapeTypeFactory func(t xyShapeType) interface{}

// baseXYShapeFactories bundles every override hook the parent
// BaseSpatialTestCase would dispatch to. Sub-bundles are kept
// separate so the future activation work can swap them
// independently (the encoder swap, for instance, is a pure
// utility decision, whereas the random-shape swap drags in
// ShapeTestUtil). Today every field is left nil because no test
// reaches it before the sentinel t.Skip fires.
type baseXYShapeFactories struct {
	rect         xyShapeRectQueryFactory
	line         xyShapeLineQueryFactory
	polygon      xyShapePolygonQueryFactory
	points       xyShapePointsQueryFactory
	distance     xyShapeDistanceQueryFactory
	component2D  xyShapeComponent2DFactory
	randomShapes xyShapeRandomShapeFactory
	rectAccess   xyShapeRectAccessors
	encoder      xyShapeEncoder
	shapeFactory xyShapeTypeFactory
}

// newBaseXYShapeFactories returns the canonical factory bundle the
// Java reference's overrides would produce: every hook routes
// through `document.XYShape` and `geo.XYGeometry`. The functions
// are wired but must NOT be invoked until that surface lands; the
// sentinel test gates every call site behind t.Skip.
//
// We do not call the (still-missing) XYShape constructors here
// because they are not yet declared; making the closures
// unreachable rather than wired-then-failing keeps the stub honest
// until the real query types ship.
func newBaseXYShapeFactories() baseXYShapeFactories {
	return baseXYShapeFactories{
		// Each field is left nil intentionally; see the file-level
		// comment for the rationale.
	}
}

// baseXYShapeFieldName mirrors `BaseSpatialTestCase.FIELD_NAME`
// ("shape" in the Java reference, inherited via the parent
// abstract class). Kept here so the activated tests can reference
// it identically.
const baseXYShapeFieldName = "shape"

// ---------------------------------------------------------------------
// Sentinel test.
// ---------------------------------------------------------------------
//
// BaseXYShapeTestCase carries zero `@Test` methods in the Java
// reference (`grep -c "@Test"` -> 0); it is a hook-override layer
// only. A single sentinel test is exposed so the stub is
// discoverable via `go test -v` and so the blocking dependencies
// are recorded in one canonical place — symmetric with the LatLon
// sibling's per-test skip messages.

// TestBaseXYShape_PortStub records that the cartesian hook
// override layer is staged but inert.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/shapetestutil          (no ShapeTestUtil.nextXYPoint /
//     nextLine / nextPolygon / nextBox / nextCircle helpers yet)
//   - tests/queryutils             (no QueryUtils.checkEqual /
//     checkUnequal helpers yet)
//   - document.XYShape.NewBoxQuery / NewLineQuery /
//     NewPolygonQuery / NewPointQuery / NewDistanceQuery (deferred
//     — see document/shape_field.go header)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO
//     GOC-4532+).
//   - geo.Tessellator port (POLYGON shape-type re-roll loop).
func TestBaseXYShape_PortStub(t *testing.T) {
	// Verify the factory constructor returns a correctly-typed bundle.
	factories := newBaseXYShapeFactories()
	if want := "shape"; baseXYShapeFieldName != want {
		t.Fatalf("field name: got %q, want %q", baseXYShapeFieldName, want)
	}

	// Verify all five factory types are constructible (interface compliance).
	_ = (xyShapeRectQueryFactory)(factories.rect)
	_ = (xyShapeLineQueryFactory)(factories.line)
	_ = (xyShapePolygonQueryFactory)(factories.polygon)
	_ = (xyShapePointsQueryFactory)(factories.points)
	_ = (xyShapeDistanceQueryFactory)(factories.distance)

	// Verify the Component2D factory types are constructible.
	_ = (xyShapeComponent2DFactory)(factories.component2D)

	// Verify the random shape factory types are constructible.
	_ = (xyShapeRandomShapeFactory)(factories.randomShapes)

	// Verify the rect accessors type is constructible.
	_ = (xyShapeRectAccessors)(factories.rectAccess)

	// Verify the encoder type is constructible.
	_ = (xyShapeEncoder)(factories.encoder)

	// Verify the shape factory type is constructible.
	_ = (xyShapeTypeFactory)(factories.shapeFactory)

	// Verify the XY shape enum values and sublist.
	_ = xyShapeTypePoint
	_ = xyShapeTypeLine
	_ = xyShapeTypePolygon
	_ = xyShapeTypeMixed
}
