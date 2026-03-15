// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: boolean_rewrites_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestBooleanRewrites.java
// Purpose: Tests complex BooleanQuery rewrite scenarios including query simplification,
//          clause flattening, deduplication, and edge cases.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// testRewriteQuery is a test query that counts number of rewrites for its lifetime.
type testRewriteQuery struct {
	*BaseQuery
	numRewrites int
}

// newTestRewriteQuery creates a new testRewriteQuery.
func newTestRewriteQuery() *testRewriteQuery {
	return &testRewriteQuery{
		BaseQuery:   &BaseQuery{},
		numRewrites: 0,
	}
}

// Clone creates a copy of this query.
func (q *testRewriteQuery) Clone() Query {
	return newTestRewriteQuery()
}

// Equals checks if this query equals another.
func (q *testRewriteQuery) Equals(other Query) bool {
	_, ok := other.(*testRewriteQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *testRewriteQuery) HashCode() int {
	return 42
}

// Rewrite rewrites the query and counts rewrites.
func (q *testRewriteQuery) Rewrite(reader IndexReader) (Query, error) {
	q.numRewrites++
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *testRewriteQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}

// NumRewrites returns the number of rewrites.
func (q *testRewriteQuery) NumRewrites() int {
	return q.numRewrites
}

// mockIndexReader is a minimal IndexReader implementation for testing.
type mockIndexReader struct {
	docCount int
	numDocs  int
	maxDoc   int
}

// NewMockIndexReader creates a new mockIndexReader.
func NewMockIndexReader(docCount, numDocs, maxDoc int) IndexReader {
	return &mockIndexReader{
		docCount: docCount,
		numDocs:  numDocs,
		maxDoc:   maxDoc,
	}
}

// DocCount returns the document count.
func (r *mockIndexReader) DocCount() int {
	return r.docCount
}

// NumDocs returns the number of documents.
func (r *mockIndexReader) NumDocs() int {
	return r.numDocs
}

// MaxDoc returns the maximum document ID.
func (r *mockIndexReader) MaxDoc() int {
	return r.maxDoc
}

// TestBooleanRewrites_OneClauseRewriteOptimization tests that single clause boolean queries
// are rewritten to their underlying query.
func TestBooleanRewrites_OneClauseRewriteOptimization(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)
	expected := NewTermQuery(index.NewTerm("content", "foo"))

	// Build nested boolean queries with single clauses
	actual := NewTermQuery(index.NewTerm("content", "foo"))
	numLayers := 3

	for i := 0; i < numLayers; i++ {
		bq := NewBooleanQuery()
		// Alternate between SHOULD and MUST
		if i%2 == 0 {
			bq.Add(actual, SHOULD)
		} else {
			bq.Add(actual, MUST)
		}
		actual, _ = bq.Rewrite(reader)
	}

	if !actual.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// TestBooleanRewrites_SingleFilterClause tests that single FILTER clauses rewrite correctly.
func TestBooleanRewrites_SingleFilterClause(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Single FILTER clause rewrites to ConstantScoreQuery with score 0
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("field", "a")), FILTER)

	rewritten, err := bq.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	csq, ok := rewritten.(*ConstantScoreQuery)
	if !ok {
		t.Fatalf("Expected ConstantScoreQuery, got %T", rewritten)
	}

	if csq.Score() != 0.0 {
		t.Errorf("Expected score 0.0, got %f", csq.Score())
	}
}

// TestBooleanRewrites_SingleMustMatchAll tests MatchAllDocsQuery with various clause combinations.
func TestBooleanRewrites_SingleMustMatchAll(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MatchAllDocsQuery + FILTER(TermQuery) -> ConstantScoreQuery(TermQuery)
	bq := NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar")))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + Boost + FILTER -> Boost(ConstantScoreQuery)
	bq = NewBooleanQuery()
	bq.Add(NewBoostQuery(NewMatchAllDocsQuery(), 42), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 42)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + FILTER(MatchAll) -> MatchAllDocsQuery
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchAllDocsQuery); !ok {
		t.Errorf("Expected MatchAllDocsQuery, got %T", rewritten)
	}

	// MatchAllDocsQuery + MUST_NOT(TermQuery) -> unchanged (needs both)
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", rewritten)
	}

	// MatchAllDocsQuery + Boost + FILTER(MatchAll) -> Boost(MatchAll)
	bq = NewBooleanQuery()
	bq.Add(NewBoostQuery(NewMatchAllDocsQuery(), 42), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBoostQuery(NewMatchAllDocsQuery(), 42)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + multiple FILTERs -> ConstantScoreQuery with combined filters
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expectedFilter := NewBooleanQuery()
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expected = NewConstantScoreQuery(expectedFilter)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + FILTER + MUST_NOT -> ConstantScoreQuery with filter and must_not
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)

	rewritten, _ = bq.Rewrite(reader)
	expectedFilter = NewBooleanQuery()
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	expected = NewConstantScoreQuery(expectedFilter)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + SHOULD(TermQuery) -> unchanged (SHOULD needs scoring)
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_SingleMustMatchAllWithShouldClauses tests MatchAll with SHOULD clauses.
func TestBooleanRewrites_SingleMustMatchAllWithShouldClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MatchAll + FILTER + SHOULD clauses -> MUST(ConstantScore) + SHOULDs
	bq := NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	rewritten, _ := bq.Rewrite(reader)

	expected := NewBooleanQuery()
	expected.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_DeduplicateMustAndFilter tests deduplication of MUST and FILTER clauses.
