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
// lucene/core/src/test/org/apache/lucene/document/TestLatLonDocValuesMultiPointPointQueries.java
// (Apache Lucene 10.4.0).
//
// The Java class is a thin subclass of `BaseLatLonDocValueTestCase`
// (itself a subclass of `BaseLatLonSpatialTestCase` → `BaseSpatialTestCase`).
// It plugs four hooks into the abstract harness:
//
//   - `getShapeType()`     → ShapeType.POINT
//   - `nextShape()`        → an array of 1..4 random `Point`s
//                            via `ShapeType.POINT.nextShape()`
//   - `createIndexableFields(name, o)` → one
//                            `LatLonDocValuesField(FIELD_NAME, lat, lon)`
//                            per Point in the array
//   - `getValidator()`     → a new `MultiPointValidator(ENCODER)`
//
// It also overrides the single `@Nightly @Test testRandomBig()` method
// to drive `doTestRandom(10000)` instead of the default 50.
//
// The body of the test matrix lives entirely on the parent classes
// already ported (as degraded stubs) in
// base_lat_lon_point_test_case_test.go and
// base_lat_lon_spatial_test_case_test.go. The MultiPointValidator
// inner class is novel to this file and is reproduced verbatim below as
// a Go-typed helper.
//
// Gocene currently lacks the test infrastructure those inherited tests
// rely on (RandomIndexWriter, GeoTestUtil random generators,
// QueryUtils, the LuceneTestCase Directory/Searcher helpers, a real
// `LatLonDocValuesField.NewSlowGeometryQuery` factory, and a wired
// `Component2D.Contains`-driven point sampler). Every inherited
// `@Test` method is therefore staged as a skipped Go stub that
// preserves the test names so activation cost, once the infra arrives,
// is a one-line removal of t.Skip. This matches the GOC-3985/3987
// pattern already established on this branch.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every `@Test` method visible at the subclass level (here:
//     `testRandomBig`) plus the five inherited from `BaseSpatialTestCase`
//     has a 1:1 Go counterpart;
//   - each Test* opens with t.Skip naming the missing piece
//     explicitly, so `go test -v` records the work without ever
//     touching the non-existent surfaces;
//   - the helpers below are typed and constructible but never invoke
//     `LatLonDocValuesField.NewSlowGeometryQuery` or any Component2D
//     query — the skip happens before any helper is exercised.

// ---------------------------------------------------------------------
// Subclass-owned hook overrides (Java lines 29-57).
// ---------------------------------------------------------------------
//
// In the Java reference these are `protected` overrides on the
// concrete subclass. The Go port models them as package-private free
// functions so a future activation patch can wire them through the
// harness without inheritance.

// latLonDocValuesMultiPointShapeType mirrors Java line 30:
//
//	@Override
//	protected ShapeType getShapeType() {
//	  return ShapeType.POINT;
//	}
//
// Kept as a constant of the existing latLonShapeType enum so the
// activated `doTestRandom` driver can dispatch on it identically to
// the Java side.
const latLonDocValuesMultiPointShapeType = latLonShapeTypePoint

// latLonDocValuesMultiPointMaxPoints mirrors the magic constant in the
// Java `nextShape()` body (line 36):
//
//	int n = random().nextInt(4) + 1;
//
// i.e. between 1 and 4 inclusive. Exported as a const so the activated
// implementation calls `rand.Intn(latLonDocValuesMultiPointMaxPoints) + 1`
// without re-deriving the literal.
const latLonDocValuesMultiPointMaxPoints = 4

// nextLatLonDocValuesMultiPointShape mirrors `nextShape()` (Java lines
// 35-42). The Java body returns an `Object` that downcasts to
// `Point[]`; the Go port returns `[]geo.Point` directly so the call
// site stays statically typed.
//
// Body intentionally returns nil: every caller is gated behind t.Skip
// because the upstream `ShapeType.POINT.nextShape()` helper routes
// through `GeoTestUtil.nextPoint()` which Gocene has not yet ported.
// The activation patch replaces this with:
//
//	n := rand.Intn(latLonDocValuesMultiPointMaxPoints) + 1
//	out := make([]geo.Point, n)
//	for i := range out {
//	    out[i] = nextLatLonPoint() // GeoTestUtil-backed once available
//	}
//	return out
func nextLatLonDocValuesMultiPointShape() []geo.Point { return nil }

