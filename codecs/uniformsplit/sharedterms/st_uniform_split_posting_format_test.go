// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms_test

// Port of
// lucene/codecs/src/test/org/apache/lucene/codecs/uniformsplit/sharedterms/TestSTUniformSplitPostingFormat.java
// (Apache Lucene 10.5.0): tests STUniformSplitPostingsFormat with block
// encoding using ROT13 cypher.
//
// The class extends TestUniformSplitPostingFormat (and through it
// org.apache.lucene.tests.index.BasePostingsFormatTestCase) and overrides only
// getPostingsFormat(), which uses the test-framework
// org.apache.lucene.tests.codecs.uniformsplit.sharedterms.STUniformSplitRot13PostingsFormat.
// Its inherited test methods fail naming the missing test-framework classes.

import "testing"

const stUniformSplitPostingFormatBlocker = "requires org.apache.lucene.tests.index.BasePostingsFormatTestCase and " +
	"org.apache.lucene.tests.codecs.uniformsplit.sharedterms.STUniformSplitRot13PostingsFormat (not ported)"

func TestSTUniformSplitPostingFormat_BasePostingsFormatTestCase(t *testing.T) {
	t.Fatal(stUniformSplitPostingFormatBlocker)
}

func TestSTUniformSplitPostingFormat_testRandomExceptions(t *testing.T) {
	t.Fatal(stUniformSplitPostingFormatBlocker)
}
