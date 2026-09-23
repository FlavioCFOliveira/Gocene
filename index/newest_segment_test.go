// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestNewestSegment.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

func TestNewestSegmentNewestSegment(t *testing.T) {
	directory := newDirectory()
	writer := mustNewIndexWriter(t, directory, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustClose(t, writer, directory)
	// assertNull(writer.newestSegment())
	t.Fatal("org.apache.lucene.index.IndexWriter#newestSegment() is not ported")
}
