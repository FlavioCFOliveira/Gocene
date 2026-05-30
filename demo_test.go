// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestDemo.java
//
// Deviation: testDemo requires IndexWriter, IndexWriterConfig, DirectoryReader,
// IndexSearcher, FSDirectory, Document, Field, TermQuery, and PhraseQuery —
// none of which are ported to Gocene yet. The test is registered as a stub
// that skips until full IndexWriter+IndexSearcher integration is available.

package gocene

import "testing"

// TestDemo mirrors testDemo (Lucene 10.4.0).
// It indexes a document and verifies retrieval via TermQuery and PhraseQuery.
func TestDemo(t *testing.T) {
	t.Fatal("requires IndexWriter, IndexSearcher, DirectoryReader, FSDirectory, Document, Field, TermQuery, PhraseQuery (not yet ported to Gocene)")
}
