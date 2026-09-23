// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFieldsReader.java
// (Apache Lucene 10.5.0).
//
// Its @BeforeClass builds the shared test document with DocHelper.setupDoc;
// org.apache.lucene.tests.index.DocHelper is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index_test

import "testing"

const testFieldsReaderMissing = "org.apache.lucene.tests.index.DocHelper is not ported: TestFieldsReader builds its class-level document with DocHelper.setupDoc"

// TestFieldsReader ports test().
func TestFieldsReader(t *testing.T) {
	t.Fatal(testFieldsReaderMissing)
}

// TestFieldsReaderExceptions ports testExceptions().
func TestFieldsReaderExceptions(t *testing.T) {
	t.Fatal(testFieldsReaderMissing)
}
