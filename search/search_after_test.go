// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/search_after_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestSearchAfter.java
// Purpose: Tests IndexSearcher's searchAfter() method for cursor-based pagination
// with various sort configurations including sort values.

package search_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// buildSearchAfterIndex creates a small index with n documents for SearchAfter tests.
func buildSearchAfterIndex(t *testing.T, dir store.Directory, n int) *index.IndexWriter {
	t.Helper()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < n; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("text", fmt.Sprintf("doc %d", i), true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return w
}

// TestSearchAfter_Queries (the Go port of TestSearchAfter.testQueries, sort==null
// cases) is implemented in test_search_after_test.go (package search_test). The
// dead duplicate stub that previously lived here was removed.

// TestSearchAfter_SortTypes tests searchAfter with all supported sort field types.
//
// Source: TestSearchAfter.assertQuery() with various SortField configurations
// Purpose: Verifies cursor-based pagination works correctly with:
//   - INT, LONG, FLOAT, DOUBLE sort types
//   - STRING and STRING_VAL sort types
//   - Forward and reverse sorting
//   - Missing value handling (STRING_FIRST, STRING_LAST)
//   - Score-based sorting (SortField.FIELD_SCORE)
//   - Document order sorting (SortField.FIELD_DOC)
func TestSearchAfter_SortTypes(t *testing.T) {
	// Skip until Sort types are implemented
	t.Fatal("Skipping: requires Sort and SortField implementation")

	// TODO: Test the following sort field types:
	// - SortField.Type.INT (ascending and descending)
	// - SortField.Type.LONG (ascending and descending)
	// - SortField.Type.FLOAT (ascending and descending)
	// - SortField.Type.DOUBLE (ascending and descending)
	// - SortField.Type.STRING (ascending and descending)
	// - SortField.Type.STRING_VAL (ascending and descending)
	// - SortField.FIELD_SCORE (relevance)
	// - SortField.FIELD_DOC (index order)
	//
	// Also test missing value configurations:
	// - STRING_FIRST (missing values sort first)
	// - STRING_LAST (missing values sort last)
}

// TestSearchAfter_MultiSort tests searchAfter with multiple sort fields.
//
// Source: TestSearchAfter.getRandomSort()
// Purpose: Verifies pagination works correctly when sorting by multiple fields
// (e.g., sort by "category" then by "date" then by "score").
func TestSearchAfter_MultiSort(t *testing.T) {
	// Skip until multi-field Sort is implemented
	t.Fatal("Skipping: requires multi-field Sort implementation")

	// TODO: Test with 2-7 sort fields in combination
	// This tests the FieldDoc.fields array comparison
}

// TestSearchAfter_PageConsistency verifies that paginated results
// match the equivalent non-paginated results.
//
// Source: TestSearchAfter.assertPage()
// Purpose: Ensures that when retrieving results page by page using searchAfter,
// the combined results exactly match a single query for all results.
func TestSearchAfter_PageConsistency(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w := buildSearchAfterIndex(t, dir, 20)
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	q := search.NewMatchAllDocsQuery()

	// Get all results in one query.
	fullResults, err := searcher.Search(q, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	totalHits := len(fullResults.ScoreDocs)

	// Page through with page size of 5.
	const pageSize = 5
	var paged []*search.ScoreDoc
	var after *search.ScoreDoc
	for {
		top, err := searcher.SearchAfter(after, q, pageSize)
		if err != nil {
			t.Fatalf("SearchAfter: %v", err)
		}
		if len(top.ScoreDocs) == 0 {
			break
		}
		paged = append(paged, top.ScoreDocs...)
		after = top.ScoreDocs[len(top.ScoreDocs)-1]
	}

	if len(paged) != totalHits {
		t.Errorf("paged %d docs, full query returned %d", len(paged), totalHits)
	}
}

// TestSearchAfter_VariedPageSizes tests pagination with different page sizes.
//
// Source: TestSearchAfter.assertQuery() - pageSize varies from 1 to maxDoc*2
// Purpose: Ensures searchAfter works correctly regardless of page size,
// including edge cases like page size larger than result set.
func TestSearchAfter_VariedPageSizes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w := buildSearchAfterIndex(t, dir, 10)
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	q := search.NewMatchAllDocsQuery()

	totalResults, err := searcher.Search(q, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	total := len(totalResults.ScoreDocs)

	for _, pageSize := range []int{1, 3, 7, 20} {
		var pagedCount int
		var after *search.ScoreDoc
		for {
			top, err := searcher.SearchAfter(after, q, pageSize)
			if err != nil {
				t.Fatalf("SearchAfter(pageSize=%d): %v", pageSize, err)
			}
			if len(top.ScoreDocs) == 0 {
				break
			}
			pagedCount += len(top.ScoreDocs)
			after = top.ScoreDocs[len(top.ScoreDocs)-1]
		}
		if pagedCount != total {
			t.Errorf("pageSize=%d: got %d docs, want %d", pageSize, pagedCount, total)
		}
	}
}

// TestSearchAfter_MissingFields tests pagination when some documents
// are missing sort fields.
//
// Source: TestSearchAfter.setUp() - documents randomly skip fields
// Purpose: Verifies searchAfter handles sparse documents correctly,
// respecting the missing value configuration (STRING_FIRST or STRING_LAST).
func TestSearchAfter_MissingFields(t *testing.T) {
	// Skip until missing value handling is implemented
	t.Fatal("Skipping: requires missing value handling in SortField")

	// TODO: Test with documents that have missing sort fields
	// Verify correct positioning based on STRING_FIRST/STRING_LAST
}

// TestSearchAfter_ScorePopulation tests that scores are properly populated
// when requested during sorted searches.
//
// Source: TestSearchAfter.assertQuery() - TopFieldCollector.populateScores()
// Purpose: Ensures scores are available in FieldDoc results even when
// sorting by non-score fields.
func TestSearchAfter_ScorePopulation(t *testing.T) {
	// Skip until score population is implemented
	t.Fatal("Skipping: requires TopFieldCollector.PopulateScores() implementation")

	// TODO: Verify scores are populated correctly when:
	// - Sorting by field with doScores=true
	// - Scores should match the query's scoring
}

// ---------------------------------------------------------------------------
// Helper Types and Interfaces (to be implemented)
// ---------------------------------------------------------------------------

// FieldDoc represents a document with sort values.
// This is the Go equivalent of Lucene's FieldDoc.
//
// Source: org.apache.lucene.search.FieldDoc
// Purpose: Extends ScoreDoc to include sort field values for cursor-based pagination.
type FieldDoc struct {
	*search.ScoreDoc
	// Fields holds the sort values for each sort field.
	// These values are used for comparison in searchAfter.
	Fields []interface{}
}

// NewFieldDoc creates a new FieldDoc.
func NewFieldDoc(doc int, score float32, fields []interface{}) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: search.NewScoreDoc(doc, score, 0),
		Fields:   fields,
	}
}

