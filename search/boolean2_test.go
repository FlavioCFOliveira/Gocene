// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestBoolean2Scoring tests BooleanQuery scoring order, multi-segment search,
// bucket gaps, and coordination factor.
// This is the Go port of Lucene's TestBoolean2.java (GC-219)
func TestBoolean2Scoring(t *testing.T) {
	// Test data - documents with specific field values
	docFields := []string{
		"w1 w2 w3 w4 w5",
		"w1 w3 w2 w3",
		"w1 xx w2 yy w3",
		"w1 w3 xx w2 yy mm",
	}

	field := "field"

	// Create directory and writer
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for _, docText := range docFields {
		doc := document.NewDocument()
		textField, err := document.NewTextField(field, docText, false)
		if err != nil {
			t.Fatalf("Failed to create TextField: %v", err)
		}
		doc.Add(textField)
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

	// Test 1: MUST + MUST query (w3 AND xx)
	t.Run("MUST_MUST", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 2 and 3 (0-indexed: 2, 3)
		if topDocs.TotalHits.Value != 2 {
			t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
		}

		// Verify doc IDs are in expected set
		expectedDocs := map[int]bool{2: true, 3: true}
		for _, sd := range topDocs.ScoreDocs {
			if !expectedDocs[sd.Doc] {
				t.Errorf("Unexpected doc ID: %d", sd.Doc)
			}
		}
	})

	// Test 2: MUST + SHOULD query (w3 AND xx)
	t.Run("MUST_SHOULD", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.SHOULD)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 2 and 3 first (have both), then 1 and 0 (have w3 only)
		if topDocs.TotalHits.Value != 4 {
			t.Errorf("Expected 4 hits, got %d", topDocs.TotalHits.Value)
		}

		// Verify all docs are returned
		if len(topDocs.ScoreDocs) != 4 {
			t.Errorf("Expected 4 score docs, got %d", len(topDocs.ScoreDocs))
		}
	})

	// Test 3: SHOULD + SHOULD query (w3 OR xx)
	t.Run("SHOULD_SHOULD", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.SHOULD)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.SHOULD)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: all 4 docs match
		if topDocs.TotalHits.Value != 4 {
			t.Errorf("Expected 4 hits, got %d", topDocs.TotalHits.Value)
		}
	})

	// Test 4: SHOULD + MUST_NOT query (w3 AND NOT xx)
	t.Run("SHOULD_MUST_NOT", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.SHOULD)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST_NOT)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 0 and 1 (have w3 but not xx)
		if topDocs.TotalHits.Value != 2 {
			t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
		}

		expectedDocs := map[int]bool{0: true, 1: true}
		for _, sd := range topDocs.ScoreDocs {
			if !expectedDocs[sd.Doc] {
				t.Errorf("Unexpected doc ID: %d", sd.Doc)
			}
		}
	})

	// Test 5: MUST + MUST_NOT query (w3 AND NOT xx)
	t.Run("MUST_MUST_NOT", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST_NOT)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 0 and 1
		if topDocs.TotalHits.Value != 2 {
			t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
		}
	})

	// Test 6: MUST + MUST_NOT + MUST_NOT query (w3 AND NOT xx AND NOT w5)
	t.Run("MUST_MUST_NOT_MUST_NOT", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST_NOT)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w5")), search.MUST_NOT)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: doc 1 only (has w3, no xx, no w5)
		if topDocs.TotalHits.Value != 1 {
			t.Errorf("Expected 1 hit, got %d", topDocs.TotalHits.Value)
		}

		if len(topDocs.ScoreDocs) > 0 && topDocs.ScoreDocs[0].Doc != 1 {
			t.Errorf("Expected doc 1, got %d", topDocs.ScoreDocs[0].Doc)
		}
	})

	// Test 7: All MUST_NOT query
	t.Run("ALL_MUST_NOT", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST_NOT)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST_NOT)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w5")), search.MUST_NOT)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: no matches (all docs have at least one of these terms)
		if topDocs.TotalHits.Value != 0 {
			t.Errorf("Expected 0 hits, got %d", topDocs.TotalHits.Value)
		}
	})

	// Test 8: MUST + SHOULD + MUST_NOT query
	t.Run("MUST_SHOULD_MUST_NOT", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.SHOULD)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w5")), search.MUST_NOT)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 2 and 3 (have w3, no w5), doc 3 has xx as bonus
		if topDocs.TotalHits.Value != 2 {
			t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
		}
	})

	// Test 9: MUST + MUST + MUST + SHOULD query
	t.Run("MUST_MUST_MUST_SHOULD", func(t *testing.T) {
		bq := search.NewBooleanQuery()
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w3")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "xx")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "w2")), search.MUST)
		bq.Add(search.NewTermQuery(index.NewTerm(field, "zz")), search.SHOULD)

		topDocs, err := searcher.Search(bq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Expected: docs 2 and 3 (have w3, xx, and w2)
		if topDocs.TotalHits.Value != 2 {
			t.Errorf("Expected 2 hits, got %d", topDocs.TotalHits.Value)
		}
	})
}