// createLatLonDocValuesMultiPointIndexableFields mirrors
// `createIndexableFields(String name, Object o)` (Java lines 45-52).
// The Java body downcasts `o` to `Point[]` and emits one
// `LatLonDocValuesField` per element, all sharing the same field name.
// The Go port restores the static typing because the caller already
// has a `[]geo.Point` in hand.
//
// Body returns nil: every caller is gated behind t.Skip until the
// abstract harness can index the resulting fields. The activation
// patch replaces this with:
//
//	out := make([]*document.LatLonDocValuesField, len(points))
//	for i, p := range points {
//	    f, err := document.NewLatLonDocValuesField(name, p.Lat(), p.Lon())
//	    if err != nil { return nil, err }
//	    out[i] = f
//	}
//	return out
//
// The signature uses the concrete `*document.LatLonDocValuesField`
// rather than a `document.Field` interface because the Java side
// returns `Field[]`, not `IndexableField[]`, and the LatLon flavour is
// monomorphic.
func createLatLonDocValuesMultiPointIndexableFields(
	name string,
	points []geo.Point,
) []*document.LatLonDocValuesField {
	return nil
}

// ---------------------------------------------------------------------
// MultiPointValidator (Java lines 59-91).
// ---------------------------------------------------------------------
//
// The Java inner class extends the abstract `Validator` declared on
// `BaseSpatialTestCase`. It delegates the per-point work to
// `TestLatLonPointShapeQueries.PointValidator` (an Encoder-driven
// truth source) and combines the per-point results according to the
// active `QueryRelation`.
//
// Gocene has neither `Validator` nor `PointValidator` yet (both are
// part of the deferred test harness — see TestXYPointShapeDVQueries
// for the corresponding XY-side skip). The port models the inner
// class as an exported-shaped struct so the activation patch can
// embed the eventual `Validator` parent without renaming.

// latLonDocValuesMultiPointValidator mirrors `MultiPointValidator`
// (Java lines 59-91). The struct holds the same two pieces of state as
// the Java original: the active `QueryRelation` and a delegate
// `PointValidator`. The delegate is typed `any` because the
// `PointValidator` Go counterpart has not been ported yet (it lives in
// the prerequisite `TestLatLonPointShapeQueries` port, not yet on this
// branch); the activation patch replaces `any` with the concrete type
// once available.
type latLonDocValuesMultiPointValidator struct {
	// queryRelation mirrors the inherited
	// `protected QueryRelation queryRelation = QueryRelation.INTERSECTS;`
	// field declared on `BaseSpatialTestCase.Validator` (Java line
	// 738). The default of `INTERSECTS` is preserved so the zero value
	// matches the Java post-construction state.
	queryRelation document.QueryRelation

	// pointValidator mirrors the
	// `TestLatLonPointShapeQueries.PointValidator POINTVALIDATOR`
	// field (Java line 60). Typed `any` because the delegate type
	// itself is not yet ported; see file-level comment.
	pointValidator any

	// encoder mirrors the inherited `Encoder encoder` field on the
	// abstract `Validator` (Java BaseSpatialTestCase line 732). Stored
	// here verbatim rather than embedded because Go has no inheritance.
	encoder latLonEncoder
}

// newLatLonDocValuesMultiPointValidator mirrors the constructor at
// Java lines 62-65:
//
//	MultiPointValidator(Encoder encoder) {
//	  super(encoder);
//	  POINTVALIDATOR = new TestLatLonPointShapeQueries.PointValidator(encoder);
//	}
//
// The `PointValidator` delegate is left nil because the prerequisite
// port is not yet on this branch; callers are gated behind t.Skip.
func newLatLonDocValuesMultiPointValidator(
	encoder latLonEncoder,
) *latLonDocValuesMultiPointValidator {
	return &latLonDocValuesMultiPointValidator{
		queryRelation:  document.QueryRelationIntersects,
		pointValidator: nil,
		encoder:        encoder,
	}
}

