// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPatienceByteVectorQuery.java
//
// Deviation: all test methods skipped — extends BaseKnnVectorQueryTestCase which
// requires IndexWriter + IndexSearcher + HNSW/patience graph integration not yet complete in Gocene.

package search

import "testing"

// TestPatienceByteVectorQuery_ToString mirrors testToString.
func TestPatienceByteVectorQuery_ToString(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}
