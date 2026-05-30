// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestShardSearching.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexSearcher
// across multiple shards with global ordinal merging, not yet complete in Gocene.

package search

import "testing"

// TestShardSearching_Basics mirrors the Java test() method.
// It verifies that distributed shard searching produces results consistent
// with a single-node search.
func TestShardSearching_Basics(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+shard integration (pre-existing failure in Gocene)")
}
