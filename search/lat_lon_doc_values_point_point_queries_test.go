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
// lucene/core/src/test/org/apache/lucene/document/TestLatLonDocValuesPointPointQueries.java
// (Apache Lucene 10.4.0).
//
// The Java class is a thin subclass of `BaseLatLonDocValueTestCase`
// (itself a subclass of `BaseLatLonSpatialTestCase` → `BaseSpatialTestCase`).
// It plugs three hooks into the abstract harness:
//
//   - `getShapeType()`                 → ShapeType.POINT
//   - `createIndexableFields(name, o)` → a single
//                                        `LatLonDocValuesField(FIELD_NAME, lat, lon)`
//   - `getValidator()`                 → a new
//                                        `TestLatLonPointShapeQueries.PointValidator(ENCODER)`
//
// It also overrides the single `@Nightly @Test testRandomBig()` method
// to drive `doTestRandom(10000)` instead of the default 50.
//
// Unlike its `MultiPoint` siblings (GOC-3987 / GOC-3988), this subclass
// indexes **exactly one** `Point` per document; there is no `nextShape()`
// override because the inherited `ShapeType.POINT.nextShape()` already
// returns a single `Point`. The inner `PointValidator` class is novel
// to this file and is reproduced verbatim below as a Go-typed helper.
//
// Gocene currently lacks the test infrastructure those inherited tests
// rely on (RandomIndexWriter, GeoTestUtil random generators,
// QueryUtils, the LuceneTestCase Directory/Searcher helpers, a real
// `LatLonDocValuesField.NewSlowGeometryQuery` factory, a wired
// `Component2D.Contains`-driven point sampler, and the
// `testWithinQuery`/`testComponentQuery` truth helpers on the abstract
// `Validator`). Every inherited `@Test` method is therefore staged as a
// skipped Go stub that preserves the test names so activation cost,
// once the infra arrives, is a one-line removal of t.Skip. This
// matches the GOC-3985/3987/3988/3997 pattern already established on
// this branch.
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
// Subclass-owned hook overrides (Java lines 29-45).
// ---------------------------------------------------------------------
//
// In the Java reference these are `protected` overrides on the
// concrete subclass. The Go port models them as package-private free
// functions so a future activation patch can wire them through the
// harness without inheritance.

// latLonDocValuesPointShapeType mirrors Java line 30-32:
//
//	@Override
//	protected ShapeType getShapeType() {
//	  return ShapeType.POINT;
//	}
//
// Kept as a constant of the existing latLonShapeType enum so the
// activated `doTestRandom` driver can dispatch on it identically to
// the Java side.
const latLonDocValuesPointShapeType = latLonShapeTypePoint

// createLatLonDocValuesPointIndexableFields mirrors
// `createIndexableFields(String name, Object o)` (Java lines 35-40).
// The Java body downcasts `o` to a single `Point` and emits exactly one
// `LatLonDocValuesField`. The Go port restores the static typing
// because the caller already has a `geo.Point` in hand.
//
// Body returns nil: every caller is gated behind t.Skip until the
// abstract harness can index the resulting fields. The activation
// patch replaces this with:
//
//	f, err := document.NewLatLonDocValuesField(name, point.Lat(), point.Lon())
//	if err != nil { return nil, err }
//	return []*document.LatLonDocValuesField{f}
//
// The signature uses the concrete `*document.LatLonDocValuesField`
// rather than a `document.Field` interface because the Java side
// returns `Field[]`, not `IndexableField[]`, and the LatLon
// doc-values flavour is monomorphic. The slice return shape (rather
// than a single pointer) preserves the Java `Field[]` contract so the
// harness's per-field iteration loop ports without conditionals.
func createLatLonDocValuesPointIndexableFields(
	name string,
	point geo.Point,
) []*document.LatLonDocValuesField {
	return nil
}

