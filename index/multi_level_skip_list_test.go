// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMultiLevelSkipList.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

// TestMultiLevelSkipListSimpleSkip ports testSimpleSkip, whose writer is
// configured with
// setCodec(TestUtil.alwaysPostingsFormat(TestUtil.getDefaultPostingsFormat())).
func TestMultiLevelSkipListSimpleSkip(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#alwaysPostingsFormat(PostingsFormat) and " +
		"TestUtil#getDefaultPostingsFormat() are not ported")
}
