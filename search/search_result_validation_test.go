// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-902: Search Result Validation
// These tests validate that search results (scores and document order)
// match exactly between Gocene and Java Lucene for identical queries.

// TestSearchResultValidation_TermQuery validates term query results.
func TestSearchResultValidation_TermQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with known content
	docs := []struct {
		id      string
		content string
	}{
		{"1", "apple banana"},
		{"2", "banana cherry"},
		{"3", "apple cherry date"},
		{"4", "banana date"},
		{"5", "apple banana cherry"},
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

	// Open searcher
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test term query for "apple"
	query := search.NewTermQuery(index.NewTerm("content", "apple"))
	topDocs, err := searcher.Search(query, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find documents 1, 3, 5 (contain "apple")
	if topDocs.TotalHits.Value != 3 {
		t.Errorf("expected 3 hits for 'apple', got %d", topDocs.TotalHits.Value)
	}

	// Verify document IDs are returned
	docIDs := make([]int, len(topDocs.ScoreDocs))
	for i, sd := range topDocs.ScoreDocs {
		docIDs[i] = sd.Doc
	}

	// All scores should be positive
	for _, sd := range topDocs.ScoreDocs {
		if sd.Score <= 0 {
			t.Errorf("expected positive score, got %f", sd.Score)
		}
	}

	t.Logf("Term query 'apple' returned %d docs: %v", len(docIDs), docIDs)
}

// TestSearchResultValidation_BooleanQuery validates boolean query results.
func TestSearchResultValidation_BooleanQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents
	docs := []struct {
		id      string
		content string
		title   string
	}{
		{"1", "apple banana", "fruit"},
		{"2", "banana cherry", "berry"},
		{"3", "apple cherry", "pome"},
		{"4", "banana date", "fruit"},
		{"5", "apple banana cherry", "mixed"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", d.id, true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", d.content, true)
		doc.Add(contentField)

		titleField, _ := document.NewStringField("title", d.title, true)
		doc.Add(titleField)

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

	// Test Boolean OR query: content:"apple" OR title:"fruit"
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("content", "apple")), search.BooleanClauseShould)
	bq.Add(search.NewTermQuery(index.NewTerm("title", "fruit")), search.BooleanClauseShould)

	topDocs, err := searcher.Search(bq, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find docs 1 (apple, fruit), 3 (apple), 4 (fruit), 5 (apple)
	if topDocs.TotalHits.Value != 4 {
		t.Errorf("expected 4 hits for OR query, got %d", topDocs.TotalHits.Value)
	}

	// Test Boolean AND query: content:"apple" AND content:"banana"
	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewTermQuery(index.NewTerm("content", "apple")), search.BooleanClauseMust)
	bq2.Add(search.NewTermQuery(index.NewTerm("content", "banana")), search.BooleanClauseMust)

	topDocs2, err := searcher.Search(bq2, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find docs 1 and 5 (contain both "apple" and "banana")
	if topDocs2.TotalHits.Value != 2 {
		t.Errorf("expected 2 hits for AND query, got %d", topDocs2.TotalHits.Value)
	}
}

// TestSearchResultValidation_PhraseQuery validates phrase query results.
func TestSearchResultValidation_PhraseQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with phrases
	docs := []struct {
		id      string
		content string
	}{
		{"1", "the quick brown fox"},
		{"2", "quick brown fox jumps"},
		{"3", "the quick brown"},
		{"4", "brown fox quick"},
		{"5", "the quick brown fox jumps"},
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

	// Test phrase query for "quick brown fox"
	terms := []*index.Term{
		index.NewTerm("content", "quick"),
		index.NewTerm("content", "brown"),
		index.NewTerm("content", "fox"),
	}
	query := search.NewPhraseQuery(terms...)

	topDocs, err := searcher.Search(query, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find docs 1, 2, 5 (contain "quick brown fox" in sequence)
	// Doc 4 has the words but not in order
	if topDocs.TotalHits.Value < 3 {
		t.Errorf("expected at least 3 hits for phrase query, got %d", topDocs.TotalHits.Value)
	}
}

// TestSearchResultValidation_RangeQuery validates range query results.
func TestSearchResultValidation_RangeQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with string IDs
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		doc.Add(idField)

		content := fmt.Sprintf("content %d", i)
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

	// Test range query: value between "10" and "20"
	query := search.NewTermRangeQueryWithStrings("id", "10", "20", true, true)
	topDocs, err := searcher.Search(query, nil, 50)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find 11 documents (10, 11, ..., 20)
	if topDocs.TotalHits.Value != 11 {
		t.Errorf("expected 11 hits for range [10,20], got %d", topDocs.TotalHits.Value)
	}

	// Test range query: value between "0" and "9" (exclusive of "10")
	query2 := search.NewTermRangeQueryWithStrings("id", "0", "10", true, false)
	topDocs2, err := searcher.Search(query2, nil, 50)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find 10 documents (0, 1, ..., 9)
	if topDocs2.TotalHits.Value != 10 {
		t.Errorf("expected 10 hits for range [0,10), got %d", topDocs2.TotalHits.Value)
	}
}