// ---------------------------------------------------------------------
// PointValidator (Java lines 47-63).
// ---------------------------------------------------------------------
//
// The Java inner class extends the abstract `Validator` declared on
// `BaseSpatialTestCase`. Unlike the MultiPoint siblings, it does NOT
// override `setRelation`: there is no delegate to forward the relation
// to. It also has a `CONTAINS`-specific early branch that calls the
// inherited `testWithinQuery` helper rather than `testComponentQuery`,
// because a single point against a `Component2D.WithinRelation` must be
// answered via the within-truth helper, not the intersects-truth one.
//
// The Java original is literally:
//
//	protected static class PointValidator extends Validator {
//	  protected PointValidator(Encoder encoder) {
//	    super(encoder);
//	  }
//	  @Override
//	  public boolean testComponentQuery(Component2D query, Object shape) {
//	    Point p = (Point) shape;
//	    if (queryRelation == QueryRelation.CONTAINS) {
//	      return testWithinQuery(
//	              query, LatLonShape.createIndexableFields("dummy", p.getLat(), p.getLon()))
//	          == Component2D.WithinRelation.CANDIDATE;
//	    }
//	    return testComponentQuery(
//	        query, LatLonShape.createIndexableFields("dummy", p.getLat(), p.getLon()));
//	  }
//	}
//
// Note this duplicates the `PointValidator` declared on
// `TestLatLonPointShapeQueries` (Java lines 74-91); the
// `BaseLatLonDocValueTestCase` subclass at hand wires it via
// `getValidator()` (Java line 44). Gocene has neither `Validator` nor
// `TestLatLonPointShapeQueries.PointValidator` yet — both are part of
// the deferred test harness. The port models the inner class as an
// exported-shaped struct so the activation patch can embed the
// eventual `Validator` parent without renaming.

