// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOutOfFileDescriptors.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

// TestIndexWriterOutOfFileDescriptors ports test(), which indexes documents
// drawn from org.apache.lucene.tests.util.LineFileDocs into a
// newMockFSDirectory with random IOExceptions on open.
func TestIndexWriterOutOfFileDescriptors(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.LineFileDocs is not ported")
}
