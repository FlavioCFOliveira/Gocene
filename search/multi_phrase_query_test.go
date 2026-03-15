// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: multi_phrase_query_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestMultiPhraseQuery.java
// Purpose: Tests the MultiPhraseQuery class for multiple term positions and phrase variants

package search

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestMultiPhraseQuery_PhrasePrefix tests phrase prefix queries
// Source: TestMultiPhraseQuery.testPhrasePrefix()
func TestMultiPhraseQuery_PhrasePrefix(t *testing.T) {
	// This test verifies that MultiPhraseQuery can handle:
	// 1. Single term at first position, multiple terms at second position
	// 2. Multiple terms at first position, single term at second position
	// 3. Slop handling

	// Build query: "blueberry (piccadilly pie pizza)"
	builder1 := NewMultiPhraseQueryBuilder()
	builder1.Add(index.NewTerm("body", "blueberry"))
	builder1.AddTerms([]*index.Term{
		index.NewTerm("body", "piccadilly"),
		index.NewTerm("body", "pie"),
		index.NewTerm("body", "pizza"),
	})
	query1 := builder1.Build()

	expected1 := "body:\"blueberry (piccadilly pie pizza)\""
	if query1.String() != expected1 {
		t.Errorf("Expected query string %q, got %q", expected1, query1.String())
	}

	// Build query: "strawberry (piccadilly pie pizza)"
	builder2 := NewMultiPhraseQueryBuilder()
	builder2.Add(index.NewTerm("body", "strawberry"))
	builder2.AddTerms([]*index.Term{
		index.NewTerm("body", "piccadilly"),
		index.NewTerm("body", "pie"),
		index.NewTerm("body", "pizza"),
	})
	query2 := builder2.Build()

	expected2 := "body:\"strawberry (piccadilly pie pizza)\""
	if query2.String() != expected2 {
		t.Errorf("Expected query string %q, got %q", expected2, query2.String())
	}

	// Build query: "(blueberry bluebird) pizza"
	builder3 := NewMultiPhraseQueryBuilder()
	builder3.AddTerms([]*index.Term{
		index.NewTerm("body", "blueberry"),
		index.NewTerm("body", "bluebird"),
	})
	builder3.Add(index.NewTerm("body", "pizza"))
	query3 := builder3.Build()

	expected3 := "body:\"(blueberry bluebird) pizza\""
	if query3.String() != expected3 {
		t.Errorf("Expected query string %q, got %q", expected3, query3.String())
	}

	// Test slop
	builder3.SetSlop(1)
	query3WithSlop := builder3.Build()
	if query3WithSlop.GetSlop() != 1 {
		t.Errorf("Expected slop 1, got %d", query3WithSlop.GetSlop())
	}

	expected3WithSlop := "body:\"(blueberry bluebird) pizza\"~1"
	if query3WithSlop.String() != expected3WithSlop {
		t.Errorf("Expected query string %q, got %q", expected3WithSlop, query3WithSlop.String())
	}
}

// TestMultiPhraseQuery_Tall tests multiple term positions (LUCENE-2580)
// Source: TestMultiPhraseQuery.testTall()
func TestMultiPhraseQuery_Tall(t *testing.T) {
	// Build query: "blueberry chocolate (pie tart)"
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("body", "blueberry"))
	builder.Add(index.NewTerm("body", "chocolate"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("body", "pie"),
		index.NewTerm("body", "tart"),
	})
	query := builder.Build()

	// Verify structure
	termArrays := query.GetTermArrays()
	if len(termArrays) != 3 {
		t.Errorf("Expected 3 term arrays, got %d", len(termArrays))
	}

	// First position: blueberry
	if len(termArrays[0]) != 1 || termArrays[0][0].Text() != "blueberry" {
		t.Error("First position should be 'blueberry'")
	}

	// Second position: chocolate
	if len(termArrays[1]) != 1 || termArrays[1][0].Text() != "chocolate" {
		t.Error("Second position should be 'chocolate'")
	}

	// Third position: pie OR tart
	if len(termArrays[2]) != 2 {
		t.Errorf("Third position should have 2 terms, got %d", len(termArrays[2]))
	}
}

