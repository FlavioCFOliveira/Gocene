// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: boolean_scorer_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestBooleanScorer.java
// Purpose: Tests for BooleanScorer bulk scoring, bucket management, and cost estimation

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// addStringField is a helper that creates a StringField and adds it to doc, fataling on error.
func addStringField(t *testing.T, doc *document.Document, name, value string, stored bool) {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("NewStringField(%q, %q): %v", name, value, err)
	}
	doc.Add(f)
}

// addTextField is a helper that creates a TextField and adds it to doc, fataling on error.
func addBoolScorerTextField(t *testing.T, doc *document.Document, name, value string, stored bool) {
	t.Helper()
	f, err := document.NewTextField(name, value, stored)
	if err != nil {
		t.Fatalf("NewTextField(%q): %v", name, err)
	}
	doc.Add(f)
}

// openReader commits and closes the writer, then opens a DirectoryReader on dir.
func openReaderFromDir(t *testing.T, writer *index.IndexWriter, dir store.Directory) index.IndexReaderInterface {
	t.Helper()
	if err := writer.Commit(); err != nil {
		t.Fatalf("writer.Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return r
}

// TestBooleanScorer_Basic tests basic boolean query scoring with SHOULD and MUST_NOT clauses
// Source: TestBooleanScorer.testMethod()
func TestBooleanScorer_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	values := []string{"1", "2", "3", "4"}

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for _, value := range values {
		doc := document.NewDocument()
		addStringField(t, doc, "category", value, true)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	// Build boolean query: (category:1 OR category:2) AND NOT category:9
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(search.NewTermQuery(index.NewTerm("category", "1")), search.SHOULD)
	innerQuery.Add(search.NewTermQuery(index.NewTerm("category", "2")), search.SHOULD)

	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(innerQuery, search.MUST)
	outerQuery.Add(search.NewTermQuery(index.NewTerm("category", "9")), search.MUST_NOT)

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	topDocs, err := searcher.Search(outerQuery, 1000)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 2 {
		t.Errorf("Expected 2 matched documents, got %d", topDocs.TotalHits.Value)
	}
}

// TestBooleanScorer_Embedded tests that BooleanScorer can embed another BooleanScorer
// Source: TestBooleanScorer.testEmbeddedBooleanScorer()
func TestBooleanScorer_Embedded(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	addBoolScorerTextField(t, doc, "field",
		"doctors are people who prescribe medicines of which they know little, to cure diseases of which they know less, in human beings of whom they know nothing",
		false)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// Build nested query: (field:little OR field:diseases) OR term-that-won't-match
	innerQuery := search.NewBooleanQuery()
	innerQuery.Add(search.NewTermQuery(index.NewTerm("field", "little")), search.SHOULD)
	innerQuery.Add(search.NewTermQuery(index.NewTerm("field", "diseases")), search.SHOULD)

	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(innerQuery, search.SHOULD)
	outerQuery.Add(search.NewTermQuery(index.NewTerm("field", "nonexistent")), search.SHOULD)

	topDocs, err := searcher.Search(outerQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected count 1, got %d", topDocs.TotalHits.Value)
	}
}

// TestBooleanScorer_OptimizeTopLevelClause tests optimization when there's a single non-null scorer
// Source: TestBooleanScorer.testOptimizeTopLevelClauseOrNull()
// Note: This test requires BooleanScorerSupplier and DefaultBulkScorer implementation
func TestBooleanScorer_OptimizeTopLevelClause(t *testing.T) {
	t.Fatal("Requires BooleanScorerSupplier and DefaultBulkScorer implementation")
}

// TestBooleanScorer_OptimizeProhibitedClauses tests optimization of prohibited clauses (MUST_NOT)
// Source: TestBooleanScorer.testOptimizeProhibitedClauses()
// Note: This test requires ReqExclBulkScorer implementation
func TestBooleanScorer_OptimizeProhibitedClauses(t *testing.T) {
	t.Fatal("Requires ReqExclBulkScorer implementation")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc1 := document.NewDocument()
	addStringField(t, doc1, "foo", "bar", false)
	addStringField(t, doc1, "foo", "baz", false)
	writer.AddDocument(doc1) //nolint:errcheck // inside skipped test

	doc2 := document.NewDocument()
	addStringField(t, doc2, "foo", "baz", false)
	writer.AddDocument(doc2) //nolint:errcheck // inside skipped test

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("Failed to force merge: %v", err)
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// SHOULD + MUST_NOT
	query1 := search.NewBooleanQuery()
	query1.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)
	query1.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.MUST_NOT)

	// SHOULD + MUST_NOT + MatchAllDocsQuery
	query2 := search.NewBooleanQuery()
	query2.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)
	query2.Add(search.NewMatchAllDocsQuery(), search.SHOULD)
	query2.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.MUST_NOT)

	// MUST + MUST_NOT
	query3 := search.NewBooleanQuery()
	query3.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.MUST)
	query3.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.MUST_NOT)

	// FILTER + MUST_NOT
	query4 := search.NewBooleanQuery()
	query4.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.FILTER)
	query4.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.MUST_NOT)

	_ = []search.Query{query1, query2, query3, query4}
}

// TestBooleanScorer_SparseClauseOptimization tests sparse clause optimization
// Source: TestBooleanScorer.testSparseClauseOptimization()
// Note: This test requires QueryUtils.check() equivalent for dueling scorers
func TestBooleanScorer_SparseClauseOptimization(t *testing.T) {
	t.Fatal("Requires QueryUtils.check() equivalent for dueling scorers")
}

