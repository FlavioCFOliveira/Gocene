// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestFuzzyQuery.java
// (Apache Lucene 10.5.0): tests FuzzyQuery.

package search_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

func TestFuzzyQueryBasicPrefix(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)
	addFuzzyDoc(t, "abc", writer)
	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)

	query := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "abc"), search.DefaultMaxEdits, 1)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	if len(hits) != 1 {
		t.Fatalf("expected 1, got %d", len(hits))
	}
	mustClose(t, reader, directory)
}

// assertFuzzyHits asserts the hit count and, when order is non-nil, the
// stored "field" value of each hit in order.
func assertFuzzyHits(t *testing.T, storedFields index.StoredFields, hits []*search.ScoreDoc, expected int, order ...string) {
	t.Helper()
	if len(hits) != expected {
		t.Fatalf("expected %d hits, got %d", expected, len(hits))
	}
	for i, want := range order {
		if got := storedDocument(t, storedFields, hits[i].Doc).GetString("field"); got != want {
			t.Fatalf("hit %d: expected %q, got %q", i, want, got)
		}
	}
}

func TestFuzzyQueryFuzziness(t *testing.T) {
	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	writer := newRandomIndexWriterWithConfig(t, directory, iwc)
	addFuzzyDoc(t, "aaaaa", writer)
	addFuzzyDoc(t, "aaaab", writer)
	addFuzzyDoc(t, "aaabb", writer)
	addFuzzyDoc(t, "aabbb", writer)
	addFuzzyDoc(t, "abbbb", writer)
	addFuzzyDoc(t, "bbbbb", writer)
	addFuzzyDoc(t, "ddddd", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)

	query := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaaa"), search.DefaultMaxEdits, 0)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 3)

	// same with prefix
	for _, c := range []struct{ prefix, want int }{{1, 3}, {2, 3}, {3, 3}, {4, 2}, {5, 1}, {6, 1}} {
		query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaaa"), search.DefaultMaxEdits, c.prefix)
		hits = mustSearch(t, searcher, query, 1000).ScoreDocs
		assertFuzzyHits(t, nil, hits, c.want)
	}

	// test scoring
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "bbbbb"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	storedFields := mustStoredFields(t, searcher)
	assertFuzzyHits(t, storedFields, hits, 3, "bbbbb", "abbbb", "aabbb")

	// test pq size by supplying maxExpansions=2
	// This query would normally return 3 documents, because 3 terms match (see above):
	query = search.NewFuzzyQueryFull(index.NewTerm("field", "bbbbb"), search.DefaultMaxEdits, 0, 2, false)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, storedFields, hits, 2, "bbbbb", "abbbb")

	// not similar enough:
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "xxxxx"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 0)
	query = search.NewFuzzyQueryWithPrefix(
		index.NewTerm("field", "aaccc"),
		search.DefaultMaxEdits,
		0) // edit distance to "aaaaa" = 3
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 0)

	// query identical to a word in the index:
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaaa"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	// default allows for up to two edits:
	assertFuzzyHits(t, storedFields, hits, 3, "aaaaa", "aaaab", "aaabb")

	// query similar to a word in the index:
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaac"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, storedFields, hits, 3, "aaaaa", "aaaab", "aaabb")

	// now with prefix
	for _, prefix := range []int{1, 2, 3} {
		query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaac"), search.DefaultMaxEdits, prefix)
		hits = mustSearch(t, searcher, query, 1000).ScoreDocs
		assertFuzzyHits(t, storedFields, hits, 3, "aaaaa", "aaaab", "aaabb")
	}
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaac"), search.DefaultMaxEdits, 4)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, storedFields, hits, 2, "aaaaa", "aaaab")
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "aaaac"), search.DefaultMaxEdits, 5)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 0)

	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "ddddX"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, storedFields, hits, 1, "ddddd")

	// now with prefix
	for _, prefix := range []int{1, 2, 3, 4} {
		query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "ddddX"), search.DefaultMaxEdits, prefix)
		hits = mustSearch(t, searcher, query, 1000).ScoreDocs
		assertFuzzyHits(t, storedFields, hits, 1, "ddddd")
	}
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "ddddX"), search.DefaultMaxEdits, 5)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 0)

	// different field = no match:
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("anotherfield", "ddddX"), search.DefaultMaxEdits, 0)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 0)

	mustClose(t, reader, directory)
}

