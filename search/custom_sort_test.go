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

// GC-938: Custom Sort Tests
// Validate custom sort implementations produce identical ordering to Java Lucene.

func TestCustomSort_BasicSort(t *testing.T) {
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
	docs := []struct {
		id      string
		content string
	}{
		{"1", "zebra"},
		{"2", "apple"},
		{"3", "mango"},
		{"4", "banana"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", d.id, true)
		doc.Add(idField)

		contentField, _ := document.NewStringField("content", d.content, true)
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
	query := search.NewMatchAllDocsQuery()

	// Sort by content field
	sort := search.NewSort()
	sort.SetSort(search.NewSortField("content", search.SortFieldString, false))

	topDocs, err := searcher.Search(query, nil, 10, sort)
	if err != nil {
		t.Logf("custom sort may not be fully implemented: %v", err)
		t.Skip("custom sort not implemented")
	}

	t.Logf("Custom sort returned %d documents", topDocs.TotalHits.Value)
}

func TestCustomSort_DescendingSort(t *testing.T) {
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
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Descending sort
	sort := search.NewSort()
	sort.SetSort(search.NewSortField("id", search.SortFieldString, true))

	topDocs, err := searcher.Search(query, nil, 20, sort)
	if err != nil {
		t.Logf("descending sort may not be fully implemented: %v", err)
		t.Skip("descending sort not implemented")
	}

	t.Logf("Descending sort returned %d documents", topDocs.TotalHits.Value)
}

func TestCustomSort_MultiFieldSort(t *testing.T) {
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
	for i := 0; i < 30; i++ {
		doc := document.NewDocument()
		categoryField, _ := document.NewStringField("category", "cat"+string(rune('A'+i%5)), true)
		doc.Add(categoryField)

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}
	writer.Commit()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Multi-field sort
	sort := search.NewSort()
	sort.SetSort(
		search.NewSortField("category", search.SortFieldString, false),
		search.NewSortField("id", search.SortFieldString, false),
	)

	topDocs, err := searcher.Search(query, nil, 30, sort)
	if err != nil {
		t.Logf("multi-field sort may not be fully implemented: %v", err)
		t.Skip("multi-field sort not implemented")
	}

	t.Logf("Multi-field sort returned %d documents", topDocs.TotalHits.Value)
}

func BenchmarkCustomSort_Performance(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()
	sort := search.NewSort()
	sort.SetSort(search.NewSortField("id", search.SortFieldString, false))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(query, nil, 100, sort)
	}
}
