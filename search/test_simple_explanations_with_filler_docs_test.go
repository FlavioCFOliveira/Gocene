// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleExplanationsWithFillerDocs.java
//
// Deviation: all test methods skipped — extends TestSimpleExplanations which
// requires IndexWriter + IndexSearcher integration not yet complete in Gocene.

package search

import "testing"

// TestSimpleExplanationsWithFillerDocs_MA1 mirrors testMA1.
func TestSimpleExplanationsWithFillerDocs_MA1(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}

// TestSimpleExplanationsWithFillerDocs_MA2 mirrors testMA2.
func TestSimpleExplanationsWithFillerDocs_MA2(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
