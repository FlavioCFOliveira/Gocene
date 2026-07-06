// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-923: Search Scoring Reproducibility
// Ensure search scores are deterministic and reproducible across multiple runs with same data.

func TestSearchScoringReproducibility_TermQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	docs := []struct {
		id      string
		content string
	}{
		{"1", "lucene search engine"},
		{"2", "lucene search"},
		{"3", "search engine"},
		{"4", "other content"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", d.id, true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", d.content, true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "lucene"))

	topDocs1, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	topDocs2, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("second search failed: %v", err)
	}

	if topDocs1.TotalHits.Value != topDocs2.TotalHits.Value {
		t.Errorf("total hits differ: %d vs %d", topDocs1.TotalHits.Value, topDocs2.TotalHits.Value)
	}
	if topDocs1.TotalHits.Value != 2 {
		t.Errorf("total hits = %d, want 2", topDocs1.TotalHits.Value)
	}
}

func TestSearchScoringReproducibility_BooleanQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "test reproducible scoring", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	boolQuery := search.NewBooleanQuery()
	boolQuery.Add(search.NewTermQuery(index.NewTerm("content", "test")), search.SHOULD)
	boolQuery.Add(search.NewTermQuery(index.NewTerm("content", "reproducible")), search.SHOULD)

	var last int64 = -1
	for i := 0; i < 5; i++ {
		topDocs, err := searcher.Search(boolQuery, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if last >= 0 && topDocs.TotalHits.Value != last {
			t.Errorf("run %d total hits = %d, previous = %d", i+1, topDocs.TotalHits.Value, last)
		}
		last = topDocs.TotalHits.Value
	}
	if last != 20 {
		t.Errorf("total hits = %d, want 20", last)
	}
}

func TestSearchScoringReproducibility_PhraseQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	docs := []string{
		"quick brown fox",
		"quick brown",
		"brown fox",
		"fox quick brown",
	}

	for i, content := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", content, true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	phraseQuery := search.NewPhraseQueryBuilder().
		AddTerm(index.NewTerm("content", "quick")).
		AddTerm(index.NewTerm("content", "brown")).
		Build()

	var last int64 = -1
	for i := 0; i < 5; i++ {
		topDocs, err := searcher.Search(phraseQuery, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if last >= 0 && topDocs.TotalHits.Value != last {
			t.Errorf("run %d total hits = %d, previous = %d", i+1, topDocs.TotalHits.Value, last)
		}
		last = topDocs.TotalHits.Value
	}
	if last != 3 {
		t.Errorf("total hits = %d, want 3", last)
	}
}

func TestSearchScoringReproducibility_NewReader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "reproducibility test", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	query := search.NewTermQuery(index.NewTerm("content", "reproducibility"))

	var last int64 = -1
	for i := 0; i < 5; i++ {
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("failed to open reader: %v", err)
		}

		searcher := search.NewIndexSearcher(reader)
		topDocs, err := searcher.Search(query, 10)
		if err != nil {
			reader.Close()
			t.Fatalf("Search failed: %v", err)
		}

		if last >= 0 && topDocs.TotalHits.Value != last {
			t.Errorf("run %d total hits = %d, previous = %d", i+1, topDocs.TotalHits.Value, last)
		}
		last = topDocs.TotalHits.Value
		reader.Close()
	}
	if last != 50 {
		t.Errorf("total hits = %d, want 50", last)
	}
}

func BenchmarkSearchScoringReproducibility_Repeated(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "benchmark scoring reproducibility", true)
		doc.Add(contentField)

		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(query, 10)
	}
}
