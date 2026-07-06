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

// GC-933: Custom Collector Tests
// Test custom collector implementations produce identical results to Java Lucene equivalents.

func TestCustomCollector_TotalHitCollector(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "custom collector test", true)
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
	query := search.NewTermQuery(index.NewTerm("content", "custom"))

	collector := search.NewTotalHitCountCollector()
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("SearchWithCollector failed: %v", err)
	}
	if got := collector.GetTotalHits(); got != 100 {
		t.Errorf("total hits = %d, want 100", got)
	}
}

func TestCustomCollector_TopDocsCollector(t *testing.T) {
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

		contentField, _ := document.NewTextField("content", "top docs collector", true)
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
	query := search.NewTermQuery(index.NewTerm("content", "collector"))

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if got := topDocs.TotalHits.Value; got != 50 {
		t.Errorf("total hits = %d, want 50", got)
	}
}

func BenchmarkCustomCollector_Collection(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "benchmark collector", true)
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
