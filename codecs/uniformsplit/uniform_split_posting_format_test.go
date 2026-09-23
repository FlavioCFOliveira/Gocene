// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit_test

// Port of
// lucene/codecs/src/test/org/apache/lucene/codecs/uniformsplit/TestUniformSplitPostingFormat.java
// (Apache Lucene 10.5.0): tests UniformSplitPostingsFormat with block encoding
// using ROT13 cypher.
//
// Blockers: the class extends org.apache.lucene.tests.index.BasePostingsFormatTestCase,
// and its constructor, @Before initialize() and @After checkEncodingCalled()
// use the test-framework org.apache.lucene.tests.codecs.uniformsplit.UniformSplitRot13PostingsFormat;
// testRandomExceptions delegates to super.testRandomExceptions(). None of
// these test-framework classes is ported.

import "testing"

const uniformSplitPostingFormatBlocker = "requires org.apache.lucene.tests.index.BasePostingsFormatTestCase and " +
	"org.apache.lucene.tests.codecs.uniformsplit.UniformSplitRot13PostingsFormat (not ported)"

func TestUniformSplitPostingFormat_BasePostingsFormatTestCase(t *testing.T) {
	t.Fatal(uniformSplitPostingFormatBlocker)
}

func TestUniformSplitPostingFormat_testRandomExceptions(t *testing.T) {
	t.Fatal(uniformSplitPostingFormatBlocker)
}
