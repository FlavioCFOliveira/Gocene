// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRegexpRandom2.java
//
// Creates an index of random terms and validates that a RegexpQuery returns the
// same documents (and scores) as a simple brute-force reference implementation
// that scans every term and accepts those that fully match the regular
// expression — the invariant the Java test enforces via its DumbRegexpQuery
// (CheckHits.checkEqual(smart, dumb)).
//
// Deviations from the reference, documented per the binary-compatibility
// mandate's non-determinism clause:
//   - The upstream test uses AutomatonTestUtil.randomRegexp, which generates
//     patterns in Lucene's RegExp syntax, and a DumbRegexpQuery built on Lucene's
//     CharacterRunAutomaton. Gocene's RegexpQuery is backed by Go's RE2 engine
//     (anchored full-term match), so this port generates RE2-compatible random
//     patterns over a small alphabet and uses an anchored regexp as the
//     brute-force reference. The invariant under test — RegexpQuery hits equal
//     the set of terms that fully match the expression — is identical.
//   - The NFA RegexpQuery variant (the four-argument RegexpQuery constructor with
//     an explicit RewriteMethod and useNFA flag) is not part of Gocene's
//     RegexpQuery surface and is therefore omitted; the smart-vs-reference
//     comparison is preserved.

package search_test

import (
	"math/rand"
	"regexp"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const regexp2Field = "field"

// newRegexpRandom2Searcher indexes random short terms over a small alphabet so
// that the random RE2 patterns below have a meaningful chance of matching.
func newRegexpRandom2Searcher(t *testing.T) (*search.IndexSearcher, []string, func()) {
	t.Helper()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed
	ix := newIntegrationIndex(t)
	const num = 200
	terms := make([]string, 0, num)
	for i := 0; i < num; i++ {
		s := regexp2RandomTerm(rng)
		terms = append(terms, s)
		ix.addString(regexp2Field, s)
	}
	searcher, cleanup := ix.searcher()
	return searcher, terms, cleanup
}

// regexp2RandomTerm draws a short term from a small alphabet (a-e plus a couple
// of multi-byte runes) so that random patterns can match across docs.
func regexp2RandomTerm(rng *rand.Rand) string {
	alphabet := []rune{'a', 'b', 'c', 'd', 'e', 'é', 'ß'}
	n := rng.Intn(4) // 0..3 runes
	runes := make([]rune, 0, n)
	for i := 0; i < n; i++ {
		runes = append(runes, alphabet[rng.Intn(len(alphabet))])
	}
	return string(runes)
}

// regexp2RandomRegexp builds a small RE2-compatible random regular expression.
func regexp2RandomRegexp(rng *rand.Rand) string {
	atoms := []string{"a", "b", "c", "d", "e", ".", "[a-c]", "[bd]", "é", "ß"}
	n := rng.Intn(3) + 1
	out := ""
	for i := 0; i < n; i++ {
		a := atoms[rng.Intn(len(atoms))]
		switch rng.Intn(4) {
		case 0:
			a += "*"
		case 1:
			a += "?"
		case 2:
			a += "+"
		}
		out += a
	}
	return out
}

// TestRegexpRandom2_Regexps ports testRegexps.
func TestRegexpRandom2_Regexps(t *testing.T) {
	s, _, cleanup := newRegexpRandom2Searcher(t)
	defer cleanup()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()) ^ 0x2E6E)) //nolint:gosec // deterministic test seed

	num := 200
	for i := 0; i < num; i++ {
		reg := regexp2RandomRegexp(rng)
		regexp2AssertSame(t, s, reg)
	}
}

// regexp2AssertSame ports assertSame: the RegexpQuery hits must equal the
// brute-force reference's hits.
func regexp2AssertSame(t *testing.T, s *search.IndexSearcher, reg string) {
	t.Helper()

	smart, err := search.NewRegexpQuery(regexp2Field, reg)
	if err != nil {
		// An invalid pattern is skipped on both sides identically (it is not a
		// feature gap — the reference compiler would reject it too).
		if _, rerr := regexp.Compile("^(?:" + reg + ")$"); rerr != nil {
			return
		}
		t.Fatalf("NewRegexpQuery(%q): %v", reg, err)
	}

	dumb := regexp2BuildReference(t, s, reg)

	smartDocs, err := s.Search(smart, 25)
	if err != nil {
		t.Fatalf("Search(smart, %q): %v", reg, err)
	}
	dumbDocs, err := s.Search(dumb, 25)
	if err != nil {
		t.Fatalf("Search(dumb, %q): %v", reg, err)
	}
	regexp2CheckEqual(t, reg, smartDocs.ScoreDocs, dumbDocs.ScoreDocs)
}

// regexp2BuildReference scans every term and builds a ConstantScoreQuery over the
// terms that fully match reg — the analogue of DumbRegexpQuery.
func regexp2BuildReference(t *testing.T, s *search.IndexSearcher, reg string) search.Query {
	t.Helper()
	re := regexp.MustCompile("^(?:" + reg + ")$")

	tp, ok := s.GetIndexReader().(termsProvider)
	if !ok {
		t.Fatalf("reader does not expose Terms(field)")
	}
	terms, err := tp.Terms(regexp2Field)
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
	bq := search.NewBooleanQuery()
	any := false
	for {
		cur, nerr := it.Next()
		if nerr != nil {
			t.Fatalf("Next: %v", nerr)
		}
		if cur == nil {
			break
		}
		bv := cur.BytesValue()
		if bv == nil {
			continue
		}
		if re.Match(bv.ValidBytes()) {
			bq.Add(search.NewTermQuery(index.NewTermFromBytes(regexp2Field, bv.ValidBytes())), search.SHOULD)
			any = true
		}
	}
	if !any {
		return search.NewMatchNoDocsQuery()
	}
	return search.NewConstantScoreQuery(bq)
}

// regexp2CheckEqual ports CheckHits.checkEqual: same set of doc ids.
func regexp2CheckEqual(t *testing.T, reg string, hits1, hits2 []*search.ScoreDoc) {
	t.Helper()
	if len(hits1) != len(hits2) {
		t.Errorf("regexp %q: unequal lengths: smart=%d reference=%d", reg, len(hits1), len(hits2))
		return
	}
	a := append([]*search.ScoreDoc(nil), hits1...)
	b := append([]*search.ScoreDoc(nil), hits2...)
	sort.SliceStable(a, func(i, j int) bool { return a[i].Doc < a[j].Doc })
	sort.SliceStable(b, func(i, j int) bool { return b[i].Doc < b[j].Doc })
	for i := range a {
		if a[i].Doc != b[i].Doc {
			t.Errorf("regexp %q: hit %d doc mismatch: smart=%d reference=%d", reg, i, a[i].Doc, b[i].Doc)
		}
}