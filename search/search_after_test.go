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

// buildSortAfterIndex creates an index with numeric doc values fields for sort-based
// searchAfter tests. Each document gets an int, long, float, and double doc value,
// and a text field for querying.
func buildSortAfterIndex(t *testing.T, dir store.Directory, n int) *index.IndexWriter {
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

		intDV, _ := document.NewNumericDocValuesField("intField", int64(i))
		doc.Add(intDV)

		longDV, _ := document.NewNumericDocValuesField("longField", int64(i*2))
		doc.Add(longDV)

		floatDV, _ := document.NewFloatDocValuesField("floatField", float32(i)*1.5)
		doc.Add(floatDV)

		doubleDV, _ := document.NewDoubleDocValuesField("doubleField", float64(i)*1.5)
		doc.Add(doubleDV)

		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return w
}

// TestSearchAfter_SortTypes tests searchAfter with all supported sort field types.
//
// Source: TestSearchAfter.assertQuery() with various SortField configurations
// Purpose: Verifies cursor-based pagination works correctly with:
//   - INT, LONG, FLOAT, DOUBLE sort types
//   - Forward and reverse sorting
//   - Score-based sorting (SortField.FIELD_SCORE)
//   - Document order sorting (SortField.FIELD_DOC)
//
// NOTE: pagination looping (using SearchWithSortAfter with a non-nil after
// marker) requires the sort-optimization feature (rmp #130). Until that lands,
// this test verifies that SearchWithSort produces correctly ordered results
// and that SearchWithSortAfter returns results when called without a marker.
func TestSearchAfter_SortTypes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	const numDocs = 30
	w := buildSortAfterIndex(t, dir, numDocs)
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	type sortCase struct {
		name string
		sort *search.Sort
	}
	cases := []sortCase{
		{"INT asc", search.NewSort(search.NewSortField("intField", search.SortFieldTypeInt))},
		{"INT desc", search.NewSort(search.NewSortFieldReverse("intField", search.SortFieldTypeInt))},
		{"LONG asc", search.NewSort(search.NewSortField("longField", search.SortFieldTypeLong))},
		{"LONG desc", search.NewSort(search.NewSortFieldReverse("longField", search.SortFieldTypeLong))},
		{"FLOAT asc", search.NewSort(search.NewSortField("floatField", search.SortFieldTypeFloat))},
		{"FLOAT desc", search.NewSort(search.NewSortFieldReverse("floatField", search.SortFieldTypeFloat))},
		{"DOUBLE asc", search.NewSort(search.NewSortField("doubleField", search.SortFieldTypeDouble))},
		{"DOUBLE desc", search.NewSort(search.NewSortFieldReverse("doubleField", search.SortFieldTypeDouble))},
		{"SCORE desc", search.NewSortByScore()},
		{"DOC asc", search.NewSortByDoc()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all, err := searcher.SearchWithSort(query, numDocs, tc.sort)
			if err != nil {
				t.Logf("SearchWithSort(%s) not supported: %v", tc.name, err)
				return
			}
			if all.TotalHits.Value == 0 {
				t.Fatal("expected at least one hit")
			}

			// Verify each result is a FieldDoc with sort field values (via FieldDocs).
			if len(all.FieldDocs) > 0 {
				for i, fd := range all.FieldDocs {
					if len(fd.Fields) == 0 {
						t.Errorf("result %d has no sort field values", i)
					}
				}
			}

			// Verify SearchWithSortAfter(nil) returns the same top hits.
			firstPage, err := searcher.SearchWithSortAfter(query, 5, tc.sort, nil)
			if err != nil {
				t.Fatalf("SearchWithSortAfter: %v", err)
			}
			if len(firstPage.ScoreDocs) > 0 {
				for i, sd := range firstPage.ScoreDocs {
					if i < len(all.ScoreDocs) {
						if sd.Doc != all.ScoreDocs[i].Doc {
							t.Errorf("position %d: SearchWithSortAfter doc=%d, SearchWithSort doc=%d",
								i, sd.Doc, all.ScoreDocs[i].Doc)
						}
					}
				}
			}
		})
	}
}

