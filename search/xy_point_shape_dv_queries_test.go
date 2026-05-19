// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYPointShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPointShapeDVQueries (GOC-3986).
//
// The Java class is a thin subclass of BaseXYShapeDocValueTestCase that:
//   - selects ShapeType.POINT,
//   - delegates indexable-field creation to XYShape.createDocValueField, and
//   - reuses TestXYPointShapeQueries.PointValidator.
//
// All four verifyRandom* hooks are empty in upstream Lucene 10.4.0
// (commented "NOT IMPLEMENTED YET"), so the subclass exists purely to wire
// the abstract harness onto cartesian point doc values.
//
// Gocene currently lacks the entire shape-DV random-test harness:
//   - BaseXYShapeTestCase / BaseXYShapeDocValueTestCase (abstract parents),
//   - TestXYPointShapeQueries.PointValidator (Encoder-based truth source),
//   - RandomIndexWriter + GeoTestUtil + CheckHits + QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and TestXYPointShapeQueries.PointValidator land in Go.
func TestXYPointShapeDVQueries(t *testing.T) {
	t.Skip("blocked by BaseXYShapeDocValueTestCase parent harness, " +
		"TestXYPointShapeQueries.PointValidator, and RandomIndexWriter/" +
		"GeoTestUtil/CheckHits/QueryUtils plumbing; remove when fixed")
}
