// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestNRTThreads.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

// TestNRTThreads ports testNRTThreads(), which calls runTest of its base class
// org.apache.lucene.tests.index.ThreadedIndexingAndSearchingTestCase.
func TestNRTThreads(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.index.ThreadedIndexingAndSearchingTestCase is not ported")
}