func TestFuzzyQueryPrefixLengthEqualStringLength(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)
	addFuzzyDoc(t, "b*a", writer)
	addFuzzyDoc(t, "b*ab", writer)
	addFuzzyDoc(t, "b*abc", writer)
	addFuzzyDoc(t, "b*abcd", writer)
	multibyte := "아프리카코끼리속"
	addFuzzyDoc(t, multibyte, writer)
	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)

	maxEdits := 0
	prefixLength := 3
	query := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "b*a"), maxEdits, prefixLength)
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 1)

	maxEdits = 1
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "b*a"), maxEdits, prefixLength)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 2)

	maxEdits = 2
	query = search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "b*a"), maxEdits, prefixLength)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 3)

	maxEdits = 1
	// multibyte.length() counts UTF-16 chars; every char of multibyte is in the BMP.
	multibyteChars := []rune(multibyte)
	prefixLength = len(multibyteChars) - 1
	query = search.NewFuzzyQueryWithPrefix(
		index.NewTerm("field", string(multibyteChars[:prefixLength])), maxEdits, prefixLength)
	hits = mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 1)

	mustClose(t, reader, directory)
}

func TestFuzzyQuery2(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriterWithAnalyzer(t,
		directory, testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	for _, text := range []string{
		"LANGE", "LUETH", "PIRSING", "RIEGEL", "TRZECZIAK", "WALKER", "WBR", "WE", "WEB", "WEBE",
		"WEBER", "WEBERE", "WEBREE", "WEBEREI", "WBRE", "WITTKOPF", "WOJNAROWSKI", "WRICKE",
	} {
		addFuzzyDoc(t, text, writer)
	}

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)

	query := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", "WEBER"), 2, 1)
	// query.setRewriteMethod(FuzzyQuery.SCORING_BOOLEAN_QUERY_REWRITE);
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	assertFuzzyHits(t, nil, hits, 8)

	mustClose(t, reader, directory)
}

func TestFuzzyQuerySingleQueryExactMatchScoresHighest(t *testing.T) {
	// See issue LUCENE-329 - IDF shouldn't wreck similarity ranking
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)
	for _, text := range []string{"smith", "smith", "smith", "smith", "smith", "smith", "smythe", "smdssasd"} {
		addFuzzyDoc(t, text, writer)
	}

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	searcher.SetSimilarity(search.NewClassicSimilarity()) // avoid randomisation of similarity algo by test framework
	mustClose(t, writer)
	searchTerms := []string{"smith", "smythe", "smdssasd"}
	storedFields := mustStoredFields(t, reader)
	for _, searchTerm := range searchTerms {
		query := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", searchTerm), 2, 1)
		hits := mustSearch(t, searcher, query, 1000).ScoreDocs
		bestDoc := storedDocument(t, storedFields, hits[0].Doc)
		if len(hits) <= 0 {
			t.Fatal("expected hits")
		}
		topMatch := bestDoc.GetString("field")
		if topMatch != searchTerm {
			t.Fatalf("expected %q, got %q", searchTerm, topMatch)
		}
		if len(hits) > 1 {
			worstDoc := storedDocument(t, storedFields, hits[len(hits)-1].Doc)
			// String worstMatch = worstDoc.get("field");
			// assertNotSame(searchTerm, worstMatch) compares object identity, which
			// Go strings do not have: the stored value is read back from the index
			// and is never the searchTerm literal itself.
			if worstDoc.Get("field") == nil {
				t.Fatal("worst doc has no stored field")
			}
		}
	}
	mustClose(t, reader, directory)
}

