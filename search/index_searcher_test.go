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

func TestIndexSearcherBasic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, err := document.NewTextField("content", "hello world", true)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit, got %d", topDocs.TotalHits.Value)
	}
}

func TestIndexSearcherMultiSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add docs to first segment
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		field, _ := document.NewTextField("content", "hello", true)
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.Commit()

	// Add docs to second segment
	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		field, _ := document.NewTextField("content", "world", true)
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.MaxDoc() != 5 {
		t.Errorf("Expected 5 docs, got %d", reader.MaxDoc())
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 5 {
		t.Errorf("Expected 5 hits, got %d", topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) != 5 {
		t.Errorf("Expected 5 score docs, got %d", len(topDocs.ScoreDocs))
	}

	// Check doc IDs (0, 1, 2, 3, 4)
	seen := make(map[int]bool)
	for _, sd := range topDocs.ScoreDocs {
		seen[sd.Doc] = true
	}
	for i := 0; i < 5; i++ {
		if !seen[i] {
			t.Errorf("Doc ID %d not found in results", i)
		}
	}
}
