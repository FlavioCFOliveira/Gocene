// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterExceptions2.java
// (Apache Lucene 10.5.0): causes a bunch of non-aborting and aborting
// exceptions and checks that no index corruption is ever created.

package index_test

import "testing"

// TestIndexWriterExceptions2Basics ports testBasics, which injects failures
// with the test-framework CrankyTokenFilter, MockVariableLengthPayloadFilter,
// CrankyCodec and AssertingCodec and verifies the index with
// TestUtil.checkReader / TestUtil.checkIndex.
func TestIndexWriterExceptions2Basics(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.analysis.CrankyTokenFilter, MockVariableLengthPayloadFilter, " +
		"org.apache.lucene.tests.codecs.cranky.CrankyCodec, org.apache.lucene.tests.codecs.asserting.AssertingCodec " +
		"and TestUtil#checkReader / TestUtil#checkIndex are not ported")
}
