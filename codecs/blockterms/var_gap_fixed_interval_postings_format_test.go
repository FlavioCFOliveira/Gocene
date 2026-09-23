// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

// Port of
// lucene/codecs/src/test/org/apache/lucene/codecs/blockterms/TestVarGapFixedIntervalPostingsFormat.java
// (Apache Lucene 10.5.0). Basic tests of a PF using VariableGap terms dictionary (fixed interval, docFreq threshold).
//
// The class only builds its codec with
// TestUtil.alwaysPostingsFormat(new LuceneVarGapFixedInterval(...)) and inherits every test method
// from org.apache.lucene.tests.index.BasePostingsFormatTestCase; neither the
// test-framework postings format org.apache.lucene.tests.codecs.blockterms.LuceneVarGapFixedInterval
// nor the base test case is ported.

import "testing"

func TestVarGapFixedIntervalPostingsFormat_BasePostingsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BasePostingsFormatTestCase and " +
		"org.apache.lucene.tests.codecs.blockterms.LuceneVarGapFixedInterval (not ported)")
}
