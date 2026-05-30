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
// lucene/core/src/test/org/apache/lucene/document/TestLatLonPointPointQueries.java
// (Apache Lucene 10.4.0).
//
// The Java class is a thin subclass of `BaseLatLonPointTestCase`
// (itself a subclass of `BaseLatLonSpatialTestCase` → `BaseSpatialTestCase`).
// It plugs three hooks into the abstract harness:
//
//   - `getShapeType()`     → ShapeType.POINT
//   - `getValidator()`     → a new `PointValidator(ENCODER)` (the
//                            same inner class TestLatLonPointShapeQueries
//                            declares, not the multi-point flavour)
//   - `createIndexableFields(name, o)` → a single
//                            `LatLonPoint(FIELD_NAME, lat, lon)` built
//                            from the cast-to-`Point` shape argument
//
// It also overrides the single `@Nightly @Test testRandomBig()` method
// to drive `doTestRandom(10000)` instead of the inherited default of
// `atLeast(50)`.
//
// The body of the test matrix lives entirely on the parent classes
// already ported (as degraded stubs) in
// base_lat_lon_point_test_case_test.go and
// base_lat_lon_spatial_test_case_test.go. The PointValidator inner
// class is novel to TestLatLonPointShapeQueries (the sibling Java
// source) but is referenced by name here through `getValidator()`; it
// is reproduced verbatim below as a Go-typed helper because the sibling
// port has not yet landed on this branch.
//
// The structural sibling of this file is
// lat_lon_multi_point_point_queries_test.go (GOC-3987); the only
// meaningful difference is that the indexable shape is a single
// `geo.Point` rather than a slice and the validator does not iterate.
// The two files are kept in lock-step on purpose.
//
// Gocene currently lacks the test infrastructure those inherited tests
// rely on (RandomIndexWriter, GeoTestUtil random generators,
// QueryUtils, the LuceneTestCase Directory/Searcher helpers, a real
// `LatLonPoint.NewSlowGeometryQuery` factory, the
// `LatLonShape.createIndexableFields` helper used by the within-relation
// validator path, and a wired `Component2D.Contains`-driven point
// sampler). Every inherited `@Test` method is therefore staged as a
// skipped Go stub that preserves the test names so activation cost,
// once the infra arrives, is a one-line removal of t.Skip. This matches
// the GOC-3985/3987/3988 pattern already established on this branch.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every `@Test` method visible at the subclass level (here:
//     `testRandomBig`) plus the two inherited from
//     `BaseLatLonPointTestCase` and the four inherited from
//     `BaseSpatialTestCase` has a 1:1 Go counterpart;
//   - each Test* opens with t.Skip naming the missing piece
//     explicitly, so `go test -v` records the work without ever
//     touching the non-existent surfaces;
//   - the helpers below are typed and constructible but never invoke
//     `LatLonPoint.NewSlowGeometryQuery` or any Component2D
//     query — the skip happens before any helper is exercised.

// ---------------------------------------------------------------------
// Subclass-owned hook overrides (Java lines 29-43).
// ---------------------------------------------------------------------
//
// In the Java reference these are `protected` overrides on the
// concrete subclass. The Go port models them as package-private free
// functions so a future activation patch can wire them through the
// harness without inheritance.

// latLonPointShapeType mirrors Java lines 29-32:
//
//	@Override
//	protected ShapeType getShapeType() {
//	  return ShapeType.POINT;
//	}
//
// Kept as a constant of the existing latLonShapeType enum so the
// activated `doTestRandom` driver can dispatch on it identically to
// the Java side.
const latLonPointShapeType = latLonShapeTypePoint

// createLatLonPointIndexableFields mirrors
// `createIndexableFields(String name, Object o)` (Java lines 39-43).
// The Java body downcasts `o` to `Point` and emits a single
// `LatLonPoint` field. The Go port restores the static typing because
// the caller already has a `geo.Point` in hand.
//
// Body returns nil: every caller is gated behind t.Skip until the
// abstract harness can index the resulting field. The activation
// patch replaces this with:
//
//	f, err := document.NewLatLonPoint(name, point.Lat(), point.Lon())
//	if err != nil { return nil, err }
//	return []*document.LatLonPoint{f}
//
// The signature uses the concrete `*document.LatLonPoint` rather than a
// `document.Field` interface because the Java side returns `Field[]`,
// not `IndexableField[]`, and the LatLon flavour is monomorphic. The
// return type is a slice (not a single pointer) for parity with the
// multi-point sibling's signature; the slice has length 1 in the
// activation path.
func createLatLonPointIndexableFields(
	name string,
	point geo.Point,
) []*document.LatLonPoint {
	_ = name
	_ = point
	return nil
}

