// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene87/TestLucene87StoredFieldsFormatMergeInstance.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled ("N-2 formats are only tested on
// nightly runs").
//
// Blocker: the class extends TestLucene87StoredFieldsFormat (BaseStoredFieldsFormatTestCase over a Lucene87RWCodec) and only overrides shouldTestMergeInstance(); neither class is ported.

package lucene87

import "testing"

const TestLucene87StoredFieldsFormatMergeInstanceBlocker = "requires org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase (shouldTestMergeInstance) and the test-side org.apache.lucene.backward_codecs.lucene87.Lucene87RWCodec (not ported)"

func TestLucene87StoredFieldsFormatMergeInstance_BaseStoredFieldsFormatTestCase(t *testing.T) {
	t.Fatal(TestLucene87StoredFieldsFormatMergeInstanceBlocker)
}
