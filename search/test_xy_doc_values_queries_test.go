// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestXYDocValuesQueries.java
//
// Deviation: all test methods skipped — extends BaseXYPointTestCase which
// requires IndexWriter + IndexSearcher + XY spatial integration not yet complete in Gocene.

package search

import "testing"

// TestXYDocValuesQueries_Basics mirrors the inherited spatial query tests.
// It verifies distance, bounding-box and polygon queries on XYDocValues fields.
func TestXYDocValuesQueries_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+XY spatial integration (pre-existing failure in Gocene)")
}
