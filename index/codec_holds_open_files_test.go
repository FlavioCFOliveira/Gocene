// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestCodecHoldsOpenFiles.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

// TestCodecHoldsOpenFiles ports test(): it disables the check-index on close
// of the BaseDirectoryWrapper, deletes every index file behind an open reader
// and verifies each leaf with TestUtil.checkReader.
func TestCodecHoldsOpenFiles(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.store.BaseDirectoryWrapper#setCheckIndexOnClose(boolean) and " +
		"org.apache.lucene.tests.util.TestUtil#checkReader(IndexReader) are not ported")
}
