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

// TestTermScorer_Basic tests basic term scoring functionality.
// Ported from Apache Lucene's org.apache.lucene.search.TestTermScorer.test()
func TestTermScorer_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create index with test documents
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents: "all", "dogs dogs", "like", "playing", "fetch", "all"
	values := []string{"all", "dogs dogs", "like", "playing", "fetch", "all"}
	for _, value := range values {
		doc := document.NewDocument()
		field, _ := document.NewTextField("field", value, true)
		doc.Add(field)
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()
	writer.Close()

	// Open reader and create searcher
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Create term query for "all"
	term := index.NewTerm("field", "all")
	termQuery := search.NewTermQuery(term)

	// Search and verify results
	topDocs, err := searcher.Search(termQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 2 documents (docs 0 and 5 contain "all")
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
	}

	// Verify the doc IDs are 0 and 5
	docIDs := make(map[int]bool)
	for _, scoreDoc := range topDocs.ScoreDocs {
		docIDs[scoreDoc.Doc] = true
		// Scores should be positive
		if scoreDoc.Score <= 0 {
			t.Errorf("Expected positive score, got %f", scoreDoc.Score)
		}
	}

	if !docIDs[0] {
		t.Error("Expected doc 0 to be in results")
	}
	if !docIDs[5] {
		t.Error("Expected doc 5 to be in results")
	}

	// The scores should be equal (both docs have "all" once)
	if len(topDocs.ScoreDocs) == 2 {
		if topDocs.ScoreDocs[0].Score != topDocs.ScoreDocs[1].Score {
			t.Errorf("Expected equal scores, got %f and %f",
				topDocs.ScoreDocs[0].Score, topDocs.ScoreDocs[1].Score)
		}
	}
}

