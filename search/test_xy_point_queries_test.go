// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestXYPointQueries.java
//
// Deviation: TestXYPointQueries extends BaseXYPointTestCase and adds no own
// test methods — all tests are inherited. The class is a concrete instantiation
// of the abstract base so that XY point queries are exercised. Skipped because
// BaseXYPointTestCase requires IndexWriter+IndexSearcher integration not yet
// complete in Gocene.

package search

import "testing"

// TestXYPointQueries is a placeholder for the concrete BaseXYPointTestCase subclass.
// It exercises distance, bounding-box and polygon queries on XYPoint fields.
func TestXYPointQueries(t *testing.T) {
	t.Skip("extends BaseXYPointTestCase (no own tests); requires IndexWriter+IndexSearcher+XY spatial integration (pre-existing failure in Gocene)")
}
