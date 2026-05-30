// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestSearchForDuplicates.java
//
// Deviation: testRun requires IndexWriter, IndexSearcher, DirectoryReader,
// BooleanQuery, Document, Field, StoredField, NumericDocValuesField, Sort, and
// ScoreDoc — none of which are ported to Gocene yet. The test is registered
// as a stub that skips until full IndexWriter+IndexSearcher integration is
// available.

package gocene

import "testing"

// TestSearchForDuplicates_Run mirrors testRun (Lucene 10.4.0).
// It indexes documents with duplicate content and verifies that the search
// returns the correct number of hits via BooleanQuery and Sort.
func TestSearchForDuplicates_Run(t *testing.T) {
	t.Fatal("requires IndexWriter, IndexSearcher, DirectoryReader, BooleanQuery, Document, Field, Sort (not yet ported to Gocene)")
}
