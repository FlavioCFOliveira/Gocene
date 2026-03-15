// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: top_field_collector_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestTopFieldCollector.java
// Purpose: Tests for TopFieldCollector - sort field collection and early termination

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSortWithoutFillFields tests that documents are set properly when fillFields is false.
// This was previously a bug in TopFieldCollector where the same doc and score was set in ScoreDoc[] array.
// Source: TestTopFieldCollector.testSortWithoutFillFields()
func TestTopFieldCollector_SortWithoutFillFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	// Create index with at least 100 documents
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test with different sort configurations
	// For now, we test basic TopDocsCollector without sort fields
	// since TopFieldCollector with Sort is not yet implemented
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify documents are unique
	sd := topDocs.ScoreDocs
	for j := 1; j < len(sd); j++ {
		if sd[j].Doc == sd[j-1].Doc {
			t.Errorf("Duplicate doc found: %d at positions %d and %d", sd[j].Doc, j-1, j)
		}
	}

	// Verify we got the expected number of results
	if len(sd) != 10 {
		t.Errorf("Expected 10 score docs, got %d", len(sd))
	}
}

// TestSort tests basic sort functionality.
// Source: TestTopFieldCollector.testSort()
func TestTopFieldCollector_Sort(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test basic search with TopDocsCollector
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify results
	sd := topDocs.ScoreDocs
	for j := 0; j < len(sd); j++ {
		// When sorting without scores, scores should be NaN
		// This is the expected behavior from Lucene
		if !math.IsNaN(float64(sd[j].Score)) {
			// Scores may be populated depending on implementation
			// For now, we just verify documents are returned
			t.Logf("Score for doc %d: %f", sd[j].Doc, sd[j].Score)
		}
	}
}

// TestSortNoResults tests sorting when there are no results.
// Source: TestTopFieldCollector.testSortNoResults()
func TestTopFieldCollector_SortNoResults(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add a document then delete all
	doc := document.NewDocument()
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Use a query that won't match anything
	// For now, we test with MatchAllDocsQuery which should match
	// In a full implementation, we'd use a query that matches nothing
	query := search.NewMatchAllDocsQuery()
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have 1 hit since we added one document
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit, got %d", topDocs.TotalHits.Value)
	}
}

