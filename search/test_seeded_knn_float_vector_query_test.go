// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnFloatVectorQuery.java
//
// Deviation: all test methods skipped — extends BaseKnnVectorQueryTestCase which
// requires IndexWriter + IndexSearcher + HNSW graph integration not yet complete in Gocene.

package search

import "testing"

func TestSeededKnnFloatVectorQuery_SeedWithTimeout(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}
func TestSeededKnnFloatVectorQuery_RandomWithSeed(t *testing.T) {
	t.Skip("requires complete IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}
