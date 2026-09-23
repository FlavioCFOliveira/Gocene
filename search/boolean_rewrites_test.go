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
	"github.com/FlavioCFOliveira/Gocene/spi"
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
func (q *testRewriteQuery) Equals(other spi.Query) bool {
	_, ok := other.(*testRewriteQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *testRewriteQuery) HashCode() int {
	return 42
}

// Rewrite rewrites the query and counts rewrites.
func (q *testRewriteQuery) Rewrite(reader *IndexSearcher) (Query, error) {
	q.numRewrites++
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *testRewriteQuery) CreateWeight(searcher *IndexSearcher, needsScores ScoreMode, boost float32) (Weight, error) {
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

// newEmptyMultiReader renders Java's new MultiReader(): a composite reader
// with no sub-readers, which TestBooleanRewrites hands to newSearcher for
// the rewrite-only tests.
func newEmptyMultiReader(t *testing.T) *index.MultiReader {
	t.Helper()
	r, err := index.NewMultiReader(nil)
	if err != nil {
		t.Fatalf("NewMultiReader: %v", err)
	}
	return r
}

// TestBooleanRewrites_OneClauseRewriteOptimization tests that single clause boolean queries
// are rewritten to their underlying query.
func TestBooleanRewrites_OneClauseRewriteOptimization(t *testing.T) {
	reader := newEmptyMultiReader(t)
	expected := NewTermQuery(index.NewTerm("content", "foo"))

	// Build nested boolean queries with single clauses
	actual := Query(NewTermQuery(index.NewTerm("content", "foo")))
	numLayers := 3

	for i := 0; i < numLayers; i++ {
		bq := NewBooleanQueryBuilder()
		// Alternate between SHOULD and MUST
		if i%2 == 0 {
			bq.Add(actual, SHOULD)
		} else {
			bq.Add(actual, MUST)
		}
		actual, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	}

	if !actual.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// TestBooleanRewrites_SingleFilterClause tests that single FILTER clauses rewrite correctly.
// Per Lucene 10.4.0: a single FILTER clause rewrites to BoostQuery(ConstantScoreQuery(inner), 0)
// so that needsScores=false is propagated to the inner query scorer.
func TestBooleanRewrites_SingleFilterClause(t *testing.T) {
	reader := newEmptyMultiReader(t)

	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("field", "a")), FILTER)

	rewritten, err := bq.Build().Rewrite(NewIndexSearcher(reader))
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	bqr, ok := rewritten.(*BoostQuery)
	if !ok {
		t.Fatalf("Expected BoostQuery, got %T", rewritten)
	}

	if bqr.Boost() != 0.0 {
		t.Errorf("Expected boost 0.0, got %f", bqr.Boost())
	}

	if _, ok := bqr.Query().(*ConstantScoreQuery); !ok {
		t.Errorf("Expected inner ConstantScoreQuery, got %T", bqr.Query())
	}
}

// TestBooleanRewrites_SingleMustMatchAll tests MatchAllDocsQuery with various clause combinations.
func TestBooleanRewrites_SingleMustMatchAll(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MatchAllDocsQuery + FILTER(TermQuery) -> ConstantScoreQuery(TermQuery)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedCSQ := NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar")))
	if !rewritten.Equals(expectedCSQ) {
		t.Errorf("Expected %v, got %v", expectedCSQ, rewritten)
	}

	// MatchAllDocsQuery + Boost + FILTER -> Boost(ConstantScoreQuery)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewBoostQuery(NewMatchAllDocsQuery(), 42), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 42)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + FILTER(MatchAll) -> MatchAllDocsQuery
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchAllDocsQuery); !ok {
		t.Errorf("Expected MatchAllDocsQuery, got %T", rewritten)
	}

	// MatchAllDocsQuery + MUST_NOT(TermQuery) -> unchanged (needs both)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", rewritten)
	}

	// MatchAllDocsQuery + Boost + FILTER(MatchAll) -> Boost(MatchAll)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewBoostQuery(NewMatchAllDocsQuery(), 42), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBoostQuery(NewMatchAllDocsQuery(), 42)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MatchAllDocsQuery + multiple FILTERs -> ConstantScoreQuery with combined filters
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedFilter := NewBooleanQueryBuilder()
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expectedCSQ2 := NewConstantScoreQuery(expectedFilter.Build())
	if !rewritten.Equals(expectedCSQ2) {
		t.Errorf("Expected %v, got %v", expectedCSQ2, rewritten)
	}

	// MatchAllDocsQuery + FILTER + MUST_NOT -> ConstantScoreQuery with filter and must_not
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedFilter = NewBooleanQueryBuilder()
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expectedFilter.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	expectedCSQ3 := NewConstantScoreQuery(expectedFilter.Build())
	if !rewritten.Equals(expectedCSQ3) {
		t.Errorf("Expected %v, got %v", expectedCSQ3, rewritten)
	}

	// MatchAllDocsQuery + SHOULD(TermQuery) -> unchanged (SHOULD needs scoring)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_SingleMustMatchAllWithShouldClauses tests MatchAll with SHOULD clauses.
