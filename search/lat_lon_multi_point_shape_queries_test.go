// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonMultiPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiPointShapeQueries (GOC-4007).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.POINT,
//   - overrides nextShape to return an array of 1..4 random Point values
//     (multi-point geometry), each produced by ShapeType.POINT.nextShape(),
//   - delegates indexable-field creation to LatLonShape.createIndexableFields
//     (called once per Point with getLat()/getLon() and concatenated into a
//     single flat Field[]),
//   - exposes a MultiPointValidator that composes the per-Point
//     PointValidator (defined in TestLatLonPointShapeQueries) under a
//     short-circuit loop honouring INTERSECTS/CONTAINS/DISJOINT/WITHIN
//     semantics across the multi-point set,
//   - reuses the inherited @Nightly testRandomBig with 10_000 iterations.
//
// The class itself declares no non-nightly @Test methods; every @Test is
// inherited from BaseLatLonShapeTestCase (which in turn inherits from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase). The subclass exists
// solely to wire the abstract harness onto a multi-Point geographic
// geometry array.
//
// Gocene currently lacks the entire shape random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase /
//     BaseSpatialTestCase (abstract parents — see
//     [[base_lat_lon_shape_test_case_test]] and
//     [[base_lat_lon_spatial_test_case_test]] which themselves t.Skip),
//   - document.LatLonShape.CreateIndexableFields (point overload taking
//     lat/lon, used per element of the multi-Point array),
//   - geo.Point with getLat/getLon accessors plus the ShapeType.POINT
//     random generator,
//   - Component2D / WithinRelation truth-source plumbing used by the
//     composed PointValidator,
//   - TestLatLonPointShapeQueries.PointValidator (the inner truth source
//     this multi-point validator wraps; see
//     [[lat_lon_point_shape_dv_queries_test]] and related stubs),
//   - RandomIndexWriter + GeoTestUtil (nextLatitude / nextLongitude) +
//     CheckHits + QueryUtils plumbing inherited transitively from
//     LuceneTestCase.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles,
//   - the single inherited test name is preserved as a Go counterpart,
//   - the test opens with t.Skip naming the missing pieces explicitly,
//     so `go test -v` records the work without touching the non-existent
//     surfaces.
//
// This stub must be replaced with a real roundtrip once the parent
// harness, document.LatLonShape point overload, the PointValidator from
// the sibling stub, and the GeoTestUtil/RandomIndexWriter/QueryUtils/
// CheckHits helpers land in Go.
func TestLatLonMultiPointShapeQueries(t *testing.T) {
	t.Fatal("blocked by BaseLatLonShapeTestCase parent harness, " +
		"document.LatLonShape.CreateIndexableFields(lat,lon), geo.Point " +
		"with ShapeType.POINT random generator, Component2D/WithinRelation " +
		"truth source, TestLatLonPointShapeQueries.PointValidator (itself a " +
		"stub), and RandomIndexWriter/GeoTestUtil/CheckHits/QueryUtils " +
		"plumbing; remove when fixed")
}
