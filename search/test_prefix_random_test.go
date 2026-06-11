// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPrefixRandom.java
//
// An index of random (deterministically seeded) Unicode terms — one
// KEYWORD-style term per document — is queried with many random prefixes. For
// each prefix the production PrefixQuery ("smart") is compared, hit-for-hit and
// score-for-score, against a brute-force reference ("dumb") that enumerates the
// field's TermsEnum and accepts every term whose raw bytes start with the
// prefix. Both must return identical results, mirroring
// CheckHits.checkEqual(smart, smartDocs, dumbDocs).
//
// Translation of the reference DumbPrefixQuery: Lucene models it as a
// MultiTermQuery with a FilteredTermsEnum and CONSTANT_SCORE_BLENDED_REWRITE.
// Gocene's PrefixQuery rewrites (via AutomatonQuery) to
// ConstantScoreQuery(BooleanQuery{SHOULD TermQuery...}) over the matched terms
// in term-dictionary byte order. The dumb reference here builds exactly that
// same shape from a brute-force, byte-level prefix scan, so the two queries are
// guaranteed to select the same term set and emit identical constant scores —
// which is precisely the invariant the reference asserts.
//
// Deviation from the reference, immaterial to the assertions: the randomness is
// seeded deterministically (Go has no global test PRNG akin to Lucene's
// random()), keeping the suite reproducible while still exercising a broad
// range of prefixes against a broad range of terms.

package search_test

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const prefixRandomField = "field"

// prefixRandomScoreTolerance matches CheckHits.SCORE_TOLERANCE_DELTA-class
// comparisons used by the reference checkEqual.
const prefixRandomScoreTolerance = 1e-5

// buildPrefixRandomIndex indexes num random Unicode strings (one term per
// document) and returns a searcher plus cleanup, mirroring
// TestPrefixRandom.setUp (atLeast(1000) docs).
func buildPrefixRandomIndex(t *testing.T, rng *rand.Rand, num int) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < num; i++ {
		ix.addString(prefixRandomField, randomUnicodeStringMaxLen(rng, 10))
	}
	return ix.searcher()
}

// TestPrefixRandom_Prefixes ports testPrefixes: assert the smart PrefixQuery
// and the brute-force reference agree for a batch of random prefixes.
func TestPrefixRandom_Prefixes(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1E3779B97F4A7C15))
	s, done := buildPrefixRandomIndex(t, rng, 1000)
	defer done()

	const num = 100
	for i := 0; i < num; i++ {
		assertPrefixSame(t, s, randomUnicodeStringMaxLen(rng, 5))
	}
}

// assertPrefixSame mirrors TestPrefixRandom.assertSame: search the smart and
// dumb prefix queries and assert their top-25 hit lists are equal.
func assertPrefixSame(t *testing.T, s *search.IndexSearcher, prefix string) {
	t.Helper()
	smart := search.NewPrefixQuery(index.NewTerm(prefixRandomField, prefix))
	dumb := buildDumbPrefixQuery(t, s, prefix)

	smartDocs, err := s.Search(smart, 25)
	if err != nil {
		t.Fatalf("search smart prefix %q: %v", prefix, err)
	}
	dumbDocs, err := s.Search(dumb, 25)
	if err != nil {
		t.Fatalf("search dumb prefix %q: %v", prefix, err)
	}
	checkPrefixEqual(t, prefix, smartDocs.ScoreDocs, dumbDocs.ScoreDocs)
}

// buildDumbPrefixQuery is the Go port of TestPrefixRandom.DumbPrefixQuery: it
// scans the field's full TermsEnum, accepts every term whose raw bytes start
// with the prefix, and wraps the resulting SHOULD-of-TermQuery disjunction in a
// ConstantScoreQuery — the exact shape PrefixQuery rewrites into, so scores
// match.
func buildDumbPrefixQuery(t *testing.T, s *search.IndexSearcher, prefix string) search.Query {
	t.Helper()
	tp, ok := s.GetIndexReader().(termsProvider)
	if !ok {
		t.Fatalf("reader does not expose Terms(field)")
	}
	terms, err := tp.Terms(prefixRandomField)
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		return search.NewMatchNoDocsQuery()
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	prefixBytes := []byte(prefix)
	bq := search.NewBooleanQuery()
	any := false
	for {
		cur, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if cur == nil {
			break
		}
		bv := cur.BytesValue()
		if bv == nil {
			continue
		}
		if bytes.HasPrefix(bv.ValidBytes(), prefixBytes) {
			bq.Add(search.NewTermQuery(index.NewTermFromBytes(prefixRandomField, bv.ValidBytes())), search.SHOULD)
			any = true
		}
	}
	if !any {
		return search.NewMatchNoDocsQuery()
	}
	return search.NewConstantScoreQuery(bq)
}

// checkPrefixEqual ports CheckHits.checkEqual: same length, same doc ids, and
// scores within tolerance. The order is normalised by (doc, score) so the two
// constant-score top-K lists compare deterministically even when ties are
// returned in different orders by the two query shapes.
func checkPrefixEqual(t *testing.T, prefix string, hits1, hits2 []*search.ScoreDoc) {
	t.Helper()
	if len(hits1) != len(hits2) {
		t.Fatalf("prefix %q: unequal lengths: smart=%d dumb=%d", prefix, len(hits1), len(hits2))
	}
	a := sortedScoreDocs(hits1)
	b := sortedScoreDocs(hits2)
	for i := range a {
		if a[i].Doc != b[i].Doc {
			t.Fatalf("prefix %q: hit %d doc mismatch: smart=%d dumb=%d", prefix, i, a[i].Doc, b[i].Doc)
		}
		if math.Abs(float64(a[i].Score-b[i].Score)) > prefixRandomScoreTolerance {
			t.Fatalf("prefix %q: hit %d (doc %d) score mismatch: smart=%v dumb=%v",
				prefix, i, a[i].Doc, a[i].Score, b[i].Score)
		}
	}
}

// sortedScoreDocs returns a copy of hits ordered by ascending doc id (scores
// are constant under both query shapes, so doc id is a stable key).
func sortedScoreDocs(hits []*search.ScoreDoc) []*search.ScoreDoc {
	out := make([]*search.ScoreDoc, len(hits))
	copy(out, hits)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Doc < out[j].Doc })
	return out
}

// randomUnicodeStringMaxLen returns a random Unicode string of up to maxLen
// code points, the Go analogue of TestUtil.randomUnicodeString(random, maxLen).
// It draws code points across the BMP and supplementary planes (excluding
// surrogates, which are not valid scalar values), matching the breadth of the
// reference generator.
func randomUnicodeStringMaxLen(rng *rand.Rand, maxLen int) string {
	n := rng.Intn(maxLen + 1)
	runes := make([]rune, 0, n)
	for i := 0; i < n; i++ {
		runes = append(runes, randomScalarValue(rng))
	}
	return string(runes)
}

// randomScalarValue returns a random valid Unicode scalar value (U+0000..U+10FFFF
// excluding the surrogate range U+D800..U+DFFF).
func randomScalarValue(rng *rand.Rand) rune {
	for {
		r := rune(rng.Intn(0x110000))
		if r < 0xD800 || r > 0xDFFF {
			return r
		}
}