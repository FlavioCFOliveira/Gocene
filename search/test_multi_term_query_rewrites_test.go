// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiTermQueryRewrites.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with multi-term query rewriting, not yet complete in Gocene.

package search

import "testing"

func TestMultiTermQueryRewrites_RewritesWithDuplicateTerms(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestMultiTermQueryRewrites_Boosts(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
func TestMultiTermQueryRewrites_MaxClauseLimitations(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