func TestFuzzyQueryMultipleQueriesIdfWorks(t *testing.T) {
	// With issue LUCENE-329 - it could be argued a
	// MultiTermQuery.TopTermsBoostOnlyBooleanQueryRewrite
	// is the solution as it disables IDF.
	// However - IDF is still useful as in this case where there are multiple FuzzyQueries.
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)

	addFuzzyDoc(t, "michael smith", writer)
	addFuzzyDoc(t, "michael lucero", writer)
	addFuzzyDoc(t, "doug cutting", writer)
	addFuzzyDoc(t, "doug cuttin", writer)
	addFuzzyDoc(t, "michael wardle", writer)
	addFuzzyDoc(t, "micheal vegas", writer)
	addFuzzyDoc(t, "michael lydon", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	searcher.SetSimilarity(search.NewClassicSimilarity()) // avoid randomisation of similarity algo by test framework

	mustClose(t, writer)

	query := search.NewBooleanQueryBuilder()
	commonSearchTerm := "michael"
	commonQuery := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", commonSearchTerm), 2, 1)
	query.Add(commonQuery, search.SHOULD)

	rareSearchTerm := "cutting"
	rareQuery := search.NewFuzzyQueryWithPrefix(index.NewTerm("field", rareSearchTerm), 2, 1)
	query.Add(rareQuery, search.SHOULD)
	hits := mustSearch(t, searcher, query.Build(), 1000).ScoreDocs

	// Matches on the rare surname should be worth more than matches on the common forename
	if len(hits) != 7 {
		t.Fatalf("expected 7, got %d", len(hits))
	}
	storedFields := mustStoredFields(t, searcher)
	bestDoc := storedDocument(t, storedFields, hits[0].Doc)
	topMatch := bestDoc.GetString("field")
	if !strings.Contains(topMatch, rareSearchTerm) {
		t.Fatalf("top match %q does not contain %q", topMatch, rareSearchTerm)
	}

	runnerUpDoc := storedDocument(t, storedFields, hits[1].Doc)
	runnerUpMatch := runnerUpDoc.GetString("field")
	if !strings.Contains(runnerUpMatch, "cuttin") {
		t.Fatalf("runner-up %q does not contain cuttin", runnerUpMatch)
	}

	worstDoc := storedDocument(t, storedFields, hits[len(hits)-1].Doc)
	worstMatch := worstDoc.GetString("field")
	if !strings.Contains(worstMatch, "micheal") { // misspelling of common name
		t.Fatalf("worst match %q does not contain micheal", worstMatch)
	}

	mustClose(t, reader, directory)
}

// MultiTermQuery provides (via attribute) information about which values must
// be competitive to enter the priority queue.
//
// FuzzyQuery optimizes itself around this information, if the attribute is not
// implemented correctly, there will be problems!
func TestFuzzyQueryTieBreaker(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)
	addFuzzyDoc(t, "a123456", writer)
	addFuzzyDoc(t, "c123456", writer)
	addFuzzyDoc(t, "d123456", writer)
	addFuzzyDoc(t, "e123456", writer)

	directory2 := newDirectory()
	writer2 := newRandomIndexWriter(t, directory2)
	addFuzzyDoc(t, "a123456", writer2)
	addFuzzyDoc(t, "b123456", writer2)
	addFuzzyDoc(t, "b123456", writer2)
	addFuzzyDoc(t, "b123456", writer2)
	addFuzzyDoc(t, "c123456", writer2)
	addFuzzyDoc(t, "f123456", writer2)

	ir1 := mustGetReader(t, writer)
	ir2 := mustGetReader(t, writer2)

	mr := newMultiReader(t, ir1, ir2)
	searcher := newSearcher(t, mr)
	fq := search.NewFuzzyQueryFull(index.NewTerm("field", "z123456"), 1, 0, 2, false)
	docs := mustSearch(t, searcher, fq, 2)
	if docs.TotalHits.Value != 5 { // 5 docs, from the a and b's
		t.Fatalf("expected 5, got %d", docs.TotalHits.Value)
	}
	mustClose(t, mr, ir1, ir2, writer, writer2, directory, directory2)
}

