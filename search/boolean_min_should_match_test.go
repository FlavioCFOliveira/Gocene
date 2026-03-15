// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: boolean_min_should_match_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestBooleanMinShouldMatch.java
// Purpose: Tests BooleanQuery.setMinimumNumberShouldMatch functionality

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Test data from Java test:
// "A 1 2 3 4 5 6"
// "Z       4 5 6"
// null
// "B   2   4 5 6"
// "Y     3   5 6"
// null
// "C     3     6"
// "X       4 5 6"

// testDocument represents a document in the test index
type testDocument struct {
	id   string
	all  string
	data string
	hasData bool
}

// getTestData returns the test documents used in the Java test
func getTestData() []testDocument {
	return []testDocument{
		{id: "0", all: "all", data: "A 1 2 3 4 5 6", hasData: true},
		{id: "1", all: "all", data: "Z 4 5 6", hasData: true},
		{id: "2", all: "all", data: "", hasData: false},
		{id: "3", all: "all", data: "B 2 4 5 6", hasData: true},
		{id: "4", all: "all", data: "Y 3 5 6", hasData: true},
		{id: "5", all: "all", data: "", hasData: false},
		{id: "6", all: "all", data: "C 3 6", hasData: true},
		{id: "7", all: "all", data: "X 4 5 6", hasData: true},
	}
}

// setupTestIndex creates an in-memory index with test data
func setupTestIndex(t *testing.T) (search.IndexReader, *search.IndexSearcher) {
	dir := store.NewByteBuffersDirectory()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig())
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	testData := getTestData()
	for _, td := range testData {
		doc := document.NewDocument()
		doc.Add(document.NewStringField("id", td.id, true))
		doc.Add(document.NewStringField("all", td.all, true))
		if td.hasData {
			doc.Add(document.NewTextField("data", td.data, true))
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("Failed to get reader: %v", err)
	}

	searcher := search.NewIndexSearcher(reader)
	return reader, searcher
}

// verifyNrHits verifies that a query returns the expected number of hits
func verifyNrHits(t *testing.T, searcher *search.IndexSearcher, q search.Query, expected int) {
	t.Helper()

	// Test basic search
	topDocs, err := searcher.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if int(topDocs.TotalHits().Value()) != expected {
		t.Errorf("Expected %d hits, got %d", expected, topDocs.TotalHits().Value())
	}

	// Test with collector manager (equivalent to bs2 in Java)
	collector := search.NewTopScoreDocCollectorManager(1000, math.MaxInt32)
	topDocs2, err := searcher.SearchWithCollector(q, collector)
	if err != nil {
		t.Fatalf("Search with collector failed: %v", err)
	}

	if int(topDocs2.TotalHits().Value()) != expected {
		t.Errorf("Expected %d hits (collector), got %d", expected, topDocs2.TotalHits().Value())
	}
}

// TestBooleanMinShouldMatch_AllOptional tests all optional clauses with minShouldMatch
func TestBooleanMinShouldMatch_AllOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	for i := 1; i <= 4; i++ {
		bq.Add(search.NewTermQuery(index.NewTerm("data", string(rune('0'+i)))), search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(2) // match at least 2 of 4

	verifyNrHits(t, searcher, bq, 2)
}

// TestBooleanMinShouldMatch_OneReqAndSomeOptional tests one required clause with optional clauses
func TestBooleanMinShouldMatch_OneReqAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(2) // 2 of 3 optional

	verifyNrHits(t, searcher, bq, 5)
}

// TestBooleanMinShouldMatch_SomeReqAndSomeOptional tests multiple required clauses with optional clauses
func TestBooleanMinShouldMatch_SomeReqAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(2) // 2 of 3 optional

	verifyNrHits(t, searcher, bq, 5)
}

// TestBooleanMinShouldMatch_OneProhibAndSomeOptional tests one prohibited clause with optional clauses
func TestBooleanMinShouldMatch_OneProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(2) // 2 of 3 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_SomeProhibAndSomeOptional tests multiple prohibited clauses with optional clauses
func TestBooleanMinShouldMatch_SomeProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "C")), search.MUST_NOT)

	bq.SetMinimumNumberShouldMatch(2) // 2 of 3 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_OneReqOneProhibAndSomeOptional tests one required, one prohibited, and optional clauses
func TestBooleanMinShouldMatch_OneReqOneProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(3) // 3 of 4 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_SomeReqOneProhibAndSomeOptional tests multiple required, one prohibited, and optional clauses
func TestBooleanMinShouldMatch_SomeReqOneProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(3) // 3 of 4 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_OneReqSomeProhibAndSomeOptional tests one required, multiple prohibited, and optional clauses
func TestBooleanMinShouldMatch_OneReqSomeProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "C")), search.MUST_NOT)

	bq.SetMinimumNumberShouldMatch(3) // 3 of 4 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_SomeReqSomeProhibAndSomeOptional tests multiple required, multiple prohibited, and optional clauses
func TestBooleanMinShouldMatch_SomeReqSomeProhibAndSomeOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "C")), search.MUST_NOT)

	bq.SetMinimumNumberShouldMatch(3) // 3 of 4 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_MinHigherThanNumOptional tests when minShouldMatch exceeds optional clause count
func TestBooleanMinShouldMatch_MinHigherThanNumOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "5")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "4")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "C")), search.MUST_NOT)

	bq.SetMinimumNumberShouldMatch(90) // 90 of 4 optional - impossible

	verifyNrHits(t, searcher, bq, 0)
}

