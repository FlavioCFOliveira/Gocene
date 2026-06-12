// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestCustomSearcherSort.java
//
// The upstream test builds an index where:
//   - documents with i%5 != 0 carry a SortedDocValuesField "publicationDate_"
//     holding a (random) distinct date string; the rest leave it missing;
//   - documents with i%7 == 0 carry an indexed text field content="test";
//   - every document carries a stored string field mandant = (i%3).
//
// A CustomSearcher subclasses IndexSearcher to AND every incoming query with a
// mandant:<switcher> clause before delegating. matchHits then runs the
// content:test query both rank-sorted (search(q, MAX)) and field-sorted
// (search(q, MAX, sort) with sort = publicationDate_ STRING then FIELD_SCORE)
// and asserts the two result sets contain exactly the same set of Lucene doc
// IDs (sorting reorders but does not add or drop hits), with no duplicates in
// either set.
//
// This is a faithful port driving the real IndexWriter + IndexSearcher +
// field-sort path. Go has no method override, so CustomSearcher is modelled as a
// struct wrapping an *IndexSearcher that prepends the mandant clause, exactly
// reproducing the override's behaviour.
//
// Deviations, documented per the binary-compatibility mandate:
//   - The reference fills publicationDate_ with random DateTools day strings; the
//     date content is irrelevant to the assertion (only the matched doc set is
//     checked), so this port uses deterministic distinct strings. The
//     SortedDocValues presence/absence pattern (i%5) is preserved exactly.
//   - The reference uses RandomIndexWriter + a randomized INDEX_SIZE >= 2000;
//     this port uses the production IndexWriter and the reduced fixed size the
//     reference's own comment cites (2000), which is large enough to exercise the
//     sort/rank consistency the test guards.

package search_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so postings / doc-values are flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

const customSearcherSortIndexSize = 2000

// customSearcher is the Go counterpart of the reference's CustomSearcher: it
// wraps an IndexSearcher and ANDs every query with mandant:<switcher> before
// delegating, mirroring the two search() overrides.
type customSearcher struct {
	searcher *search.IndexSearcher
	switcher int
}

func newCustomSearcher(searcher *search.IndexSearcher, switcher int) *customSearcher {
	return &customSearcher{searcher: searcher, switcher: switcher}
}

// wrap builds the mandant-constrained BooleanQuery the overrides build.
func (c *customSearcher) wrap(query search.Query) search.Query {
	bq := search.NewBooleanQuery()
	bq.Add(query, search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("mandant", fmt.Sprintf("%d", c.switcher))), search.MUST)
	return bq
}

// search mirrors CustomSearcher.search(query, nDocs).
func (c *customSearcher) search(query search.Query, nDocs int) (*search.TopDocs, error) {
	return c.searcher.Search(c.wrap(query), nDocs)
}

// searchWithSort mirrors CustomSearcher.search(query, nDocs, sort).
func (c *customSearcher) searchWithSort(query search.Query, nDocs int, srt *search.Sort) (*search.TopFieldDocs, error) {
	return c.searcher.SearchWithSort(c.wrap(query), nDocs, srt)
}

// buildCustomSearcherSortIndex indexes the fixture and returns a reader.
func buildCustomSearcherSortIndex(t *testing.T) (*index.DirectoryReader, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < customSearcherSortIndexSize; i++ {
		doc := document.NewDocument()
		if i%5 != 0 { // some documents must not have an entry in the first sort field
			dv, derr := document.NewSortedDocValuesField("publicationDate_", []byte(fmt.Sprintf("d%08d", i)))
			if derr != nil {
				t.Fatalf("doc %d publicationDate_: %v", i, derr)
			}
			doc.Add(dv)
		}
		if i%7 == 0 { // some documents to match the query
			cf, cerr := document.NewTextField("content", "test", true)
			if cerr != nil {
				t.Fatalf("doc %d content: %v", i, cerr)
			}
			doc.Add(cf)
		}
		mf, merr := document.NewStringField("mandant", fmt.Sprintf("%d", i%3), true)
		if merr != nil {
			t.Fatalf("doc %d mandant: %v", i, merr)
		}
		doc.Add(mf)
		if aerr := w.AddDocument(doc); aerr != nil {
			t.Fatalf("doc %d AddDocument: %v", i, aerr)
		}
	}
	if cerr := w.Commit(); cerr != nil {
		t.Fatalf("Commit: %v", cerr)
	}
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("writer.Close: %v", cerr)
	}
	reader, rerr := index.OpenDirectoryReader(dir)
	if rerr != nil {
		t.Fatalf("OpenDirectoryReader: %v", rerr)
	}
	return reader, dir
}

