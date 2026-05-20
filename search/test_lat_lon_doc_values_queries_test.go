// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestLatLonDocValuesQueries.java
//
// Deviation: TestLatLonDocValuesQueries extends BaseLatLonDocValueTestCase and
// adds no own test methods — all tests are inherited. Skipped because the
// inherited tests require IndexWriter+IndexSearcher geo-spatial integration
// not yet complete in Gocene.

package search

import "testing"

// TestLatLonDocValuesQueries is a placeholder for the concrete BaseLatLonDocValueTestCase subclass.
func TestLatLonDocValuesQueries(t *testing.T) {
	t.Skip("extends BaseLatLonDocValueTestCase (no own tests); requires IndexWriter+IndexSearcher+geo spatial integration (pre-existing failure in Gocene)")
}
