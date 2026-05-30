// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYMultiLineShapeQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYMultiLineShapeQueries (GOC-4004).
//
// The Java class is a thin subclass of BaseXYShapeTestCase that:
//   - selects ShapeType.LINE,
//   - emits 1..4 random XYLines per "shape" via nextLine(),
//   - delegates indexable-field creation to XYShape.createIndexableFields
//     (called once per line and flattened into a single Field[]),
//   - wires a MultiLineValidator that wraps
//     TestXYLineShapeQueries.LineValidator and folds per-line results
//     according to the active QueryRelation (INTERSECTS / CONTAINS /
//     DISJOINT / WITHIN), and
//   - overrides the @Nightly testRandomBig hook with doTestRandom(10000).
//
// Gocene currently lacks the entire random shape-test harness on which
// this class depends:
//   - BaseXYShapeTestCase (abstract parent + doTestRandom orchestration),
//   - TestXYLineShapeQueries.LineValidator (Encoder-based truth source),
//   - nextLine() random XYLine generator,
//   - XYShape.createIndexableFields cartesian-shape field factory,
//   - the @Nightly gate equivalent for the 10k-doc big run,
//   - RandomIndexWriter / CheckHits / QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip
// when blocked by absent infrastructure), this port records the gap as a
// skipped stub. It must be replaced with a real roundtrip once the parent
// harness, LineValidator, nextLine generator, and XYShape field factory
// land in Go. Sibling of GOC-4003 (TestXYLineShapeQueries) and GOC-3997
// (TestXYMultiPointShapeQueries).
func TestXYMultiLineShapeQueries(t *testing.T) {
	t.Fatal("blocked by BaseXYShapeTestCase parent harness, " +
		"TestXYLineShapeQueries.LineValidator, nextLine() XYLine generator, " +
		"XYShape.createIndexableFields, and RandomIndexWriter/CheckHits/" +
		"QueryUtils plumbing; remove when fixed")
}
