// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiThreadTermVectors.java
//
// Deviation: all test methods skipped — requires IndexWriter + IndexReader with
// term vector storage, accessed concurrently from multiple goroutines; integration
// not yet complete in Gocene.

package search

import "testing"

// TestMultiThreadTermVectors_Concurrency mirrors the Java test() method.
// It verifies that concurrent reads of stored term vectors return correct,
// consistent results across multiple goroutines.
func TestMultiThreadTermVectors_Concurrency(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexReader term-vector integration (pre-existing failure in Gocene)")
}
