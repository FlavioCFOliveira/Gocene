// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonLineShapeQueries (GOC-3996).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.LINE,
//   - overrides randomQueryLine to occasionally synthesize a query line
//     that shares vertices with the indexed Line set (1-in-100 odds),
//     and otherwise delegates to nextLine() from the parent harness,
//   - delegates indexable-field creation to LatLonShape.createIndexableFields,
//   - exposes a LineValidator that calls testWithinQuery for CONTAINS
//     and testComponentQuery otherwise, both routed through
//     LatLonShape.createIndexableFields("dummy", line).
//
// The class itself declares no `@Test` methods; every @Test is inherited
// from BaseLatLonShapeTestCase (which in turn inherits from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase). The subclass exists
// solely to wire the abstract harness onto geographic Line geometry.
//
// Gocene currently lacks the entire shape random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase /
//     BaseSpatialTestCase (abstract parents — see
//     [[base_lat_lon_shape_test_case_test]] and
//     [[base_lat_lon_spatial_test_case_test]] which themselves t.Skip),
//   - document.LatLonShape.CreateIndexableFields (Line overload),
//   - Component2D / WithinRelation truth-source plumbing used by the
//     LineValidator,
//   - RandomIndexWriter + GeoTestUtil (nextLine / nextLatitude /
//     nextLongitude) + CheckHits + QueryUtils plumbing inherited
//     transitively from LuceneTestCase.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles,
//   - the single inherited test name is preserved as a Go counterpart,
//   - the test opens with t.Skip naming the missing pieces explicitly,
//     so `go test -v` records the work without touching the non-existent
//     surfaces.
//
// This stub must be replaced with a real roundtrip once the parent
// harness, document.LatLonShape Line overloads, and the GeoTestUtil/
// RandomIndexWriter/QueryUtils/CheckHits helpers land in Go.
func TestLatLonLineShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseLatLonShapeTestCase parent harness, " +
		"document.LatLonShape.CreateIndexableFields(Line), Component2D/" +
		"WithinRelation truth source, and RandomIndexWriter/GeoTestUtil/" +
		"CheckHits/QueryUtils plumbing; remove when fixed")
}
