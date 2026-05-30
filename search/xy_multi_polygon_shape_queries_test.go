// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYMultiPolygonShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiPolygonShapeQueries (GOC-4017).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.POLYGON,
//   - emits 1..4 random XYPolygons per "shape" via nextShape(),
//   - delegates indexable-field creation to XYShape.createIndexableFields
//     (called once per polygon and flattened into a single Field[]),
//   - wires a MultiPolygonValidator that wraps
//     TestXYPolygonShapeQueries.PolygonValidator and folds per-polygon
//     results according to the active QueryRelation (INTERSECTS /
//     CONTAINS / DISJOINT / WITHIN), resolving CONTAINS via
//     Component2D.WithinRelation (DISJOINT / CANDIDATE / NOTWITHIN), and
//   - overrides the @Nightly testRandomBig hook with doTestRandom(10000).
//
// Gocene currently lacks the entire random shape-test harness on which
// this class depends:
//   - BaseXYShapeTestCase (abstract parent + doTestRandom orchestration),
//   - TestXYPolygonShapeQueries.PolygonValidator (Encoder-based truth
//     source) with its testWithinQuery CONTAINS path,
//   - nextShape() random XYPolygon generator (ShapeTestUtil /
//     RandomNumbers / GeoTestUtil),
//   - XYShape.createIndexableFields cartesian-shape field factory,
//   - Component2D.WithinRelation plumbing on the XY side,
//   - the @Nightly gate equivalent for the 10k-doc big run,
//   - RandomIndexWriter / CheckHits / QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip
// when blocked by absent infrastructure), this port records the gap as a
// skipped stub. It must be replaced with a real roundtrip once the parent
// harness, PolygonValidator, nextShape generator, XYShape field factory,
// and Component2D.WithinRelation land in Go. Sibling of GOC-4009
// (TestXYPolygonShapeQueries) and GOC-4004 (TestXYMultiLineShapeQueries).
func TestXYMultiPolygonShapeQueries(t *testing.T) {
	t.Fatal("blocked by BaseXYShapeTestCase parent harness, " +
		"TestXYPolygonShapeQueries.PolygonValidator, nextShape() XYPolygon " +
		"generator, XYShape.createIndexableFields, Component2D.WithinRelation, " +
		"and RandomIndexWriter/CheckHits/QueryUtils plumbing; remove when fixed")
}