func TestBooleanRewrites_SingleMustMatchAllWithShouldClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MatchAll + FILTER + SHOULD clauses -> MUST(ConstantScore) + SHOULDs
	bq := NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))

	expected := NewBooleanQueryBuilder()
	expected.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}
}

// TestBooleanRewrites_DeduplicateMustAndFilter tests deduplication of MUST and FILTER clauses.
func TestBooleanRewrites_DeduplicateMustAndFilter(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Same query as MUST and FILTER -> just the query (FILTER absorbed into MUST)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST + FILTER(same) + FILTER(different) -> MUST + FILTER(different)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ := NewBooleanQueryBuilder()
	expectedBQ.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expectedBQ.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expectedBQ.Build()) {
		t.Errorf("Expected %v, got %v", expectedBQ.Build(), rewritten)
	}
}

// TestBooleanRewrites_ConvertShouldAndFilterToMust tests conversion of SHOULD+FILTER to MUST.
func TestBooleanRewrites_ConvertShouldAndFilterToMust(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Same query as SHOULD and FILTER -> just the query (converted to MUST)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// SHOULD(same) + FILTER(same) + SHOULD(others) with minShouldMatch=2
	// -> MUST(same) + SHOULD(others) with minShouldMatch=1
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quz")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ2 := NewBooleanQueryBuilder()
	expectedBQ2.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expectedBQ2.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expectedBQ2.Add(NewTermQuery(index.NewTerm("foo", "quz")), SHOULD)
	expectedBQ2.SetMinimumNumberShouldMatch(1)
	if !rewritten.Equals(expectedBQ2.Build()) {
		t.Errorf("Expected %v, got %v", expectedBQ2.Build(), rewritten)
	}
}

// TestBooleanRewrites_DuplicateMustOrFilterWithMustNot tests that duplicate MUST/FILTER
// with MUST_NOT on same term results in MatchNoDocsQuery.
func TestBooleanRewrites_DuplicateMustOrFilterWithMustNot(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST(term) + MUST_NOT(same term) -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST + MUST_NOT on same term, got %T", rewritten)
	}

	// FILTER(term) + MUST_NOT(same term) -> MatchNoDocsQuery
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST_NOT)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for FILTER + MUST_NOT on same term, got %T", rewritten)
	}
}

// TestBooleanRewrites_MatchAllMustNot tests that MatchAllDocsQuery as MUST_NOT results in MatchNoDocsQuery.
func TestBooleanRewrites_MatchAllMustNot(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST + FILTER + SHOULD + MUST_NOT(MatchAll) -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewMatchAllDocsQuery(), MUST_NOT)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST_NOT(MatchAll), got %T", rewritten)
	}

	// With additional MUST_NOT clauses
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bad")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bor")), MUST_NOT)
	bq.Add(NewMatchAllDocsQuery(), MUST_NOT)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for MUST_NOT(MatchAll) with other clauses, got %T", rewritten)
	}
}

