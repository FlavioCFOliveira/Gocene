// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSeededKnnByteVectorQuery.java
//
// Deviation: all test methods skipped — extends BaseKnnVectorQueryTestCase which
// requires IndexWriter + IndexSearcher + HNSW graph integration not yet complete in Gocene.

package search

import "testing"

// TestSeededKnnByteVectorQuery_SeedWithTimeout mirrors testSeedWithTimeout.
func TestSeededKnnByteVectorQuery_SeedWithTimeout(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}

// TestSeededKnnByteVectorQuery_RandomWithSeed mirrors testRandomWithSeed.
func TestSeededKnnByteVectorQuery_RandomWithSeed(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher+HNSW integration (pre-existing failure in Gocene)")
}