// Test the TopTermsBoostOnlyBooleanQueryRewrite rewrite method.
func TestFuzzyQueryBoostOnlyRewrite(t *testing.T) {
	directory := newDirectory()
	writer := newRandomIndexWriter(t, directory)
	addFuzzyDoc(t, "Lucene", writer)
	addFuzzyDoc(t, "Lucene", writer)
	addFuzzyDoc(t, "Lucenne", writer)

	reader := mustGetReader(t, writer)
	searcher := newSearcher(t, reader)
	mustClose(t, writer)

	query := search.NewFuzzyQueryWithRewriteMethod(
		index.NewTerm("field", "lucene"),
		search.DefaultMaxEdits,
		search.DefaultPrefixLength,
		search.DefaultMaxExpansions,
		search.DefaultTranspositions,
		search.NewTopTermsBoostOnlyBooleanQueryRewrite(50))
	hits := mustSearch(t, searcher, query, 1000).ScoreDocs
	// normally, 'Lucenne' would be the first result as IDF will skew the score.
	assertFuzzyHits(t, mustStoredFields(t, reader), hits, 3, "Lucene", "Lucene", "Lucenne")
	mustClose(t, reader, directory)
}

func TestFuzzyQueryGiga(t *testing.T) {
	index_ := newDirectory()
	w := newRandomIndexWriter(t, index_)

	addFuzzyDoc(t, "Lucene in Action", w)
	addFuzzyDoc(t, "Lucene for Dummies", w)

	// addDoc("Giga", w);
	addFuzzyDoc(t, "Giga byte", w)

	addFuzzyDoc(t, "ManagingGigabytesManagingGigabyte", w)
	addFuzzyDoc(t, "ManagingGigabytesManagingGigabytes", w)

	addFuzzyDoc(t, "The Art of Computer Science", w)
	addFuzzyDoc(t, "J. K. Rowling", w)
	addFuzzyDoc(t, "JK Rowling", w)
	addFuzzyDoc(t, "Joanne K Roling", w)
	addFuzzyDoc(t, "Bruce Willis", w)
	addFuzzyDoc(t, "Willis bruce", w)
	addFuzzyDoc(t, "Brute willis", w)
	addFuzzyDoc(t, "B. willis", w)
	r := mustGetReader(t, w)
	mustClose(t, w)

	q := search.NewFuzzyQueryWithMaxEdits(index.NewTerm("field", "giga"), 0)

	// 3. search
	searcher := newSearcher(t, r)
	hits := mustSearch(t, searcher, q, 10).ScoreDocs
	assertFuzzyHits(t, mustStoredFields(t, searcher), hits, 1, "Giga byte")
	mustClose(t, r, w, index_)
}

func TestFuzzyQueryDistanceAsEditsSearching(t *testing.T) {
	index_ := newDirectory()
	w := newRandomIndexWriter(t, index_)
	addFuzzyDoc(t, "foobar", w)
	addFuzzyDoc(t, "test", w)
	addFuzzyDoc(t, "working", w)
	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	mustClose(t, w)

	q := search.NewFuzzyQueryWithMaxEdits(index.NewTerm("field", "fouba"), 2)
	hits := mustSearch(t, searcher, q, 10).ScoreDocs
	assertFuzzyHits(t, mustStoredFields(t, searcher), hits, 1, "foobar")

	q = search.NewFuzzyQueryWithMaxEdits(index.NewTerm("field", "foubara"), 2)
	hits = mustSearch(t, searcher, q, 10).ScoreDocs
	assertFuzzyHits(t, mustStoredFields(t, searcher), hits, 1, "foobar")

	expectThrowsPanic(t, func() {
		search.NewFuzzyQueryWithMaxEdits(index.NewTerm("field", "t"), 3)
	})

	mustClose(t, reader, index_)
}

func TestFuzzyQueryValidation(t *testing.T) {
	expected := expectThrowsPanic(t, func() {
		search.NewFuzzyQueryFull(index.NewTerm("field", "foo"), -1, 0, 1, false)
	})
	if !strings.Contains(expected, "maxEdits") {
		t.Fatalf("unexpected message %q", expected)
	}

	expected = expectThrowsPanic(t, func() {
		search.NewFuzzyQueryFull(
			index.NewTerm("field", "foo"),
			automaton.MaximumSupportedLevenshteinDistance+1,
			0,
			1,
			false)
	})
	if !strings.Contains(expected, "maxEdits must be between") {
		t.Fatalf("unexpected message %q", expected)
	}

	expected = expectThrowsPanic(t, func() {
		search.NewFuzzyQueryFull(index.NewTerm("field", "foo"), 1, -1, 1, false)
	})
	if !strings.Contains(expected, "prefixLength cannot be negative") {
		t.Fatalf("unexpected message %q", expected)
	}

	expected = expectThrowsPanic(t, func() {
		search.NewFuzzyQueryFull(index.NewTerm("field", "foo"), 1, 0, -1, false)
	})
	if !strings.Contains(expected, "maxExpansions must be positive") {
		t.Fatalf("unexpected message %q", expected)
	}

	expected = expectThrowsPanic(t, func() {
		search.NewFuzzyQueryFull(index.NewTerm("field", "foo"), 1, 0, -1, false)
	})
	if !strings.Contains(expected, "maxExpansions must be positive") {
		t.Fatalf("unexpected message %q", expected)
	}
}

