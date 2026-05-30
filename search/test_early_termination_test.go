// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestEarlyTermination.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// with early-termination collector, not yet complete in Gocene.

package search

import "testing"

// TestEarlyTermination_TestEarlyTermination mirrors testEarlyTermination.
func TestEarlyTermination_TestEarlyTermination(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
