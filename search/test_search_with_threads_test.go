// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSearchWithThreads.java
//
// Deviation: all test methods skipped — requires IndexWriter + DirectoryReader
// + IndexSearcher integration with concurrent search, not yet complete in Gocene.

package search

import "testing"

// TestSearchWithThreads_Search mirrors the Java test() method.
// It verifies that concurrent searches across multiple threads produce
// consistent results matching single-threaded searches.
func TestSearchWithThreads_Search(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