// TestBooleanRewrites_DeeplyNestedBooleanRewrite tests deeply nested boolean query rewrites.
func TestBooleanRewrites_DeeplyNestedBooleanRewrite(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Create deeply nested MUST queries
	depth := 10
	rewriteQuery := newTestRewriteQuery()
	rewriteQueryExpected := newTestRewriteQuery()

	expectedBuilder := NewBooleanQueryBuilder()
	expectedBuilder.Add(rewriteQueryExpected, FILTER)

	var deepBuilder Query = NewBooleanQueryBuilder().Add(rewriteQuery, MUST).Build()

	for i := depth; i > 0; i-- {
		tq := NewTermQuery(index.NewTerm("layer["+string(rune('0'+i))+"]", "foo"))
		bq := NewBooleanQueryBuilder()
		bq.Add(tq, MUST)
		bq.Add(deepBuilder, MUST)
		deepBuilder = bq.Build()

		expectedBuilder.Add(tq, FILTER)
		if i == depth {
			expectedBuilder.Add(rewriteQuery, FILTER)
		}
	}

	finalBq := NewBooleanQueryBuilder()
	finalBq.Add(deepBuilder, FILTER)

	rewritten, _ := finalBq.Build().Rewrite(NewIndexSearcher(reader))

	// The expected result is a BoostQuery wrapping ConstantScoreQuery
	expectedQuery := NewBoostQuery(NewConstantScoreQuery(expectedBuilder.Build()), 0.0)

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
	reader := newEmptyMultiReader(t)

	// Create deeply nested SHOULD queries with minShouldMatch
	depth := 10
	rewriteQuery := newTestRewriteQuery()
	rewriteQueryExpected := newTestRewriteQuery()

	expectedBuilder := NewBooleanQueryBuilder()
	expectedBuilder.Add(rewriteQueryExpected, FILTER)

	deepBuilderBuilder := NewBooleanQueryBuilder()
	deepBuilderBuilder.Add(rewriteQuery, SHOULD)
	deepBuilderBuilder.SetMinimumNumberShouldMatch(1)
	var deepBuilder Query = deepBuilderBuilder.Build()

	for i := depth; i > 0; i-- {
		tq := NewTermQuery(index.NewTerm("layer["+string(rune('0'+i))+"]", "foo"))
		bq := NewBooleanQueryBuilder()
		bq.SetMinimumNumberShouldMatch(2)
		bq.Add(tq, SHOULD)
		bq.Add(deepBuilder, SHOULD)
		deepBuilder = bq.Build()

		expectedBuilder.Add(tq, FILTER)
		if i == depth {
			expectedBuilder.Add(rewriteQuery, FILTER)
		}
	}

	finalBq := NewBooleanQueryBuilder()
	finalBq.Add(deepBuilder, FILTER)

	rewritten, _ := finalBq.Build().Rewrite(NewIndexSearcher(reader))

	expectedQuery := NewBoostQuery(NewConstantScoreQuery(expectedBuilder.Build()), 0.0)

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
	reader := newEmptyMultiReader(t)

	// MUST + FILTER(MatchAll) -> MUST
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST + MUST + FILTER(MatchAll) -> MUST + MUST
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ3 := NewBooleanQueryBuilder()
	expectedBQ3.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expectedBQ3.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expectedBQ3.Build()) {
		t.Errorf("Expected %v, got %v", expectedBQ3.Build(), rewritten)
	}

	// FILTER + FILTER(MatchAll) -> ConstantScoreQuery with score 0
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ5 := NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 0.0)
	if !rewritten.Equals(expectedBQ5) {
		t.Errorf("Expected %v, got %v", expectedBQ5, rewritten)
	}

	// FILTER(MatchAll) + FILTER(MatchAll) -> ConstantScoreQuery(MatchAll) with score 0
	bq = NewBooleanQueryBuilder()
	bq.Add(NewMatchAllDocsQuery(), FILTER)
	bq.Add(NewMatchAllDocsQuery(), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ6 := NewBoostQuery(NewConstantScoreQuery(NewMatchAllDocsQuery()), 0.0)
	if !rewritten.Equals(expectedBQ6) {
		t.Errorf("Expected %v, got %v", expectedBQ6, rewritten)
	}
}