// customSearcherSortDocs collects the doc IDs from a TopDocs, asserting there
// are no duplicates (the reference's checkHits helper).
func customSearcherSortDocs(t *testing.T, prefix string, docs []*search.ScoreDoc) map[int]struct{} {
	t.Helper()
	seen := make(map[int]struct{}, len(docs))
	for i, sd := range docs {
		if _, dup := seen[sd.Doc]; dup {
			t.Errorf("%s duplicate doc ID at hit %d: %d", prefix, i, sd.Doc)
			continue
		}
		seen[sd.Doc] = struct{}{}
	}
	return seen
}

// matchHits ports TestCustomSearcherSort.matchHits: the rank-sorted and
// field-sorted searches must return exactly the same set of doc IDs.
func matchCustomSearcherHits(t *testing.T, c *customSearcher, srt *search.Sort) {
	t.Helper()
	query := search.NewTermQuery(index.NewTerm("content", "test"))

	// Query without sorting first.
	byRank, err := c.search(query, customSearcherSortIndexSize)
	if err != nil {
		t.Fatalf("rank search: %v", err)
	}
	rankSet := customSearcherSortDocs(t, "Sort by rank:", byRank.ScoreDocs)

	// Now query using the sort criteria.
	bySort, err := c.searchWithSort(query, customSearcherSortIndexSize, srt)
	if err != nil {
		t.Fatalf("sorted search: %v", err)
	}
	sortSet := customSearcherSortDocs(t, "Sort by custom criteria:", bySort.ScoreDocs)

	// Both sets must be identical: every sorted hit appears in the rank set and
	// the two sets have the same size, so removing them all empties the map.
	if len(rankSet) != len(sortSet) {
		t.Fatalf("rank set size %d != sort set size %d", len(rankSet), len(sortSet))
	}
	missing := make([]int, 0)
	for doc := range sortSet {
		if _, ok := rankSet[doc]; !ok {
			missing = append(missing, doc)
		}
	}
	if len(missing) != 0 {
		sort.Ints(missing)
		t.Fatalf("couldn't match %d sorted hits against the rank set: %v", len(missing), missing)
	}
}

// customSearcherSort builds the (publicationDate_ STRING, FIELD_SCORE) sort.
func customSearcherSort() *search.Sort {
	return search.NewSort(
		search.NewSortField("publicationDate_", search.SortFieldTypeString),
		search.NewSortField("", search.SortFieldTypeScore),
	)
}

// TestCustomSearcherSort_FieldSortCustomSearcher ports
// testFieldSortCustomSearcher.
func TestCustomSearcherSort_FieldSortCustomSearcher(t *testing.T) {
	reader, dir := buildCustomSearcherSortIndex(t)
	defer func() { _ = reader.Close(); _ = dir.Close() }()

	c := newCustomSearcher(search.NewIndexSearcher(reader), 2)
	matchCustomSearcherHits(t, c, customSearcherSort())
}

// TestCustomSearcherSort_FieldSortSingleSearcher ports
// testFieldSortSingleSearcher (identical fixture and assertion in the
// reference, differing only in the no-longer-existing MultiSearcher wrapping).
func TestCustomSearcherSort_FieldSortSingleSearcher(t *testing.T) {
	reader, dir := buildCustomSearcherSortIndex(t)
	defer func() { _ = reader.Close(); _ = dir.Close() }()

	c := newCustomSearcher(search.NewIndexSearcher(reader), 2)
	matchCustomSearcherHits(t, c, customSearcherSort())
}