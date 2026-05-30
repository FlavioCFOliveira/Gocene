// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRegexpRandom.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// integration with AutomatonQuery/RegexpQuery, not yet complete in Gocene.

package search

import "testing"

// TestRegexpRandom_Regexps mirrors testRegexps.
// It verifies that random regular expressions produce results consistent with
// brute-force term enumeration.
func TestRegexpRandom_Regexps(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration (pre-existing failure in Gocene)")
}