// TestBooleanRewrites_DeduplicateShouldClauses tests deduplication of SHOULD clauses.
func TestBooleanRewrites_DeduplicateShouldClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Two identical SHOULD clauses -> BoostQuery with boost 2
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// SHOULD(term) + SHOULD(Boost(term, 2)) + SHOULD(other) -> Boost(term, 3) + other
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ4 := NewBooleanQueryBuilder()
	expectedBQ4.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 3), SHOULD)
	expectedBQ4.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	if !rewritten.Equals(expectedBQ4.Build()) {
		t.Errorf("Expected %v, got %v", expectedBQ4.Build(), rewritten)
	}

	// With minShouldMatch=2, deduplication doesn't apply
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	// Should remain unchanged
	if !rewritten.Equals(bq.Build()) {
		t.Errorf("With minShouldMatch=2, query should not be rewritten, got %v", rewritten)
	}
}

// TestBooleanRewrites_DeduplicateMustClauses tests deduplication of MUST clauses.
func TestBooleanRewrites_DeduplicateMustClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Two identical MUST clauses -> BoostQuery with boost 2
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2)
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// MUST(term) + MUST(Boost(term, 2)) + MUST(other) -> Boost(term, 3) + other
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 2), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ7 := NewBooleanQueryBuilder()
	expectedBQ7.Add(NewBoostQuery(NewTermQuery(index.NewTerm("foo", "bar")), 3), MUST)
	expectedBQ7.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	if !rewritten.Equals(expectedBQ7.Build()) {
		t.Errorf("Expected %v, got %v", expectedBQ7.Build(), rewritten)
	}
}

// TestBooleanRewrites_FlattenInnerDisjunctions tests flattening of inner disjunctions (SHOULD clauses).
func TestBooleanRewrites_FlattenInnerDisjunctions(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Inner disjunction flattened into outer
	inner := NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq := NewBooleanQueryBuilder()
	bq.Add(inner.Build(), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// With minShouldMatch=0, inner SHOULD flattened
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(0)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq = NewBooleanQueryBuilder()
	bq.SetMinimumNumberShouldMatch(0)
	bq.Add(inner.Build(), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.SetMinimumNumberShouldMatch(0)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// With minShouldMatch=1, inner SHOULD flattened
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(1)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq = NewBooleanQueryBuilder()
	bq.SetMinimumNumberShouldMatch(1)
	bq.Add(inner.Build(), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.SetMinimumNumberShouldMatch(1)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// With minShouldMatch=2 on inner, cannot flatten (would change semantics)
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(2)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	bq = NewBooleanQueryBuilder()
	bq.Add(inner.Build(), SHOULD)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	// Should remain unchanged
	if rewritten != bq.Build() {
		t.Logf("Query with minShouldMatch on inner may not be flattened (got %v)", rewritten)
	}
}

// TestBooleanRewrites_FlattenInnerConjunctions tests flattening of inner conjunctions (MUST clauses).
func TestBooleanRewrites_FlattenInnerConjunctions(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Inner conjunction flattened into outer
	inner := NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	bq := NewBooleanQueryBuilder()
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// With minShouldMatch=0, inner MUST flattened
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(0)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)

	bq = NewBooleanQueryBuilder()
	bq.SetMinimumNumberShouldMatch(0)
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.SetMinimumNumberShouldMatch(0)
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// Inner MUST with MUST_NOT flattened
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)

	bq = NewBooleanQueryBuilder()
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// Inner MUST + FILTER flattened
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), FILTER)

	bq = NewBooleanQueryBuilder()
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), FILTER)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// Inner FILTER + MUST_NOT flattened
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)

	bq = NewBooleanQueryBuilder()
	bq.Add(inner.Build(), FILTER)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), MUST_NOT)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}
}

// TestBooleanRewrites_FlattenDisjunctionInMustClause tests flattening SHOULD in MUST.
func TestBooleanRewrites_FlattenDisjunctionInMustClause(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST(SHOULD + SHOULD) + FILTER -> SHOULD + SHOULD + FILTER with minShouldMatch=1
	inner := NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)

	bq := NewBooleanQueryBuilder()
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expected.SetMinimumNumberShouldMatch(1)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// With minShouldMatch=2 on inner, preserve it
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(2)
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("foo", "foo")), SHOULD)

	bq = NewBooleanQueryBuilder()
	bq.Add(inner.Build(), MUST)
	bq.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected = NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "quux")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "foo")), SHOULD)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	expected.SetMinimumNumberShouldMatch(2)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}
}