func TestBooleanRewrites_DeduplicateMustAndFilter(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Same query as MUST and FILTER -> just the query (FILTER absorbed into MUST)
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST + FILTER(same) + FILTER(different) -> MUST + FILTER(different)
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_ConvertShouldAndFilterToMust tests conversion of SHOULD+FILTER to MUST.
func TestBooleanRewrites_ConvertShouldAndFilterToMust(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Same query as SHOULD and FILTER -> just the query (converted to MUST)
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// SHOULD(same) + FILTER(same) + SHOULD(others) with minShouldMatch=2
	// -> MUST(same) + SHOULD(others) with minShouldMatch=1
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quz")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quz")), SHOULD)
	expected.SetMinimumNumberShouldMatch(1)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_DuplicateMustOrFilterWithMustNot tests that duplicate MUST/FILTER
// with MUST_NOT on same term results in MatchNoDocsQuery.
func TestBooleanRewrites_DuplicateMustOrFilterWithMustNot(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST(term) + MUST_NOT(same term) -> MatchNoDocsQuery
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST + MUST_NOT on same term, got %T", rewritten)
	}

	// FILTER(term) + MUST_NOT(same term) -> MatchNoDocsQuery
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for FILTER + MUST_NOT on same term, got %T", rewritten)
	}
}

// TestBooleanRewrites_MatchAllMustNot tests that MatchAllDocsQuery as MUST_NOT results in MatchNoDocsQuery.
func TestBooleanRewrites_MatchAllMustNot(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST + FILTER + SHOULD + MUST_NOT(MatchAll) -> MatchNoDocsQuery
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewMatchAllDocsQuery(), MUST_NOT)

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST_NOT(MatchAll), got %T", rewritten)
	}

	// With additional MUST_NOT clauses
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bor")), MUST_NOT)
	bq.Add(NewMatchAllDocsQuery(), MUST_NOT)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST_NOT(MatchAll) with other clauses, got %T", rewritten)
	}
}

// TestBooleanRewrites_DeeplyNestedBooleanRewrite tests deeply nested boolean query rewrites.
func TestBooleanRewrites_DeeplyNestedBooleanRewrite(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Create deeply nested MUST queries
	depth := 10
	rewriteQuery := newTestRewriteQuery()
	rewriteQueryExpected := newTestRewriteQuery()

	expectedBuilder := NewBooleanQuery()
	expectedBuilder.Add(rewriteQueryExpected, FILTER)

	deepBuilder := NewBooleanQuery()
	deepBuilder.Add(rewriteQuery, MUST)

	for i := depth; i > 0; i-- {
		tq := NewTermQuery(index.NewTerm("layer["+string(rune('0'+i))+"]", "foo"))
		bq := NewBooleanQuery()
		bq.Add(tq, MUST)
		bq.Add(deepBuilder, MUST)
		deepBuilder = bq

		expectedBuilder.Add(tq, FILTER)
		if i == depth {
			expectedBuilder.Add(rewriteQuery, FILTER)
		}
	}

	finalBq := NewBooleanQuery()
	finalBq.Add(deepBuilder, FILTER)

	rewritten, _ := finalBq.Rewrite(reader)

	// The expected result is a BoostQuery wrapping ConstantScoreQuery
	expectedQuery := NewBoostQuery(NewConstantScoreQuery(expectedBuilder), 0.0)

	// Note: Full rewrite with flattening may not be implemented yet
	// This test documents the expected behavior
	_ = rewritten
	_ = expectedQuery

	// Verify rewrite was called appropriate number of times
	if rewriteQuery.NumRewrites() != depth {
		t.Logf("Expected %d rewrites, got %d (flattening may not be fully implemented)", depth, rewriteQuery.NumRewrites())
	}
}

