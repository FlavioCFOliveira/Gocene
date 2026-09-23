// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/perfield/TestPerFieldKnnVectorsFormat.java
// (Apache Lucene 10.5.0).
//
// Blockers: the class extends org.apache.lucene.tests.index.BaseKnnVectorsFormatTestCase,
// and setUp() (run before every test method) builds its codec with
// org.apache.lucene.tests.index.RandomCodec; every own test method also
// configures an anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec
// subclass overriding getKnnVectorsFormatForField. None of these test-framework
// classes is ported (codecs/asserting.AssertingCodec is an empty struct), so
// every test fails naming them.

import "testing"

const perFieldKnnVectorsFormatBlocker = "requires org.apache.lucene.tests.index.RandomCodec (setUp), " +
	"org.apache.lucene.tests.index.BaseKnnVectorsFormatTestCase and " +
	"org.apache.lucene.tests.codecs.asserting.AssertingCodec (not ported)"

func TestPerFieldKnnVectorsFormat_BaseKnnVectorsFormatTestCase(t *testing.T) {
	t.Fatal(perFieldKnnVectorsFormatBlocker)
}

func TestPerFieldKnnVectorsFormat_testMissingFieldReturnsNoResults(t *testing.T) {
	t.Fatal(perFieldKnnVectorsFormatBlocker)
}

func TestPerFieldKnnVectorsFormat_testTwoFieldsTwoFormats(t *testing.T) {
	t.Fatal(perFieldKnnVectorsFormatBlocker)
}

func TestPerFieldKnnVectorsFormat_testMergeUsesNewFormat(t *testing.T) {
	t.Fatal(perFieldKnnVectorsFormatBlocker)
}

func TestPerFieldKnnVectorsFormat_testMaxDimensionsPerFieldFormat(t *testing.T) {
	t.Fatal(perFieldKnnVectorsFormatBlocker)
}
