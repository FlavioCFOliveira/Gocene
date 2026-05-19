// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYMultiPointShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiPointShapeQueries (GOC-3997).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.POINT,
//   - emits 1..4 random XYPoints per "shape" via ShapeTestUtil.nextXYPoint,
//   - delegates indexable-field creation to XYShape.createIndexableFields
//     (called once per point and flattened into a single Field[]),
//   - wires a MultiPointValidator that wraps
//     TestXYPointShapeQueries.PointValidator and folds per-point results
//     according to the active QueryRelation (INTERSECTS / CONTAINS /
//     DISJOINT / WITHIN), and
//   - overrides the @Nightly testRandomBig hook with doTestRandom(10000).
//
// Gocene currently lacks the entire random shape-test harness on which
// this class depends:
//   - BaseXYShapeTestCase (abstract parent + doTestRandom orchestration),
//   - TestXYPointShapeQueries.PointValidator (Encoder-based truth source),
//   - ShapeTestUtil.nextXYPoint random generator,
//   - XYShape.createIndexableFields cartesian-shape field factory,
//   - the @Nightly gate equivalent for the 10k-doc big run,
//   - RandomIndexWriter / CheckHits / QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip
// when blocked by absent infrastructure), this port records the gap as a
// skipped stub. It must be replaced with a real roundtrip once the parent
// harness, PointValidator, ShapeTestUtil, and XYShape field factory land
// in Go.
func TestXYMultiPointShapeQueries(t *testing.T) {
	t.Skip("blocked by BaseXYShapeTestCase parent harness, " +
		"TestXYPointShapeQueries.PointValidator, ShapeTestUtil.nextXYPoint, " +
		"XYShape.createIndexableFields, and RandomIndexWriter/CheckHits/" +
		"QueryUtils plumbing; remove when fixed")
}
