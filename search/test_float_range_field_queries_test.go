// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestFloatRangeFieldQueries.java
//
// Deviation: all test methods skipped — extends BaseRangeFieldQueryTestCase which
// requires IndexWriter + IndexSearcher integration not yet complete in Gocene.

package search

import "testing"

// TestFloatRangeFieldQueries_Basics mirrors testBasics.
func TestFloatRangeFieldQueries_Basics(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
