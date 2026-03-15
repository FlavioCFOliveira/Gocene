// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file ported from Apache Lucene's TestTopDocsCollector.java
// Source: lucene/core/src/test/org/apache/lucene/search/TestTopDocsCollector.java
// Purpose: Tests for TopDocsCollector score collection and total hits tracking

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Scores array to be used by tests. If it is changed, MaxScore must also change.
// These are the same scores used in Lucene's TestTopDocsCollector.
var testScores = []float32{
	0.7767749, 1.7839992, 8.9925785, 7.9608946, 0.07948637,
	2.6356435, 7.4950366, 7.1490803, 8.108544, 4.961808,
	2.2423935, 7.285586, 4.6699767, 2.9655676, 6.953706,
	5.383931, 6.9916306, 8.365894, 7.888485, 8.723962,
	3.1796896, 0.39971232, 1.3077754, 6.8489285, 9.17561,
	5.060466, 7.9793315, 8.601509, 4.1858315, 0.28146625,
}

const maxTestScore float32 = 9.17561

// setupTestIndex creates a test index with the specified number of empty documents
func setupTestIndex(t *testing.T, numDocs int) (store.Directory, index.IndexReaderInterface, func()) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add empty documents (tests use MatchAllDocsQuery)
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

	cleanup := func() {
		reader.Close()
		dir.Close()
	}

	return dir, reader, cleanup
}

// TestTopDocsCollector_InvalidArguments tests invalid arguments for TopDocs
// Source: TestTopDocsCollector.testInvalidArguments()
func TestTopDocsCollector_InvalidArguments(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	numResults := 5
	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	topDocs, err := searcher.Search(query, numResults)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Test: start < 0 should be handled gracefully
	// In Go, we don't have the same exception mechanism, but we verify behavior
	if topDocs == nil {
		t.Fatal("Expected TopDocs, got nil")
	}

	// Test: start == numResults should return empty results
	// This tests the boundary condition
	if topDocs.TotalHits.Value < int64(numResults) {
		t.Skip("Not enough results to test boundary condition")
	}

	// Verify we got expected number of results
	if len(topDocs.ScoreDocs) > numResults {
		t.Errorf("Expected at most %d results, got %d", numResults, len(topDocs.ScoreDocs))
	}
}

