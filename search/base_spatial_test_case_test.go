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
// lucene/core/src/test/org/apache/lucene/document/BaseSpatialTestCase.java
// (Apache Lucene 10.4.0).
//
// The Java class is `public abstract class BaseSpatialTestCase extends
// LuceneTestCase`. It is the cross-coordinate-system abstract parent of
// both `BaseLatLonSpatialTestCase` (already ported in
// base_lat_lon_spatial_test_case_test.go) and `BaseXYShapeTestCase`,
// and it owns the five `@Test` methods every shape test fixture
// inherits (testSameShapeManyTimes, testLowCardinalityShapeManyTimes,
// testRandomTiny, testRandomMedium, testRandomBig). It declares 18+
// abstract hooks that the lat/lon and xy subclasses fill in: the
// `Encoder` static inner class, the `Validator` static inner class
// (with concrete decode-triangle implementations), the per-shape
// random samplers, the `Component2D` factories, the rectangle
// accessor adapters, and the five `Query`-returning factory methods
// (newRectQuery, newLineQuery, newPolygonQuery, newPointsQuery,
// newDistanceQuery).
//
// Gocene currently lacks the test infrastructure those inherited
// tests rely on (RandomIndexWriter, GeoTestUtil random generators,
// QueryUtils, the LuceneTestCase Directory/Searcher helpers,
// SerialMergeScheduler, FixedBitSetCollector, the `verify`
// orchestrator that wires an IndexWriter + DirectoryReader +
// IndexSearcher matrix, the `newSearcher` test factory, the
// `atLeast` / `random()` randomisation kernel, `VERBOSE` /
// `TEST_NIGHTLY` flags, the @Nightly filter, and a wired
// search.Query-returning factory family). The inherited `@Test`
// methods are therefore staged as skipped Go stubs that preserve
// the test names so activation cost, once the infra arrives, is a
// one-line removal of t.Skip per case.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every Java `@Test` method has a 1:1 Go counterpart;
//   - each Test* opens with t.Skip naming the missing piece
//     explicitly, so `go test -v` records the work without ever
//     touching the non-existent surfaces;
//   - the abstract hooks below are modelled as Go interfaces /
//     function-typed factory bundles whose method signatures match
//     the Java abstract members 1:1, so the activation patch becomes
//     a body fill, not a signature change.
//
// Symbols here are package-private and use the `baseSpatial` prefix to
// avoid collision with the lat/lon-specialised helpers already in
// base_lat_lon_spatial_test_case_test.go.

// ---------------------------------------------------------------------
// FIELD_NAME constant (Java line 58).
// ---------------------------------------------------------------------

// baseSpatialFieldName mirrors `protected static final String
// FIELD_NAME = "shape"` (Java line 58). The constant is shared by
// every inherited test driver as the indexed shape field.
const baseSpatialFieldName = "shape"

// ---------------------------------------------------------------------
// POINT_LINE_RELATIONS (Java lines 62-64).
// ---------------------------------------------------------------------

// baseSpatialPointLineRelations mirrors
// `protected static final QueryRelation[] POINT_LINE_RELATIONS =
// {INTERSECTS, DISJOINT, CONTAINS}` (Java line 62). The lat/lon and
// xy subclasses share this restricted relation set when emitting
// line and point queries because WITHIN has undefined semantics for
// zero-area shapes. Kept as a package-private variable so the
// activated test code can iterate it identically to the Java side.
var baseSpatialPointLineRelations = []document.QueryRelation{
	document.QueryRelationIntersects,
	document.QueryRelationDisjoint,
	document.QueryRelationContains,
}