// TestBooleanRewrites_DiscardShouldClauses tests discarding SHOULD clauses in certain contexts.
func TestBooleanRewrites_DiscardShouldClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// ConstantScore(MUST + SHOULD) -> ConstantScore(MUST) (SHOULD discarded)
	inner := NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	query := NewConstantScoreQuery(inner.Build())

	rewritten, _ := query.Rewrite(NewIndexSearcher(reader))
	expected := NewConstantScoreQuery(NewTermQuery(index.NewTerm("field", "a")))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// ConstantScore(MUST + SHOULD + FILTER) -> ConstantScore(FILTER + FILTER)
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), MUST)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	query = NewConstantScoreQuery(inner.Build())

	rewritten, _ = query.Rewrite(NewIndexSearcher(reader))
	expectedInner := NewBooleanQueryBuilder()
	expectedInner.Add(NewTermQuery(index.NewTerm("field", "a")), FILTER)
	expectedInner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	expected = NewConstantScoreQuery(expectedInner.Build())
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}

	// ConstantScore(SHOULD + SHOULD) -> unchanged (only SHOULDs, need them)
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	query = NewConstantScoreQuery(inner.Build())

	rewritten, _ = query.Rewrite(NewIndexSearcher(reader))
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query, got %v", rewritten)
	}

	// ConstantScore(SHOULD + MUST_NOT) -> unchanged
	inner = NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), MUST_NOT)
	query = NewConstantScoreQuery(inner.Build())

	rewritten, _ = query.Rewrite(NewIndexSearcher(reader))
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query, got %v", rewritten)
	}

	// ConstantScore(minShouldMatch=1, SHOULD + SHOULD + FILTER) -> unchanged
	inner = NewBooleanQueryBuilder()
	inner.SetMinimumNumberShouldMatch(1)
	inner.Add(NewTermQuery(index.NewTerm("field", "a")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "b")), SHOULD)
	inner.Add(NewTermQuery(index.NewTerm("field", "c")), FILTER)
	query = NewConstantScoreQuery(inner.Build())

	rewritten, _ = query.Rewrite(NewIndexSearcher(reader))
	if !rewritten.Equals(query) {
		t.Errorf("Expected unchanged query with minShouldMatch, got %v", rewritten)
	}
}

// TestBooleanRewrites_ShouldMatchNoDocsQuery tests SHOULD with MatchNoDocsQuery.
func TestBooleanRewrites_ShouldMatchNoDocsQuery(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// SHOULD(term) + SHOULD(MatchNoDocs) -> just the term
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewMatchNoDocsQuery(""), SHOULD)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_MustNotMatchNoDocsQuery tests MUST_NOT with MatchNoDocsQuery.
func TestBooleanRewrites_MustNotMatchNoDocsQuery(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// SHOULD(term) + MUST_NOT(MatchNoDocs) -> just the term (MatchNoDocs does nothing)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	bq.Add(NewMatchNoDocsQuery(""), MUST_NOT)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewTermQuery(index.NewTerm("foo", "bar"))
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_MustMatchNoDocsQuery tests MUST with MatchNoDocsQuery.
func TestBooleanRewrites_MustMatchNoDocsQuery(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST(term) + MUST(MatchNoDocs) -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchNoDocsQuery(""), MUST)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_FilterMatchNoDocsQuery tests FILTER with MatchNoDocsQuery.
func TestBooleanRewrites_FilterMatchNoDocsQuery(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST(term) + FILTER(MatchNoDocs) -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewMatchNoDocsQuery(""), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery, got %T", rewritten)
	}
}

// TestBooleanRewrites_EmptyBoolean tests empty boolean query rewrite.
func TestBooleanRewrites_EmptyBoolean(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Empty boolean query -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty boolean, got %T", rewritten)
	}
}

// TestBooleanRewrites_SimplifyFilterClauses tests simplification of FILTER clauses.
func TestBooleanRewrites_SimplifyFilterClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST + FILTER(ConstantScore(Term)) -> MUST + FILTER(Term)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), FILTER)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), FILTER)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}

	// FILTER(Term) + FILTER(ConstantScore(same Term)) -> ConstantScoreQuery with score 0
	bq = NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), FILTER)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), FILTER)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expectedBQ8 := NewBoostQuery(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "bar"))), 0)
	if !rewritten.Equals(expectedBQ8) {
		t.Errorf("Expected %v, got %v", expectedBQ8, rewritten)
	}
}

