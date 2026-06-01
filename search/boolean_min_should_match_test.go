// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: boolean_min_should_match_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestBooleanMinShouldMatch.java
// Purpose: Tests BooleanQuery.setMinimumNumberShouldMatch functionality

package search_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
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
	id      string
	all     string
	data    string
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
func setupTestIndex(t *testing.T) (index.IndexReaderInterface, *search.IndexSearcher) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	testData := getTestData()
	for _, td := range testData {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", td.id, true)
		doc.Add(idField)

		allField, _ := document.NewStringField("all", td.all, true)
		doc.Add(allField)

		if td.hasData {
			dataField, _ := document.NewTextField("data", td.data, true)
			doc.Add(dataField)
		}

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}

	searcher := search.NewIndexSearcher(reader)
	return reader, searcher
}

// verifyNrHits verifies that a query returns the expected number of hits
func verifyNrHits(t *testing.T, searcher *search.IndexSearcher, q search.Query, expected int) {
	t.Helper()

	topDocs, err := searcher.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if int(topDocs.TotalHits.Value) != expected {
		t.Errorf("Expected %d hits, got %d", expected, topDocs.TotalHits.Value)
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
	if top1.TotalHits.Value != top2.TotalHits.Value {
		t.Errorf("Expected same hit count, got %d and %d", top1.TotalHits.Value, top2.TotalHits.Value)
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
	if top2.TotalHits.Value > top1.TotalHits.Value {
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

// TestBooleanMinShouldMatch_RandomQueries ports testRandomQueries: it builds a
// random BooleanQuery tree q1 (unconstrained) and an identical q2 to which a
// random minimumNumberShouldMatch (and possibly a random negation) is applied,
// then asserts the constrained q2 result is a subset of q1 with matching scores.
func TestBooleanMinShouldMatch_RandomQueries(t *testing.T) {
	reader, searcher := setupTestIndex(t)
	defer reader.Close()

	field := "data"
	vals := []string{"1", "2", "3", "4", "5", "6", "A", "Z", "B", "Y", "Z", "X", "foo"}

	num := 20
	for i := 0; i < num; i++ {
		seed := int64(i*1009 + 17)
		lev := int(seed % 4)
		q1 := bmsmRandBoolQuery(rand.New(rand.NewSource(seed)), true, lev, field, vals)
		q2 := bmsmRandBoolQuery(rand.New(rand.NewSource(seed)), true, lev, field, vals)

		// minNrCB.postCreate, applied only to the top-level q2.
		cbRng := rand.New(rand.NewSource(seed ^ 0x5DEECE66D))
		opt := 0
		for _, c := range q2.Clauses() {
			if c.Occur == search.SHOULD {
				opt++
			}
		}
		q2.SetMinimumNumberShouldMatch(cbRng.Intn(opt + 2))
		if cbRng.Intn(2) == 1 {
			q2.Add(search.NewTermQuery(index.NewTerm(field, vals[cbRng.Intn(len(vals))])), search.MUST_NOT)
		}

		top1, err := searcher.Search(q1, 100)
		if err != nil {
			t.Fatalf("iter %d Search q1: %v", i, err)
		}
		top2, err := searcher.Search(q2, 100)
		if err != nil {
			t.Fatalf("iter %d Search q2: %v", i, err)
		}
		assertSubsetOfSameScores(t, q2, top1, top2)
	}
}

// assertSubsetOfSameScores ports assertSubsetOfSameScores: the constrained query
// (top2) must be a subset of the unconstrained query (top1), and every shared
// document must score within ulp(score) * numScoringClauses.
func assertSubsetOfSameScores(t *testing.T, q *search.BooleanQuery, top1, top2 *search.TopDocs) {
	t.Helper()
	if top2.TotalHits.Value > top1.TotalHits.Value {
		t.Fatalf("Constrained results not a subset: %d > %d", top2.TotalHits.Value, top1.TotalHits.Value)
	}

	numScoringClauses := 0
	for _, c := range q.Clauses() {
		if c.Occur == search.SHOULD || c.Occur == search.MUST {
			numScoringClauses++
		}
	}

	for hit := 0; hit < len(top2.ScoreDocs); hit++ {
		id := top2.ScoreDocs[hit].Doc
		score := top2.ScoreDocs[hit].Score
		found := false
		for other := 0; other < len(top1.ScoreDocs); other++ {
			if top1.ScoreDocs[other].Doc == id {
				found = true
				otherScore := top1.ScoreDocs[other].Score
				tolerance := bmsmUlp32(score) * float32(numScoringClauses)
				if math.Abs(float64(score-otherScore)) > float64(tolerance) {
					t.Errorf("Doc %d scores don't match: %v vs %v (tol %v)", id, score, otherScore, tolerance)
				}
			}
		}
		if !found {
			t.Errorf("Doc %d not found in unconstrained results", id)
		}
	}
}

// bmsmUlp32 returns the unit in the last place of x for float32, mirroring Math.ulp.
func bmsmUlp32(x float32) float32 {
	if x == 0 {
		return math.SmallestNonzeroFloat32
	}
	next := math.Nextafter32(x, float32(math.Inf(1)))
	return float32(math.Abs(float64(next - x)))
}

// bmsmRandBoolQuery is the local port of TestBoolean2.randBoolQuery used by the
// random-query test: it builds a reproducible random BooleanQuery tree.
func bmsmRandBoolQuery(rng *rand.Rand, allowMust bool, level int, field string, vals []string) *search.BooleanQuery {
	bq := search.NewBooleanQuery()
	numClauses := rng.Intn(len(vals)) + 1
	for i := 0; i < numClauses; i++ {
		var occur search.Occur
		switch rng.Intn(4) {
		case 0:
			if allowMust {
				occur = search.MUST
			} else {
				occur = search.SHOULD
			}
		case 1:
			occur = search.MUST_NOT
		default:
			occur = search.SHOULD
		}
		var q search.Query
		if level > 0 && rng.Intn(10) >= 5 {
			q = bmsmRandBoolQuery(rng, allowMust, level-1, field, vals)
		} else {
			q = search.NewTermQuery(index.NewTerm(field, vals[rng.Intn(len(vals))]))
		}
		bq.Add(q, occur)
	}
	return bq
}