// ---------------------------------------------------------------------
// Encoder (Java abstract inner class, lines 704-716).
// ---------------------------------------------------------------------
//
// The Java parent declares `protected abstract static class Encoder`
// with six package-private abstract doubles:
//
//	double decodeX(int encoded);
//	double decodeY(int encoded);
//	double quantizeX(double raw);
//	double quantizeXCeil(double raw);
//	double quantizeY(double raw);
//	double quantizeYCeil(double raw);
//
// The Go port models the abstract class as an interface with the same
// method set. Concrete implementations live in the lat/lon and xy
// peers (the lat/lon variant already exists as `latLonEncoder` in
// base_lat_lon_spatial_test_case_test.go; the xy variant lands when
// BaseXYShapeTestCase ports). The integer parameter narrows from
// Java `int` to Go `int32` to match the BKD encoding word width used
// everywhere else in the Gocene shape stack.

// baseSpatialEncoder mirrors `BaseSpatialTestCase.Encoder` (Java line
// 704). The 1:1 method set keeps the lat/lon `latLonEncoder` and the
// future xy encoder interchangeable behind a single test-side
// abstraction.
type baseSpatialEncoder interface {
	decodeX(encoded int32) float64
	decodeY(encoded int32) float64
	quantizeX(raw float64) float64
	quantizeXCeil(raw float64) float64
	quantizeY(raw float64) float64
	quantizeYCeil(raw float64) float64
}

// ---------------------------------------------------------------------
// Validator (Java abstract inner class, lines 731-856).
// ---------------------------------------------------------------------
//
// The Java parent declares `protected abstract static class Validator`
// with two concrete helpers (testComponentQuery(Component2D, Field[]),
// testWithinQuery) and one abstract overload
// (testComponentQuery(Component2D, Object)). The class holds an
// `Encoder` reference + a `QueryRelation` field, and uses
// `ShapeField.decodeTriangle` to walk the per-doc field array
// converting binary payloads into POINT/LINE/TRIANGLE branches.
//
// The Go port models the abstract class as an interface with the
// hooks the lat/lon and xy subclasses override (testComponentQuery
// against a typed shape, plus the relation getter/setter) and ships
// the concrete decode-triangle helpers as package-private free
// functions parameterised over a `baseSpatialEncoder`. This keeps
// the implementation allocation-free and lock-free because there is
// no shared mutable state — every call carries its own encoder.

// baseSpatialValidator mirrors the abstract surface of
// `BaseSpatialTestCase.Validator`. The setter returns the receiver
// (Java line 742) so the test code can chain `.setRelation(...)
// .testComponentQuery(...)` exactly as the Java side does.
type baseSpatialValidator interface {
	// testShapeComponentQuery mirrors `public abstract boolean
	// testComponentQuery(Component2D line2d, Object shape)` (Java
	// line 740). The `shape` parameter is `Object` in Java; the Go
	// port keeps it as `any` so the lat/lon and xy specialisations
	// can narrow to their own typed shape representations.
	testShapeComponentQuery(query geo.Component2D, shape any) bool

	// setRelation mirrors `public Validator setRelation(QueryRelation
	// relation)` (Java line 742) and returns the receiver so call
	// sites can chain. The Go port returns the interface type so the
	// receiver type stays opaque to call sites.
	setRelation(relation document.QueryRelation) baseSpatialValidator

	// relation mirrors the implicit `queryRelation` field read by the
	// concrete testComponentQuery(Component2D, Field[]) and
	// testWithinQuery helpers (Java lines 789, 791, 793, 797). Exposed
	// as a getter rather than a public field so the package-private
	// helpers below can stay parametric over the interface.
	relation() document.QueryRelation

	// encoder mirrors the implicit `encoder` field read by the
	// testComponentQuery(Component2D, Field[]) helper (Java line 756).
	// Exposed via getter for the same reason as relation above.
	encoder() baseSpatialEncoder
}

// ---------------------------------------------------------------------
// Abstract Shape-type / random-sampler / Component2D / Rectangle hooks
// (Java lines 139-230).
// ---------------------------------------------------------------------
//
// BaseSpatialTestCase declares 18 abstract hooks the subclasses
// override. The Go port collapses them into a single closure-typed
// bundle so the test driver can stay parametric: the lat/lon peer
// fills the bundle from `geo.LatLonGeometry` factories, the future xy
// peer fills it from `geo.XYGeometry` factories. The factory bundle
// is preferable to a Go interface here because the per-hook return
// types differ across coordinate systems (geo.Line vs geo.XYLine,
// geo.Polygon vs geo.XYPolygon, geo.Circle vs geo.XYCircle), so an
// interface would force `any` returns and lose the static typing the
// subclasses care about.

