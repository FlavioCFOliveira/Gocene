// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90TermVectorsFormat.java
// (Apache Lucene 10.5.0). The class extends
// org.apache.lucene.tests.index.BaseTermVectorsFormatTestCase (not ported);
// testSkipRedundantPrefetches indexes with
// org.apache.lucene.tests.codecs.compressing.dummy.DummyCompressingCodec (not
// ported). Both fail naming the missing test-framework classes.

import "testing"

func TestLucene90TermVectorsFormat_BaseTermVectorsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BaseTermVectorsFormatTestCase (not ported)")
}

func TestLucene90TermVectorsFormat_testSkipRedundantPrefetches(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.codecs.compressing.dummy.DummyCompressingCodec (not ported)")
}