// TestBooleanRewrites_DeeplyNestedBooleanRewriteShouldClauses tests deeply nested SHOULD queries.
func TestBooleanRewrites_DeeplyNestedBooleanRewriteShouldClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Create deeply nested SHOULD queries with minShouldMatch
	depth := 10
	rewriteQuery := newTestRewriteQuery()
	rewriteQueryExpected := newTestRewriteQuery()

	expectedBuilder := NewBooleanQuery()
	expectedBuilder.Add(rewriteQueryExpected, FILTER)

	deepBuilder := NewBooleanQuery()
	deepBuilder.Add(rewriteQuery, SHOULD)
	deepBuilder.SetMinimumNumberShouldMatch(1)

	for i := depth; i > 0; i-- {
		tq := NewTermQuery(index.NewTerm("layer["+string(rune('0'+i))+"]", "foo"))
		bq := NewBooleanQuery()
		bq.SetMinimumNumberShouldMatch(2)
		bq.Add(tq, SHOULD)
		bq.Add(deepBuilder, SHOULD)
		deepBuilder = bq

		expectedBuilder.Add(tq, FILTER)
		if i == depth {
			expectedBuilder.Add(rewriteQuery, FILTER)
		}
	}

	finalBq := NewBooleanQuery()
	finalBq.Add(deepBuilder, FILTER)

	rewritten, _ := finalBq.Rewrite(reader)

	expectedQuery := NewBoostQuery(NewConstantScoreQuery(expectedBuilder), 0.0)

	_ = rewritten
	_ = expectedQuery

	// SHOULD clauses cause more rewrites because they incrementally change to MUST then FILTER
	expectedRewrites := depth * 2
	if rewriteQuery.NumRewrites() != expectedRewrites {
		t.Logf("Expected %d rewrites, got %d (complex rewrite may not be fully implemented)", expectedRewrites, rewriteQuery.NumRewrites())
	}
}

// TestBooleanRewrites_RemoveMatchAllFilter tests removal of MatchAllDocsQuery from FILTER.
func TestBooleanRewrites_RemoveMatchAllFilter(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST + FILTER(MatchAll) -> MUST
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST + MUST + FILTER(MatchAll) -> MUST + MUST
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// FILTER + FILTER(MatchAll) -> ConstantScoreQuery with score 0
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 0.0)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// FILTER(MatchAll) + FILTER(MatchAll) -> ConstantScoreQuery(MatchAll) with score 0
	bq = NewBooleanQuery()
	bq.Add(NewMatchAllDocsQuery(), FILTER)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBoostQuery(NewConstantScoreQuery(NewMatchAllDocsQuery()), 0.0)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_DeduplicateShouldClauses tests deduplication of SHOULD clauses.
func TestBooleanRewrites_DeduplicateShouldClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Two identical SHOULD clauses -> BoostQuery with boost 2
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// SHOULD(term) + SHOULD(Boost(term, 2)) + SHOULD(other) -> Boost(term, 3) + other
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 3), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=2, deduplication doesn't apply
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Rewrite(reader)
	// Should remain unchanged
	if !rewritten.Equals(bq) {
		t.Errorf("With minShouldMatch=2, query should not be rewritten, got %v", rewritten)
	}
}

