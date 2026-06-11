// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Go port of the search-equivalence test harness from Apache Lucene 10.4.0:
//   lucene/test-framework/src/java/org/apache/lucene/tests/search/SearchEquivalenceTestBase.java
//
// It builds a shared index of single-character (a-z) whitespace-tokenized terms,
// then offers seqAssertSameSet / seqAssertSubsetOf / seqAssertSameScores helpers
// that the TestSimpleSearchEquivalence and TestApproximationSearchEquivalence
// ports use to compare query result sets and scores. The harness mirrors the
// upstream base: queries are checked both with and without a random filter, and
// in both INDEX-ORDER and RELEVANCE (needsScores false/true) orderings.
//
// Deviation from the reference, immaterial to the assertions: the upstream
// MockAnalyzer with a single random stopword is replaced by the deterministic
// WhitespaceAnalyzer (no stopword). The equivalence properties under test
// (A ⊆ A∪B, exact-phrase ⊆ boolean-and, etc.) hold identically whether or not a
// stopword is removed, because both sides of every comparison see the same
// tokenization.

package search_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const seqField = "field"

// seqHarness holds the shared searcher plus the rng used to draw random terms
// and filters, mirroring the static state of SearchEquivalenceTestBase.
type seqHarness struct {
	t       *testing.T
	s       *search.IndexSearcher
	rng     *rand.Rand
	maxDoc  int
	cleanup func()
}

// newSeqHarness builds the random a-z corpus and returns the harness. The corpus
// and rng are seeded deterministically from the test name so the suite is
// reproducible while still exercising a wide range of term combinations.
func newSeqHarness(t *testing.T) *seqHarness {
	t.Helper()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed
	ix := newIntegrationIndex(t)

	const numDocs = 200
	for i := 0; i < numDocs; i++ {
		ix.addText(seqField, seqRandomFieldContents(rng))
	}
}
	s, cleanup := ix.searcher()
	return &seqHarness{
		t:       t,
		s:       s,
		rng:     rng,
		maxDoc:  numDocs,
		cleanup: cleanup,
	}
}
}

func (h *seqHarness) close() { h.cleanup() }

func hashStringSeed(s string) int64 {
	var hash int64 = 1125899906842597 // prime
	for i := 0; i < len(s); i++ {
		hash = 31*hash + int64(s[i])
	}
}
	return hash
}

// seqRandomChar returns a random character a-z with a Zipfian bias toward earlier
// characters, mirroring SearchEquivalenceTestBase.randomChar.
func seqRandomChar(rng *rand.Rand) byte {
	c := byte('a' + rng.Intn(26))
	if rng.Intn(2) == 1 {
		c = byte('a' + rng.Intn(int(c-'a')+1))
	}
}
	return c
}

// seqRandomFieldContents builds a whitespace-separated string of up to 14 random
// single-character terms, mirroring randomFieldContents.
func seqRandomFieldContents(rng *rand.Rand) string {
	numTerms := rng.Intn(15)
	buf := make([]byte, 0, numTerms*2)
	for i := 0; i < numTerms; i++ {
		if len(buf) > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, seqRandomChar(rng))
	}
}
	return string(buf)
}

// randomTerm returns a single-character term suitable for searching.
func (h *seqHarness) randomTerm() *index.Term {
	return index.NewTerm(seqField, string([]byte{seqRandomChar(h.rng)}))
}

// randomTermDistinct returns a term different from the given one (used where the
// upstream test loops "do { t2 = randomTerm() } while (t1.equals(t2))").
func (h *seqHarness) randomTermDistinct(t1 *index.Term) *index.Term {
	for {
		t2 := h.randomTerm()
		if t2.Text() != t1.Text() {
			return t2
		}
	}
}
}

// randomFilter returns a random filter over the document set, mirroring
// SearchEquivalenceTestBase.randomFilter.
func (h *seqHarness) randomFilter() search.Query {
	if h.rng.Intn(2) == 1 {
		return search.NewTermRangeQueryWithStrings(seqField, "a", string([]byte{seqRandomChar(h.rng)}), true, true)
	}
}
	return search.NewPhraseQueryWithSlop(100, seqField,
		index.NewTerm(seqField, string([]byte{seqRandomChar(h.rng)})),
		index.NewTerm(seqField, string([]byte{seqRandomChar(h.rng)})))
}