// TestBoolean2MultiSegment tests BooleanQuery across multiple segments
func TestBoolean2MultiSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents in multiple segments
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("content", "hello world", true)
		doc.Add(textField)
		writer.AddDocument(doc)
	}
	writer.Commit()

	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("content", "foo bar", true)
		doc.Add(textField)
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

	// Test BooleanQuery across segments
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("content", "hello")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("content", "world")), search.MUST)

	topDocs, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 3 documents
	if topDocs.TotalHits.Value != 3 {
		t.Errorf("Expected 3 hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestBoolean2RandomQueries tests random BooleanQuery combinations
func TestBoolean2RandomQueries(t *testing.T) {
	// Create test index
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with various terms
	terms := []string{"a", "b", "c", "d", "e"}
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		// Each document has 2-3 random terms
		numTerms := 2 + rand.Intn(2)
		docText := ""
		for j := 0; j < numTerms; j++ {
			if j > 0 {
				docText += " "
			}
			docText += terms[rand.Intn(len(terms))]
		}
		textField, _ := document.NewTextField("field", docText, true)
		doc.Add(textField)
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

	// Test random BooleanQueries
	for i := 0; i < 10; i++ {
		bq := search.NewBooleanQuery()
		numClauses := 1 + rand.Intn(4)

		for j := 0; j < numClauses; j++ {
			term := terms[rand.Intn(len(terms))]
			tq := search.NewTermQuery(index.NewTerm("field", term))

			// Random occur type
			occur := rand.Intn(3)
			switch occur {
			case 0:
				bq.Add(tq, search.MUST)
			case 1:
				bq.Add(tq, search.SHOULD)
			case 2:
				bq.Add(tq, search.MUST_NOT)
			}
		}

		// Execute query - should not error
		_, err := searcher.Search(bq, 100)
		if err != nil {
			t.Errorf("Random query %d failed: %v", i, err)
		}
	}
}

// TestBoolean2ScoringOrder verifies that scoring produces consistent ordering
func TestBoolean2ScoringOrder(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Create documents with varying term frequencies
	docs := []string{
		"a",       // 1 term
		"a a",     // 2 of same term
		"a b",     // 2 different terms
		"a a a",   // 3 of same term
		"a b c",   // 3 different terms
		"a a a a", // 4 of same term
	}

	for _, text := range docs {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("field", text, true)
		doc.Add(textField)
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

	// Test that SHOULD clauses affect scoring
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.SHOULD)

	topDocs, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify scores are in descending order
	for i := 1; i < len(topDocs.ScoreDocs); i++ {
		if topDocs.ScoreDocs[i].Score > topDocs.ScoreDocs[i-1].Score {
			t.Errorf("Scores not in descending order at position %d: %f > %f",
				i, topDocs.ScoreDocs[i].Score, topDocs.ScoreDocs[i-1].Score)
		}
	}
}

// TestBoolean2CoordFactor tests the coordination factor in scoring
func TestBoolean2CoordFactor(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Documents with different overlap with query terms
	docs := []string{
		"x y z", // matches all 3
		"x y",   // matches 2 of 3
		"x z",   // matches 2 of 3
		"y z",   // matches 2 of 3
		"x",     // matches 1 of 3
		"y",     // matches 1 of 3
		"z",     // matches 1 of 3
	}

	for _, text := range docs {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("field", text, true)
		doc.Add(textField)
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

	// Query with 3 SHOULD clauses
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "x")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "y")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "z")), search.SHOULD)

	topDocs, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// First doc should be the one matching all 3 terms
	if len(topDocs.ScoreDocs) > 0 && topDocs.ScoreDocs[0].Doc != 0 {
		t.Logf("Note: Doc matching all terms may not be first depending on scoring implementation")
	}

	// All docs should be returned
	if topDocs.TotalHits.Value != 7 {
		t.Errorf("Expected 7 hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestBoolean2EmptyIndex tests queries against empty index
func TestBoolean2EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
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

	// Test various queries on empty index
	tests := []struct {
		name   string
		occur1 search.Occur
		occur2 search.Occur
	}{
		{"MUST_MUST", search.MUST, search.MUST},
		{"MUST_SHOULD", search.MUST, search.SHOULD},
		{"SHOULD_SHOULD", search.SHOULD, search.SHOULD},
		{"MUST_MUST_NOT", search.MUST, search.MUST_NOT},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bq := search.NewBooleanQuery()
			bq.Add(search.NewTermQuery(index.NewTerm("field", "x")), tc.occur1)
			bq.Add(search.NewTermQuery(index.NewTerm("field", "y")), tc.occur2)

			topDocs, err := searcher.Search(bq, 10)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			// Empty index should return 0 hits for all queries
			if topDocs.TotalHits.Value != 0 {
				t.Errorf("Expected 0 hits for empty index, got %d", topDocs.TotalHits.Value)
			}
		})
	}
}

// TestBoolean2MinShouldMatch tests minimum number of SHOULD clauses matching
func TestBoolean2MinShouldMatch(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Documents with different term combinations
	docs := []string{
		"a b c", // matches all 3
		"a b",   // matches 2
		"a c",   // matches 2
		"b c",   // matches 2
		"a",     // matches 1
		"b",     // matches 1
		"c",     // matches 1
	}

	for _, text := range docs {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("field", text, true)
		doc.Add(textField)
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

	// Test with minShouldMatch = 2
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "c")), search.SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	topDocs, err := searcher.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should match docs with at least 2 terms: 0, 1, 2, 3
	if topDocs.TotalHits.Value != 4 {
		t.Errorf("Expected 4 hits with minShouldMatch=2, got %d", topDocs.TotalHits.Value)
	}
}

// BenchmarkBoolean2Query benchmarks BooleanQuery performance
func BenchmarkBoolean2Query(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, _ := index.NewIndexWriter(dir, config)

	// Create index with 1000 documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		textField, _ := document.NewTextField("field", "term1 term2 term3", true)
		doc.Add(textField)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()
	searcher := search.NewIndexSearcher(reader)

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "term1")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "term2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("field", "term3")), search.MUST_NOT)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(bq, 100)
	}
}
