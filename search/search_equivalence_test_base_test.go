// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Rendering of lucene/test-framework/src/java/org/apache/lucene/tests/search/SearchEquivalenceTestBase.java
// (Apache Lucene 10.5.0) for the search_test package: simple base class for
// checking search equivalence. Extend it, and write tests that create
// randomTerm()s (all terms are single characters a-z), and use
// assertSameSet(Query, Query) and assertSubsetOf(Query, Query).
//
// The static @BeforeClass state is rebuilt for every test (newSeqHarness);
// every test reads it only, so the data each test sees is the same.

package search_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// seqHarness renders the static fields of SearchEquivalenceTestBase.
type seqHarness struct {
	t         *testing.T
	s1, s2    *search.IndexSearcher
	directory store.Directory
	reader    *index.DirectoryReader
	analyzer  *testanalysis.MockAnalyzer
	stopword  string // we always pick a character as a stopword
}

// newSeqHarness renders beforeClass().
func newSeqHarness(t *testing.T) *seqHarness {
	t.Helper()
	h := &seqHarness{t: t}
	rnd := random()
	directory := newDirectory()
	h.directory = directory
	h.stopword = string(rune(seqRandomChar()))
	// CharacterRunAutomaton stopset = new CharacterRunAutomaton(Automata.makeString(stopword));
	stopset := map[string]struct{}{h.stopword: {}}
	h.analyzer = testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, false, 0, stopset, true)
	iw := newRandomIndexWriterWithAnalyzer(t, directory, h.analyzer)
	doc := document.NewDocument()
	id, err := document.NewStringField("id", "", false)
	if err != nil {
		t.Fatal(err)
	}
	field, err := document.NewTextField("field", "", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(id)
	doc.Add(field)

	// index some docs
	numDocs := atLeast(100) // TEST_NIGHTLY ? atLeast(1000) : atLeast(100)
	for i := 0; i < numDocs; i++ {
		id.SetStringValue(strconv.Itoa(i))
		field.SetStringValue(seqRandomFieldContents())
		mustAddDocument(t, iw, doc)
	}

	// delete some docs
	numDeletes := numDocs / 20
	for i := 0; i < numDeletes; i++ {
		toDelete := index.NewTerm("id", strconv.Itoa(rnd.Intn(numDocs)))
		if rnd.Intn(2) == 0 {
			if _, err := iw.DeleteDocuments(toDelete); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
		} else {
			if _, err := iw.DeleteDocumentsQuery(search.NewTermQuery(toDelete)); err != nil {
				t.Fatalf("deleteDocuments(Query): %v", err)
			}
		}
	}

	h.reader = mustGetReader(t, iw)
	t.Cleanup(h.close)
	h.s1 = newSearcher(t, h.reader)
	// Disable the query cache, which converts two-phase iterators to normal iterators, while we
	// want to make sure two-phase iterators are exercised.
	h.s1.SetQueryCache(nil)
	h.s2 = newSearcher(t, h.reader)
	h.s2.SetQueryCache(nil)
	mustClose(t, iw)
	return h
}

// close renders afterClass().
func (h *seqHarness) close() {
	if h.reader == nil {
		return
	}
	mustClose(h.t, h.reader, h.directory, h.analyzer)
	h.reader = nil
	h.directory = nil
	h.analyzer = nil
	h.s1, h.s2 = nil, nil
}

// seqRandomFieldContents renders randomFieldContents(): populate a field with
// random contents. terms should be single characters in lowercase (a-z)
// tokenization can be assumed to be on whitespace.
func seqRandomFieldContents() string {
	var sb []byte
	numTerms := random().Intn(15)
	for i := 0; i < numTerms; i++ {
		if len(sb) > 0 {
			sb = append(sb, ' ') // whitespace
		}
		sb = append(sb, seqRandomChar())
	}
	return string(sb)
}

// seqRandomChar renders randomChar(): returns random character (a-z).
func seqRandomChar() byte {
	c := byte(nextInt('a', 'z'))
	if random().Intn(2) == 0 {
		// bias towards earlier chars, so that chars have a ~ zipfian distribution with earlier chars
		// having a higher frequency
		c = byte(nextInt('a', int(c)))
	}
	return c
}

// randomTerm returns a term suitable for searching. terms are single
// characters in lowercase (a-z).
func (h *seqHarness) randomTerm() *index.Term {
	return index.NewTerm("field", string(rune(seqRandomChar())))
}

// randomTermDistinct renders `do { t2 = randomTerm(); } while (t1.equals(t2));`.
func (h *seqHarness) randomTermDistinct(t1 *index.Term) *index.Term {
	for {
		t2 := h.randomTerm()
		if !t1.Equals(t2) {
			return t2
		}
	}
}

// randomFilter returns a random filter over the document set.
func (h *seqHarness) randomFilter() search.Query {
	if random().Intn(2) == 0 {
		return search.NewTermRangeQueryWithStrings("field", "a", string(rune(seqRandomChar())), true, true)
	}
	// use a query with a two-phase approximation
	return search.NewPhraseQuery(100, "field", string(rune(seqRandomChar())), string(rune(seqRandomChar())))
}

// assertSameSet asserts that the documents returned by q1 are the same as of
// those returned by q2.
func (h *seqHarness) assertSameSet(q1, q2 search.Query) {
	h.t.Helper()
	h.assertSubsetOf(q1, q2)
	h.assertSubsetOf(q2, q1)
}

