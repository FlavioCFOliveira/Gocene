// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBooleanQueryVisitSubscorers.java
//
// Simplified tests that verify basic BooleanQuery construction and search
// over a simple three-document index. The full scorer-tree traversal tests
// (freqCollector / scorerSummaries) are deferred until the Scorer/Scorable
// bridge lands (Gocene's composite scorers do not yet expose GetChildren).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	visitF1 = "title"
	visitF2 = "body"
)

// visitSubscorersIndex builds the three-document corpus used by the suite.
func visitSubscorersIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(v1, v2 string) {
		doc := document.NewDocument()
		f1, e1 := document.NewTextField(visitF1, v1, true)
		if e1 != nil {
			t.Fatalf("NewTextField(title): %v", e1)
		}
		f2, e2 := document.NewTextField(visitF2, v2, true)
		if e2 != nil {
			t.Fatalf("NewTextField(body): %v", e2)
		}
		doc.Add(f1)
		doc.Add(f2)
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
	add("lucene", "lucene is a very popular search engine library")
	add("solr", "solr is a very popular search server and is using lucene")
	add("nutch", "nutch is an internet search engine with web crawler and is using lucene and hadoop")
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(reader), func() {
		_ = reader.Close()
		_ = dir.Close()
	}
}

// visitTerm is a TermQuery on a named field.
func visitTerm(field, text string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(field, text))
}

// TestBooleanQueryVisitSubscorers_Disjunctions verifies a SHOULD-only query
// matches the correct documents.
func TestBooleanQueryVisitSubscorers_Disjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "search"), search.SHOULD)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// All three docs should match (each contains at least one of the terms).
	if top.TotalHits.Value != 3 {
		t.Errorf("totalHits = %d, want 3", top.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_NestedDisjunctions verifies nested
// SHOULD-only queries match the correct documents.
func TestBooleanQueryVisitSubscorers_NestedDisjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq2 := search.NewBooleanQuery()
	bq2.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq2.Add(visitTerm(visitF2, "search"), search.SHOULD)
	bq.Add(bq2, search.SHOULD)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 3 {
		t.Errorf("totalHits = %d, want 3", top.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_Conjunctions verifies a MUST query matches
// only documents containing all required terms.
func TestBooleanQueryVisitSubscorers_Conjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF2, "lucene"), search.MUST)
	bq.Add(visitTerm(visitF2, "is"), search.MUST)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// All three docs contain "is" and "lucene" in body.
	if top.TotalHits.Value != 3 {
		t.Errorf("totalHits = %d, want 3", top.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_DisjunctionMatches verifies a disjunction
// with a mix of TermQuery and PhraseQuery produces correct results.
func TestBooleanQueryVisitSubscorers_DisjunctionMatches(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq1 := search.NewBooleanQuery()
	bq1.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq1.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "engine"), search.SHOULD)

	top1, err := searcher.Search(bq1, 10)
	if err != nil {
		t.Fatalf("Search bq1: %v", err)
	}
	// Doc 0 has "lucene" in title and "search engine" in body.
	// Doc 2 has "search engine" in body.
	if top1.TotalHits.Value < 2 {
		t.Errorf("bq1 totalHits = %d, want >= 2", top1.TotalHits.Value)
	}

	bq2 := search.NewBooleanQuery()
	bq2.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq2.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "library"), search.SHOULD)

	top2, err := searcher.Search(bq2, 10)
	if err != nil {
		t.Fatalf("Search bq2: %v", err)
	}
	// "search library" only appears in doc 0.
	if top2.TotalHits.Value != 1 {
		t.Errorf("bq2 totalHits = %d, want 1", top2.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_MinShouldMatchMatches verifies a disjunction
// with minShouldMatch=2 produces correct results.
func TestBooleanQueryVisitSubscorers_MinShouldMatchMatches(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "library"), search.SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Doc 0 has "lucene" (title) + "lucene" (body) + "search library" (body)
	// = at least 2 matching clauses.
	if top.TotalHits.Value != 1 {
		t.Errorf("totalHits = %d, want 1", top.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_GetChildrenMinShouldMatchSumScorer
// verifies a BooleanQuery with minShouldMatch and a MUST clause works.
func TestBooleanQueryVisitSubscorers_GetChildrenMinShouldMatchSumScorer(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF2, "nutch"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "web"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "crawler"), search.SHOULD)
	bq.SetMinimumNumberShouldMatch(2)
	bq.Add(search.NewMatchAllDocsQuery(), search.MUST)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Only doc 2 ("nutch is an internet search engine with web crawler ...")
	// has all three terms in body, so at least 2 should match.
	if top.TotalHits.Value != 1 {
		t.Errorf("totalHits = %d, want 1", top.TotalHits.Value)
	}
}

// TestBooleanQueryVisitSubscorers_GetChildrenBoosterScorer verifies a simple
// disjunction returns one hit.
func TestBooleanQueryVisitSubscorers_GetChildrenBoosterScorer(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF2, "nutch"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "miss"), search.SHOULD)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("totalHits = %d, want 1", top.TotalHits.Value)
	}
}
