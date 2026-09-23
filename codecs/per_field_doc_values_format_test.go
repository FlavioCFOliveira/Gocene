// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/perfield/TestPerFieldDocValuesFormat.java
// (Apache Lucene 10.5.0).
//
// Blockers: the class extends org.apache.lucene.tests.index.BaseDocValuesFormatTestCase;
// setUp() (run before every test method) builds its codec with
// org.apache.lucene.tests.index.RandomCodec; every own test method configures
// an anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec subclass
// overriding getDocValuesFormatForField, and testTwoFieldsTwoFormats also
// searches through LuceneTestCase.newSearcher
// (org.apache.lucene.tests.search.AssertingIndexSearcher). None of these
// test-framework classes is ported, so every test fails naming them.

import "testing"

const perFieldDocValuesFormatBlocker = "requires org.apache.lucene.tests.index.RandomCodec (setUp), " +
	"org.apache.lucene.tests.index.BaseDocValuesFormatTestCase, " +
	"org.apache.lucene.tests.codecs.asserting.AssertingCodec and " +
	"org.apache.lucene.tests.search.AssertingIndexSearcher (not ported)"

func TestPerFieldDocValuesFormat_BaseDocValuesFormatTestCase(t *testing.T) {
	t.Fatal(perFieldDocValuesFormatBlocker)
}

// just a simple trivial test
func TestPerFieldDocValuesFormat_testTwoFieldsTwoFormats(t *testing.T) {
	t.Fatal(perFieldDocValuesFormatBlocker)
}

func TestPerFieldDocValuesFormat_testMergeCalledOnTwoFormats(t *testing.T) {
	t.Fatal(perFieldDocValuesFormatBlocker)
}

func TestPerFieldDocValuesFormat_testDocValuesMergeWithIndexedFields(t *testing.T) {
	t.Fatal(perFieldDocValuesFormatBlocker)
}