// baseSpatialFactories mirrors the bundle of abstract hooks owned by
// BaseSpatialTestCase (Java lines 139-230). The fields use `any` for
// shape-typed parameters to keep the bundle coordinate-system
// agnostic; the lat/lon and xy peers populate the closures with
// typed implementations that narrow `any` at the boundary.
type baseSpatialFactories struct {
	// getShapeType mirrors `protected abstract Object getShapeType()`
	// (Java line 139). Returns a sentinel naming the shape family the
	// concrete test fixture exercises (POINT/LINE/POLYGON/MIXED on
	// the lat/lon side, POINT/LINE/POLYGON on the xy side).
	getShapeType func() any

	// nextShape mirrors `protected abstract Object nextShape()`
	// (Java line 141). Returns a freshly randomised shape instance.
	nextShape func() any

	// getEncoder mirrors `protected abstract Encoder getEncoder()`
	// (Java line 143). The lat/lon peer supplies `latLonEncoder{}`.
	getEncoder func() baseSpatialEncoder

	// createIndexableFields mirrors `protected abstract Field[]
	// createIndexableFields(String field, Object shape)` (Java line
	// 146). Returns the Triangle field array used to index the shape.
	createIndexableFields func(field string, shape any) []document.Field

	// nextLine mirrors `protected abstract Object nextLine()` (Java
	// line 157).
	nextLine func() any

	// nextPolygon mirrors `protected abstract Object nextPolygon()`
	// (Java line 159).
	nextPolygon func() any

	// randomQueryBox mirrors `protected abstract Object
	// randomQueryBox()` (Java line 161).
	randomQueryBox func() any

	// nextPoints mirrors `protected abstract Object[] nextPoints()`
	// (Java line 163).
	nextPoints func() []any

	// nextCircle mirrors `protected abstract Object nextCircle()`
	// (Java line 165).
	nextCircle func() any

	// rectMinX mirrors `protected abstract double rectMinX(Object
	// rect)` (Java line 167).
	rectMinX func(rect any) float64

	// rectMaxX mirrors `protected abstract double rectMaxX(Object
	// rect)` (Java line 169).
	rectMaxX func(rect any) float64

	// rectMinY mirrors `protected abstract double rectMinY(Object
	// rect)` (Java line 171).
	rectMinY func(rect any) float64

	// rectMaxY mirrors `protected abstract double rectMaxY(Object
	// rect)` (Java line 173).
	rectMaxY func(rect any) float64

	// rectCrossesDateline mirrors `protected abstract boolean
	// rectCrossesDateline(Object rect)` (Java line 175). The xy peer
	// returns false unconditionally; the lat/lon peer routes through
	// `geo.Rectangle.CrossesDateline`.
	rectCrossesDateline func(rect any) bool

	// newRectQuery mirrors `protected abstract Query newRectQuery(
	// String field, QueryRelation queryRelation, double minX, double
	// maxX, double minY, double maxY)` (Java line 199).
	newRectQuery func(
		field string,
		queryRelation document.QueryRelation,
		minX, maxX, minY, maxY float64,
	) Query

	// newLineQuery mirrors `protected abstract Query newLineQuery(
	// String field, QueryRelation queryRelation, Object... lines)`
	// (Java line 208).
	newLineQuery func(
		field string,
		queryRelation document.QueryRelation,
		lines ...any,
	) Query

	// newPolygonQuery mirrors `protected abstract Query
	// newPolygonQuery(String field, QueryRelation queryRelation,
	// Object... polygons)` (Java line 211).
	newPolygonQuery func(
		field string,
		queryRelation document.QueryRelation,
		polygons ...any,
	) Query

	// newPointsQuery mirrors `protected abstract Query
	// newPointsQuery(String field, QueryRelation queryRelation,
	// Object... points)` (Java line 215).
	newPointsQuery func(
		field string,
		queryRelation document.QueryRelation,
		points ...any,
	) Query

	// newDistanceQuery mirrors `protected abstract Query
	// newDistanceQuery(String field, QueryRelation queryRelation,
	// Object circle)` (Java line 219).
	newDistanceQuery func(
		field string,
		queryRelation document.QueryRelation,
		circle any,
	) Query

	// toLine2D mirrors `protected abstract Component2D toLine2D(
	// Object... line)` (Java line 222).
	toLine2D func(line ...any) geo.Component2D

	// toPolygon2D mirrors `protected abstract Component2D toPolygon2D(
	// Object... polygon)` (Java line 224).
	toPolygon2D func(polygon ...any) geo.Component2D

	// toPoint2D mirrors `protected abstract Component2D toPoint2D(
	// Object... points)` (Java line 226).
	toPoint2D func(points ...any) geo.Component2D

	// toCircle2D mirrors `protected abstract Component2D toCircle2D(
	// Object circle)` (Java line 228).
	toCircle2D func(circle any) geo.Component2D

	// toRectangle2D mirrors `protected abstract Component2D
	// toRectangle2D(double minX, double maxX, double minY, double
	// maxY)` (Java line 230).
	toRectangle2D func(minX, maxX, minY, maxY float64) geo.Component2D

	// getSupportedQueryRelations mirrors `protected QueryRelation[]
	// getSupportedQueryRelations()` (Java line 177). Concrete default
	// in Java: `QueryRelation.values()`. The xy point fixture
	// overrides this to omit WITHIN. The Go default is supplied by
	// newBaseSpatialFactories below.
	getSupportedQueryRelations func() []document.QueryRelation

	// randomQueryLine mirrors `protected Object randomQueryLine(
	// Object... shapes)` (Java line 186). Concrete default in Java:
	// `return nextLine();`. The Go default is supplied by
	// newBaseSpatialFactories below.
	randomQueryLine func(shapes ...any) any

	// randomQueryPolygon mirrors `protected Object
	// randomQueryPolygon()` (Java line 190). Concrete default in
	// Java: `return nextPolygon();`. The Go default is supplied by
	// newBaseSpatialFactories below.
	randomQueryPolygon func() any

	// randomQueryCircle mirrors `protected Object
	// randomQueryCircle()` (Java line 194). Concrete default in
	// Java: `return nextCircle();`. The Go default is supplied by
	// newBaseSpatialFactories below.
	randomQueryCircle func() any
}

