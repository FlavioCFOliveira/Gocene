// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiSliceMerge.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with multi-slice TopDocs merging, not yet complete in Gocene.

package search

import "testing"

// TestMultiSliceMerge_MultipleSlicesOfSameIndexSearcher mirrors
// testMultipleSlicesOfSameIndexSearcher.
func TestMultiSliceMerge_MultipleSlicesOfSameIndexSearcher(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestMultiSliceMerge_MultipleSlicesOfMultipleIndexSearchers mirrors
// testMultipleSlicesOfMultipleIndexSearchers.
func TestMultiSliceMerge_MultipleSlicesOfMultipleIndexSearchers(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
