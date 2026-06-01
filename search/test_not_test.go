// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestNot.java
//
// Indexes a single document containing the tokenized terms "a" and "b" and
// verifies that the BooleanQuery (a SHOULD, b MUST_NOT) excludes the document
// because of the prohibited "b" clause, yielding zero hits — identical to the
// Lucene assertion assertEquals(0, hits.length).
//
// Deviation from the reference, immaterial to the assertion: MockAnalyzer is
// replaced by the WhitespaceAnalyzer (the deterministic stand-in used by the
// shared integration harness); both tokenize "a b" into "a"@0 and "b"@1.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestNot_TestNot ports testNot.
func TestNot_TestNot(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addText("field", "a b")
	s, cleanup := ix.searcher()
	defer cleanup()

	q := search.NewBooleanQuery()
	q.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	q.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.MUST_NOT)

	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(top.ScoreDocs) != 0 {
		t.Errorf("hits = %d, want 0", len(top.ScoreDocs))
	}
}