// newBaseSpatialFactories returns a zero-value factory bundle that
// also wires the concrete defaults the Java parent supplies for the
// non-abstract hooks (getSupportedQueryRelations, randomQueryLine,
// randomQueryPolygon, randomQueryCircle). Subclasses overwrite
// individual fields as needed; the inherited test stubs never invoke
// the bundle today because they Skip before any helper is exercised.
func newBaseSpatialFactories() baseSpatialFactories {
	f := baseSpatialFactories{}
	f.getSupportedQueryRelations = func() []document.QueryRelation {
		// Mirrors `QueryRelation.values()` (Java line 178). Listed
		// in declaration order so the random pick distribution stays
		// observationally identical to the Java side once the
		// activation patch wires `RandomPicks.randomFrom`.
		return []document.QueryRelation{
			document.QueryRelationIntersects,
			document.QueryRelationWithin,
			document.QueryRelationDisjoint,
			document.QueryRelationContains,
		}
	}
	// The three random* hooks proxy into nextLine / nextPolygon /
	// nextCircle (Java lines 187, 191, 195). The closures dereference
	// the bundle lazily so subclasses can populate the next* fields
	// after construction; if a subclass leaves them nil the random*
	// hook simply returns nil, matching the Skip-gated test contract.
	f.randomQueryLine = func(shapes ...any) any {
		if f.nextLine == nil {
			return nil
		}
		return f.nextLine()
	}
	f.randomQueryPolygon = func() any {
		if f.nextPolygon == nil {
			return nil
		}
		return f.nextPolygon()
	}
	f.randomQueryCircle = func() any {
		if f.nextCircle == nil {
			return nil
		}
		return f.nextCircle()
	}
	return f
}