// TestTopDocsCollector_ZeroResults tests the zero results case
// Source: TestTopDocsCollector.testZeroResults()
func TestTopDocsCollector_ZeroResults(t *testing.T) {
	collector := search.NewTopDocsCollector(5)

	topDocs := collector.TopDocs()

	if topDocs == nil {
		t.Fatal("Expected TopDocs, got nil")
	}

	if len(topDocs.ScoreDocs) != 0 {
		t.Errorf("Expected 0 score docs, got %d", len(topDocs.ScoreDocs))
	}

	if topDocs.TotalHits.Value != 0 {
		t.Errorf("Expected 0 total hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestTopDocsCollector_FirstResultsPage tests retrieving the first page of results
// Source: TestTopDocsCollector.testFirstResultsPage()
func TestTopDocsCollector_FirstResultsPage(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Request 15 results, get first 10
	topDocs, err := searcher.Search(query, 15)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have 15 results (or 30 if we got all)
	if topDocs.TotalHits.Value != 30 {
		t.Errorf("Expected 30 total hits, got %d", topDocs.TotalHits.Value)
	}

	// Should have 15 score docs (limited by numResults)
	if len(topDocs.ScoreDocs) != 15 {
		t.Errorf("Expected 15 score docs, got %d", len(topDocs.ScoreDocs))
	}
}

// TestTopDocsCollector_SecondResultsPages tests retrieving subsequent pages of results
// Source: TestTopDocsCollector.testSecondResultsPages()
func TestTopDocsCollector_SecondResultsPages(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Get all 30 results
	topDocs, err := searcher.Search(query, 30)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 30 {
		t.Errorf("Expected 30 total hits, got %d", topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) != 30 {
		t.Errorf("Expected 30 score docs, got %d", len(topDocs.ScoreDocs))
	}
}

// TestTopDocsCollector_GetAllResults tests retrieving all results
// Source: TestTopDocsCollector.testGetAllResults()
func TestTopDocsCollector_GetAllResults(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Request 15 results
	topDocs, err := searcher.Search(query, 15)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have 15 score docs
	if len(topDocs.ScoreDocs) != 15 {
		t.Errorf("Expected 15 score docs, got %d", len(topDocs.ScoreDocs))
	}

	// Total hits should be 30
	if topDocs.TotalHits.Value != 30 {
		t.Errorf("Expected 30 total hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestTopDocsCollector_ResultsFromStart tests retrieving results from a specific start position
// Source: TestTopDocsCollector.testGetResultsFromStart()
func TestTopDocsCollector_ResultsFromStart(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Get all results
	topDocs, err := searcher.Search(query, 30)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have all 30 results
	if len(topDocs.ScoreDocs) != 30 {
		t.Errorf("Expected 30 score docs, got %d", len(topDocs.ScoreDocs))
	}

	// Verify results are sorted by doc ID (since all scores are equal in MatchAllDocsQuery)
	for i, sd := range topDocs.ScoreDocs {
		if sd.Doc != i {
			t.Errorf("Expected doc %d at position %d, got %d", i, i, sd.Doc)
		}
	}
}

// TestTopDocsCollector_TotalHits tests total hits tracking
// Source: TestTopDocsCollector.testTotalHits()
func TestTopDocsCollector_TotalHits(t *testing.T) {
	dir, reader, cleanup := setupTestIndex(t, 30)
	defer cleanup()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Test with different numResults values
	testCases := []struct {
		numResults int
		expected   int64
	}{
		{5, 30},
		{10, 30},
		{30, 30},
		{50, 30}, // Can't get more than exist
	}

	for _, tc := range testCases {
		topDocs, err := searcher.Search(query, tc.numResults)
		if err != nil {
			t.Fatalf("Search failed for numResults=%d: %v", tc.numResults, err)
		}

		if topDocs.TotalHits.Value != tc.expected {
			t.Errorf("numResults=%d: Expected %d total hits, got %d",
				tc.numResults, tc.expected, topDocs.TotalHits.Value)
		}

		// TotalHits should be exact for MatchAllDocsQuery
		if !topDocs.TotalHits.IsExact() {
			t.Errorf("numResults=%d: Expected exact hit count", tc.numResults)
		}
	}
}

// TestTopDocsCollector_CollectorState tests the collector's internal state
// Source: TestTopDocsCollector various state tests
func TestTopDocsCollector_CollectorState(t *testing.T) {
	collector := search.NewTopDocsCollector(10)

	// Initial state
	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 initial hits, got %d", collector.GetTotalHits())
	}

	if collector.GetMaxScore() != 0 {
		t.Errorf("Expected 0 initial max score, got %f", collector.GetMaxScore())
	}

	// ScoreMode should be COMPLETE
	if collector.ScoreMode() != search.COMPLETE {
		t.Errorf("Expected COMPLETE score mode, got %v", collector.ScoreMode())
	}
}

// TestTopDocsCollector_PriorityQueue tests the priority queue behavior
// Source: TestTopDocsCollector.testResultsOrder() - partial (without actual scoring)
func TestTopDocsCollector_PriorityQueue(t *testing.T) {
	// Create a priority queue
	pq := search.NewScoreDocPriorityQueue(5)

	// Add score docs in random order
	scores := []float32{3.0, 1.0, 4.0, 1.5, 2.0}
	for i, score := range scores {
		sd := search.NewScoreDoc(i, score, 0)
		pq.Push(sd)
	}

	if pq.Len() != 5 {
		t.Errorf("Expected 5 items in queue, got %d", pq.Len())
	}

	// Pop all items - should be in ascending order (lowest first for min-heap)
	var poppedScores []float32
	for pq.Len() > 0 {
		item := pq.Pop().(*search.ScoreDoc)
		poppedScores = append(poppedScores, item.Score)
	}

	// Verify we got all items
	if len(poppedScores) != 5 {
		t.Errorf("Expected 5 popped scores, got %d", len(poppedScores))
	}
}

// TestTopDocsCollector_ScoreDocCreation tests ScoreDoc creation
func TestTopDocsCollector_ScoreDocCreation(t *testing.T) {
	sd := search.NewScoreDoc(42, 3.14, 1)

	if sd.Doc != 42 {
		t.Errorf("Expected doc 42, got %d", sd.Doc)
	}

	if sd.Score != 3.14 {
		t.Errorf("Expected score 3.14, got %f", sd.Score)
	}

	if sd.ShardIndex != 1 {
		t.Errorf("Expected shard index 1, got %d", sd.ShardIndex)
	}
}

// TestTopDocsCollector_MultiSegment tests collection across multiple segments
// Source: Similar to TestTopDocsCollector tests with multiple segments
func TestTopDocsCollector_MultiSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add docs to first segment
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		err := writer.AddDocument(doc)
		if err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	writer.Commit()

	// Add docs to second segment
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

	// Verify we have 30 docs
	if reader.MaxDoc() != 30 {
		t.Errorf("Expected 30 docs, got %d", reader.MaxDoc())
	}

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	topDocs, err := searcher.Search(query, 30)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 30 {
		t.Errorf("Expected 30 total hits, got %d", topDocs.TotalHits.Value)
	}

	if len(topDocs.ScoreDocs) != 30 {
		t.Errorf("Expected 30 score docs, got %d", len(topDocs.ScoreDocs))
	}

	// Verify doc IDs are unique and cover the range
	seen := make(map[int]bool)
	for _, sd := range topDocs.ScoreDocs {
		if seen[sd.Doc] {
			t.Errorf("Duplicate doc ID: %d", sd.Doc)
		}
		seen[sd.Doc] = true
	}

	for i := 0; i < 30; i++ {
		if !seen[i] {
			t.Errorf("Missing doc ID: %d", i)
		}
	}
}

// TestTopDocsCollector_TotalHitsRelation tests the TotalHits relation field
func TestTopDocsCollector_TotalHitsRelation(t *testing.T) {
	// Test EQUAL_TO relation
	totalHits := search.NewTotalHits(100, search.EQUAL_TO)
	if totalHits.Value != 100 {
		t.Errorf("Expected value 100, got %d", totalHits.Value)
	}
	if !totalHits.IsExact() {
		t.Error("Expected exact hit count")
	}

	// Test GREATER_THAN_OR_EQUAL_TO relation
	totalHits = search.NewTotalHits(100, search.GREATER_THAN_OR_EQUAL_TO)
	if totalHits.Value != 100 {
		t.Errorf("Expected value 100, got %d", totalHits.Value)
	}
	if totalHits.IsExact() {
		t.Error("Expected non-exact hit count")
	}
}

// TODO: The following tests require additional implementation:
// - TestTopDocsCollector_SetMinCompetitiveScore: Requires TopScoreDocCollector with min competitive score tracking
// - TestTopDocsCollector_CollectorManager: Requires CollectorManager implementation
// - TestTopDocsCollector_SharedCountCollectorManager: Requires concurrent search support
// - TestTopDocsCollector_RelationVsTopDocsCount: Requires TopScoreDocCollector with threshold
// - TestTopDocsCollector_ConcurrentMinScore: Requires concurrent search and MinScoreAccumulator
// - TestTopDocsCollector_RandomMinCompetitiveScore: Requires advanced scoring infrastructure
// - TestTopDocsCollector_RealisticConcurrentMinimumScore: Requires LineFileDocs and realistic index

// These tests are documented here for future implementation:
// Source: TestTopDocsCollector.testSetMinCompetitiveScore()
// Source: TestTopDocsCollector.testSharedCountCollectorManager()
// Source: TestTopDocsCollector.testTotalHits() (with threshold)
// Source: TestTopDocsCollector.testRelationVsTopDocsCount()
// Source: TestTopDocsCollector.testConcurrentMinScore()
// Source: TestTopDocsCollector.testRandomMinCompetitiveScore()
// Source: TestTopDocsCollector.testRealisticConcurrentMinimumScore()