// ---------------------------------------------------------------------
// PointValidator (Java lines 45-61).
// ---------------------------------------------------------------------
//
// The Java inner class extends the abstract `Validator` declared on
// `BaseSpatialTestCase`. It delegates the truth-source decision to a
// pair of static helpers from `BaseShapeTestCase` (`testComponentQuery`
// and `testWithinQuery`) that synthesise an indexable shape via
// `LatLonShape.createIndexableFields(...)` and exercise the
// component-2D predicate against it.
//
// Gocene has neither `Validator`, `BaseShapeTestCase.testComponentQuery`
// nor `LatLonShape.createIndexableFields` yet; the latter is the
// triangle-tessellation index path, distinct from the
// `LatLonPoint`-encoded indexable field returned by the subclass's
// own `createIndexableFields`. The port models the inner class as an
// exported-shaped struct so the activation patch can embed the
// eventual `Validator` parent without renaming.
//
// The struct below is intentionally distinct from
// `latLonMultiPointValidator`: the multi-point flavour delegates to
// this very type, whereas this type performs the per-point check
// directly. The two `Validator` flavours are kept side by side so
// each subclass can evolve its activation strategy independently.

// latLonPointValidator mirrors `PointValidator` (Java lines 45-61).
// The struct holds the same two pieces of state as the abstract
// `Validator` parent: the active `QueryRelation` and the spatial
// `Encoder`. The Java inner class adds no fields of its own.
type latLonPointValidator struct {
	// queryRelation mirrors the inherited
	// `protected QueryRelation queryRelation = QueryRelation.INTERSECTS;`
	// field declared on `BaseSpatialTestCase.Validator` (Java line
	// 738). The default of `INTERSECTS` is preserved so the zero value
	// matches the Java post-construction state.
	queryRelation document.QueryRelation

	// encoder mirrors the inherited `Encoder encoder` field on the
	// abstract `Validator` (Java BaseSpatialTestCase line 732). Stored
	// here verbatim rather than embedded because Go has no inheritance.
	encoder latLonEncoder
}

// newLatLonPointValidator mirrors the constructor at Java lines 46-48:
//
//	protected PointValidator(Encoder encoder) {
//	  super(encoder);
//	}
//
// The body adds nothing beyond the super call; the Go port mirrors
// that minimal contract.
func newLatLonPointValidator(
	encoder latLonEncoder,
) *latLonPointValidator {
	return &latLonPointValidator{
		queryRelation: document.QueryRelationIntersects,
		encoder:       encoder,
	}
}

// testComponentQuery mirrors the override at Java lines 50-60:
//
//	@Override
//	public boolean testComponentQuery(Component2D query, Object shape) {
//	  Point p = (Point) shape;
//	  if (queryRelation == QueryRelation.CONTAINS) {
//	    return testWithinQuery(
//	            query, LatLonShape.createIndexableFields("dummy", p.getLat(), p.getLon()))
//	        == Component2D.WithinRelation.CANDIDATE;
//	  }
//	  return testComponentQuery(
//	      query, LatLonShape.createIndexableFields("dummy", p.getLat(), p.getLon()));
//	}
//
// Returns false unconditionally because the two static delegate
// helpers (`testComponentQuery`, `testWithinQuery`) and the
// triangle-tessellation `LatLonShape.createIndexableFields` factory
// are all unported. The CONTAINS short-circuit and the default branch
// are preserved as commented-out scaffolding so the activation patch
// only has to swap the constant return for the real helper calls.
// The live signature is preserved so callers compile against the
// activation-ready surface.
func (v *latLonPointValidator) testComponentQuery(
	query geo.Component2D,
	shape geo.Point,
) bool {
	// Defensive use: keep the parameters live so future activation
	// edits surface as one-line body changes rather than signature
	// changes.
	_ = query
	_ = shape

	// TODO(activation): replace the body with the CONTAINS short-circuit
	// and default-branch dispatch below once the BaseShapeTestCase
	// truth-source helpers and LatLonShape.createIndexableFields land.
	//
	//	tess := document.LatLonShapeCreateIndexableFields("dummy", shape.Lat(), shape.Lon())
	//	if v.queryRelation == document.QueryRelationContains {
	//	    return testWithinQuery(query, tess) == geo.WithinRelationCandidate
	//	}
	//	return testComponentQuery(query, tess)
	return false
}

// ---------------------------------------------------------------------
// Ported @Test methods.
// ---------------------------------------------------------------------
//
// The subclass declares a single @Test override (`testRandomBig`,
// Java lines 63-67) and inherits six from its ancestors:
//   - two from BaseLatLonPointTestCase (boundingBoxQueriesEquivalence,
//     queryEqualsAndHashcode);
//   - four from BaseSpatialTestCase (sameShapeManyTimes,
//     lowCardinalityShapeManyTimes, randomTiny, randomMedium).
//
// All seven surface as Go Test* stubs below so `go test -v` enumerates
// the activation budget.
//
// The inherited tests share the same blocker list as the degraded
// ports in base_lat_lon_point_test_case_test.go and
// base_lat_lon_spatial_test_case_test.go; the per-Test Skip strings
// are intentionally per-test (not file-wide) so future activation can
// chip away at one at a time.