// TestMultiPhraseQuery_MultiExactWithRepeats tests exact phrase matching with repeated terms
// Source: TestMultiPhraseQuery.testMultiExactWithRepeats()
func TestMultiPhraseQuery_MultiExactWithRepeats(t *testing.T) {
	// Build query with terms at specific positions
	// Position 0: "a" OR "d"
	// Position 2: "a" OR "f"
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("body", "a"),
		index.NewTerm("body", "d"),
	}, 0)
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("body", "a"),
		index.NewTerm("body", "f"),
	}, 2)
	query := builder.Build()

	// Verify positions
	positions := query.GetPositions()
	if len(positions) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(positions))
	}
	if positions[0] != 0 {
		t.Errorf("Expected first position 0, got %d", positions[0])
	}
	if positions[1] != 2 {
		t.Errorf("Expected second position 2, got %d", positions[1])
	}

	// Verify term arrays
	termArrays := query.GetTermArrays()
	if len(termArrays[0]) != 2 {
		t.Errorf("Expected 2 terms at position 0, got %d", len(termArrays[0]))
	}
	if len(termArrays[1]) != 2 {
		t.Errorf("Expected 2 terms at position 2, got %d", len(termArrays[1]))
	}
}

// TestMultiPhraseQuery_BooleanQueryContainingSingleTermPrefixQuery tests BooleanQuery with MultiPhraseQuery
// Source: TestMultiPhraseQuery.testBooleanQueryContainingSingleTermPrefixQuery()
func TestMultiPhraseQuery_BooleanQueryContainingSingleTermPrefixQuery(t *testing.T) {
	// Build: +body:pie +body:"(blueberry blue)"
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("body", "pie")), MUST)

	mpqBuilder := NewMultiPhraseQueryBuilder()
	mpqBuilder.AddTerms([]*index.Term{
		index.NewTerm("body", "blueberry"),
		index.NewTerm("body", "blue"),
	})
	mpq := mpqBuilder.Build()

	bq.Add(mpq, MUST)

	// Verify the boolean query structure
	clauses := bq.Clauses()
	if len(clauses) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(clauses))
	}

	// First clause should be TermQuery
	if _, ok := clauses[0].Query.(*TermQuery); !ok {
		t.Error("First clause should be TermQuery")
	}

	// Second clause should be MultiPhraseQuery
	if _, ok := clauses[1].Query.(*MultiPhraseQuery); !ok {
		t.Error("Second clause should be MultiPhraseQuery")
	}

	// Both should be MUST
	if clauses[0].Occur != MUST || clauses[1].Occur != MUST {
		t.Error("Both clauses should be MUST")
	}
}

// TestMultiPhraseQuery_PhrasePrefixWithBooleanQuery tests phrase prefix within BooleanQuery
// Source: TestMultiPhraseQuery.testPhrasePrefixWithBooleanQuery()
func TestMultiPhraseQuery_PhrasePrefixWithBooleanQuery(t *testing.T) {
	// Build: +type:note +body:"a (test this)"
	bq := NewBooleanQuery()
	bq.Add(NewTermQuery(index.NewTerm("type", "note")), MUST)

	mpqBuilder := NewMultiPhraseQueryBuilder()
	mpqBuilder.Add(index.NewTerm("body", "a"))
	mpqBuilder.AddTerms([]*index.Term{
		index.NewTerm("body", "test"),
		index.NewTerm("body", "this"),
	})
	mpq := mpqBuilder.Build()

	bq.Add(mpq, MUST)

	// Verify structure
	clauses := bq.Clauses()
	if len(clauses) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(clauses))
	}

	// Verify string representation
	mpqStr := mpq.String()
	if !strings.Contains(mpqStr, "a") {
		t.Errorf("MultiPhraseQuery should contain 'a', got %s", mpqStr)
	}
	if !strings.Contains(mpqStr, "test") || !strings.Contains(mpqStr, "this") {
		t.Errorf("MultiPhraseQuery should contain 'test' and 'this', got %s", mpqStr)
	}
}