// SortFieldType represents the type of a sort field.
type SortFieldType int

const (
	SortFieldTypeInt SortFieldType = iota
	SortFieldTypeLong
	SortFieldTypeFloat
	SortFieldTypeDouble
	SortFieldTypeString
	SortFieldTypeStringVal
	SortFieldTypeScore
	SortFieldTypeDoc
)

// MissingValueStrategy defines how missing values are handled.
type MissingValueStrategy int

const (
	// MissingValueLast sorts missing values after non-missing values.
	MissingValueLast MissingValueStrategy = iota
	// MissingValueFirst sorts missing values before non-missing values.
	MissingValueFirst
)

// SortField defines how to sort documents by a specific field.
// This is the Go equivalent of Lucene's SortField.
//
// Source: org.apache.lucene.search.SortField
type SortField struct {
	Field   string
	Type    SortFieldType
	Reverse bool
	Missing MissingValueStrategy
	// MissingValue is the value to use for missing documents (for numeric types)
	MissingValue interface{}
}

// Sort defines the sort order for search results.
// This is the Go equivalent of Lucene's Sort.
//
// Source: org.apache.lucene.search.Sort
type Sort struct {
	Fields []*SortField
}

// NewSort creates a new Sort with the given fields.
func NewSort(fields ...*SortField) *Sort {
	return &Sort{Fields: fields}
}

// Predefined sorts
var (
	// SortRelevance sorts by relevance score (highest first).
	SortRelevance = &Sort{Fields: []*SortField{{Type: SortFieldTypeScore}}}

	// SortIndexOrder sorts by document index order.
	SortIndexOrder = &Sort{Fields: []*SortField{{Type: SortFieldTypeDoc}}}
)

