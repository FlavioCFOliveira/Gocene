// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPolygonShapeQueries (GOC-4015).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.POLYGON,
//   - delegates indexable-field creation to LatLonShape.createIndexableFields
//     against a geographic Polygon,
//   - exposes a PolygonValidator, an Encoder-based truth source that resolves
//     CONTAINS via testWithinQuery (expecting Component2D.WithinRelation.CANDIDATE)
//     and the remaining relations via testComponentQuery against the polygon's
//     triangulated indexable fields, plus a testWithinPolygon helper used by
//     downstream validators,
//   - overrides @Nightly testRandomBig to drive 25 000 random documents through
//     the parent BaseLatLonShapeTestCase random sweep.
//
// The class itself declares no `@Test` methods; every @Test is inherited from
// BaseLatLonShapeTestCase (which in turn inherits from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase). The subclass exists solely
// to wire the abstract harness onto geographic Polygon geometry.
//
// Gocene currently lacks the entire shape random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase / BaseSpatialTestCase
//     (abstract parents — see [[base_lat_lon_shape_test_case_test]] and
//     [[base_lat_lon_spatial_test_case_test]] which themselves t.Skip),
//   - document.LatLonShape.CreateIndexableFields (Polygon overload),
//   - Component2D / WithinRelation truth-source plumbing used by the
//     PolygonValidator,
//   - RandomIndexWriter + GeoTestUtil (nextPolygon / nextLatitude /
//     nextLongitude) + CheckHits + QueryUtils plumbing inherited transitively
//     from LuceneTestCase,
//   - a @Nightly equivalent gate for the 25 000-document testRandomBig sweep.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles,
//   - the single inherited test name is preserved as a Go counterpart,
//   - the test opens with t.Skip naming the missing pieces explicitly, so
//     `go test -v` records the work without touching the non-existent surfaces.
//
// This stub must be replaced with a real roundtrip once the parent harness,
// document.LatLonShape Polygon overloads, and the GeoTestUtil/
// RandomIndexWriter/QueryUtils/CheckHits helpers land in Go.
func TestLatLonPolygonShapeQueries(t *testing.T) {
	t.Fatal("blocked by BaseLatLonShapeTestCase parent harness, " +
		"document.LatLonShape.CreateIndexableFields(Polygon), Component2D/" +
		"WithinRelation truth source, RandomIndexWriter/GeoTestUtil/CheckHits/" +
		"QueryUtils plumbing, and @Nightly gate; remove when fixed")
}
