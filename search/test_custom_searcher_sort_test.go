// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestCustomSearcherSort.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with custom field sort (SortedDocValuesField), not yet complete in Gocene.

package search

import "testing"

func TestCustomSearcherSort_FieldSortCustomSearcher(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+custom sort integration (pre-existing failure in Gocene)")
}
func TestCustomSearcherSort_FieldSortSingleSearcher(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+custom sort integration (pre-existing failure in Gocene)")
}
