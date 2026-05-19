// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonMultiLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonMultiLineShapeQueries (GOC-4003).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.LINE,
//   - overrides nextShape to return an array of 1..4 random Line values
//     (multi-line geometry), each produced by nextLine() from the parent
//     harness,
//   - delegates indexable-field creation to LatLonShape.createIndexableFields
//     for every Line in the array, concatenating the resulting Field[] into
//     a single flat array,
//   - exposes a MultiLineValidator that composes the per-Line
//     LineValidator (defined in TestLatLonLineShapeQueries) under a
//     short-circuit loop honouring INTERSECTS/CONTAINS/DISJOINT/WITHIN
//     short-circuit semantics across the multi-line set,
//   - reuses the inherited @Nightly testRandomBig with 10_000 iterations.
//
// The class itself declares no non-nightly @Test methods; every @Test is
// inherited from BaseLatLonShapeTestCase (which in turn inherits from
// BaseLatLonSpatialTestCase -> BaseSpatialTestCase). The subclass exists
// solely to wire the abstract harness onto a multi-Line geographic
// geometry array.
//
// Gocene currently lacks the entire shape random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase /
//     BaseSpatialTestCase (abstract parents — see
//     [[base_lat_lon_shape_test_case_test]] and
//     [[base_lat_lon_spatial_test_case_test]] which themselves t.Skip),
//   - document.LatLonShape.CreateIndexableFields (Line overload, used per
//     element of the multi-Line array),
//   - Component2D / WithinRelation truth-source plumbing used by the
//     composed LineValidator,
//   - TestLatLonLineShapeQueries.LineValidator (the inner truth source
//     this multi-line validator wraps; see
//     [[lat_lon_line_shape_queries_test]] which is itself a stub),
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
// harness, document.LatLonShape Line overloads, the LineValidator from
// the sibling stub, and the GeoTestUtil/RandomIndexWriter/QueryUtils/
// CheckHits helpers land in Go.
func TestLatLonMultiLineShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseLatLonShapeTestCase parent harness, " +
		"document.LatLonShape.CreateIndexableFields(Line), Component2D/" +
		"WithinRelation truth source, TestLatLonLineShapeQueries.LineValidator " +
		"(itself a stub), and RandomIndexWriter/GeoTestUtil/CheckHits/" +
		"QueryUtils plumbing; remove when fixed")
}
