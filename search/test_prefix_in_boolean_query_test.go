// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPrefixInBooleanQuery.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with prefix query inside boolean query, not yet complete in Gocene.

package search

import "testing"

func TestPrefixInBooleanQuery_PrefixQuery(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestPrefixInBooleanQuery_TermQuery(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestPrefixInBooleanQuery_TermBooleanQuery(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestPrefixInBooleanQuery_PrefixBooleanQuery(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