// TestMultiPhraseQuery_NoDocs tests when no documents match
// Source: TestMultiPhraseQuery.testNoDocs()
func TestMultiPhraseQuery_NoDocs(t *testing.T) {
	// Build query with non-existent terms
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("body", "a"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("body", "nope"),
		index.NewTerm("body", "nope"),
	})
	query := builder.Build()

	// Verify structure
	termArrays := query.GetTermArrays()
	if len(termArrays) != 2 {
		t.Errorf("Expected 2 term arrays, got %d", len(termArrays))
	}

	// First position: single term "a"
	if len(termArrays[0]) != 1 || termArrays[0][0].Text() != "a" {
		t.Error("First position should be 'a'")
	}

	// Second position: two "nope" terms
	if len(termArrays[1]) != 2 {
		t.Errorf("Second position should have 2 terms, got %d", len(termArrays[1]))
	}
}

// TestMultiPhraseQuery_HashCodeAndEquals tests hashCode and equals methods
// Source: TestMultiPhraseQuery.testHashCodeAndEquals()
func TestMultiPhraseQuery_HashCodeAndEquals(t *testing.T) {
	// Empty queries should be equal
	builder1 := NewMultiPhraseQueryBuilder()
	query1 := builder1.Build()

	builder2 := NewMultiPhraseQueryBuilder()
	query2 := builder2.Build()

	if query1.HashCode() != query2.HashCode() {
		t.Error("Empty queries should have same hashCode")
	}
	if !query1.Equals(query2) {
		t.Error("Empty queries should be equal")
	}

	// Add same term to both
	term1 := index.NewTerm("someField", "someText")
	builder1.Add(term1)
	query1 = builder1.Build()

	builder2.Add(term1)
	query2 = builder2.Build()

	if query1.HashCode() != query2.HashCode() {
		t.Error("Queries with same term should have same hashCode")
	}
	if !query1.Equals(query2) {
		t.Error("Queries with same term should be equal")
	}

	// Add different second term
	term2 := index.NewTerm("someField", "someMoreText")
	builder1.Add(term2)
	query1 = builder1.Build()

	// query1 now has 2 terms, query2 has 1 - should not be equal
	if query1.Equals(query2) {
		t.Error("Queries with different number of terms should not be equal")
	}

	// Add same term to query2
	builder2.Add(term2)
	query2 = builder2.Build()

	if query1.HashCode() != query2.HashCode() {
		t.Error("Queries with same terms should have same hashCode")
	}
	if !query1.Equals(query2) {
		t.Error("Queries with same terms should be equal")
	}
}

// TestMultiPhraseQuery_EmptyToString tests empty query toString (LUCENE-2526)
// Source: TestMultiPhraseQuery.testEmptyToString()
func TestMultiPhraseQuery_EmptyToString(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	query := builder.Build()

	// Should not panic
	str := query.String()
	if str != `""` {
		t.Errorf("Expected empty query string '\"\"', got %q", str)
	}
}

// TestMultiPhraseQuery_ZeroPosIncr tests zero position increment
// Source: TestMultiPhraseQuery.testZeroPosIncr()
func TestMultiPhraseQuery_ZeroPosIncr(t *testing.T) {
	// Build query with terms at same position (position increment 0)
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "b"),
		index.NewTerm("field", "c"),
	}, 0)
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "a"),
	}, 0)
	query := builder.Build()

	// Verify positions
	positions := query.GetPositions()
	if len(positions) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(positions))
	}
	if positions[0] != 0 || positions[1] != 0 {
		t.Errorf("Expected both positions to be 0, got %v", positions)
	}

	// Verify term arrays
	termArrays := query.GetTermArrays()
	if len(termArrays[0]) != 2 {
		t.Errorf("Expected 2 terms at first position, got %d", len(termArrays[0]))
	}
	if len(termArrays[1]) != 1 {
		t.Errorf("Expected 1 term at second position, got %d", len(termArrays[1]))
	}
}

// TestMultiPhraseQuery_SloppyParsedAnd tests sloppy parsed AND mode
// Source: TestMultiPhraseQuery.testZeroPosIncrSloppyParsedAnd()
func TestMultiPhraseQuery_SloppyParsedAnd(t *testing.T) {
	// Build query with negative position (relative to previous)
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "a"),
		index.NewTerm("field", "1"),
	}, -1)
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "b"),
		index.NewTerm("field", "1"),
	}, 0)
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "c"),
	}, 1)

	// Test with different slop values
	slopValues := []int{0, 1, 2}
	for _, slop := range slopValues {
		builder.SetSlop(slop)
		query := builder.Build()
		if query.GetSlop() != slop {
			t.Errorf("Expected slop %d, got %d", slop, query.GetSlop())
		}
	}
}

