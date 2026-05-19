// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonPointShapeQueries (GOC-4020).
//
// The Java class is a thin subclass of BaseLatLonShapeTestCase that:
//   - selects ShapeType.POINT,
//   - delegates indexable-field creation to
//     LatLonShape.createIndexableFields(field, lat, lon),
//   - overrides randomQueryLine to occasionally seed line vertices from
//     the indexed point set (the "share vertices" branch), and
//   - exposes PointValidator (Encoder-based truth source) which both the
//     point-shape and point-shape-doc-values test classes reuse.
//
// PointValidator.testComponentQuery() is the canonical reference
// implementation for "does this point intersect / contain / lie within
// the query Component2D?" and is referenced by the sibling DV test
// class (see lat_lon_point_shape_dv_queries_test.go).
//
// The parent (BaseLatLonShapeTestCase) inherits the bulk of its
// `@Test` matrix from BaseLatLonSpatialTestCase -> BaseSpatialTestCase;
// the subclass adds no `@Test` methods of its own, so this file exists
// purely to wire the abstract harness onto geographic point shapes.
//
// Gocene currently lacks:
//   - BaseLatLonShapeTestCase / BaseLatLonSpatialTestCase / BaseSpatialTestCase
//     (abstract parents, staged as skipped stubs — see
//     base_lat_lon_shape_test_case_test.go and peers),
//   - LatLonShape.createIndexableFields wiring on document.LatLonShape,
//   - RandomIndexWriter + GeoTestUtil + CheckHits + QueryUtils plumbing,
//   - the Component2D / Encoder bridge required by PointValidator.
//
// Per Sprint 55 stub-degraded contract (option c): the test file exists
// and compiles, the Java test names are preserved 1:1, and t.Skip carries
// the activation gate. Replace with a real roundtrip once the parent
// harness and supporting plumbing land in Go.
func TestLatLonPointShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseLatLonShapeTestCase parent harness, " +
		"LatLonShape.createIndexableFields, PointValidator (Component2D + " +
		"Encoder bridge), and RandomIndexWriter/GeoTestUtil/CheckHits/" +
		"QueryUtils plumbing; remove when fixed")
}