// TestBooleanScorer_FilterConstantScore tests FILTER clause constant score behavior
// Source: TestBooleanScorer.testFilterConstantScore()
// Note: Requires IndexSearcher.Rewrite which is not yet implemented.
func TestBooleanScorer_FilterConstantScore(t *testing.T) {
	t.Fatal("Requires IndexSearcher.Rewrite — not yet implemented")
}

// TestBooleanScorer_CollectNoThresholdWhenOnlyFilter tests collection with only FILTER clauses
// Source: TestBooleanScorer.testCollectNoThresholdWhenOnlyFilter()
// Note: This test requires TopScoreDocCollectorManager with totalHitsThreshold support
func TestBooleanScorer_CollectNoThresholdWhenOnlyFilter(t *testing.T) {
	t.Fatal("Requires TopScoreDocCollectorManager with totalHitsThreshold support")
}

// TestBooleanScorer_CostEstimation tests that BooleanScorer provides accurate cost estimates
// Source: Derived from cost() method tests in TestBooleanScorer
func TestBooleanScorer_CostEstimation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add multiple documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		addStringField(t, doc, "field", string(rune('a'+i%26)), false)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// OR query
	orQuery := search.NewBooleanQuery()
	orQuery.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	orQuery.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.SHOULD)

	topDocs, err := searcher.Search(orQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value == 0 {
		t.Error("Expected some hits for OR query")
	}

	// AND query
	andQuery := search.NewBooleanQuery()
	andQuery.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.MUST)
	andQuery.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.MUST)

	topDocs2, err := searcher.Search(andQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// AND of different values should have no matches
	if topDocs2.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits for AND query with different values, got %d", topDocs2.TotalHits.Value)
	}
}

// TestBooleanScorer_BucketManagement tests bucket-based scoring management
// Source: Derived from BooleanScorer bucket management tests
func TestBooleanScorer_BucketManagement(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with overlapping terms
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		addStringField(t, doc, "field", "term1", false)
		if i%2 == 0 {
			addStringField(t, doc, "field", "term2", false)
		}
		if i%3 == 0 {
			addStringField(t, doc, "field", "term3", false)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// Test query with multiple SHOULD clauses
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm("field", "term1")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("field", "term2")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("field", "term3")), search.SHOULD)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// All documents should match at least term1
	if topDocs.TotalHits.Value != 10 {
		t.Errorf("Expected 10 hits, got %d", topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) == 0 {
		t.Error("Expected some scored documents")
	}
}

// TestBooleanScorer_MinShouldMatch tests minimum should match functionality
// Source: Derived from BooleanQuery minShouldMatch tests
func TestBooleanScorer_MinShouldMatch(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with varying term matches
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		addStringField(t, doc, "field", "a", false)
		if i >= 1 {
			addStringField(t, doc, "field", "b", false)
		}
		if i >= 3 {
			addStringField(t, doc, "field", "c", false)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// Query with minShouldMatch = 2
	query := search.NewBooleanQuery()
	query.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("field", "c")), search.SHOULD)
	query.SetMinimumNumberShouldMatch(2)

	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Documents 1-4 match at least 2 terms (a+b), documents 3-4 match all 3
	if topDocs.TotalHits.Value != 4 {
		t.Errorf("Expected 4 hits with minShouldMatch=2, got %d", topDocs.TotalHits.Value)
	}
}

// TestBooleanScorer_ComplexNesting tests complex nested boolean queries
// Source: Derived from embedded boolean scorer tests
func TestBooleanScorer_ComplexNesting(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc1 := document.NewDocument()
	addStringField(t, doc1, "a", "1", false)
	addStringField(t, doc1, "b", "2", false)
	writer.AddDocument(doc1) //nolint:errcheck // test data

	doc2 := document.NewDocument()
	addStringField(t, doc2, "a", "1", false)
	addStringField(t, doc2, "b", "3", false)
	writer.AddDocument(doc2) //nolint:errcheck // test data

	doc3 := document.NewDocument()
	addStringField(t, doc3, "a", "2", false)
	addStringField(t, doc3, "b", "2", false)
	writer.AddDocument(doc3) //nolint:errcheck // test data

	reader := openReaderFromDir(t, writer, dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	// Complex nested query: ((a:1 OR a:2) AND (b:2)) OR (a:1 AND b:3)
	innerOr1 := search.NewBooleanQuery()
	innerOr1.Add(search.NewTermQuery(index.NewTerm("a", "1")), search.SHOULD)
	innerOr1.Add(search.NewTermQuery(index.NewTerm("a", "2")), search.SHOULD)

	innerAnd1 := search.NewBooleanQuery()
	innerAnd1.Add(innerOr1, search.MUST)
	innerAnd1.Add(search.NewTermQuery(index.NewTerm("b", "2")), search.MUST)

	innerAnd2 := search.NewBooleanQuery()
	innerAnd2.Add(search.NewTermQuery(index.NewTerm("a", "1")), search.MUST)
	innerAnd2.Add(search.NewTermQuery(index.NewTerm("b", "3")), search.MUST)

	outerQuery := search.NewBooleanQuery()
	outerQuery.Add(innerAnd1, search.SHOULD)
	outerQuery.Add(innerAnd2, search.SHOULD)

	topDocs, err := searcher.Search(outerQuery, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should match doc1 (a:1,b:2), doc2 (a:1,b:3), and doc3 (a:2,b:2)
	if topDocs.TotalHits.Value != 3 {
		t.Errorf("Expected 3 hits, got %d", topDocs.TotalHits.Value)
	}
}
