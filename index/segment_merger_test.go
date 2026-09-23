// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSegmentMerger.java
// (Apache Lucene 10.5.0).
//
// Its setUp() writes both merge inputs with DocHelper.setupDoc/writeDoc;
// org.apache.lucene.tests.index.DocHelper is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index_test

import "testing"

const testSegmentMergerMissing = "org.apache.lucene.tests.index.DocHelper is not ported: TestSegmentMerger sets up every test with DocHelper.setupDoc/writeDoc"

// TestSegmentMerger ports test().
func TestSegmentMerger(t *testing.T) {
	t.Fatal(testSegmentMergerMissing)
}

// TestSegmentMergerMerge ports testMerge().
func TestSegmentMergerMerge(t *testing.T) {
	t.Fatal(testSegmentMergerMissing)
}

// TestSegmentMergerBuildDocMap ports testBuildDocMap().
func TestSegmentMergerBuildDocMap(t *testing.T) {
	t.Fatal(testSegmentMergerMissing)
}