// TestBooleanMinShouldMatch_MinEqualToNumOptional tests when minShouldMatch equals optional clause count
func TestBooleanMinShouldMatch_MinEqualToNumOptional(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "6")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.SHOULD)

	bq.SetMinimumNumberShouldMatch(2) // 2 of 2 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_OneOptionalEqualToMin tests with one optional clause equal to min
func TestBooleanMinShouldMatch_OneOptionalEqualToMin(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "3")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.MUST)

	bq.SetMinimumNumberShouldMatch(1) // 1 of 1 optional

	verifyNrHits(t, searcher, bq, 1)
}

// TestBooleanMinShouldMatch_NoOptionalButMin tests with no optional clauses but minShouldMatch > 0
func TestBooleanMinShouldMatch_NoOptionalButMin(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.MUST)

	bq.SetMinimumNumberShouldMatch(1) // 1 of 0 optional

	verifyNrHits(t, searcher, bq, 0)
}

// TestBooleanMinShouldMatch_NoOptionalButMin2 tests with one required clause and minShouldMatch > 0
func TestBooleanMinShouldMatch_NoOptionalButMin2(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.MUST)

	bq.SetMinimumNumberShouldMatch(1) // 1 of 0 optional

	verifyNrHits(t, searcher, bq, 0)
}

// TestBooleanMinShouldMatch_RewriteMSM1 tests query rewrite with minShouldMatch=1
func TestBooleanMinShouldMatch_RewriteMSM1(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq1 := search.NewBooleanQuery()
	bq1.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)

	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq2.SetMinimumNumberShouldMatch(1)

	top1, err := searcher.Search(bq1, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	top2, err := searcher.Search(bq2, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Both should return the same results
	if top1.TotalHits().Value() != top2.TotalHits().Value() {
		t.Errorf("Expected same hit count, got %d and %d", top1.TotalHits().Value(), top2.TotalHits().Value())
	}
}

// TestBooleanMinShouldMatch_RewriteNegate tests query rewrite with negation
func TestBooleanMinShouldMatch_RewriteNegate(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	bq1 := search.NewBooleanQuery()
	bq1.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)

	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq2.Add(search.NewTermQuery(index.NewTerm("data", "Z")), search.MUST_NOT)

	top1, err := searcher.Search(bq1, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	top2, err := searcher.Search(bq2, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// bq2 should be a subset of bq1
	if top2.TotalHits().Value() > top1.TotalHits().Value() {
		t.Error("Constrained results should be a subset")
	}
}

// TestBooleanMinShouldMatch_FlattenInnerDisjunctions tests flattening of inner disjunctions
func TestBooleanMinShouldMatch_FlattenInnerDisjunctions(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	// Flattened version - should match 1 document
	bq1 := search.NewBooleanQuery()
	bq1.SetMinimumNumberShouldMatch(2)
	bq1.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.SHOULD)
	bq1.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)
	bq1.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.MUST)

	verifyNrHits(t, searcher, bq1, 1)

	// Nested version - inner query doesn't have minShouldMatch, so it matches more
	inner := search.NewBooleanQuery()
	inner.Add(search.NewTermQuery(index.NewTerm("all", "all")), search.SHOULD)
	inner.Add(search.NewTermQuery(index.NewTerm("data", "1")), search.SHOULD)

	bq2 := search.NewBooleanQuery()
	bq2.SetMinimumNumberShouldMatch(2)
	bq2.Add(inner, search.SHOULD)
	bq2.Add(search.NewTermQuery(index.NewTerm("data", "2")), search.MUST)

	verifyNrHits(t, searcher, bq2, 0)
}

// TestBooleanMinShouldMatch_Basics tests basic minShouldMatch functionality
func TestBooleanMinShouldMatch_Basics(t *testing.T) {
	bq := search.NewBooleanQuery()
	bq.SetMinimumNumberShouldMatch(2)

	if bq.MinimumNumberShouldMatch() != 2 {
		t.Errorf("Expected minShouldMatch=2, got %d", bq.MinimumNumberShouldMatch())
	}

	// Test that Clone preserves minShouldMatch
	cloned := bq.Clone().(*search.BooleanQuery)
	if cloned.MinimumNumberShouldMatch() != 2 {
		t.Error("Cloned query should preserve minShouldMatch")
	}

	// Test that Equals considers minShouldMatch
	bq2 := search.NewBooleanQuery()
	bq2.SetMinimumNumberShouldMatch(3)
	if bq.Equals(bq2) {
		t.Error("Queries with different minShouldMatch should not be equal")
	}
}

// TestBooleanMinShouldMatch_ClauseCounting tests that clauses are counted correctly
func TestBooleanMinShouldMatch_ClauseCounting(t *testing.T) {
	bq := search.NewBooleanQuery()

	// Add clauses of different types
	bq.Add(search.NewTermQuery(index.NewTerm("f", "v1")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm("f", "v2")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("f", "v3")), search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("f", "v4")), search.MUST_NOT)
	bq.Add(search.NewTermQuery(index.NewTerm("f", "v5")), search.FILTER)

	clauses := bq.Clauses()
	if len(clauses) != 5 {
		t.Errorf("Expected 5 clauses, got %d", len(clauses))
	}

	// Count SHOULD clauses
	shouldCount := 0
	for _, c := range clauses {
		if c.Occur == search.SHOULD {
			shouldCount++
		}
	}
	if shouldCount != 2 {
		t.Errorf("Expected 2 SHOULD clauses, got %d", shouldCount)
	}
}