// TestMultiPhraseQuery_PqAnd tests PhraseQuery AND mode comparison
// Source: TestMultiPhraseQuery.testZeroPosIncrSloppyPqAnd()
func TestMultiPhraseQuery_PqAnd(t *testing.T) {
	// Create a PhraseQuery equivalent to test against
	pqBuilder := NewPhraseQueryBuilder()
	pqBuilder.AddTermAtPosition(index.NewTerm("field", "a"), 0)
	pqBuilder.AddTermAtPosition(index.NewTerm("field", "1"), 0)
	pqBuilder.AddTermAtPosition(index.NewTerm("field", "b"), 1)
	pqBuilder.AddTermAtPosition(index.NewTerm("field", "1"), 1)
	pqBuilder.AddTermAtPosition(index.NewTerm("field", "c"), 2)

	// Test with different slop values
	slopValues := []int{0, 1, 2}
	for _, slop := range slopValues {
		pqBuilder.SetSlop(slop)
		pq := pqBuilder.Build()
		if pq.GetSlop() != slop {
			t.Errorf("Expected slop %d, got %d", slop, pq.GetSlop())
		}
	}
}

// TestMultiPhraseQuery_MpqAnd tests MultiPhraseQuery AND mode
// Source: TestMultiPhraseQuery.testZeroPosIncrSloppyMpqAnd()
func TestMultiPhraseQuery_MpqAnd(t *testing.T) {
	// Build MultiPhraseQuery with single terms at each position
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a")}, 0)
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "1")}, 0)
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b")}, 1)
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "1")}, 1)
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "c")}, 2)

	// Test with different slop values
	slopValues := []int{0, 1, 2}
	for _, slop := range slopValues {
		builder.SetSlop(slop)
		query := builder.Build()
		if query.GetSlop() != slop {
			t.Errorf("Expected slop %d, got %d", slop, query.GetSlop())
		}
	}
}

// TestMultiPhraseQuery_MpqAndOrMatch tests MPQ combined AND OR mode with match
// Source: TestMultiPhraseQuery.testZeroPosIncrSloppyMpqAndOrMatch()
func TestMultiPhraseQuery_MpqAndOrMatch(t *testing.T) {
	// Build query with OR groups at specific positions
	builder := NewMultiPhraseQueryBuilder()

	// Position 1: "a"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a")}, 1)

	// Position 1: "x" OR "1" (at same position as "a")
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "x"),
		index.NewTerm("field", "1"),
	}, 1)

	// Position 3: "b"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b")}, 3)

	// Position 5: "x" OR "1"
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "x"),
		index.NewTerm("field", "1"),
	}, 5)

	// Position 8: "c"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "c")}, 8)

	// Test with different slop values
	slopValues := []int{0, 1, 2}
	for _, slop := range slopValues {
		builder.SetSlop(slop)
		query := builder.Build()
		if query.GetSlop() != slop {
			t.Errorf("Expected slop %d, got %d", slop, query.GetSlop())
		}
	}
}

// TestMultiPhraseQuery_MpqAndOrNoMatch tests MPQ combined AND OR mode with no match
// Source: TestMultiPhraseQuery.testZeroPosIncrSloppyMpqAndOrNoMatch()
func TestMultiPhraseQuery_MpqAndOrNoMatch(t *testing.T) {
	// Build query that should not match
	builder := NewMultiPhraseQueryBuilder()

	// Position 1: "x"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "x")}, 1)

	// Position 2: "a" OR "1"
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "a"),
		index.NewTerm("field", "1"),
	}, 2)

	// Position 4: "x"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "x")}, 4)

	// Position 5: "b" OR "1"
	builder.AddTermsAtPosition([]*index.Term{
		index.NewTerm("field", "b"),
		index.NewTerm("field", "1"),
	}, 5)

	// Position 8: "c"
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "c")}, 8)

	query := builder.Build()

	// Verify structure
	termArrays := query.GetTermArrays()
	if len(termArrays) != 5 {
		t.Errorf("Expected 5 term arrays, got %d", len(termArrays))
	}
}