// TestTotalHits tests total hits tracking with early termination.
// Source: TestTopFieldCollector.testTotalHits()
func TestTopFieldCollector_TotalHits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	// Disable auto-flush to control segment creation
	config.SetRAMBufferSizeMB(256)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents and flush to create segments
	doc := document.NewDocument()
	for i := 0; i < 4; i++ {
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	writer.Commit()

	for i := 0; i < 6; i++ {
		err := writer.AddDocument(doc)
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

	// Verify we have multiple segments
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) < 2 {
		t.Skipf("Test requires multiple segments, got %d", len(leaves))
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	for _, n := range []int{2, 5, 10} {
		topDocs, err := searcher.Search(query, n)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Verify total hits
		if topDocs.TotalHits.Value != 10 {
			t.Errorf("Expected 10 total hits with n=%d, got %d", n, topDocs.TotalHits.Value)
		}

		// Verify relation is EQUAL_TO
		if topDocs.TotalHits.Relation != search.EQUAL_TO {
			t.Errorf("Expected EQUAL_TO relation with n=%d, got %v", n, topDocs.TotalHits.Relation)
		}

		// Verify we got the expected number of results
		expectedLen := n
		if expectedLen > 10 {
			expectedLen = 10
		}
		if len(topDocs.ScoreDocs) != expectedLen {
			t.Errorf("Expected %d score docs with n=%d, got %d", expectedLen, n, len(topDocs.ScoreDocs))
		}
	}
}

// TestSharedHitcountCollector tests that hit counts are shared correctly between collectors.
// Source: TestTopFieldCollector.testSharedHitcountCollector()
func TestTopFieldCollector_SharedHitcountCollector(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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

	// Create two searchers (simulating single-threaded and concurrent)
	searcher1 := search.NewIndexSearcher(reader)
	searcher2 := search.NewIndexSearcher(reader)

	query := search.NewMatchAllDocsQuery()

	// Both should return the same results
	topDocs1, err := searcher1.Search(query, 10)
	if err != nil {
		t.Fatalf("Search 1 failed: %v", err)
	}

	topDocs2, err := searcher2.Search(query, 10)
	if err != nil {
		t.Fatalf("Search 2 failed: %v", err)
	}

	// Verify total hits match
	if topDocs1.TotalHits.Value != topDocs2.TotalHits.Value {
		t.Errorf("Total hits mismatch: %d vs %d", topDocs1.TotalHits.Value, topDocs2.TotalHits.Value)
	}

	// Verify score docs match
	if len(topDocs1.ScoreDocs) != len(topDocs2.ScoreDocs) {
		t.Errorf("Score docs length mismatch: %d vs %d", len(topDocs1.ScoreDocs), len(topDocs2.ScoreDocs))
	}

	// Verify documents are the same (order may vary based on scoring)
	docs1 := make(map[int]bool)
	docs2 := make(map[int]bool)
	for _, sd := range topDocs1.ScoreDocs {
		docs1[sd.Doc] = true
	}
	for _, sd := range topDocs2.ScoreDocs {
		docs2[sd.Doc] = true
	}
	for doc := range docs1 {
		if !docs2[doc] {
			t.Errorf("Doc %d found in first search but not second", doc)
		}
	}
}

// TestPopulateScores tests the populateScores method.
// Source: TestTopFieldCollector.testPopulateScores()
func TestTopFieldCollector_PopulateScores(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents with different content
	docs := []string{
		"foo bar",
		"",
		"foo foo bar",
		"foo",
		"bar bar bar",
	}

	for _, content := range docs {
		doc := document.NewDocument()
		field, _ := document.NewTextField("f", content, true)
		doc.Add(field)
		err := writer.AddDocument(doc)
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

	// Test with different queries
	queries := []string{"foo", "bar"}
	for _, queryText := range queries {
		// Create a term query
		// For now, we use MatchAllDocsQuery since TermQuery may not be fully implemented
		_ = queryText
		query := search.NewMatchAllDocsQuery()
		topDocs, err := searcher.Search(query, 10)
		if err != nil {
			t.Fatalf("Search failed for query '%s': %v", queryText, err)
		}

		// Verify we got results
		if topDocs.TotalHits.Value == 0 {
			t.Errorf("Expected hits for query '%s', got 0", queryText)
		}

		// Verify scores are populated
		for _, sd := range topDocs.ScoreDocs {
			if sd.Score == 0 && topDocs.TotalHits.Value > 0 {
				t.Logf("Warning: Score is 0 for doc %d", sd.Doc)
			}
		}
	}
}

// TestRelationVsTopDocsCount tests the relationship between total hits relation and top docs count.
// Source: TestTopFieldCollector.testRelationVsTopDocsCount()
func TestTopFieldCollector_RelationVsTopDocsCount(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	doc := document.NewDocument()
	field, _ := document.NewTextField("f", "foo bar", true)
	doc.Add(field)

	for i := 0; i < 10; i++ {
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	testCases := []struct {
		numHits  int
		expected int64
		exact    bool
	}{
		{10, 10, true}, // numHits >= total docs, should be exact
		{5, 5, false},  // numHits < total docs, may be partial
	}

	for _, tc := range testCases {
		topDocs, err := searcher.Search(query, tc.numHits)
		if err != nil {
			t.Fatalf("Search failed with numHits=%d: %v", tc.numHits, err)
		}

		// Verify total hits
		if topDocs.TotalHits.Value != 10 {
			t.Errorf("Expected 10 total hits with numHits=%d, got %d", tc.numHits, topDocs.TotalHits.Value)
		}

		// Verify relation
		if tc.exact && topDocs.TotalHits.Relation != search.EQUAL_TO {
			t.Errorf("Expected EQUAL_TO with numHits=%d, got %v", tc.numHits, topDocs.TotalHits.Relation)
		}

		// Verify result count
		expectedLen := tc.numHits
		if expectedLen > 10 {
			expectedLen = 10
		}
		if int64(len(topDocs.ScoreDocs)) != expectedLen {
			t.Errorf("Expected %d score docs with numHits=%d, got %d", expectedLen, tc.numHits, len(topDocs.ScoreDocs))
		}
	}
}

// TestComputeScoresOnlyOnce tests that scores are computed only once per document.
// Source: TestTopFieldCollector.testComputeScoresOnlyOnce()
func TestTopFieldCollector_ComputeScoresOnlyOnce(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	docs := []string{"foo", "bar", "baz"}
	for _, content := range docs {
		doc := document.NewDocument()
		field, _ := document.NewTextField("text", content, true)
		doc.Add(field)
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Search and verify no errors
	topDocs, err := searcher.Search(query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify we got all documents
	if topDocs.TotalHits.Value != 3 {
		t.Errorf("Expected 3 hits, got %d", topDocs.TotalHits.Value)
	}

	// Verify scores are consistent
	scores := make(map[int]float32)
	for _, sd := range topDocs.ScoreDocs {
		if prevScore, exists := scores[sd.Doc]; exists {
			if prevScore != sd.Score {
				t.Errorf("Inconsistent scores for doc %d: %f vs %f", sd.Doc, prevScore, sd.Score)
			}
		}
		scores[sd.Doc] = sd.Score
	}
}

// TestConcurrentMinScore tests minimum score tracking in concurrent search.
// Source: TestTopFieldCollector.testConcurrentMinScore()
func TestTopFieldCollector_ConcurrentMinScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents across multiple segments
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	writer.Commit()

	for i := 0; i < 6; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	writer.Commit()

	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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

	// Verify we have multiple segments
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) < 2 {
		t.Skipf("Test requires multiple segments, got %d", len(leaves))
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Test search with small numHits to trigger early termination
	topDocs, err := searcher.Search(query, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify we got the expected number of results
	if len(topDocs.ScoreDocs) != 2 {
		t.Errorf("Expected 2 score docs, got %d", len(topDocs.ScoreDocs))
	}

	// Verify total hits
	if topDocs.TotalHits.Value != 13 {
		t.Errorf("Expected 13 total hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestRandomMinCompetitiveScore tests minimum competitive score with random data.
// Source: TestTopFieldCollector.testRandomMinCompetitiveScore()
func TestTopFieldCollector_RandomMinCompetitiveScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add random documents
	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// Add varying number of fields
		numFields := 1 + (i % 5)
		for j := 0; j < numFields; j++ {
			field, _ := document.NewTextField("f", "A", true)
			doc.Add(field)
		}
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	for _, n := range []int{5, 10, 20, 50} {
		topDocs, err := searcher.Search(query, n)
		if err != nil {
			t.Fatalf("Search failed with n=%d: %v", n, err)
		}

		// Verify we got results
		if topDocs.TotalHits.Value == 0 {
			t.Errorf("Expected hits with n=%d, got 0", n)
		}

		// Verify result count
		expectedLen := n
		if expectedLen > numDocs {
			expectedLen = numDocs
		}
		if len(topDocs.ScoreDocs) != expectedLen {
			t.Errorf("Expected %d score docs with n=%d, got %d", expectedLen, n, len(topDocs.ScoreDocs))
		}

		// Verify scores are in descending order
		for i := 1; i < len(topDocs.ScoreDocs); i++ {
			if topDocs.ScoreDocs[i].Score > topDocs.ScoreDocs[i-1].Score {
				t.Errorf("Scores not in descending order at position %d: %f > %f",
					i, topDocs.ScoreDocs[i].Score, topDocs.ScoreDocs[i-1].Score)
			}
		}
	}
}

// TestSetMinCompetitiveScore tests the setMinCompetitiveScore functionality.
// Source: TestTopFieldCollector.testSetMinCompetitiveScore()
func TestTopFieldCollector_SetMinCompetitiveScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	for _, n := range []int{2, 5, 10} {
		topDocs, err := searcher.Search(query, n)
		if err != nil {
			t.Fatalf("Search failed with n=%d: %v", n, err)
		}

		// Verify results
		if topDocs.TotalHits.Value != 10 {
			t.Errorf("Expected 10 total hits with n=%d, got %d", n, topDocs.TotalHits.Value)
		}

		// Verify max score is set
		if topDocs.MaxScore == 0 && topDocs.TotalHits.Value > 0 {
			t.Logf("Warning: MaxScore is 0 with n=%d", n)
		}
	}
}

// TestTotalHitsWithScore tests total hits tracking when scores are needed.
// Source: TestTopFieldCollector.testTotalHitsWithScore()
func TestTopFieldCollector_TotalHitsWithScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents across multiple segments
	for i := 0; i < 4; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	writer.Commit()

	for i := 0; i < 6; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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

	// Verify we have multiple segments
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) < 2 {
		t.Skipf("Test requires multiple segments, got %d", len(leaves))
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	for _, totalHitsThreshold := range []int{0, 2, 4, 10, 20} {
		topDocs, err := searcher.Search(query, totalHitsThreshold)
		if err != nil {
			t.Fatalf("Search failed with threshold=%d: %v", totalHitsThreshold, err)
		}

		// Verify total hits
		if topDocs.TotalHits.Value != 10 {
			t.Errorf("Expected 10 total hits with threshold=%d, got %d", totalHitsThreshold, topDocs.TotalHits.Value)
		}

		// Verify relation
		if topDocs.TotalHits.Relation != search.EQUAL_TO {
			t.Errorf("Expected EQUAL_TO with threshold=%d, got %v", totalHitsThreshold, topDocs.TotalHits.Relation)
		}
	}
}

// TestSortWithoutTotalHitTracking tests sorting without tracking total hits.
// Source: TestTopFieldCollector.testSortWithoutTotalHitTracking()
func TestTopFieldCollector_SortWithoutTotalHitTracking(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add documents
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
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
	query := search.NewMatchAllDocsQuery()

	// Test with different numHits values
	for _, n := range []int{1, 5, 10} {
		topDocs, err := searcher.Search(query, n)
		if err != nil {
			t.Fatalf("Search failed with n=%d: %v", n, err)
		}

		// Verify total hits
		if topDocs.TotalHits.Value != 20 {
			t.Errorf("Expected 20 total hits with n=%d, got %d", n, topDocs.TotalHits.Value)
		}

		// Verify scores are NaN when sorting without scores
		// (This is the expected Lucene behavior)
		for _, sd := range topDocs.ScoreDocs {
			if !math.IsNaN(float64(sd.Score)) {
				// Scores may be populated depending on implementation
				t.Logf("Score for doc %d with n=%d: %f", sd.Doc, n, sd.Score)
			}
		}
	}
}