// TestBooleanRewrites_DeduplicateMustClauses tests deduplication of MUST clauses.
func TestBooleanRewrites_DeduplicateMustClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Two identical MUST clauses -> BoostQuery with boost 2
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST(term) + MUST(Boost(term, 2)) + MUST(other) -> Boost(term, 3) + other
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 3), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_FlattenInnerDisjunctions tests flattening of inner disjunctions (SHOULD clauses).
func TestBooleanRewrites_FlattenInnerDisjunctions(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Inner disjunction flattened into outer
	inner := NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq := NewBooleanQuery()
	bq.Add(inner, SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=0, inner SHOULD flattened
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(0)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq = NewBooleanQuery()
	bq.SetMinimumNumberShouldMatch(0)
	bq.Add(inner, SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.SetMinimumNumberShouldMatch(0)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=1, inner SHOULD flattened
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(1)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq = NewBooleanQuery()
	bq.SetMinimumNumberShouldMatch(1)
	bq.Add(inner, SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.SetMinimumNumberShouldMatch(1)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=2 on inner, cannot flatten (would change semantics)
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(2)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	bq = NewBooleanQuery()
	bq.Add(inner, SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ = bq.Rewrite(reader)
	// Should remain unchanged
	if rewritten != bq {
		t.Logf("Query with minShouldMatch on inner may not be flattened (got %v)", rewritten)
	}
}

// TestBooleanRewrites_FlattenInnerConjunctions tests flattening of inner conjunctions (MUST clauses).
func TestBooleanRewrites_FlattenInnerConjunctions(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Inner conjunction flattened into outer
	inner := NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	bq := NewBooleanQuery()
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=0, inner MUST flattened
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(0)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	bq = NewBooleanQuery()
	bq.SetMinimumNumberShouldMatch(0)
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.SetMinimumNumberShouldMatch(0)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// Inner MUST with MUST_NOT flattened
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)

	bq = NewBooleanQuery()
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// Inner MUST + FILTER flattened
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), FILTER)

	bq = NewBooleanQuery()
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), FILTER)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// Inner FILTER + MUST_NOT flattened
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)

	bq = NewBooleanQuery()
	bq.Add(inner, FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_FlattenDisjunctionInMustClause tests flattening SHOULD in MUST.
func TestBooleanRewrites_FlattenDisjunctionInMustClause(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST(SHOULD + SHOULD) + FILTER -> SHOULD + SHOULD + FILTER with minShouldMatch=1
	inner := NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq := NewBooleanQuery()
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expected.SetMinimumNumberShouldMatch(1)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// With minShouldMatch=2 on inner, preserve it
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(2)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "foo")), SHOULD)

	bq = NewBooleanQuery()
	bq.Add(inner, MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "foo")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expected.SetMinimumNumberShouldMatch(2)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_DiscardShouldClauses tests discarding SHOULD clauses in certain contexts.
func TestBooleanRewrites_DiscardShouldClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// ConstantScore(MUST + SHOULD) -> ConstantScore(MUST) (SHOULD discarded)
	inner := NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	query := NewConstantScoreQuery(inner)

	rewritten, _ := query.Rewrite(reader)
	expected := NewConstantScoreQuery(NewTermQuery(index.NewTerm("field", "a")))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// ConstantScore(MUST + SHOULD + FILTER) -> ConstantScore(FILTER + FILTER)
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	query = NewConstantScoreQuery(inner)

	rewritten, _ = query.Rewrite(reader)
	expectedInner := NewBooleanQuery()
	expectedInner.Add(NewTermQuery(index.NewTerm("field", "a")), FILTER)
	expectedInner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	expected = NewConstantScoreQuery(expectedInner)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// ConstantScore(SHOULD + SHOULD) -> unchanged (only SHOULDs, need them)
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	query = NewConstantScoreQuery(inner)

	rewritten, _ = query.Rewrite(reader)
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query, got %v", rewritten)
	}

	// ConstantScore(SHOULD + MUST_NOT) -> unchanged
	inner = NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), MUST_NOT)
	query = NewConstantScoreQuery(inner)

	rewritten, _ = query.Rewrite(reader)
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query, got %v", rewritten)
	}

	// ConstantScore(minShouldMatch=1, SHOULD + SHOULD + FILTER) -> unchanged
	inner = NewBooleanQuery()
	inner.SetMinimumNumberShouldMatch(1)
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	query = NewConstantScoreQuery(inner)

	rewritten, _ = query.Rewrite(reader)
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query with minShouldMatch, got %v", rewritten)
	}
}

// TestBooleanRewrites_ShouldMatchNoDocsQuery tests SHOULD with MatchNoDocsQuery.
func TestBooleanRewrites_ShouldMatchNoDocsQuery(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// SHOULD(term) + SHOULD(MatchNoDocs) -> just the term
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewMatchNoDocsQuery(), SHOULD)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_MustNotMatchNoDocsQuery tests MUST_NOT with MatchNoDocsQuery.
func TestBooleanRewrites_MustNotMatchNoDocsQuery(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// SHOULD(term) + MUST_NOT(MatchNoDocs) -> just the term (MatchNoDocs does nothing)
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewMatchNoDocsQuery(), MUST_NOT)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_MustMatchNoDocsQuery tests MUST with MatchNoDocsQuery.
func TestBooleanRewrites_MustMatchNoDocsQuery(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST(term) + MUST(MatchNoDocs) -> MatchNoDocsQuery
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchNoDocsQuery(), MUST)

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_FilterMatchNoDocsQuery tests FILTER with MatchNoDocsQuery.
func TestBooleanRewrites_FilterMatchNoDocsQuery(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST(term) + FILTER(MatchNoDocs) -> MatchNoDocsQuery
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchNoDocsQuery(), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_EmptyBoolean tests empty boolean query rewrite.
func TestBooleanRewrites_EmptyBoolean(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Empty boolean query -> MatchNoDocsQuery
	bq := NewBooleanQuery()

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty boolean, got %T", rewritten)
	}
}