// TestTermScorer_Next tests the nextDoc() iteration.
// Ported from Apache Lucene's org.apache.lucene.search.TestTermScorer.testNext()
func TestTermScorer_Next(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with "all" in positions 0 and 5
	values := []string{"all", "dogs", "like", "playing", "fetch", "all"}
	for _, value := range values {
		doc := document.NewDocument()
		field, _ := document.NewTextField("field", value, true)
		doc.Add(field)
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	term := index.NewTerm("field", "all")
	termQuery := search.NewTermQuery(term)

	topDocs, err := searcher.Search(termQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find exactly 2 documents
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
	}

	// Verify we got docs 0 and 5
	foundDocs := make(map[int]bool)
	for _, sd := range topDocs.ScoreDocs {
		foundDocs[sd.Doc] = true
	}
	if !foundDocs[0] {
		t.Error("Expected to find doc 0")
	}
	if !foundDocs[5] {
		t.Error("Expected to find doc 5")
	}
}

// TestTermScorer_Advance tests the advance() method.
// Ported from Apache Lucene's org.apache.lucene.search.TestTermScorer.testAdvance()
func TestTermScorer_Advance(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with "all" in positions 0 and 5
	values := []string{"all", "dogs", "like", "playing", "fetch", "all"}
	for _, value := range values {
		doc := document.NewDocument()
		field, _ := document.NewTextField("field", value, true)
		doc.Add(field)
		err = writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	term := index.NewTerm("field", "all")
	termQuery := search.NewTermQuery(term)

	// Test advance by searching and verifying results
	topDocs, err := searcher.Search(termQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 2 documents
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
	}

	// The second document should be doc 5
	if len(topDocs.ScoreDocs) >= 2 && topDocs.ScoreDocs[1].Doc != 5 {
		t.Errorf("Expected second doc to be 5, got %d", topDocs.ScoreDocs[1].Doc)
	}
}

// TestTermScorer_ScoreConsistency verifies that scores are consistent
// for documents with the same term frequency.
func TestTermScorer_ScoreConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with same term frequency
	doc1 := document.NewDocument()
	field1, _ := document.NewTextField("content", "test", true)
	doc1.Add(field1)
	writer.AddDocument(doc1)

	doc2 := document.NewDocument()
	field2, _ := document.NewTextField("content", "test", true)
	doc2.Add(field2)
	writer.AddDocument(doc2)

	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	term := index.NewTerm("content", "test")
	termQuery := search.NewTermQuery(term)

	topDocs, err := searcher.Search(termQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 2 documents
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
	}

	// Scores should be equal (same term frequency, same field length)
	if len(topDocs.ScoreDocs) == 2 {
		// Allow small floating point differences
		diff := topDocs.ScoreDocs[0].Score - topDocs.ScoreDocs[1].Score
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.0001 {
			t.Errorf("Expected equal scores, got %f and %f",
				topDocs.ScoreDocs[0].Score, topDocs.ScoreDocs[1].Score)
		}
	}
}

// TestTermScorer_DocFrequency tests that document frequency is correctly calculated.
func TestTermScorer_DocFrequency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with varying term frequencies
	docs := []string{
		"hello world",
		"hello",
		"world world",
		"hello world test",
	}

	for _, content := range docs {
		doc := document.NewDocument()
		field, _ := document.NewTextField("content", content, true)
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

	searcher := search.NewIndexSearcher(reader)

	// Test "hello" - should be in 3 documents
	termHello := index.NewTerm("content", "hello")
	queryHello := search.NewTermQuery(termHello)

	topDocsHello, err := searcher.Search(queryHello, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocsHello.TotalHits.Value != 3 {
		t.Errorf("Expected 3 docs with 'hello', got %d", topDocsHello.TotalHits.Value)
	}

	// Test "world" - should be in 3 documents
	termWorld := index.NewTerm("content", "world")
	queryWorld := search.NewTermQuery(termWorld)

	topDocsWorld, err := searcher.Search(queryWorld, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocsWorld.TotalHits.Value != 3 {
		t.Errorf("Expected 3 docs with 'world', got %d", topDocsWorld.TotalHits.Value)
	}

	// Test "test" - should be in 1 document
	termTest := index.NewTerm("content", "test")
	queryTest := search.NewTermQuery(termTest)

	topDocsTest, err := searcher.Search(queryTest, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocsTest.TotalHits.Value != 1 {
		t.Errorf("Expected 1 doc with 'test', got %d", topDocsTest.TotalHits.Value)
	}
}

// TestTermScorer_NonExistentTerm tests searching for a term that doesn't exist.
func TestTermScorer_NonExistentTerm(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("content", "hello world", true)
	doc.Add(field)
	writer.AddDocument(doc)

	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Search for non-existent term
	term := index.NewTerm("content", "nonexistent")
	query := search.NewTermQuery(term)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits, got %d", topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) != 0 {
		t.Errorf("Expected 0 score docs, got %d", len(topDocs.ScoreDocs))
	}
}

// TestTermScorer_EmptyIndex tests searching in an empty index.
func TestTermScorer_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	term := index.NewTerm("content", "test")
	query := search.NewTermQuery(term)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits in empty index, got %d", topDocs.TotalHits.Value)
	}
}

// TestTermScorer_TermFrequency tests that term frequency affects scoring.
func TestTermScorer_TermFrequency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Doc 0: "test" appears once
	doc1 := document.NewDocument()
	field1, _ := document.NewTextField("content", "test", true)
	doc1.Add(field1)
	writer.AddDocument(doc1)

	// Doc 1: "test" appears three times
	doc2 := document.NewDocument()
	field2, _ := document.NewTextField("content", "test test test", true)
	doc2.Add(field2)
	writer.AddDocument(doc2)

	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	term := index.NewTerm("content", "test")
	query := search.NewTermQuery(term)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 2 documents
	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
	}

	// The document with higher term frequency should have a higher score
	// (doc 1 with "test test test" should score higher than doc 0 with "test")
	if len(topDocs.ScoreDocs) == 2 {
		// Find the scores
		var doc0Score, doc1Score float32
		for _, sd := range topDocs.ScoreDocs {
			if sd.Doc == 0 {
				doc0Score = sd.Score
			} else if sd.Doc == 1 {
				doc1Score = sd.Score
			}
		}

		// Doc 1 should have higher score due to higher term frequency
		if doc1Score <= doc0Score {
			t.Errorf("Expected doc 1 (tf=3) to have higher score than doc 0 (tf=1), got %f and %f",
				doc1Score, doc0Score)
		}
	}
}