// setRelation mirrors the override at Java lines 67-72:
//
//	@Override
//	public Validator setRelation(QueryRelation relation) {
//	  super.setRelation(relation);
//	  POINTVALIDATOR.queryRelation = relation;
//	  return this;
//	}
//
// Returns the receiver pointer so the call chains identically. The
// pointValidator field is `any`-typed so we cannot push the relation
// into it here; the activation patch must unwrap once
// `latLonPointValidator` exists.
func (v *latLonDocValuesMultiPointValidator) setRelation(
	relation document.QueryRelation,
) *latLonDocValuesMultiPointValidator {
	v.queryRelation = relation
	// TODO(activation): once the LatLonPointValidator Go port lands
	// (paired with the TestLatLonPointShapeQueries port), unwrap
	// `v.pointValidator` and propagate `relation` into its
	// `queryRelation` field, mirroring the Java
	// `POINTVALIDATOR.queryRelation = relation` assignment.
	return v
}

// testComponentQuery mirrors the override at Java lines 74-90:
//
//	@Override
//	public boolean testComponentQuery(Component2D query, Object shape) {
//	  Point[] points = (Point[]) shape;
//	  for (Point p : points) {
//	    boolean b = POINTVALIDATOR.testComponentQuery(query, p);
//	    if (b == true && queryRelation == QueryRelation.INTERSECTS)  return true;
//	    else if (b == true && queryRelation == QueryRelation.CONTAINS) return true;
//	    else if (b == false && queryRelation == QueryRelation.DISJOINT) return false;
//	    else if (b == false && queryRelation == QueryRelation.WITHIN)  return false;
//	  }
//	  return queryRelation != QueryRelation.INTERSECTS
//	      && queryRelation != QueryRelation.CONTAINS;
//	}
//
// Returns false unconditionally because the delegate is nil. The four
// early-return branches are preserved as commented-out scaffolding so
// the activation patch only has to swap the constant return for the
// real delegate invocation. The live signature is preserved so callers
// compile against the activation-ready surface.
func (v *latLonDocValuesMultiPointValidator) testComponentQuery(
	query geo.Component2D,
	shape []geo.Point,
) bool {
	// Defensive use: keep the parameters live so future activation
	// edits surface as one-line body changes rather than signature
	// changes.
	_ = query
	_ = shape

	// TODO(activation): replace the body with the per-point loop and
	// short-circuit rules below once the LatLonPointValidator Go port
	// lands.
	//
	//	for _, p := range shape {
	//	    b := v.pointValidator.testComponentQuery(query, p)
	//	    switch {
	//	    case b && v.queryRelation == document.QueryRelationIntersects:
	//	        return true
	//	    case b && v.queryRelation == document.QueryRelationContains:
	//	        return true
	//	    case !b && v.queryRelation == document.QueryRelationDisjoint:
	//	        return false
	//	    case !b && v.queryRelation == document.QueryRelationWithin:
	//	        return false
	//	    }
	//	}
	//	return v.queryRelation != document.QueryRelationIntersects &&
	//	    v.queryRelation != document.QueryRelationContains
	return false
}

// ---------------------------------------------------------------------
// Ported @Test methods.
// ---------------------------------------------------------------------
//
// The subclass declares a single @Test override (`testRandomBig`,
// Java lines 93-97) and inherits five from `BaseSpatialTestCase`.
// All six surface as Go Test* stubs below so `go test -v` enumerates
// the activation budget.
//
// The five inherited tests share the same blocker list as the
// degraded port in base_lat_lon_spatial_test_case_test.go; the
// per-Test Skip strings are intentionally per-test (not file-wide)
// so future activation can chip away at one at a time.