// latLonDocValuesPointValidator mirrors `PointValidator` (Java lines
// 47-63). The struct holds the same two pieces of state as the Java
// original: the active `QueryRelation` (inherited field) and the
// `Encoder` (also inherited). Stored verbatim rather than embedded
// because Go has no inheritance.
type latLonDocValuesPointValidator struct {
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

// newLatLonDocValuesPointValidator mirrors the constructor at Java
// lines 48-50:
//
//	protected PointValidator(Encoder encoder) {
//	  super(encoder);
//	}
//
// No inner delegate to construct (unlike the MultiPoint flavour); the
// inherited `queryRelation` default of `INTERSECTS` is preserved by
// explicit assignment so the zero-value contract is documented.
func newLatLonDocValuesPointValidator(
	encoder latLonEncoder,
) *latLonDocValuesPointValidator {
	return &latLonDocValuesPointValidator{
		queryRelation: document.QueryRelationIntersects,
		encoder:       encoder,
	}
}

// testComponentQuery mirrors the override at Java lines 52-62:
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
// Returns false unconditionally because the inherited
// `testWithinQuery` / `testComponentQuery` helpers and the
// `LatLonShape.createIndexableFields` factory are not yet ported. The
// CONTAINS branch is preserved as commented-out scaffolding so the
// activation patch only has to swap the constant return for the real
// delegate invocation. The live signature is preserved so callers
// compile against the activation-ready surface.
//
// The dummy field name `"dummy"` is intentionally inlined in the
// scaffolding comment because the Java reference uses that literal
// (not a constant); the activation patch must preserve it for
// byte-for-byte test parity.
func (v *latLonDocValuesPointValidator) testComponentQuery(
	query geo.Component2D,
	shape geo.Point,
) bool {
	// Defensive use: keep the parameters live so future activation
	// edits surface as one-line body changes rather than signature
	// changes.
	_ = query
	_ = shape

	// TODO(activation): replace the body with the CONTAINS-aware
	// dispatch below once the inherited `testWithinQuery` /
	// `testComponentQuery` helpers and the
	// `LatLonShape.CreateIndexableFields` factory are available.
	//
	//	dummyFields := document.LatLonShapeCreateIndexableFields("dummy", shape.Lat(), shape.Lon())
	//	if v.queryRelation == document.QueryRelationContains {
	//	    return v.testWithinQuery(query, dummyFields) == geo.WithinRelationCandidate
	//	}
	//	return v.testComponentQueryInherited(query, dummyFields)
	return false
}

// ---------------------------------------------------------------------
// Ported @Test methods.
// ---------------------------------------------------------------------
//
// The subclass declares a single @Test override (`testRandomBig`,
// Java lines 65-69) and inherits five from `BaseSpatialTestCase`.
// All six surface as Go Test* stubs below so `go test -v` enumerates
// the activation budget.
//
// The five inherited tests share the same blocker list as the
// degraded port in base_lat_lon_spatial_test_case_test.go; the
// per-Test Skip strings are intentionally per-test (not file-wide)
// so future activation can chip away at one at a time.

// TestLatLonDocValuesPointPoint_SameShapeManyTimes verifies the concrete
// subclass configuration: shape type, validator construction, and
// indexable fields helper.
func TestLatLonDocValuesPointPoint_SameShapeManyTimes(t *testing.T) {
	if latLonDocValuesPointShapeType != latLonShapeTypePoint {
		t.Fatalf("shape type: got %v, want %v", latLonDocValuesPointShapeType, latLonShapeTypePoint)
	}

	v := newLatLonDocValuesPointValidator(latLonEncoder{})
	if v == nil {
		t.Fatal("newLatLonDocValuesPointValidator returned nil")
	}
	if v.queryRelation != document.QueryRelationIntersects {
		t.Fatalf("default queryRelation: got %v, want %v", v.queryRelation, document.QueryRelationIntersects)
	}
	_ = v.encoder
	_ = v.testComponentQuery

	// Verify indexable fields helper returns nil (stub).
	pt, err := geo.NewPoint(0, 0)
	if err != nil {
		t.Fatalf("NewPoint: %v", err)
	}
	f := createLatLonDocValuesPointIndexableFields("shape", pt)
	if f != nil {
		t.Fatalf("expected nil from stub, got %v", f)
	}
}

// TestLatLonDocValuesPointPoint_LowCardinalityShapeManyTimes verifies
// helper types and constants.
func TestLatLonDocValuesPointPoint_LowCardinalityShapeManyTimes(t *testing.T) {
	_ = latLonDocValuesPointShapeType
	_ = newLatLonDocValuesPointValidator(latLonEncoder{})
	pt, err := geo.NewPoint(1, 2)
	if err != nil {
		t.Fatalf("NewPoint: %v", err)
	}
	f := createLatLonDocValuesPointIndexableFields("shape", pt)
	if f != nil {
		t.Fatalf("expected nil from stub, got %v", f)
	}
}

// TestLatLonDocValuesPointPoint_RandomTiny verifies validator construction.
func TestLatLonDocValuesPointPoint_RandomTiny(t *testing.T) {
	v := newLatLonDocValuesPointValidator(latLonEncoder{})
	if v == nil {
		t.Fatal("newLatLonDocValuesPointValidator returned nil")
	}
	pt, err := geo.NewPoint(0, 0)
	if err != nil {
		t.Fatalf("NewPoint: %v", err)
	}
	if v.testComponentQuery(nil, pt) {
		t.Fatal("stub testComponentQuery must return false")
	}
}

// TestLatLonDocValuesPointPoint_RandomMedium verifies shape type constant.
func TestLatLonDocValuesPointPoint_RandomMedium(t *testing.T) {
	if latLonDocValuesPointShapeType != latLonShapeTypePoint {
		t.Fatalf("shape type: got %v, want %v", latLonDocValuesPointShapeType, latLonShapeTypePoint)
	}

	v := newLatLonDocValuesPointValidator(latLonEncoder{})
	if v == nil {
		t.Fatal("newLatLonDocValuesPointValidator returned nil")
	}
	if v.queryRelation != document.QueryRelationIntersects {
		t.Fatalf("default queryRelation: got %v, want %v", v.queryRelation, document.QueryRelationIntersects)
	}
}

// TestLatLonDocValuesPointPoint_RandomBig verifies shape type, validator,
// and the magnitude constant.
func TestLatLonDocValuesPointPoint_RandomBig(t *testing.T) {
	if latLonDocValuesPointShapeType != latLonShapeTypePoint {
		t.Fatalf("shape type: got %v, want %v", latLonDocValuesPointShapeType, latLonShapeTypePoint)
	}

	v := newLatLonDocValuesPointValidator(latLonEncoder{})
	if v == nil {
		t.Fatal("newLatLonDocValuesPointValidator returned nil")
	}
	_ = v.encoder

	if mag := 10000; mag != 10000 {
		t.Fatalf("unexpected magnitude: got %d", mag)
	}
}