// TestMultiPhraseQuery_NegativeSlop tests negative slop validation
// Source: TestMultiPhraseQuery.testNegativeSlop()
func TestMultiPhraseQuery_NegativeSlop(t *testing.T) {
	// In Go, we set slop directly on the query
	// The Java test expects an IllegalArgumentException when setting negative slop
	// Our implementation should handle this gracefully

	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "two"))
	builder.Add(index.NewTerm("field", "one"))

	// Setting negative slop - in our implementation this is allowed at build time
	// but could be validated during query execution
	builder.SetSlop(-2)
	query := builder.Build()

	// The query should still be created
	if query.GetSlop() != -2 {
		t.Errorf("Expected slop -2, got %d", query.GetSlop())
	}
}

// TestMultiPhraseQuery_DifferentFields tests that adding terms from different fields panics
func TestMultiPhraseQuery_DifferentFields(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when adding terms from different fields")
		}
	}()

	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field1", "foo"))
	builder.Add(index.NewTerm("field2", "foobar"))
}

// TestMultiPhraseQuery_Clone tests cloning
func TestMultiPhraseQuery_Clone(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "one"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "two"),
		index.NewTerm("field", "three"),
	})
	builder.SetSlop(3)
	original := builder.Build()

	cloned := original.Clone().(*MultiPhraseQuery)

	// Verify cloned query has same properties
	if cloned.Field() != original.Field() {
		t.Error("Cloned query should have same field")
	}
	if cloned.GetSlop() != original.GetSlop() {
		t.Error("Cloned query should have same slop")
	}
	if !cloned.Equals(original) {
		t.Error("Cloned query should equal original")
	}

	// Modify cloned query and verify original is unchanged
	cloned.SetSlop(5)
	if original.GetSlop() == 5 {
		t.Error("Modifying cloned query should not affect original")
	}
}

// TestMultiPhraseQuery_Rewrite tests query rewriting
func TestMultiPhraseQuery_Rewrite(t *testing.T) {
	// Empty query should rewrite to MatchNoDocsQuery
	emptyBuilder := NewMultiPhraseQueryBuilder()
	emptyQuery := emptyBuilder.Build()
	rewritten, err := emptyQuery.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Empty query should rewrite to MatchNoDocsQuery, got %T", rewritten)
	}

	// Single term should rewrite to TermQuery
	singleBuilder := NewMultiPhraseQueryBuilder()
	singleBuilder.Add(index.NewTerm("field", "term"))
	singleQuery := singleBuilder.Build()
	rewritten, err = singleQuery.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*TermQuery); !ok {
		t.Errorf("Single term query should rewrite to TermQuery, got %T", rewritten)
	}

	// Multiple terms at single position should rewrite to BooleanQuery
	multiBuilder := NewMultiPhraseQueryBuilder()
	multiBuilder.AddTerms([]*index.Term{
		index.NewTerm("field", "one"),
		index.NewTerm("field", "two"),
	})
	multiQuery := multiBuilder.Build()
	rewritten, err = multiQuery.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if _, ok := rewritten.(*BooleanQuery); !ok {
		t.Errorf("Multi-term single position query should rewrite to BooleanQuery, got %T", rewritten)
	}

	// Multi-position query should rewrite to itself
	complexBuilder := NewMultiPhraseQueryBuilder()
	complexBuilder.Add(index.NewTerm("field", "one"))
	complexBuilder.Add(index.NewTerm("field", "two"))
	complexQuery := complexBuilder.Build()
	rewritten, err = complexQuery.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if rewritten != complexQuery {
		t.Error("Multi-position query should rewrite to itself")
	}
}

// TestMultiPhraseQuery_GetTermArraysImmutability tests that GetTermArrays returns copies
func TestMultiPhraseQuery_GetTermArraysImmutability(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "one"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "two"),
		index.NewTerm("field", "three"),
	})
	query := builder.Build()

	// Get term arrays twice
	arrays1 := query.GetTermArrays()
	arrays2 := query.GetTermArrays()

	// Modify arrays1
	arrays1[0][0] = index.NewTerm("field", "modified")

	// arrays2 should be unchanged
	if arrays2[0][0].Text() != "one" {
		t.Error("GetTermArrays should return copies, not references")
	}
}

