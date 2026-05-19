// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPointShapeQueries (GOC-4002).
//
// The Java class is a subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.POINT,
//   - overrides randomQueryLine to occasionally weave indexed vertices into
//     the generated query line (forcing intersections),
//   - delegates indexable-field creation to XYShape.createIndexableFields,
//   - exposes PointValidator, an Encoder-based truth source consumed by
//     sibling stubs (TestXYPointShapeDVQueries) and by BaseXYShapeTestCase
//     when checking CONTAINS/INTERSECTS/WITHIN/DISJOINT relations against
//     random Component2D queries.
//
// Gocene currently lacks the random-test harness this subclass depends on:
//   - BaseXYShapeTestCase (abstract parent driving the verifyRandom* sweep),
//   - ShapeTestUtil / RandomNumbers / GeoTestUtil generators,
//   - Component2D.WithinRelation plumbing on the XY side,
//   - RandomIndexWriter + CheckHits + QueryUtils support.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and supporting cartesian-shape utilities land in Go.
func TestXYPointShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseXYShapeTestCase parent harness, ShapeTestUtil/" +
		"RandomNumbers/GeoTestUtil generators, Component2D.WithinRelation, " +
		"and RandomIndexWriter/CheckHits/QueryUtils plumbing; remove when fixed")
}