// addFuzzyDoc renders the private addDoc(String, RandomIndexWriter).
func addFuzzyDoc(t *testing.T, text string, writer *testindex.RandomIndexWriter) {
	t.Helper()
	doc := newTestDocument(newTextField(t, "field", text, true))
	mustAddDocument(t, writer, doc)
}

// randomSimpleString renders the private randomSimpleString(int).
func randomSimpleString(digits int) string {
	termLength := nextInt(1, 8)
	chars := make([]byte, termLength)
	for i := 0; i < termLength; i++ {
		chars[i] = byte('a' + random().Intn(digits))
	}
	return string(chars)
}

func TestFuzzyQueryRandom(t *testing.T) {
	digits := nextInt(2, 3)
	// underestimated total number of unique terms that randomSimpleString
	// maybe generate, it assumes all terms have a length of 7
	vocabularySize := digits << 7
	numTerms := min(atLeast(100), vocabularySize)
	terms := map[string]struct{}{}
	for len(terms) < numTerms {
		terms[randomSimpleString(digits)] = struct{}{}
	}

	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	for term := range terms {
		doc := newTestDocument(mustStringField(t, "field", term, true))
		mustAddDocument(t, w, doc)
	}
	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)
	iters := atLeast(200)
	for iter := 0; iter < iters; iter++ {
		queryTerm := randomSimpleString(digits)
		prefixLength := random().Intn(len(queryTerm))
		queryPrefix := queryTerm[:prefixLength]

		// we don't look at scores here:
		var expected [3][]fuzzyTermAndScore
		for term := range terms {
			if !strings.HasPrefix(term, queryPrefix) {
				continue
			}
			ed := fuzzyGetDistance(term, queryTerm)
			score := 1 - float32(ed)/float32(min(len(queryTerm), len(term)))
			for ed < 3 {
				expected[ed] = append(expected[ed], fuzzyTermAndScore{term: term, score: score})
				ed++
			}
		}

		for ed := 0; ed < 3; ed++ {
			sort.Slice(expected[ed], func(i, j int) bool { return expected[ed][i].compareTo(expected[ed][j]) < 0 })
			queueSize := nextInt(1, len(terms))
			query := search.NewFuzzyQueryFull(index.NewTerm("field", queryTerm), ed, prefixLength, queueSize, true)
			hits := mustSearch(t, s, query, len(terms))
			actual := map[string]struct{}{}
			storedFields := mustStoredFields(t, s)
			for _, hit := range hits.ScoreDocs {
				doc := storedDocument(t, storedFields, hit.Doc)
				actual[doc.GetString("field")] = struct{}{}
			}
			expectedTop := map[string]struct{}{}
			limit := min(queueSize, len(expected[ed]))
			for i := 0; i < limit; i++ {
				expectedTop[expected[ed][i].term] = struct{}{}
			}

			if !fuzzySetsEqual(actual, expectedTop) {
				var sb strings.Builder
				sb.WriteString("FAILED: query=" + queryTerm)
				sb.WriteString(" ed=" + strconv.Itoa(ed))
				sb.WriteString(" queueSize=" + strconv.Itoa(queueSize))
				sb.WriteString(" vs expected match size=" + strconv.Itoa(len(expected[ed])))
				sb.WriteString(" prefixLength=" + strconv.Itoa(prefixLength) + "\n")

				first := true
				for term := range actual {
					if _, ok := expectedTop[term]; !ok {
						if first {
							sb.WriteString("  these matched but shouldn't:\n")
							first = false
						}
						sb.WriteString("    " + term + "\n")
					}
				}
				first = true
				for term := range expectedTop {
					if _, ok := actual[term]; !ok {
						if first {
							sb.WriteString("  these did not match but should:\n")
							first = false
						}
						sb.WriteString("    " + term + "\n")
					}
				}
				t.Fatal(sb.String())
			}
		}
	}

	mustClose(t, r, dir)
}

func fuzzySetsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// fuzzyTermAndScore renders the private record TermAndScore.
type fuzzyTermAndScore struct {
	term  string
	score float32
}

// compareTo: higher score sorts first, and if scores are tied, lower term
// sorts first.
func (a fuzzyTermAndScore) compareTo(other fuzzyTermAndScore) int {
	if a.score > other.score {
		return -1
	} else if a.score < other.score {
		return 1
	}
	return strings.Compare(a.term, other.term)
}

// fuzzyGetDistance renders the private getDistance(String, String), poached
// from LuceneLevenshteinDistance.java (from suggest module): it supports
// transpositions (treats them as ed=1, not ed=2).
func fuzzyGetDistance(target, other string) int {
	// cheaper to do this up front once
	targetPoints := []rune(target)
	otherPoints := []rune(other)
	n := len(targetPoints)
	m := len(otherPoints)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
	}

	if n == 0 || m == 0 {
		if n == m {
			return 0
		}
		return max(n, m)
	}

	for i := 0; i <= n; i++ {
		d[i][0] = i
	}

	for j := 0; j <= m; j++ {
		d[0][j] = j
	}

	for j := 1; j <= m; j++ {
		tJ := otherPoints[j-1]

		for i := 1; i <= n; i++ {
			cost := 1
			if targetPoints[i-1] == tJ {
				cost = 0
			}
			// minimum of cell to the left+1, to the top+1, diagonally left and up +cost
			d[i][j] = min(min(d[i-1][j]+1, d[i][j-1]+1), d[i-1][j-1]+cost)
			// transposition
			if i > 1 &&
				j > 1 &&
				targetPoints[i-1] == otherPoints[j-2] &&
				targetPoints[i-2] == otherPoints[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+cost)
			}
		}
	}

	return d[n][m]
}

// fuzzyTestVisitor renders the anonymous QueryVisitor of testVisitor: it
// overrides consumeTermsMatching and inherits QueryVisitor's defaults.
type fuzzyTestVisitor struct {
	t       *testing.T
	visited *bool
}

func (v *fuzzyTestVisitor) ConsumeTerms(query search.Query, terms ...*index.Term) {}

func (v *fuzzyTestVisitor) ConsumeTermsMatching(query search.Query, field string, a func() search.ByteRunAutomaton) {
	*v.visited = true
	ra := a()
	assertFuzzyAutomatonMatches(v.t, ra, "blob")
	assertFuzzyAutomatonMatches(v.t, ra, "bolb")
	assertFuzzyAutomatonMatches(v.t, ra, "blobby")
	assertFuzzyAutomatonNoMatches(v.t, ra, "bolbby")
}

func (v *fuzzyTestVisitor) VisitLeaf(query search.Query) {}

func (v *fuzzyTestVisitor) AcceptField(field string) bool { return true }

func (v *fuzzyTestVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	if occur == search.MUST_NOT {
		return search.EmptyQueryVisitor
	}
	return v
}

func TestFuzzyQueryVisitor(t *testing.T) {
	q := search.NewFuzzyQueryWithMaxEdits(index.NewTerm("field", "blob"), 2)
	visited := false
	q.Visit(&fuzzyTestVisitor{t: t, visited: &visited})
	if !visited {
		t.Fatal("consumeTermsMatching was not called")
	}
}

func assertFuzzyAutomatonMatches(t *testing.T, a search.ByteRunAutomaton, text string) {
	t.Helper()
	if !a.Run([]byte(text)) {
		t.Fatalf("automaton should accept %q", text)
	}
}

func assertFuzzyAutomatonNoMatches(t *testing.T, a search.ByteRunAutomaton, text string) {
	t.Helper()
	if a.Run([]byte(text)) {
		t.Fatalf("automaton should reject %q", text)
	}
}