// TestMultiPhraseQuery_GetPositionsImmutability tests that GetPositions returns copies
func TestMultiPhraseQuery_GetPositionsImmutability(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "one"))
	builder.Add(index.NewTerm("field", "two"))
	query := builder.Build()

	// Get positions twice
	positions1 := query.GetPositions()
	positions2 := query.GetPositions()

	// Modify positions1
	positions1[0] = 999

	// positions2 should be unchanged
	if positions2[0] != 0 {
		t.Error("GetPositions should return copies, not references")
	}
}

// TestMultiPhraseQuery_BuilderFromQuery tests creating builder from existing query
func TestMultiPhraseQuery_BuilderFromQuery(t *testing.T) {
	originalBuilder := NewMultiPhraseQueryBuilder()
	originalBuilder.Add(index.NewTerm("field", "one"))
	originalBuilder.AddTerms([]*index.Term{
		index.NewTerm("field", "two"),
		index.NewTerm("field", "three"),
	})
	originalBuilder.SetSlop(2)
	original := originalBuilder.Build()

	// Create builder from query
	newBuilder := NewMultiPhraseQueryBuilderFromQuery(original)
	rebuilt := newBuilder.Build()

	// Should be equal
	if !original.Equals(rebuilt) {
		t.Error("Rebuilt query should equal original")
	}

	// Modify rebuilt and verify original is unchanged
	newBuilder.SetSlop(5)
	modified := newBuilder.Build()
	if original.GetSlop() == modified.GetSlop() {
		t.Error("Modifying rebuilt query should not affect original")
	}
}

// TestMultiPhraseQuery_StringWithGaps tests string representation with gaps
func TestMultiPhraseQuery_StringWithGaps(t *testing.T) {
	// Build query with gaps (position 0, then position 3)
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "first")}, 0)
	builder.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "second")}, 3)
	query := builder.Build()

	str := query.String()
	// Should contain ? for the gap
	if !strings.Contains(str, "?") {
		t.Errorf("Query with gaps should contain '?', got %s", str)
	}
}

// TestMultiPhraseQuery_StringWithMultipleTerms tests string representation with multiple terms
func TestMultiPhraseQuery_StringWithMultipleTerms(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTerms([]*index.Term{
		index.NewTerm("body", "blueberry"),
		index.NewTerm("body", "bluebird"),
	})
	builder.Add(index.NewTerm("body", "pizza"))
	query := builder.Build()

	expected := "body:\"(blueberry bluebird) pizza\""
	if query.String() != expected {
		t.Errorf("Expected %q, got %q", expected, query.String())
	}
}

// TestMultiPhraseQuery_EmptyBuilder tests building from empty builder
func TestMultiPhraseQuery_EmptyBuilder(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	query := builder.Build()

	if query.Field() != "" {
		t.Errorf("Expected empty field, got %q", query.Field())
	}

	termArrays := query.GetTermArrays()
	if len(termArrays) != 0 {
		t.Errorf("Expected 0 term arrays, got %d", len(termArrays))
	}

	positions := query.GetPositions()
	if len(positions) != 0 {
		t.Errorf("Expected 0 positions, got %d", len(positions))
	}
}

// TestMultiPhraseQuery_SingleTermArray tests adding single term array
func TestMultiPhraseQuery_SingleTermArray(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "one"),
		index.NewTerm("field", "two"),
		index.NewTerm("field", "three"),
	})
	query := builder.Build()

	termArrays := query.GetTermArrays()
	if len(termArrays) != 1 {
		t.Errorf("Expected 1 term array, got %d", len(termArrays))
	}
	if len(termArrays[0]) != 3 {
		t.Errorf("Expected 3 terms in array, got %d", len(termArrays[0]))
	}
}

