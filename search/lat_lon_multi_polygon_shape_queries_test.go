// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonMultiPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiPolygonShapeQueries (GOC-4019).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.POLYGON,
//   - overrides nextShape to return a disjoint array of 1..4 random Polygons
//     (rejection-sampling against quantized bounding boxes via the Encoder so
//     CONTAINS is well-defined),
//   - fans out indexable-field creation across the array, concatenating the
//     per-polygon Field[] from LatLonShape.createIndexableFields,
//   - exposes a MultiPolygonValidator that wraps
//     TestLatLonPolygonShapeQueries.PolygonValidator and folds per-relation
//     results across the array (INTERSECTS/CONTAINS as OR, DISJOINT/WITHIN as
//     AND), with a dedicated testWithinPolygon that promotes CANDIDATE only
//     when no member returns NOTWITHIN,
//   - overrides @Nightly testRandomBig to drive 10 000 random documents
//     through the parent BaseLatLonShapeTestCase random sweep.
//
// The class itself declares no `@Test` methods; every @Test is inherited from
// BaseLatLonShapeTestCase (which in turn inherits from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase). The subclass exists solely
// to wire the abstract harness onto arrays of geographic Polygon geometry.
//
// Gocene currently lacks the entire shape random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase / BaseSpatialTestCase
//     (abstract parents — see [[base_lat_lon_shape_test_case_test]] and
//     [[base_lat_lon_spatial_test_case_test]] which themselves t.Skip),
//   - document.LatLonShape.CreateIndexableFields (Polygon overload),
//   - Component2D / WithinRelation truth-source plumbing used by the
//     MultiPolygonValidator and the inner PolygonValidator,
//   - TestLatLonPolygonShapeQueries.PolygonValidator (sibling stub, see
//     [[lat_lon_polygon_shape_queries_test]]),
//   - RandomIndexWriter + GeoTestUtil (nextPolygon / nextLatitude /
//     nextLongitude) + CheckHits + QueryUtils plumbing inherited transitively
//     from LuceneTestCase,
//   - a @Nightly equivalent gate for the 10 000-document testRandomBig sweep.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles,
//   - the single inherited test name is preserved as a Go counterpart,
//   - the test opens with t.Skip naming the missing pieces explicitly, so
//     `go test -v` records the work without touching the non-existent surfaces.
//
// This stub must be replaced with a real roundtrip once the parent harness,
// document.LatLonShape Polygon overloads, the sibling PolygonValidator, and
// the GeoTestUtil/RandomIndexWriter/QueryUtils/CheckHits helpers land in Go.
func TestLatLonMultiPolygonShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseLatLonShapeTestCase parent harness, " +
		"document.LatLonShape.CreateIndexableFields(Polygon), Component2D/" +
		"WithinRelation truth source, sibling TestLatLonPolygonShapeQueries." +
		"PolygonValidator, RandomIndexWriter/GeoTestUtil/CheckHits/QueryUtils " +
		"plumbing, and @Nightly gate; remove when fixed")
}