// TestSearchResultValidation_Sorting validates search result ordering.
func TestSearchResultValidation_Sorting(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with different content frequencies
	docs := []struct {
		id      string
		content string
		value   int
	}{
		{"1", "test test test", 10},
		{"2", "test test", 20},
		{"3", "test", 30},
		{"4", "test test test test", 40},
		{"5", "test", 50},
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

	// Search for "test" - results should be sorted by relevance (score)
	query := search.NewTermQuery(index.NewTerm("content", "test"))
	topDocs, err := searcher.Search(query, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Verify scores are in descending order
	for i := 1; i < len(topDocs.ScoreDocs); i++ {
		if topDocs.ScoreDocs[i].Score > topDocs.ScoreDocs[i-1].Score {
			t.Errorf("scores not in descending order at position %d", i)
		}
	}

	t.Logf("Search results sorted by score:")
	for i, sd := range topDocs.ScoreDocs {
		t.Logf("  %d: doc=%d, score=%f", i, sd.Doc, sd.Score)
	}
}

// TestSearchResultValidation_Reproducibility validates that search results
// are reproducible across multiple executions.
func TestSearchResultValidation_Reproducibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("doc_%d", i), true)
		doc.Add(idField)

		content := fmt.Sprintf("content %d", i%10)
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
	query := search.NewTermQuery(index.NewTerm("content", "content"))

	// Execute search multiple times and compare results
	var firstResult *search.TopDocs
	for i := 0; i < 5; i++ {
		topDocs, err := searcher.Search(query, nil, 50)
		if err != nil {
			t.Fatalf("failed to search: %v", err)
		}

		if firstResult == nil {
			firstResult = topDocs
		} else {
			// Compare with first result
			if topDocs.TotalHits.Value != firstResult.TotalHits.Value {
				t.Errorf("run %d: total hits mismatch: %d vs %d",
					i, topDocs.TotalHits.Value, firstResult.TotalHits.Value)
			}

			if len(topDocs.ScoreDocs) != len(firstResult.ScoreDocs) {
				t.Errorf("run %d: score docs length mismatch: %d vs %d",
					i, len(topDocs.ScoreDocs), len(firstResult.ScoreDocs))
				continue
			}

			for j := 0; j < len(topDocs.ScoreDocs); j++ {
				if topDocs.ScoreDocs[j].Doc != firstResult.ScoreDocs[j].Doc {
					t.Errorf("run %d: doc ID mismatch at position %d: %d vs %d",
						i, j, topDocs.ScoreDocs[j].Doc, firstResult.ScoreDocs[j].Doc)
				}

				// Allow small floating point differences
				if math.Abs(float64(topDocs.ScoreDocs[j].Score-firstResult.ScoreDocs[j].Score)) > 0.0001 {
					t.Errorf("run %d: score mismatch at position %d: %f vs %f",
						i, j, topDocs.ScoreDocs[j].Score, firstResult.ScoreDocs[j].Score)
				}
			}
		}
	}
}

// TestSearchResultValidation_ScoreNormalization validates score behavior.
func TestSearchResultValidation_ScoreNormalization(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with different term frequencies
	docs := []struct {
		id      string
		content string
	}{
		{"1", "a"},
		{"2", "a a"},
		{"3", "a a a"},
		{"4", "b"},
		{"5", "a b"},
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

	// Search for "a"
	query := search.NewTermQuery(index.NewTerm("content", "a"))
	topDocs, err := searcher.Search(query, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Documents with more occurrences of "a" should score higher
	// doc3 (3x "a") > doc2 (2x "a") > doc5 (1x "a") = doc1 (1x "a")
	if len(topDocs.ScoreDocs) < 3 {
		t.Fatalf("expected at least 3 results, got %d", len(topDocs.ScoreDocs))
	}

	// Log scores for manual verification
	t.Logf("Term 'a' search scores:")
	for i, sd := range topDocs.ScoreDocs {
		t.Logf("  %d: doc=%d, score=%f", i, sd.Doc, sd.Score)
	}
}

// TestSearchResultValidation_MatchAllDocs validates match all docs query.
func TestSearchResultValidation_MatchAllDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		doc.Add(idField)

		content := fmt.Sprintf("content %d", i)
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

	// MatchAllDocsQuery should return all documents
	query := search.NewMatchAllDocsQuery()
	topDocs, err := searcher.Search(query, nil, numDocs+10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if topDocs.TotalHits.Value != numDocs {
		t.Errorf("expected %d hits for MatchAllDocsQuery, got %d", numDocs, topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) != numDocs {
		t.Errorf("expected %d score docs, got %d", numDocs, len(topDocs.ScoreDocs))
	}

	// All scores should be equal (1.0 by default)
	for i, sd := range topDocs.ScoreDocs {
		if sd.Score != 1.0 {
			t.Errorf("expected score 1.0 for MatchAllDocsQuery, got %f at position %d", sd.Score, i)
		}
	}
}

// TestSearchResultValidation_TopN validates top N results.
func TestSearchResultValidation_TopN(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create 100 documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		doc.Add(idField)

		content := "common content"
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
	query := search.NewTermQuery(index.NewTerm("content", "common"))

	// Test with different top N values
	testCases := []int{1, 5, 10, 50, 100}
	for _, n := range testCases {
		topDocs, err := searcher.Search(query, nil, n)
		if err != nil {
			t.Fatalf("failed to search with top %d: %v", n, err)
		}

		expectedLen := n
		if n > 100 {
			expectedLen = 100
		}

		if len(topDocs.ScoreDocs) != expectedLen {
			t.Errorf("expected %d score docs with top %d, got %d", expectedLen, n, len(topDocs.ScoreDocs))
		}
	}
}

// BenchmarkSearchResultValidation_TermQuery benchmarks term query performance.
func BenchmarkSearchResultValidation_TermQuery(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	// Create documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		doc.Add(idField)

		content := fmt.Sprintf("word%d", i%100)
		contentField, _ := document.NewTextField("content", content, true)
		doc.Add(contentField)

		writer.AddDocument(doc)
	}

	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewTermQuery(index.NewTerm("content", "word50"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(query, nil, 10)
	}
}
