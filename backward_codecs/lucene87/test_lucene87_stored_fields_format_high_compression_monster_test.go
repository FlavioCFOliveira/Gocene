// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene87/TestLucene87StoredFieldsFormatHighCompression.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled ("N-2 formats are only tested on
// nightly runs").
//
// Blocker: the class extends BaseStoredFieldsFormatTestCase and every own method (testMixedCompressions, testInvalidOptions) builds a Lucene87RWCodec; neither is ported.

package lucene87

import "testing"

const TestLucene87StoredFieldsFormatHighCompressionBlocker = "requires org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase and the test-side org.apache.lucene.backward_codecs.lucene87.Lucene87RWCodec (not ported)"

func TestLucene87StoredFieldsFormatHighCompression_BaseStoredFieldsFormatTestCase(t *testing.T) {
	t.Fatal(TestLucene87StoredFieldsFormatHighCompressionBlocker)
}

func TestLucene87StoredFieldsFormatHighCompression_testMixedCompressions(t *testing.T) {
	t.Fatal(TestLucene87StoredFieldsFormatHighCompressionBlocker)
}

func TestLucene87StoredFieldsFormatHighCompression_testInvalidOptions(t *testing.T) {
	t.Fatal(TestLucene87StoredFieldsFormatHighCompressionBlocker)
}