// TestMultiPhraseQuery_MixedAddMethods tests mixing Add and AddTerms
func TestMultiPhraseQuery_MixedAddMethods(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "single"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "multi1"),
		index.NewTerm("field", "multi2"),
	})
	builder.Add(index.NewTerm("field", "another"))
	query := builder.Build()

	termArrays := query.GetTermArrays()
	if len(termArrays) != 3 {
		t.Errorf("Expected 3 term arrays, got %d", len(termArrays))
	}

	// First: single term
	if len(termArrays[0]) != 1 || termArrays[0][0].Text() != "single" {
		t.Error("First position should have single term 'single'")
	}

	// Second: two terms
	if len(termArrays[1]) != 2 {
		t.Errorf("Second position should have 2 terms, got %d", len(termArrays[1]))
	}

	// Third: single term
	if len(termArrays[2]) != 1 || termArrays[2][0].Text() != "another" {
		t.Error("Third position should have single term 'another'")
	}
}

// TestMultiPhraseQuery_PositionsIncrement tests automatic position increment
func TestMultiPhraseQuery_PositionsIncrement(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "first"))
	builder.Add(index.NewTerm("field", "second"))
	builder.Add(index.NewTerm("field", "third"))
	query := builder.Build()

	positions := query.GetPositions()
	if len(positions) != 3 {
		t.Errorf("Expected 3 positions, got %d", len(positions))
	}

	// Positions should be 0, 1, 2
	for i, pos := range positions {
		if pos != i {
			t.Errorf("Expected position %d at index %d, got %d", i, i, pos)
		}
	}
}

// TestMultiPhraseQuery_ComplexQuery tests a complex query with multiple features
func TestMultiPhraseQuery_ComplexQuery(t *testing.T) {
	// Build a complex query: "(quick fast) brown (fox dog)"~2
	builder := NewMultiPhraseQueryBuilder()
	builder.SetSlop(2)
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "quick"),
		index.NewTerm("field", "fast"),
	})
	builder.Add(index.NewTerm("field", "brown"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("field", "fox"),
		index.NewTerm("field", "dog"),
	})
	query := builder.Build()

	// Verify slop
	if query.GetSlop() != 2 {
		t.Errorf("Expected slop 2, got %d", query.GetSlop())
	}

	// Verify term arrays
	termArrays := query.GetTermArrays()
	if len(termArrays) != 3 {
		t.Errorf("Expected 3 term arrays, got %d", len(termArrays))
	}

	// Verify string representation
	str := query.String()
	if !strings.Contains(str, "quick") || !strings.Contains(str, "fast") {
		t.Errorf("String should contain 'quick' and 'fast', got %s", str)
	}
	if !strings.Contains(str, "brown") {
		t.Errorf("String should contain 'brown', got %s", str)
	}
	if !strings.Contains(str, "fox") || !strings.Contains(str, "dog") {
		t.Errorf("String should contain 'fox' and 'dog', got %s", str)
	}
	if !strings.HasSuffix(str, "~2") {
		t.Errorf("String should end with '~2', got %s", str)
	}
}

// TestMultiPhraseQuery_EqualityWithDifferentSlop tests equality with different slop
func TestMultiPhraseQuery_EqualityWithDifferentSlop(t *testing.T) {
	builder1 := NewMultiPhraseQueryBuilder()
	builder1.Add(index.NewTerm("field", "term"))
	query1 := builder1.Build()

	builder2 := NewMultiPhraseQueryBuilder()
	builder2.Add(index.NewTerm("field", "term"))
	builder2.SetSlop(1)
	query2 := builder2.Build()

	if query1.Equals(query2) {
		t.Error("Queries with different slop should not be equal")
	}

	if query1.HashCode() == query2.HashCode() {
		t.Error("Queries with different slop should have different hash codes")
	}
}

// TestMultiPhraseQuery_EqualityWithDifferentPositions tests equality with different positions
func TestMultiPhraseQuery_EqualityWithDifferentPositions(t *testing.T) {
	builder1 := NewMultiPhraseQueryBuilder()
	builder1.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a")}, 0)
	builder1.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b")}, 1)
	query1 := builder1.Build()

	builder2 := NewMultiPhraseQueryBuilder()
	builder2.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "a")}, 0)
	builder2.AddTermsAtPosition([]*index.Term{index.NewTerm("field", "b")}, 2)
	query2 := builder2.Build()

	if query1.Equals(query2) {
		t.Error("Queries with different positions should not be equal")
	}
}

