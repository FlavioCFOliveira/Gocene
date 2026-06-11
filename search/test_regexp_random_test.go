// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRegexpRandom.java
//
// An index with the 1000 terms "000".."999" is built; random regular expressions
// are generated from fixed templates (where 'N' is replaced by a random digit)
// and run as RegexpQuery, asserting the exact number of matching documents that
// Lucene asserts for each template — e.g. "NNN" matches exactly 1 term, ".NN"
// matches 10, "[1-5][2-6][3-7]" matches 125, ".*" matches all 1000.
//
// Deviation: the MockAnalyzer is replaced by the WhitespaceAnalyzer (the
// deterministic stand-in used by the shared harness); each document carries a
// single three-digit token, so tokenization is identical for these terms.

package search_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// newRegexpRandomSearcher indexes the 1000 zero-padded terms 000..999.
func newRegexpRandomSearcher(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < 1000; i++ {
		ix.addText("field", fmt.Sprintf("%03d", i))
	}
	return ix.searcher()
}

// regexpFillPattern replaces each 'N' in the template with a random digit,
// mirroring TestRegexpRandom.fillPattern / N().
func regexpFillPattern(rng *rand.Rand, template string) string {
	var sb strings.Builder
	for _, c := range template {
		if c == 'N' {
			sb.WriteByte(byte('0' + rng.Intn(10)))
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// TestRegexpRandom_Regexps ports testRegexps.
func TestRegexpRandom_Regexps(t *testing.T) {
	s, cleanup := newRegexpRandomSearcher(t)
	defer cleanup()

	rng := rand.New(rand.NewSource(7919)) //nolint:gosec // deterministic test seed

	assertPatternHits := func(template string, numHits int64) {
		t.Helper()
		pattern := regexpFillPattern(rng, template)
		wq, err := search.NewRegexpQuery("field", pattern)
		if err != nil {
			t.Fatalf("NewRegexpQuery(%q): %v", pattern, err)
		}
		top, err := s.Search(wq, 25)
		if err != nil {
			t.Fatalf("Search(%q): %v", pattern, err)
		}
		if top.TotalHits.Value != numHits {
			t.Errorf("pattern %q (template %q): hits = %d, want %d", pattern, template, top.TotalHits.Value, numHits)
		}

	num := 1
	for i := 0; i < num; i++ {
		assertPatternHits("NNN", 1)
		assertPatternHits(".NN", 10)
		assertPatternHits("N.N", 10)
		assertPatternHits("NN.", 10)
	}

	for i := 0; i < num; i++ {
		assertPatternHits(".{1,2}N", 100)
		assertPatternHits("N.{1,2}", 100)
		assertPatternHits(".{1,3}", 1000)

		assertPatternHits("NN[3-7]", 5)
		assertPatternHits("N[2-6][3-7]", 25)
		assertPatternHits("[1-5][2-6][3-7]", 125)
		assertPatternHits("[0-4][3-7][4-8]", 125)
		assertPatternHits("[2-6][0-4]N", 25)
		assertPatternHits("[2-6]NN", 5)

		assertPatternHits("NN.*", 10)
		assertPatternHits("N.*", 100)
		assertPatternHits(".*", 1000)

		assertPatternHits(".*NN", 10)
		assertPatternHits(".*N", 100)

		assertPatternHits("N.*N", 10)

		// combo of ? and * operators
		assertPatternHits(".N.*", 100)
		assertPatternHits("N..*", 100)

		assertPatternHits(".*N.", 100)
		assertPatternHits(".*..", 1000)
		assertPatternHits(".*.N", 100)
	}
}