// assertSubsetOf asserts that the documents returned by q1 are a subset of
// those returned by q2.
func (h *seqHarness) assertSubsetOf(q1, q2 search.Query) {
	h.t.Helper()
	// test without a filter
	h.assertSubsetOfFiltered(q1, q2, nil)

	// test with some filters (this will sometimes cause advance'ing enough to test it)
	numFilters := atLeast(3) // TEST_NIGHTLY ? atLeast(10) : atLeast(3)
	for i := 0; i < numFilters; i++ {
		filter := h.randomFilter()
		// incorporate the filter in different ways.
		h.assertSubsetOfFiltered(q1, q2, filter)
		h.assertSubsetOfFiltered(h.filteredQuery(q1, filter), h.filteredQuery(q2, filter), nil)
	}
}

// assertSubsetOfFiltered renders assertSubsetOf(Query, Query, Query filter):
// both queries will be filtered by filter.
func (h *seqHarness) assertSubsetOfFiltered(q1, q2, filter search.Query) {
	t := h.t
	t.Helper()
	queryUtilsCheck(t, q1)
	queryUtilsCheck(t, q2)

	if filter != nil {
		q1 = search.NewBooleanQueryBuilder().Add(q1, search.MUST).Add(filter, search.FILTER).Build()
		q2 = search.NewBooleanQueryBuilder().Add(q2, search.MUST).Add(filter, search.FILTER).Build()
	}
	// we test both INDEXORDER and RELEVANCE because we want to test needsScores=true/false
	for _, sort := range []*search.Sort{search.INDEXORDER, search.RELEVANCE} {
		// not efficient, but simple!
		td1, err := h.s1.SearchWithSort(q1, h.reader.MaxDoc(), sort, false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		td2, err := h.s2.SearchWithSort(q2, h.reader.MaxDoc(), sort, false)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !(td1.TotalHits.Value <= td2.TotalHits.Value) {
			t.Fatalf("too many hits: %d > %d", td1.TotalHits.Value, td2.TotalHits.Value)
		}

		// fill the superset into a bitset
		bitset := map[int]struct{}{}
		for i := 0; i < len(td2.ScoreDocs); i++ {
			bitset[td2.ScoreDocs[i].Doc] = struct{}{}
		}

		// check in the subset, that every bit was set by the super
		for i := 0; i < len(td1.ScoreDocs); i++ {
			if _, ok := bitset[td1.ScoreDocs[i].Doc]; !ok {
				t.Fatalf("doc %d of q1 not matched by q2", td1.ScoreDocs[i].Doc)
			}
		}
	}
}

// assertSameScores asserts that two queries return the same documents and
// with the same scores.
func (h *seqHarness) assertSameScores(q1, q2 search.Query) {
	h.t.Helper()
	h.assertSameSet(q1, q2)

	h.assertSameScoresFiltered(q1, q2, nil)
	// also test with some filters to test advancing
	numFilters := atLeast(3) // TEST_NIGHTLY ? atLeast(10) : atLeast(3)
	for i := 0; i < numFilters; i++ {
		filter := h.randomFilter()
		// incorporate the filter in different ways.
		h.assertSameScoresFiltered(q1, q2, filter)
		h.assertSameScoresFiltered(h.filteredQuery(q1, filter), h.filteredQuery(q2, filter), nil)
	}
}

// assertSameScoresFiltered renders assertSameScores(Query, Query, Query filter).
func (h *seqHarness) assertSameScoresFiltered(q1, q2, filter search.Query) {
	t := h.t
	t.Helper()
	// not efficient, but simple!
	if filter != nil {
		q1 = search.NewBooleanQueryBuilder().Add(q1, search.MUST).Add(filter, search.FILTER).Build()
		q2 = search.NewBooleanQueryBuilder().Add(q2, search.MUST).Add(filter, search.FILTER).Build()
	}
	td1 := mustSearch(t, h.s1, q1, h.reader.MaxDoc())
	td2 := mustSearch(t, h.s2, q2, h.reader.MaxDoc())
	if td1.TotalHits.Value != td2.TotalHits.Value {
		t.Fatalf("totalHits: %d vs %d", td1.TotalHits.Value, td2.TotalHits.Value)
	}
	for i := 0; i < len(td1.ScoreDocs); i++ {
		if td1.ScoreDocs[i].Doc != td2.ScoreDocs[i].Doc {
			t.Fatalf("doc %d: %d vs %d", i, td1.ScoreDocs[i].Doc, td2.ScoreDocs[i].Doc)
		}
		if math.Abs(float64(td1.ScoreDocs[i].Score-td2.ScoreDocs[i].Score)) > 10e-5 {
			t.Fatalf("score %d: %v vs %v", i, td1.ScoreDocs[i].Score, td2.ScoreDocs[i].Score)
		}
	}
}

// filteredQuery renders filteredQuery(Query, Query).
func (h *seqHarness) filteredQuery(query, filter search.Query) search.Query {
	return search.NewBooleanQueryBuilder().Add(query, search.MUST).Add(filter, search.FILTER).Build()
}

// hashStringSeed derives a deterministic seed from a string.
func hashStringSeed(s string) int64 {
	var hash int64 = 1125899906842597 // prime
	for i := 0; i < len(s); i++ {
		hash = 31*hash + int64(s[i])
	}
	return hash
}