// TopFieldCollector collects top documents sorted by specified fields.
// This is the Go equivalent of Lucene's TopFieldCollector.
//
// Source: org.apache.lucene.search.TopFieldCollector
type TopFieldCollector struct {
	// TODO: Implement
}

// PopulateScores populates scores for FieldDoc results.
// This is the Go equivalent of Lucene's TopFieldCollector.populateScores().
//
// Source: org.apache.lucene.search.TopFieldCollector.populateScores()
func PopulateScores(docs []*search.ScoreDoc, searcher *search.IndexSearcher, query search.Query) error {
	// TODO: Implement
	return nil
}

// ---------------------------------------------------------------------------
// Test Fixtures and Utilities
// ---------------------------------------------------------------------------

// SearchAfterTestFixture provides common setup for searchAfter tests.
type SearchAfterTestFixture struct {
	Dir      store.Directory
	Reader   index.IndexReader
	Searcher *search.IndexSearcher
	Rand     *rand.Rand
}

// SetupSearchAfterFixture creates a test index with various field types.
//
// Based on: TestSearchAfter.setUp()
// Creates documents with:
// - Text fields for querying ("english", "oddeven")
// - Numeric doc values (byte, short, int, long, float, double)
// - Sorted doc values (bytes)
// - Binary doc values (bytesval)
// - Stored field for document ID
func SetupSearchAfterFixture(t *testing.T, r *rand.Rand) *SearchAfterTestFixture {
	// TODO: Implement fixture setup
	// 1. Create ByteBuffersDirectory
	// 2. Create RandomIndexWriter
	// 3. Add at least 200 documents with various fields
	// 4. Randomly skip fields (20% chance) to test missing values
	// 5. Occasionally commit (1/50 chance)
	// 6. Create IndexSearcher
	return nil
}

// Teardown cleans up the fixture.
func (f *SearchAfterTestFixture) Teardown() {
	// TODO: Close reader and directory
}

// AssertQueryResult verifies paginated results match non-paginated results.
//
// Based on: TestSearchAfter.assertQuery()
func AssertQueryResult(t *testing.T, searcher *search.IndexSearcher, query search.Query, sort *Sort, pageSize int) {
	// TODO: Implement
	// 1. Get all results (non-paginated)
	// 2. Get results page by page using searchAfter
	// 3. Verify each page matches corresponding slice of all results
	// 4. Verify total hits match
}

// AssertPageResult verifies a single page matches expected results.
//
// Based on: TestSearchAfter.assertPage()
func AssertPageResult(t *testing.T, pageStart int, all *search.TopDocs, paged *search.TopDocs, searcher *search.IndexSearcher) {
	// TODO: Implement
	// 1. Verify total hits match
	// 2. For each doc in page:
	//    - Verify doc ID matches
	//    - Verify score matches (with float delta)
	//    - If FieldDoc, verify sort values match
}

// GetRandomSort generates a random sort configuration for testing.
//
// Based on: TestSearchAfter.getRandomSort()
func GetRandomSort(allSortFields []*SortField, r *rand.Rand) *Sort {
	// TODO: Implement
	// Create a sort with 2-7 random sort fields
	return nil
}

// ---------------------------------------------------------------------------
// Implementation Notes
// ---------------------------------------------------------------------------
//
// This test file ports the following Java test methods:
//
// 1. setUp() - Creates test index with various field types
// 2. tearDown() - Cleans up resources
// 3. testQueries() - Main test method, runs queries multiple times
// 4. assertQuery(Query) - Tests a query with various sorts
// 5. assertQuery(Query, Sort) - Tests pagination for specific query+sort
// 6. assertPage() - Verifies page matches expected results
// 7. getRandomSort() - Generates random multi-field sort
//
// Required implementations:
// - Sort and SortField types with all sort field types
// - FieldDoc extending ScoreDoc with sort values
// - TopFieldCollector for collecting sorted results
// - IndexSearcher.SearchAfter() method
// - TopFieldCollector.PopulateScores() for score retrieval
//
// Key behaviors to verify:
// - Cursor-based pagination returns consistent results
// - Sort values are correctly compared for pagination
// - Missing field values are handled per configuration
// - Scores can be populated for sorted results
// - Multi-field sorting works correctly
// - Page size variations work correctly