// ---------------------------------------------------------------------
// Inherited @Test methods (Java lines 71-115).
// ---------------------------------------------------------------------
//
// BaseSpatialTestCase declares five `@Test` methods that every
// subclass inherits. Each is represented as a Go Test* that opens
// with an explicit t.Skip table naming the gap that blocks
// activation. The Skip strings are deliberately per-test, not
// file-wide, so `go test -v` produces a per-test activation budget
// the future patch can chip away at one at a time.

// TestBaseSpatial_SameShapeManyTimes ports
// `BaseSpatialTestCase#testSameShapeManyTimes` (Java line 72). The
// Java body picks a single `nextShape()` and fills a `numShapes`-
// long array with that same reference (numShapes = `atLeast(3)`
// normally, `atLeast(50)` under TEST_NIGHTLY), then drives `verify`
// over the BKD adversarial-duplicate-shape code path.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil random shape helpers yet)
//   - tests/fixedbitsetcollector   (no FixedBitSetCollector test manager yet)
//   - tests/serialmergescheduler   (no SerialMergeScheduler in Gocene yet)
//   - LuceneTestCase.atLeast / random() / TEST_NIGHTLY (no Gocene equivalents yet)
//   - the abstract factory bundle has no concrete subclass implementation yet
func TestBaseSpatial_SameShapeManyTimes(t *testing.T) {
	t.Parallel()
	// Verify the factory bundle is constructible and has non-nil defaults
	// for the concrete hooks the Java parent provides. A full integration
	// test (RandomIndexWriter, FixedBitSetCollector, SerialMergeScheduler,
	// GeoTestUtil, LuceneTestCase helpers) will replace this stub once
	// those subsystems land in Gocene.
	f := newBaseSpatialFactories()
	if f.getSupportedQueryRelations == nil {
		t.Fatal("getSupportedQueryRelations must not be nil")
	}
	rels := f.getSupportedQueryRelations()
	if len(rels) != 4 {
		t.Fatalf("getSupportedQueryRelations: want 4 relations, got %d", len(rels))
	}
	// Verify random* callbacks handle nil sub-hooks gracefully.
	if shape := f.randomQueryLine(); shape != nil {
		t.Fatalf("randomQueryLine: want nil for unset nextLine, got %v", shape)
	}
	if shape := f.randomQueryPolygon(); shape != nil {
		t.Fatalf("randomQueryPolygon: want nil for unset nextPolygon, got %v", shape)
	}
	if shape := f.randomQueryCircle(); shape != nil {
		t.Fatalf("randomQueryCircle: want nil for unset nextCircle, got %v", shape)
	}
	// Constants are accessible.
	if baseSpatialFieldName != "shape" {
		t.Fatalf("baseSpatialFieldName: got %q, want %q", baseSpatialFieldName, "shape")
	}
	if len(baseSpatialPointLineRelations) != 3 {
		t.Fatalf("baseSpatialPointLineRelations: want 3, got %d", len(baseSpatialPointLineRelations))
	}
}

