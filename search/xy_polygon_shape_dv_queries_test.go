// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestXYPolygonShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestXYPolygonShapeDVQueries (GOC-3995).
//
// The Java class is a thin subclass of BaseXYShapeDocValueTestCase that:
//   - selects ShapeType.POLYGON,
//   - delegates indexable-field creation to XYShape.createDocValueField, and
//   - reuses TestXYPolygonShapeQueries.PolygonValidator.
//
// All four verifyRandom* hooks are empty in upstream Lucene 10.4.0
// (commented "NOT IMPLEMENTED YET"), so the subclass exists purely to wire
// the abstract harness onto Cartesian polygon doc values.
//
// Gocene currently lacks the entire shape-DV random-test harness:
//   - BaseXYShapeTestCase / BaseXYShapeDocValueTestCase (abstract parents),
//   - TestXYPolygonShapeQueries.PolygonValidator (Encoder-based truth source),
//   - RandomIndexWriter + ShapeTestUtil + CheckHits + QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and TestXYPolygonShapeQueries.PolygonValidator land in Go.
func TestXYPolygonShapeDVQueries(t *testing.T) {
	t.Skip("blocked by BaseXYShapeDocValueTestCase parent harness, " +
		"TestXYPolygonShapeQueries.PolygonValidator, and RandomIndexWriter/" +
		"ShapeTestUtil/CheckHits/QueryUtils plumbing; remove when fixed")
}