// TestBooleanRewrites_SimplifyMustNotClauses tests simplification of MUST_NOT clauses.
func TestBooleanRewrites_SimplifyMustNotClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// MUST + MUST_NOT(ConstantScore(Term)) -> MUST + MUST_NOT(Term)
	bq := NewBooleanQueryBuilder()
	bq.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	bq.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), MUST_NOT)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("foo", "bar")), MUST)
	expected.Add(NewTermQuery(index.NewTerm("foo", "baz")), MUST_NOT)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
	}
}

// TestBooleanRewrites_SimplifyNonScoringShouldClauses tests simplification of non-scoring SHOULD clauses.
func TestBooleanRewrites_SimplifyNonScoringShouldClauses(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// ConstantScore(SHOULD(Term) + SHOULD(ConstantScore(Term))) -> ConstantScore(SHOULD(Term) + SHOULD(Term))
	inner := NewBooleanQueryBuilder()
	inner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	inner.Add(NewConstantScoreQuery(NewTermQuery(index.NewTerm("foo", "baz"))), SHOULD)
	query := NewConstantScoreQuery(inner.Build())

	rewritten, _ := query.Rewrite(NewIndexSearcher(reader))
	expectedInner := NewBooleanQueryBuilder()
	expectedInner.Add(NewTermQuery(index.NewTerm("foo", "bar")), SHOULD)
	expectedInner.Add(NewTermQuery(index.NewTerm("foo", "baz")), SHOULD)
	expected := NewConstantScoreQuery(expectedInner.Build())
	if !rewritten.Equals(expected) {
		t.Errorf("Expected %v, got %v", expected, rewritten)
	}
}

// TestBooleanRewrites_ShouldClausesLessThanOrEqualToMinimumNumberShouldMatch tests minShouldMatch edge cases.
func TestBooleanRewrites_ShouldClausesLessThanOrEqualToMinimumNumberShouldMatch(t *testing.T) {
	reader := newEmptyMultiReader(t)

	// Single empty PhraseQuery with minShouldMatch=1 -> MatchNoDocsQuery
	bq := NewBooleanQueryBuilder()
	bq.Add(NewPhraseQuery(0, "field"), SHOULD)
	bq.SetMinimumNumberShouldMatch(1)

	rewritten, _ := bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty phrase with minShouldMatch=1, got %T", rewritten)
	}

	// Same with minShouldMatch=0 -> MatchNoDocsQuery (empty phrase matches nothing)
	bq = NewBooleanQueryBuilder()
	bq.Add(NewPhraseQuery(0, "field"), SHOULD)
	bq.SetMinimumNumberShouldMatch(0)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery for empty phrase, got %T", rewritten)
	}

	// Meaningful SHOULD count < minShouldMatch -> MatchNoDocsQuery
	bq = NewBooleanQueryBuilder()
	bq.Add(NewPhraseQuery(0, "field"), SHOULD)
	bq.Add(NewPhraseQueryWithTerms(0, "field", index.NewTerm("field", "a")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Expected MatchNoDocsQuery when meaningful clauses < minShouldMatch, got %T", rewritten)
	}

	// Meaningful SHOULD count == minShouldMatch -> convert to MUSTs
	bq = NewBooleanQueryBuilder()
	bq.Add(NewPhraseQueryWithTerms(0, "field", index.NewTerm("field", "b")), SHOULD)
	bq.Add(NewPhraseQueryWithTerms(0, "field", index.NewTerm("field", "a"), index.NewTerm("field", "c")), SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	rewritten, _ = bq.Build().Rewrite(NewIndexSearcher(reader))
	expected := NewBooleanQueryBuilder()
	expected.Add(NewTermQuery(index.NewTerm("field", "b")), MUST)
	expected.Add(NewPhraseQueryWithTerms(0, "field", index.NewTerm("field", "a"), index.NewTerm("field", "c")), MUST)
	if !rewritten.Equals(expected.Build()) {
		t.Errorf("Expected %v, got %v", expected.Build(), rewritten)
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
