// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestLatLonPointQueries.java
//
// Deviation: all test methods skipped — extends BaseLatLonPointTestCase which
// requires IndexWriter + IndexSearcher + geo spatial integration not yet
// complete in Gocene.

package search

import "testing"

// TestLatLonPointQueries_Basics mirrors the inherited geo-spatial query tests.
func TestLatLonPointQueries_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+geo spatial integration (pre-existing failure in Gocene)")
}
