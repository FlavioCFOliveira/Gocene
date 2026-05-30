// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "testing"

// TestLatLonLineShapeDVQueries mirrors Apache Lucene 10.4.0
// org.apache.lucene.document.TestLatLonLineShapeDVQueries (GOC-4011).
//
// The Java class is a thin subclass of BaseLatLonShapeDocValueTestCase that:
//   - selects ShapeType.LINE,
//   - delegates indexable-field creation to LatLonShape.createDocValueField, and
//   - reuses TestLatLonLineShapeQueries.LineValidator.
//
// All four verifyRandom* hooks are empty in upstream Lucene 10.4.0
// (commented "NOT IMPLEMENTED YET"), so the subclass exists purely to wire
// the abstract harness onto geographic line doc values.
//
// Gocene currently lacks the entire shape-DV random-test harness:
//   - BaseLatLonShapeTestCase / BaseLatLonShapeDocValueTestCase (abstract parents),
//   - TestLatLonLineShapeQueries.LineValidator (Encoder-based truth source),
//   - RandomIndexWriter + GeoTestUtil + CheckHits + QueryUtils plumbing.
//
// Per sprint 55 policy (full roundtrip where it compiles; degraded skip when
// blocked by absent infrastructure), this port records the gap as a skipped
// stub. It must be replaced with a real roundtrip once the parent harness
// and TestLatLonLineShapeQueries.LineValidator land in Go.
func TestLatLonLineShapeDVQueries(t *testing.T) {
	t.Fatal("blocked by BaseLatLonShapeDocValueTestCase parent harness, " +
		"TestLatLonLineShapeQueries.LineValidator, and RandomIndexWriter/" +
		"GeoTestUtil/CheckHits/QueryUtils plumbing; remove when fixed")
}
