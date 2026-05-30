// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYLineShapeQueries (GOC-4014).
//
// The Java class is a subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.LINE,
//   - overrides randomQueryLine to occasionally synthesise a query line that
//     shares vertices with the indexed line set (1-in-100 sampling, capped at
//     ~10% of the corpus) and otherwise delegates to nextLine,
//   - delegates indexable-field creation to XYShape.createIndexableFields for
//     XYLine inputs,
//   - exposes LineValidator, an Encoder-based truth source that resolves
//     CONTAINS via testWithinQuery (expecting Component2D.WithinRelation.CANDIDATE)
//     and the remaining relations via testComponentQuery against the
//     line's tessellated indexable fields.
//
// Gocene currently lacks the random-test harness this subclass depends on:
//   - BaseXYShapeTestCase (abstract parent driving the verifyRandom* sweep),
//   - ShapeTestUtil / RandomNumbers generators (XYLine sampling, nextFloat),
//   - XYLine geometry type with getX/getY/numPoints accessors,
//   - Component2D.WithinRelation plumbing on the XY side,
//   - RandomIndexWriter + CheckHits + QueryUtils support.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and supporting cartesian-shape utilities land in Go.
func TestXYLineShapeQueries(t *testing.T) {
	t.Fatal("blocked by BaseXYShapeTestCase parent harness, ShapeTestUtil/" +
		"RandomNumbers generators, XYLine geometry type, " +
		"Component2D.WithinRelation, and RandomIndexWriter/CheckHits/QueryUtils " +
		"plumbing; remove when fixed")
}
