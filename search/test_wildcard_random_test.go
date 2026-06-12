// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestWildcardRandom.java
//
// An index carrying the 1000 zero-padded decimal strings "000".."999" (one per
// document, each a single un-analysed term) is searched with every wildcard
// pattern the reference exercises; the end-to-end IndexWriter -> IndexSearcher
// path (rmp #18 / #123 / #124) must return the exact hit counts Lucene asserts.
//
// Deviations from the reference, immaterial to the assertions:
//   - The reference fills the 'N' placeholder with a random digit each run and
//     repeats atLeast(1) times. Because the corpus is deterministic and dense
//     (every "000".."999" exists), the hit count for a pattern depends only on
//     its shape, not on which concrete digits are substituted. We therefore
//     enumerate representative concrete patterns of each shape directly, which
//     is deterministic and exercises the identical query/automaton machinery.
//   - MockAnalyzer is replaced by a single-token StringField (KEYWORD-style),
//     so each "NNN" value is exactly one term, matching Lucene's tokenisation
//     of the DecimalFormat output.

package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const wildcardRandomField = "field"

// buildWildcardRandomIndex mirrors TestWildcardRandom.setUp: 1000 documents,
// each carrying the field value df.format(i) == zero-padded "000".."999".
func buildWildcardRandomIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < 1000; i++ {
		ix.addString(wildcardRandomField, fmt.Sprintf("%03d", i))
	}
	return ix.searcher()
}

// assertWildcardPatternHits mirrors TestWildcardRandom.assertPatternHits: it
// runs a WildcardQuery for the concrete pattern and asserts the total hit
// count. The reference's fillPattern step (replacing 'N' with a random digit)
// is unnecessary here because every digit value exists exactly once, so the
// caller passes already-filled concrete patterns.
func assertWildcardPatternHits(t *testing.T, s *search.IndexSearcher, pattern string, numHits int64) {
	t.Helper()
	wq := search.NewWildcardQuery(index.NewTerm(wildcardRandomField, pattern))
	top, err := s.Search(wq, 25)
	if err != nil {
		t.Fatalf("search pattern %q: %v", pattern, err)
	}
	if top.TotalHits.Value != numHits {
		t.Errorf("incorrect hits for pattern %q: got %d, want %d", pattern, top.TotalHits.Value, numHits)
	}
}

// TestWildcardRandom_Wildcards ports testWildcards. The reference fills the
// 'N' placeholders with random digits; since the corpus contains every
// "000".."999" exactly once, the hit count is fixed by the pattern shape, so
// we assert one representative concrete filling per shape.
func TestWildcardRandom_Wildcards(t *testing.T) {
	s, done := buildWildcardRandomIndex(t)
	defer done()

	// shape -> (concrete filling using fixed digits, expected hits).
	// Concrete fillings substitute distinct digits for the 'N' positions; any
	// substitution yields the same count over the dense 000..999 corpus.
	cases := []struct {
		pattern string
		hits    int64
	}{
		// First reference loop.
		{"123", 1},  // NNN
		{"?23", 10}, // ?NN
		{"1?3", 10}, // N?N
		{"12?", 10}, // NN?

		// Second reference loop.
		{"??3", 100},  // ??N
		{"1??", 100},  // N??
		{"???", 1000}, // ???

		{"12*", 10}, // NN*
		{"1*", 100}, // N*
		{"*", 1000}, // *

		{"*23", 10}, // *NN
		{"*3", 100}, // *N

		{"1*3", 10}, // N*N

		// combo of ? and * operators
		{"?2*", 100}, // ?N*
		{"1?*", 100}, // N?*

		{"*2?", 100},  // *N?
		{"*??", 1000}, // *??
		{"*?3", 100},  // *?N
	}

	for _, c := range cases {
		assertWildcardPatternHits(t, s, c.pattern, c.hits)
	}
}