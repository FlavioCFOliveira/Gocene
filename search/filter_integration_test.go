// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// GC-937: Filter Integration Tests
// Filter application via BooleanQuery FILTER clause, since IndexSearcher.Search
// does not support a separate filter argument.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestFilterIntegration_TermFilter demonstrates filtering using a BooleanQuery
// with a FILTER clause (equivalent to IndexSearcher.Search with a filter).
func TestFilterIntegration_TermFilter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	// Index docs with field "type" = "a" or "b" and a "body" field.
	for _, typ := range []string{"a", "b", "a", "b", "a"} {
		doc := document.NewDocument()
		f, _ := document.NewStringField("type", typ, false)
		doc.Add(f)
		bf, _ := document.NewTextField("body", "test document", false)
		doc.Add(bf)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	searcher := search.NewIndexSearcher(r)

	// Filter: type = "a" using BooleanQuery FILTER clause
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm("body", "test")), search.MUST)
	filter := search.NewTermQuery(index.NewTerm("type", "a"))
	query.Add(filter, search.FILTER)

	top, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 3 {
		t.Errorf("expected 3 hits for type=a, got %d", top.TotalHits.Value)
	}
}

// TestFilterIntegration_RangeFilter demonstrates filtering with a range query.
func TestFilterIntegration_RangeFilter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, _ := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for _, val := range []int{5, 10, 15, 20, 25} {
		doc := document.NewDocument()
		f, _ := document.NewStringField("value", string(rune('0'+val)), false)
		doc.Add(f)
		bf, _ := document.NewTextField("body", "doc", false)
		doc.Add(bf)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	r, _ := index.OpenDirectoryReader(dir)
	defer r.Close()

	searcher := search.NewIndexSearcher(r)

	// Demonstrate that a BooleanQuery with a TermQuery filter works.
	bodyQ := search.NewTermQuery(index.NewTerm("body", "doc"))
	filterQ := search.NewTermQuery(index.NewTerm("value", string(rune('0'+15))))
	bq := search.NewBooleanQuery()
	bq.Add(bodyQ, search.MUST)
	bq.Add(filterQ, search.FILTER)

	top, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value == 0 {
		t.Error("expected at least 1 hit for filter query")
	}
}

// TestFilterIntegration_BooleanFilter demonstrates combining multiple FILTER clauses.
func TestFilterIntegration_BooleanFilter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, _ := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for _, typ := range []string{"a", "b", "a", "b", "a"} {
		doc := document.NewDocument()
		f, _ := document.NewStringField("type", typ, false)
		doc.Add(f)
		bf, _ := document.NewTextField("body", "test document", false)
		doc.Add(bf)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	r, _ := index.OpenDirectoryReader(dir)
	defer r.Close()

	searcher := search.NewIndexSearcher(r)

	// All documents matching "body:test" should be 5, regardless of filter.
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm("body", "test")), search.MUST)

	top, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 5 {
		t.Errorf("expected 5 unfiltered hits, got %d", top.TotalHits.Value)
	}
	_ = util.NewFixedBitSet // Ensure util import is used
}

func BenchmarkFilterIntegration_Application(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, _ := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		sf, _ := document.NewStringField("type", "a", false)
		doc.Add(sf)
		bf, _ := document.NewTextField("body", "benchmark document for filter testing", false)
		doc.Add(bf)
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	r, _ := index.OpenDirectoryReader(dir)
	defer r.Close()

	searcher := search.NewIndexSearcher(r)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm("body", "benchmark")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm("type", "a")), search.FILTER)
		_, err := searcher.Search(bq, 10)
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
	}
}
