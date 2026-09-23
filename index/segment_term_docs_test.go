// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSegmentTermDocs.java
// (Apache Lucene 10.5.0).
//
// Its setUp() builds and writes the test document with DocHelper.setupDoc/writeDoc;
// org.apache.lucene.tests.index.DocHelper is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index_test

import "testing"

const testSegmentTermDocsMissing = "org.apache.lucene.tests.index.DocHelper is not ported: TestSegmentTermDocs sets up every test with DocHelper.setupDoc/writeDoc"

// TestSegmentTermDocs ports test().
func TestSegmentTermDocs(t *testing.T) {
	t.Fatal(testSegmentTermDocsMissing)
}

// TestSegmentTermDocsTermDocs ports testTermDocs().
func TestSegmentTermDocsTermDocs(t *testing.T) {
	t.Fatal(testSegmentTermDocsMissing)
}

// TestSegmentTermDocsBadSeek ports testBadSeek().
func TestSegmentTermDocsBadSeek(t *testing.T) {
	t.Fatal(testSegmentTermDocsMissing)
}

// TestSegmentTermDocsSkipTo ports testSkipTo().
func TestSegmentTermDocsSkipTo(t *testing.T) {
	t.Fatal(testSegmentTermDocsMissing)
}
