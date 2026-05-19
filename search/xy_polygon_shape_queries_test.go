// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeQueries (GOC-4009).
//
// The Java class is a subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.POLYGON,
//   - delegates indexable-field creation to XYShape.createIndexableFields,
//   - exposes PolygonValidator, an Encoder-based truth source that resolves
//     CONTAINS via testWithinQuery (expecting Component2D.WithinRelation.CANDIDATE)
//     and the remaining relations via testComponentQuery against the
//     polygon's triangulated indexable fields,
//   - overrides @Nightly testRandomBig to drive 25 000 random documents
//     through the parent BaseXYShapeTestCase random sweep.
//
// Gocene currently lacks the random-test harness this subclass depends on:
//   - BaseXYShapeTestCase (abstract parent driving the verifyRandom* sweep),
//   - ShapeTestUtil / RandomNumbers / GeoTestUtil generators,
//   - Component2D.WithinRelation plumbing on the XY side,
//   - RandomIndexWriter + CheckHits + QueryUtils support,
//   - a @Nightly equivalent gate for the big-volume sweep.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and supporting cartesian-shape utilities land in Go.
func TestXYPolygonShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseXYShapeTestCase parent harness, ShapeTestUtil/" +
		"RandomNumbers/GeoTestUtil generators, Component2D.WithinRelation, " +
		"RandomIndexWriter/CheckHits/QueryUtils plumbing, and @Nightly gate; " +
		"remove when fixed")
}