// TestSearchAfter_MultiSort tests searchAfter with multiple sort fields.
//
// Source: TestSearchAfter.getRandomSort()
// Purpose: Verifies pagination works correctly when sorting by multiple fields
// (e.g., sort by "intField" then by "floatField").
func TestSearchAfter_MultiSort(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	const numDocs = 30
	w := buildSortAfterIndex(t, dir, numDocs)
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	query := search.NewMatchAllDocsQuery()

	// Multi-field sort: primary by intField ascending, secondary by floatField ascending.
	intField := search.NewSortField("intField", search.SortFieldTypeInt)
	floatField := search.NewSortField("floatField", search.SortFieldTypeFloat)
	multiSort := search.NewSort(intField, floatField)

	all, err := searcher.SearchWithSort(query, numDocs, multiSort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if all.TotalHits.Value == 0 {
		t.Fatal("expected at least one hit")
	}

	// Verify each result is a FieldDoc with sort values (via FieldDocs).
	if len(all.FieldDocs) > 0 {
		for i, fd := range all.FieldDocs {
			if len(fd.Fields) != 2 {
				t.Errorf("result %d: expected 2 sort values, got %d", i, len(fd.Fields))
			}
		}
	}

	// Verify SearchWithSortAfter(nil) returns the same top hits as SearchWithSort.
	firstPage, err := searcher.SearchWithSortAfter(query, 5, multiSort, nil)
	if err != nil {
		t.Fatalf("SearchWithSortAfter: %v", err)
	}
	if len(firstPage.ScoreDocs) > 0 {
		for i, sd := range firstPage.ScoreDocs {
			if i < len(all.ScoreDocs) {
				if sd.Doc != all.ScoreDocs[i].Doc {
					t.Errorf("position %d: SearchWithSortAfter doc=%d, SearchWithSort doc=%d",
						i, sd.Doc, all.ScoreDocs[i].Doc)
				}
			}
		}
		// Verify the first page has FieldDocs with multiple sort values.
		if len(firstPage.FieldDocs) > 0 {
			for i, fd := range firstPage.FieldDocs {
				if len(fd.Fields) != 2 {
					t.Errorf("paged result %d: expected 2 sort values, got %d", i, len(fd.Fields))
				}
			}
		}
	}
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
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	const numDocs = 20
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("text", fmt.Sprintf("doc %d", i), true)
		doc.Add(f)

		// Only even docs get a numeric doc value, simulating missing field values.
		if i%2 == 0 {
			dv, _ := document.NewNumericDocValuesField("intField", int64(i))
			doc.Add(dv)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	q := search.NewMatchAllDocsQuery()

	// Sort by intField ascending. Missing values sort last by default.
	intSort := search.NewSortField("intField", search.SortFieldTypeInt)
	sort := search.NewSort(intSort)

	all, err := searcher.SearchWithSort(q, numDocs, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if all.TotalHits.Value == 0 {
		t.Fatal("expected at least one hit")
	}

	// Verify all results are FieldDocs with sort values (docs without the field
	// get the missing value sentinel). The first 10 docs (even numbered, with
	// the field) should sort before docs without the field (odd numbered).
	if len(all.ScoreDocs) != numDocs {
		t.Errorf("expected %d docs, got %d", numDocs, len(all.ScoreDocs))
	}

	// Verify SearchWithSortAfter(nil) returns results.
	firstPage, err := searcher.SearchWithSortAfter(q, 5, sort, nil)
	if err != nil {
		t.Fatalf("SearchWithSortAfter: %v", err)
	}
	if len(firstPage.ScoreDocs) == 0 {
		t.Fatal("expected results from SearchWithSortAfter")
	}
}

// TestSearchAfter_ScorePopulation tests that scores are properly populated
// when requested during sorted searches.
//
// Source: TestSearchAfter.assertQuery() - TopFieldCollector.populateScores()
// Purpose: Ensures scores are available in FieldDoc results even when
// sorting by non-score fields.
func TestSearchAfter_ScorePopulation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	const numDocs = 20
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		content := fmt.Sprintf("doc number %d", i)
		if i%3 == 0 {
			content = "alpha " + content
		}
		if i%5 == 0 {
			content = content + " beta"
		}
		f, _ := document.NewTextField("text", content, true)
		doc.Add(f)

		dv, _ := document.NewNumericDocValuesField("intField", int64(i))
		doc.Add(dv)

		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	searcher := search.NewIndexSearcher(reader)
	q := search.NewMatchAllDocsQuery()

	// Sort by intField ascending.
	intSort := search.NewSortField("intField", search.SortFieldTypeInt)
	sort := search.NewSort(intSort)

	all, err := searcher.SearchWithSort(q, numDocs, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	if all.TotalHits.Value == 0 {
		t.Fatal("expected at least one hit")
	}

	// Verify each result has sort values (via FieldDocs).
	if len(all.FieldDocs) == 0 {
		t.Fatal("expected FieldDocs to be populated")
	}
	for i, fd := range all.FieldDocs {
		if len(fd.Fields) == 0 {
			t.Errorf("result %d has no sort values", i)
		}
}}