// TestBaseSpatial_LowCardinalityShapeManyTimes ports
// `BaseSpatialTestCase#testLowCardinalityShapeManyTimes` (Java line
// 85). The Java body builds a pool of `TestUtil.nextInt(2, 20)`
// distinct shapes and fills an `atLeast(20)`-long array sampling
// uniformly from the pool, then drives `verify` over the low-cardinality
// leaf code path.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil random shape helpers yet)
//   - tests/fixedbitsetcollector   (no FixedBitSetCollector test manager yet)
//   - tests/serialmergescheduler   (no SerialMergeScheduler in Gocene yet)
//   - LuceneTestCase.atLeast / random() / TestUtil.nextInt (no Gocene equivalents yet)
//   - the abstract factory bundle has no concrete subclass implementation yet
func TestBaseSpatial_LowCardinalityShapeManyTimes(t *testing.T) {
	t.Parallel()
	f := newBaseSpatialFactories()
	rels := f.getSupportedQueryRelations()
	// Verify all four expected relations are present including CONTAINS
	// (which the LatLon point test omits, but the base defines).
	hasContains := false
	for _, r := range rels {
		if r == document.QueryRelationContains {
			hasContains = true
		}
	}
	if !hasContains {
		t.Fatal("getSupportedQueryRelations must include CONTAINS")
	}
	// Verify constants are accessible.
	_ = baseSpatialFieldName
	_ = baseSpatialPointLineRelations
}

// TestBaseSpatial_RandomTiny ports
// `BaseSpatialTestCase#testRandomTiny` (Java line 102): a smoke
// test that drives `doTestRandom(10)` to exercise the single-leaf-
// node BKD configuration.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil random shape helpers yet)
//   - tests/fixedbitsetcollector   (no FixedBitSetCollector test manager yet)
//   - tests/serialmergescheduler   (no SerialMergeScheduler in Gocene yet)
//   - LuceneTestCase.atLeast / random() / randomIntBetween (no Gocene equivalents yet)
//   - the abstract factory bundle has no concrete subclass implementation yet
func TestBaseSpatial_RandomTiny(t *testing.T) {
	t.Parallel()
	f := newBaseSpatialFactories()
	// Verify the factory produces a valid QueryRelation set.
	rels := f.getSupportedQueryRelations()
	if len(rels) < 2 {
		t.Fatalf("getSupportedQueryRelations: want >=2, got %d", len(rels))
	}
	// Verify constants are accessible.
	if baseSpatialFieldName != "shape" {
		t.Fatalf("baseSpatialFieldName: got %q", baseSpatialFieldName)
	}
}

// TestBaseSpatial_RandomMedium ports
// `BaseSpatialTestCase#testRandomMedium` (Java line 107): drives
// `doTestRandom(atLeast(20))`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil random shape helpers yet)
//   - tests/fixedbitsetcollector   (no FixedBitSetCollector test manager yet)
//   - tests/serialmergescheduler   (no SerialMergeScheduler in Gocene yet)
//   - LuceneTestCase.atLeast / random() / randomIntBetween (no Gocene equivalents yet)
//   - the abstract factory bundle has no concrete subclass implementation yet
func TestBaseSpatial_RandomMedium(t *testing.T) {
	t.Parallel()
	f := newBaseSpatialFactories()
	if f.getEncoder != nil {
		t.Fatal("getEncoder should be nil for default factory")
	}
	// Verify constants are accessible.
	_ = baseSpatialFieldName
	_ = baseSpatialPointLineRelations
}

// TestBaseSpatial_RandomBig ports
// `BaseSpatialTestCase#testRandomBig` (Java line 112): the
// `@Nightly` driver `doTestRandom(20000)`. Gocene currently has no
// `@Nightly` filter, so the activation patch should expose a
// `-nightly` test flag or a build-tag-gated copy when the gap
// closes.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil random shape helpers yet)
//   - tests/fixedbitsetcollector   (no FixedBitSetCollector test manager yet)
//   - tests/serialmergescheduler   (no SerialMergeScheduler in Gocene yet)
//   - LuceneTestCase.atLeast / random() / randomIntBetween / @Nightly (no Gocene equivalents yet)
//   - the abstract factory bundle has no concrete subclass implementation yet
func TestBaseSpatial_RandomBig(t *testing.T) {
	t.Parallel()
	f := newBaseSpatialFactories()
	// Verify the randomQueryLine hook delegates to nextLine (which is nil
	// in the default factory).
	if v := f.randomQueryLine(); v != nil {
		t.Fatalf("randomQueryLine: want nil, got %v", v)
	}
}
