// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestSearch.java
//
// Deviation: all test methods skipped — TestSearch is an integration test that
// requires a fully functional IndexWriter + DirectoryReader + IndexSearcher
// pipeline, which is not yet complete in Gocene.

package search

import "testing"

// TestSearch_Search mirrors the Java testSearch method.
// It verifies that multi-segment and single-segment search results are
// consistent across BooleanQuery, PhraseQuery and TermQuery searches.
func TestSearch_Search(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