// TestLatLonPointPoint_BoundingBoxQueriesEquivalence ports the
// inherited `BaseLatLonPointTestCase#testBoundingBoxQueriesEquivalence`
// (Java parent line 80) as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextBox helper yet)
//   - document.LatLonPoint.NewBoxQuery (deferred)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO GOC-4532+).
func TestLatLonPointPoint_BoundingBoxQueriesEquivalence(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewBoxQuery/document.NewGeometryQuery; remove this Skip when fixed")

	// Reserved helpers: keep the symbols live for static analysis so
	// the activation patch surfaces as body fills rather than imports.
	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
	_ = latLonPointShapeType
}

// TestLatLonPointPoint_QueryEqualsAndHashcode ports the inherited
// `BaseLatLonPointTestCase#testQueryEqualsAndHashcode` (Java parent
// line 107) as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/queryutils            (no QueryUtils.checkEqual helper yet)
//   - tests/geotestutil           (no GeoTestUtil.nextPolygon helper yet)
//   - document.NewGeometryQuery is still a `nil`-returning
//     placeholder (see document/shape_doc_values.go TODO GOC-4532+).
func TestLatLonPointPoint_QueryEqualsAndHashcode(t *testing.T) {
	t.Fatal("blocked by QueryUtils/GeoTestUtil/document.NewGeometryQuery; remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
}

// TestLatLonPointPoint_SameShapeManyTimes ports the inherited
// `BaseSpatialTestCase#testSameShapeManyTimes` (Java parent line 72)
// as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonPoint.NewSlowGeometryQuery (deferred)
//   - document.LatLonShape.CreateIndexableFields (deferred — needed
//     by latLonPointValidator.testComponentQuery)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonPointPoint_SameShapeManyTimes(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewSlowGeometryQuery/LatLonShape.CreateIndexableFields/LuceneTestCase helpers; remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
	_ = latLonPointShapeType
}

// TestLatLonPointPoint_LowCardinalityShapeManyTimes ports the
// inherited `BaseSpatialTestCase#testLowCardinalityShapeManyTimes`
// (Java parent line 85) as exercised by this concrete subclass.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonPoint.NewSlowGeometryQuery (deferred)
//   - document.LatLonShape.CreateIndexableFields (deferred — needed
//     by latLonPointValidator.testComponentQuery)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonPointPoint_LowCardinalityShapeManyTimes(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewSlowGeometryQuery/LatLonShape.CreateIndexableFields/LuceneTestCase helpers; remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
}

// TestLatLonPointPoint_RandomTiny ports the inherited
// `BaseSpatialTestCase#testRandomTiny` (Java parent line 102): an
// `@Slow` 5-document smoke driver into `doTestRandom(5)`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonPoint.NewSlowGeometryQuery (deferred)
//   - document.LatLonShape.CreateIndexableFields (deferred — needed
//     by latLonPointValidator.testComponentQuery)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonPointPoint_RandomTiny(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewSlowGeometryQuery/LatLonShape.CreateIndexableFields/LuceneTestCase helpers; remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
}

// TestLatLonPointPoint_RandomMedium ports the inherited
// `BaseSpatialTestCase#testRandomMedium` (Java parent line 107): an
// `@Slow` `atLeast(20)`-document driver into `doTestRandom`.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonPoint.NewSlowGeometryQuery (deferred)
//   - document.LatLonShape.CreateIndexableFields (deferred — needed
//     by latLonPointValidator.testComponentQuery)
//   - LuceneTestCase.atLeast / random() (no Gocene equivalents yet)
func TestLatLonPointPoint_RandomMedium(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewSlowGeometryQuery/LatLonShape.CreateIndexableFields/LuceneTestCase helpers; remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
}

// TestLatLonPointPoint_RandomBig ports the inherited
// `BaseSpatialTestCase#testRandomBig` (Java parent line 112): a
// `@Nightly @Slow` `atLeast(50)`-document driver into the default
// `doTestRandom`. **The subclass overrides this method** (Java lines
// 63-67) to pass `10000` instead of the inherited default. The Go
// port records the override semantics in the Skip string so the
// activation patch wires the magnitude correctly.
//
// Blocked by:
//   - tests/randomindexwriter      (no RandomIndexWriter in Gocene yet)
//   - tests/geotestutil            (no GeoTestUtil.nextPoint helper yet)
//   - document.LatLonPoint.NewSlowGeometryQuery (deferred)
//   - document.LatLonShape.CreateIndexableFields (deferred — needed
//     by latLonPointValidator.testComponentQuery)
//   - LuceneTestCase.atLeast / random() / @Nightly (no Gocene equivalents yet)
func TestLatLonPointPoint_RandomBig(t *testing.T) {
	t.Fatal("blocked by RandomIndexWriter/GeoTestUtil/LatLonPoint.NewSlowGeometryQuery/LatLonShape.CreateIndexableFields/LuceneTestCase helpers + @Nightly gate; subclass override drives doTestRandom(10000); remove this Skip when fixed")

	_ = createLatLonPointIndexableFields
	_ = newLatLonPointValidator
	// The literal `10000` is intentionally inlined here as a reminder
	// to the activation patch (it must NOT use the inherited default
	// of `atLeast(50)`).
	const docTestRandomMagnitude = 10000
	_ = docTestRandomMagnitude
}