// TestLatLonDocValuesMultiPointPoint_SameShapeManyTimes ports the
// inherited `BaseSpatialTestCase#testSameShapeManyTimes` (Java parent
// line 72) as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred)
//   - TestLatLonPointShapeQueries.PointValidator Go port (deferred)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonDocValuesMultiPointPoint_SameShapeManyTimes(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonDocValuesField.NewSlowGeometryQuery/PointValidator/LuceneTestCase helpers; remove this Skip when fixed")

	// Reserved helpers: keep the symbols live for static analysis so
	// the activation patch surfaces as body fills rather than imports.
	_ = nextLatLonDocValuesMultiPointShape
	_ = createLatLonDocValuesMultiPointIndexableFields
	_ = newLatLonDocValuesMultiPointValidator
	_ = latLonDocValuesMultiPointShapeType
	_ = latLonDocValuesMultiPointMaxPoints
}

// TestLatLonDocValuesMultiPointPoint_LowCardinalityShapeManyTimes ports
// the inherited `BaseSpatialTestCase#testLowCardinalityShapeManyTimes`
// (Java parent line 85) as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred)
//   - TestLatLonPointShapeQueries.PointValidator Go port (deferred)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonDocValuesMultiPointPoint_LowCardinalityShapeManyTimes(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonDocValuesField.NewSlowGeometryQuery/PointValidator/LuceneTestCase helpers; remove this Skip when fixed")

	_ = nextLatLonDocValuesMultiPointShape
	_ = createLatLonDocValuesMultiPointIndexableFields
	_ = newLatLonDocValuesMultiPointValidator
}

// TestLatLonDocValuesMultiPointPoint_RandomTiny ports the inherited
// `BaseSpatialTestCase#testRandomTiny` (Java parent line 102): an
// `@Slow` 5-document smoke driver into `doTestRandom(5)`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred)
//   - TestLatLonPointShapeQueries.PointValidator Go port (deferred)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonDocValuesMultiPointPoint_RandomTiny(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonDocValuesField.NewSlowGeometryQuery/PointValidator/LuceneTestCase helpers; remove this Skip when fixed")

	_ = nextLatLonDocValuesMultiPointShape
	_ = createLatLonDocValuesMultiPointIndexableFields
	_ = newLatLonDocValuesMultiPointValidator
}

// TestLatLonDocValuesMultiPointPoint_RandomMedium ports the inherited
// `BaseSpatialTestCase#testRandomMedium` (Java parent line 107): an
// `@Slow` `atLeast(20)`-document driver into `doTestRandom`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred)
//   - TestLatLonPointShapeQueries.PointValidator Go port (deferred)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonDocValuesMultiPointPoint_RandomMedium(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonDocValuesField.NewSlowGeometryQuery/PointValidator/LuceneTestCase helpers; remove this Skip when fixed")

	_ = nextLatLonDocValuesMultiPointShape
	_ = createLatLonDocValuesMultiPointIndexableFields
	_ = newLatLonDocValuesMultiPointValidator
}

// TestLatLonDocValuesMultiPointPoint_RandomBig ports the inherited
// `BaseSpatialTestCase#testRandomBig` (Java parent line 112): a
// `@Nightly @Slow` `atLeast(50)`-document driver into the default
// `doTestRandom`. **The subclass overrides this method** (Java lines
// 93-97) to pass `10000` instead of the inherited default. The Go
// port records the override semantics in the Skip string so the
// activation patch wires the magnitude correctly.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonDocValuesField.NewSlowGeometryQuery (deferred)
//   - TestLatLonPointShapeQueries.PointValidator Go port (deferred)
//   - LuceneTestCase.atLeast / random() / @Nightly (no Gocene equivalents yet)
func TestLatLonDocValuesMultiPointPoint_RandomBig(t *testing.T) {
	t.Skip("blocked by RandomIndexWriter/GeoTestUtil/LatLonDocValuesField.NewSlowGeometryQuery/PointValidator/LuceneTestCase helpers + @Nightly gate; subclass override drives doTestRandom(10000); remove this Skip when fixed")

	_ = nextLatLonDocValuesMultiPointShape
	_ = createLatLonDocValuesMultiPointIndexableFields
	_ = newLatLonDocValuesMultiPointValidator
	// The literal `10000` is intentionally inlined here as a reminder
	// to the activation patch (it must NOT use the inherited default
	// of `atLeast(50)`).
	const docTestRandomMagnitude = 10000
	_ = docTestRandomMagnitude
}