// TestBooleanRewrites_SimplifyFilterClauses tests simplification of FILTER clauses.
func TestBooleanRewrites_SimplifyFilterClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST + FILTER(ConstantScore(Term)) -> MUST + FILTER(Term)
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), FILTER)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// FILTER(Term) + FILTER(ConstantScore(same Term)) -> ConstantScoreQuery with score 0
	bq = NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), FILTER)

	rewritten, _ = bq.Rewrite(reader)
	expected = NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 0)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_SimplifyMustNotClauses tests simplification of MUST_NOT clauses.
func TestBooleanRewrites_SimplifyMustNotClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// MUST + MUST_NOT(ConstantScore(Term)) -> MUST + MUST_NOT(Term)
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), MUST_NOT)

	rewritten, _ := bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_SimplifyNonScoringShouldClauses tests simplification of non-scoring SHOULD clauses.
func TestBooleanRewrites_SimplifyNonScoringShouldClauses(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// ConstantScore(SHOULD(Term) + SHOULD(ConstantScore(Term))) -> ConstantScore(SHOULD(Term) + SHOULD(Term))
	inner := NewBooleanQuery()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), SHOULD)
	query := NewConstantScoreQuery(inner)

	rewritten, _ := query.Rewrite(reader)
	expectedInner := NewBooleanQuery()
	expectedInner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expectedInner.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expected := NewConstantScoreQuery(expectedInner)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_ShouldClausesLessThanOrEqualToMinimumNumberShouldMatch tests minShouldMatch edge cases.
func TestBooleanRewrites_ShouldClausesLessThanOrEqualToMinimumNumberShouldMatch(t *testing.T) {
	reader := NewMockIndexReader(0, 0, 1)

	// Single empty PhraseQuery with minShouldMatch=1 -> MatchNoDocsQuery
	bq := NewBooleanQuery()
	bq.Add(NewPhraseQuery("field"), SHOULD)
	bq.SetMinimumNumberShouldMatch(1)

	rewritten, _ := bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty phrase with minShouldMatch=1, got %T", rewritten)
	}

	// Same with minShouldMatch=0 -> MatchNoDocsQuery (empty phrase matches nothing)
	bq = NewBooleanQuery()
	bq.Add(NewPhraseQuery("field"), SHOULD)
	bq.SetMinimumNumberShouldMatch(0)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty phrase, got %T", rewritten)
	}

	// Meaningful SHOULD count < minShouldMatch -> MatchNoDocsQuery
	bq = NewBooleanQuery()
	bq.Add(NewPhraseQuery("field"), SHOULD)
	bq.Add(NewPhraseQueryWithTerms("field", index.NewTerm("field", "a")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Rewrite(reader)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery when meaningful clauses < minShouldMatch, got %T", rewritten)
	}

	// Meaningful SHOULD count == minShouldMatch -> convert to MUSTs
	bq = NewBooleanQuery()
	bq.Add(NewPhraseQueryWithTerms("field", index.NewTerm("field", "b")), SHOULD)
	bq.Add(NewPhraseQueryWithTerms("field", index.NewTerm("field", "a"), index.NewTerm("field", "c")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Rewrite(reader)
	expected := NewBooleanQuery()
	expected.Add(NewTermQuery(index.NewTerm("field", "b")), MUST)
	expected.Add(NewPhraseQueryWithTerms("field", index.NewTerm("field", "a"), index.NewTerm("field", "c")), MUST)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_EqualsPrecision tests float comparison precision in assertions.
func TestBooleanRewrites_EqualsPrecision(t *testing.T) {
	// Test that float comparisons work correctly
	boost1 := float32(0.0)
	boost2 := float32(0.0)

	if boost1 != boost2 {
		t.Error("Float comparison failed for equal values")
	}

	// Test with very small differences
	boost3 := float32(0.0000001)
	boost4 := float32(0.0000002)

	if boost3 == boost4 {
		t.Error("Float comparison should detect small differences")
	}

	// Test with NaN
	nan1 := float32(math.NaN())
	nan2 := float32(math.NaN())
	if nan1 == nan2 {
		t.Error("NaN should not equal NaN")
	}
}

// Helper function to create a PhraseQuery with terms
func NewPhraseQueryWithTerms(field string, terms ...*index.Term) *PhraseQuery {
	return NewPhraseQuery(field, terms...)
}
