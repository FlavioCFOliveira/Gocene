// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSearchAfter.java
//
// testQueries paginates several query shapes (MatchAllDocsQuery, a TermQuery,
// and a SHOULD BooleanQuery) over a multi-segment index using searchAfter with
// a null sort (score-based pagination), and asserts the concatenation of pages
// is identical (docID and score) to a single unpaginated search. This is the
// sort==null branch of TestSearchAfter.assertQuery, repeated over several
// page sizes as in testQueries.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// searchAfterIntToEnglish maps i to a space-separated English digit string, so
// TermQuery("english","one") matches every doc whose decimal contains a 1
// (mirroring English.intToEnglish coverage closely enough for the consistency
// assertion, which only requires a non-trivial matching subset).
func searchAfterIntToEnglish(n int) string {
	ones := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	if n == 0 {
		return ones[0]
	}
	if n < 0 {
		n = -n
	}
	var words []string
	for n > 0 {
		words = append([]string{ones[n%10]}, words...)
		n /= 10
	}
	out := words[0]
	for i := 1; i < len(words); i++ {
		out += " " + words[i]
	}
	return out
}

// TestSearchAfter_Queries ports TestSearchAfter.testQueries (sort==null cases).
func TestSearchAfter_Queries(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	const numDocs = 200
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		ef, err := document.NewTextField("english", searchAfterIntToEnglish(i), false)
		if err != nil {
			t.Fatalf("NewTextField english: %v", err)
		}
		doc.Add(ef)
		oe := "odd"
		if i%2 == 0 {
			oe = "even"
		}
		of, err := document.NewTextField("oddeven", oe, false)
		if err != nil {
			t.Fatalf("NewTextField oddeven: %v", err)
		}
		doc.Add(of)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		// Force multiple segments.
		if i > 0 && i%50 == 0 {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)

	// MatchAllDocsQuery, TermQuery(english:one), and a SHOULD BooleanQuery
	// (english:one OR oddeven:even).
	one := search.NewTermQuery(index.NewTerm("english", "one"))
	even := search.NewTermQuery(index.NewTerm("oddeven", "even"))
	boolQuery := search.NewBooleanQuery()
	boolQuery.Add(one, search.SHOULD)
	boolQuery.Add(even, search.SHOULD)

	queries := []search.Query{
		search.NewMatchAllDocsQuery(),
		search.NewTermQuery(index.NewTerm("english", "one")),
		boolQuery,
	}

	pageSizes := []int{1, 3, 5, 17, 50}
	for qi, q := range queries {
		for _, pageSize := range pageSizes {
			assertSearchAfterPaginationConsistent(t, qi, searcher, q, pageSize, numDocs)
		}
	}
}

// assertSearchAfterPaginationConsistent ports the sort==null branch of
// TestSearchAfter.assertQuery: the concatenation of searchAfter pages must
// equal a single unpaginated top-N search (same docID and score per hit).
func assertSearchAfterPaginationConsistent(t *testing.T, qi int, searcher *search.IndexSearcher, q search.Query, pageSize, maxDoc int) {
	t.Helper()

	all, err := searcher.Search(q, maxDoc)
	if err != nil {
		t.Fatalf("q%d Search(all): %v", qi, err)
	}

	var paged []*search.ScoreDoc
	var after *search.ScoreDoc
	for {
		page, err := searcher.SearchAfter(after, q, pageSize)
		if err != nil {
			t.Fatalf("q%d SearchAfter(pageSize=%d): %v", qi, pageSize, err)
		}
		if len(page.ScoreDocs) == 0 {
			break
		}
		paged = append(paged, page.ScoreDocs...)
		after = page.ScoreDocs[len(page.ScoreDocs)-1]
	}

	if len(paged) != len(all.ScoreDocs) {
		t.Fatalf("q%d pageSize=%d: paged %d hits, unpaginated %d", qi, pageSize, len(paged), len(all.ScoreDocs))
	}
	for i := range all.ScoreDocs {
		if paged[i].Doc != all.ScoreDocs[i].Doc {
			t.Fatalf("q%d pageSize=%d hit %d: docID paged=%d all=%d", qi, pageSize, i, paged[i].Doc, all.ScoreDocs[i].Doc)
		}
		if paged[i].Score != all.ScoreDocs[i].Score {
			t.Fatalf("q%d pageSize=%d hit %d: score paged=%v all=%v", qi, pageSize, i, paged[i].Score, all.ScoreDocs[i].Score)
		}
	}
}
