// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestNRTReaderWithThreads.java
// (Apache Lucene 10.5.0).
//
// Its RunThread indexes DocHelper.createDocument(i, "index1", 10);
// org.apache.lucene.tests.index.DocHelper is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index_test

import "testing"

const testNRTReaderWithThreadsMissing = "org.apache.lucene.tests.index.DocHelper is not ported: TestNRTReaderWithThreads indexes DocHelper.createDocument documents"

// TestNRTReaderWithThreadsIndexing ports testIndexing().
func TestNRTReaderWithThreadsIndexing(t *testing.T) {
	t.Fatal(testNRTReaderWithThreadsMissing)
}