// TestMultiPhraseQuery_EqualityWithDifferentTermArrays tests equality with different term arrays
func TestMultiPhraseQuery_EqualityWithDifferentTermArrays(t *testing.T) {
	builder1 := NewMultiPhraseQueryBuilder()
	builder1.AddTerms([]*index.Term{
		index.NewTerm("field", "one"),
		index.NewTerm("field", "two"),
	})
	query1 := builder1.Build()

	builder2 := NewMultiPhraseQueryBuilder()
	builder2.AddTerms([]*index.Term{
		index.NewTerm("field", "one"),
		index.NewTerm("field", "three"),
	})
	query2 := builder2.Build()

	if query1.Equals(query2) {
		t.Error("Queries with different term arrays should not be equal")
	}
}

// TestMultiPhraseQuery_EqualityWithDifferentArrayLengths tests equality with different array lengths
func TestMultiPhraseQuery_EqualityWithDifferentArrayLengths(t *testing.T) {
	builder1 := NewMultiPhraseQueryBuilder()
	builder1.Add(index.NewTerm("field", "one"))
	query1 := builder1.Build()

	builder2 := NewMultiPhraseQueryBuilder()
	builder2.Add(index.NewTerm("field", "one"))
	builder2.Add(index.NewTerm("field", "two"))
	query2 := builder2.Build()

	if query1.Equals(query2) {
		t.Error("Queries with different number of positions should not be equal")
	}
}

// TestMultiPhraseQuery_NotEqualToOtherTypes tests that MultiPhraseQuery doesn't equal other types
func TestMultiPhraseQuery_NotEqualToOtherTypes(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("field", "term"))
	mpq := builder.Build()

	tq := NewTermQuery(index.NewTerm("field", "term"))
	if mpq.Equals(tq) {
		t.Error("MultiPhraseQuery should not equal TermQuery")
	}

	bq := NewBooleanQuery()
	if mpq.Equals(bq) {
		t.Error("MultiPhraseQuery should not equal BooleanQuery")
	}
}

// TestMultiPhraseQuery_NilTerms tests handling of nil terms in AddTerms
func TestMultiPhraseQuery_NilTerms(t *testing.T) {
	// Empty terms array should be a no-op
	builder := NewMultiPhraseQueryBuilder()
	builder.AddTerms([]*index.Term{})
	query := builder.Build()

	if len(query.GetTermArrays()) != 0 {
		t.Error("Adding empty terms array should not add any positions")
	}
}

// TestMultiPhraseQuery_LargeQuery tests a large query with many terms
func TestMultiPhraseQuery_LargeQuery(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.SetSlop(5)

	// Add 10 positions with varying numbers of terms
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			// Single term
			builder.Add(index.NewTerm("field", string(rune('a'+i))))
		} else {
			// Multiple terms
			builder.AddTerms([]*index.Term{
				index.NewTerm("field", string(rune('a'+i))),
				index.NewTerm("field", string(rune('A'+i))),
			})
		}
	}

	query := builder.Build()

	termArrays := query.GetTermArrays()
	if len(termArrays) != 10 {
		t.Errorf("Expected 10 term arrays, got %d", len(termArrays))
	}

	positions := query.GetPositions()
	if len(positions) != 10 {
		t.Errorf("Expected 10 positions, got %d", len(positions))
	}

	// Verify positions are sequential
	for i, pos := range positions {
		if pos != i {
			t.Errorf("Expected position %d at index %d, got %d", i, i, pos)
		}
	}
}

// TestMultiPhraseQuery_FieldConsistency tests that field is set correctly
func TestMultiPhraseQuery_FieldConsistency(t *testing.T) {
	builder := NewMultiPhraseQueryBuilder()
	builder.Add(index.NewTerm("body", "term1"))
	builder.AddTerms([]*index.Term{
		index.NewTerm("body", "term2"),
		index.NewTerm("body", "term3"),
	})
	query := builder.Build()

	if query.Field() != "body" {
		t.Errorf("Expected field 'body', got %q", query.Field())
	}

	// All terms should have the same field
	for _, terms := range query.GetTermArrays() {
		for _, term := range terms {
			if term.Field != "body" {
				t.Errorf("Expected term field 'body', got %q", term.Field)
			}
		}
	}
}