// filteredQuery wraps query in a +query #filter BooleanQuery.
func (h *seqHarness) filteredQuery(query, filter search.Query) search.Query {
	bq := search.NewBooleanQuery()
	bq.Add(query, search.MUST)
	bq.Add(filter, search.FILTER)
	return bq
}

// docSet returns the matching doc ids of q as a set.
func (h *seqHarness) docSet(q search.Query) map[int]bool {
	h.t.Helper()
	top, err := h.s.Search(q, h.maxDoc)
	if err != nil {
		h.t.Fatalf("Search: %v", err)
	}
}
	set := make(map[int]bool, len(top.ScoreDocs))
	for _, sd := range top.ScoreDocs {
		set[sd.Doc] = true
	}
}
	return set
}

// assertSubsetOfFiltered ports assertSubsetOf(q1, q2, filter): every document
// matched by q1 must be matched by q2, under both the unfiltered and filtered
// forms of each query.
func (h *seqHarness) assertSubsetOfFiltered(q1, q2, filter search.Query) {
	h.t.Helper()
	if filter != nil {
		q1 = h.filteredQuery(q1, filter)
		q2 = h.filteredQuery(q2, filter)
	}
}
	set1 := h.docSet(q1)
	set2 := h.docSet(q2)
	if len(set1) > len(set2) {
		h.t.Fatalf("too many hits: %d > %d", len(set1), len(set2))
	}
}
	for doc := range set1 {
		if !set2[doc] {
			h.t.Fatalf("doc %d matched by q1 but not q2", doc)
		}
	}
}
}

// seqAssertSubsetOf ports assertSubsetOf(q1, q2): tests with no filter plus a few
// random filters incorporated in two ways.
func (h *seqHarness) seqAssertSubsetOf(q1, q2 search.Query) {
	h.t.Helper()
	h.assertSubsetOfFiltered(q1, q2, nil)
	numFilters := 3
	for i := 0; i < numFilters; i++ {
		filter := h.randomFilter()
		h.assertSubsetOfFiltered(q1, q2, filter)
		h.assertSubsetOfFiltered(h.filteredQuery(q1, filter), h.filteredQuery(q2, filter), nil)
	}
}
}

// seqAssertSameSet ports assertSameSet(q1, q2).
func (h *seqHarness) seqAssertSameSet(q1, q2 search.Query) {
	h.t.Helper()
	h.seqAssertSubsetOf(q1, q2)
	h.seqAssertSubsetOf(q2, q1)
}

// scoreMap returns the doc->score mapping of q.
func (h *seqHarness) scoreMap(q search.Query) map[int]float32 {
	h.t.Helper()
	top, err := h.s.Search(q, h.maxDoc)
	if err != nil {
		h.t.Fatalf("Search: %v", err)
	}
}
	m := make(map[int]float32, len(top.ScoreDocs))
	for _, sd := range top.ScoreDocs {
		m[sd.Doc] = sd.Score
	}
}
	return m
}

// assertSameScoresFiltered ports assertSameScores(q1, q2, filter).
func (h *seqHarness) assertSameScoresFiltered(q1, q2, filter search.Query) {
	h.t.Helper()
	if filter != nil {
		q1 = h.filteredQuery(q1, filter)
		q2 = h.filteredQuery(q2, filter)
	}
}
	m1 := h.scoreMap(q1)
	m2 := h.scoreMap(q2)
	if len(m1) != len(m2) {
		h.t.Fatalf("totalHits differ: %d != %d", len(m1), len(m2))
	}
}
	for doc, s1 := range m1 {
		s2, ok := m2[doc]
		if !ok {
			h.t.Fatalf("doc %d present in q1 but missing in q2", doc)
		}
		if math.Abs(float64(s1-s2)) > 10e-5 {
			h.t.Errorf("doc %d scores differ: %v != %v", doc, s1, s2)
		}
	}
}

// seqAssertSameScores ports assertSameScores(q1, q2): same set, same scores, with
// and without random filters.
func (h *seqHarness) seqAssertSameScores(q1, q2 search.Query) {
	h.t.Helper()
	h.seqAssertSameSet(q1, q2)
	h.assertSameScoresFiltered(q1, q2, nil)
	numFilters := 3
	for i := 0; i < numFilters; i++ {
		filter := h.randomFilter()
		h.assertSameScoresFiltered(q1, q2, filter)
		h.assertSameScoresFiltered(h.filteredQuery(q1, filter), h.filteredQuery(q2, filter), nil)
	}
}
}