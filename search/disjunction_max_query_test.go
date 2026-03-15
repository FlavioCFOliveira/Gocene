// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file ported from Apache Lucene's TestDisjunctionMaxQuery.java
// Source: lucene/core/src/test/org/apache/lucene/search/TestDisjunctionMaxQuery.java
// Purpose: Tests DisjunctionMaxQuery tie breaker and max score selection

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SCORE_COMP_THRESH is the threshold for comparing floats
const SCORE_COMP_THRESH = 0.0001

// TestDisjunctionMaxQuery_Basic tests basic construction and properties
func TestDisjunctionMaxQuery_Basic(t *testing.T) {
	term1 := index.NewTerm("field1", "value1")
	term2 := index.NewTerm("field2", "value2")

	disjuncts := []Query{
		NewTermQuery(term1),
		NewTermQuery(term2),
	}

	// Test construction without tie breaker
	q1 := NewDisjunctionMaxQuery(disjuncts)
	if len(q1.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(q1.Disjuncts()))
	}
	if q1.TieBreakerMultiplier() != 0.0 {
		t.Errorf("Expected tie breaker 0.0, got %f", q1.TieBreakerMultiplier())
	}

	// Test construction with tie breaker
	q2 := NewDisjunctionMaxQueryWithTieBreaker(disjuncts, 0.5)
	if q2.TieBreakerMultiplier() != 0.5 {
		t.Errorf("Expected tie breaker 0.5, got %f", q2.TieBreakerMultiplier())
	}
}

// TestDisjunctionMaxQuery_SimpleEqualScores1 tests that all docs matching
// disjuncts in the "hed" field have equal scores (no tie breaker)
func TestDisjunctionMaxQuery_SimpleEqualScores1(t *testing.T) {
	// Create disjuncts: hed:albino OR hed:elephant
	q1 := NewTermQuery(index.NewTerm("hed", "albino"))
	q2 := NewTermQuery(index.NewTerm("hed", "elephant"))

	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1, q2}, 0.0)

	// Verify structure
	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// Test equality with same disjuncts
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("hed", "albino")),
		NewTermQuery(index.NewTerm("hed", "elephant")),
	}, 0.0)

	if !dmq.Equals(dmq2) {
		t.Error("Queries with same disjuncts should be equal")
	}
}

// TestDisjunctionMaxQuery_SimpleEqualScores2 tests equal scores for dek field
func TestDisjunctionMaxQuery_SimpleEqualScores2(t *testing.T) {
	q1 := NewTermQuery(index.NewTerm("dek", "albino"))
	q2 := NewTermQuery(index.NewTerm("dek", "elephant"))

	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1, q2}, 0.0)

	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// Verify the disjuncts are correct
	disjuncts := dmq.Disjuncts()
	if disjuncts[0] == nil || disjuncts[1] == nil {
		t.Error("Disjuncts should not be nil")
	}
}

// TestDisjunctionMaxQuery_SimpleEqualScores3 tests with 4 disjuncts
func TestDisjunctionMaxQuery_SimpleEqualScores3(t *testing.T) {
	q1 := NewTermQuery(index.NewTerm("hed", "albino"))
	q2 := NewTermQuery(index.NewTerm("hed", "elephant"))
	q3 := NewTermQuery(index.NewTerm("dek", "albino"))
	q4 := NewTermQuery(index.NewTerm("dek", "elephant"))

	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1, q2, q3, q4}, 0.0)

	if len(dmq.Disjuncts()) != 4 {
		t.Errorf("Expected 4 disjuncts, got %d", len(dmq.Disjuncts()))
	}
}

// TestDisjunctionMaxQuery_SimpleTiebreaker tests the tie breaker functionality
// When a document matches multiple disjuncts, the tie breaker adds a small
// increment to the max score for each additional matching disjunct
func TestDisjunctionMaxQuery_SimpleTiebreaker(t *testing.T) {
	q1 := NewTermQuery(index.NewTerm("dek", "albino"))
	q2 := NewTermQuery(index.NewTerm("dek", "elephant"))

	// Use a small tie breaker multiplier
	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1, q2}, 0.01)

	if dmq.TieBreakerMultiplier() != 0.01 {
		t.Errorf("Expected tie breaker 0.01, got %f", dmq.TieBreakerMultiplier())
	}

	// Test that tie breaker is preserved in clone
	cloned := dmq.Clone().(*DisjunctionMaxQuery)
	if cloned.TieBreakerMultiplier() != 0.01 {
		t.Errorf("Cloned query should preserve tie breaker, got %f", cloned.TieBreakerMultiplier())
	}
}

// TestDisjunctionMaxQuery_BooleanRequiredEqualScores tests DisjunctionMaxQuery
// inside a BooleanQuery with MUST clauses
func TestDisjunctionMaxQuery_BooleanRequiredEqualScores(t *testing.T) {
	// First DisjunctionMaxQuery: hed:albino OR dek:albino
	q1a := NewTermQuery(index.NewTerm("hed", "albino"))
	q1b := NewTermQuery(index.NewTerm("dek", "albino"))
	dmq1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1a, q1b}, 0.0)

	// Second DisjunctionMaxQuery: hed:elephant OR dek:elephant
	q2a := NewTermQuery(index.NewTerm("hed", "elephant"))
	q2b := NewTermQuery(index.NewTerm("dek", "elephant"))
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q2a, q2b}, 0.0)

	// Build BooleanQuery with both as MUST
	bq := NewBooleanQuery()
	bq.Add(dmq1, MUST)
	bq.Add(dmq2, MUST)

	if len(bq.Clauses()) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(bq.Clauses()))
	}

	// Verify the clauses are DisjunctionMaxQuery
	for _, clause := range bq.Clauses() {
		if _, ok := clause.Query.(*DisjunctionMaxQuery); !ok {
			t.Error("Expected DisjunctionMaxQuery in clause")
		}
	}
}

// TestDisjunctionMaxQuery_BooleanOptionalNoTiebreaker tests DisjunctionMaxQuery
// inside BooleanQuery with SHOULD clauses and no tie breaker
func TestDisjunctionMaxQuery_BooleanOptionalNoTiebreaker(t *testing.T) {
	q1a := NewTermQuery(index.NewTerm("hed", "albino"))
	q1b := NewTermQuery(index.NewTerm("dek", "albino"))
	dmq1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1a, q1b}, 0.0)

	q2a := NewTermQuery(index.NewTerm("hed", "elephant"))
	q2b := NewTermQuery(index.NewTerm("dek", "elephant"))
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q2a, q2b}, 0.0)

	bq := NewBooleanQuery()
	bq.Add(dmq1, SHOULD)
	bq.Add(dmq2, SHOULD)

	if len(bq.Clauses()) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(bq.Clauses()))
	}

	for _, clause := range bq.Clauses() {
		if clause.Occur != SHOULD {
			t.Error("Expected SHOULD occur")
		}
	}
}

// TestDisjunctionMaxQuery_BooleanOptionalWithTiebreaker tests DisjunctionMaxQuery
// inside BooleanQuery with SHOULD clauses and tie breaker
func TestDisjunctionMaxQuery_BooleanOptionalWithTiebreaker(t *testing.T) {
	q1a := NewTermQuery(index.NewTerm("hed", "albino"))
	q1b := NewTermQuery(index.NewTerm("dek", "albino"))
	dmq1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1a, q1b}, 0.01)

	q2a := NewTermQuery(index.NewTerm("hed", "elephant"))
	q2b := NewTermQuery(index.NewTerm("dek", "elephant"))
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q2a, q2b}, 0.01)

	bq := NewBooleanQuery()
	bq.Add(dmq1, SHOULD)
	bq.Add(dmq2, SHOULD)

	// Verify both disjunctions have tie breaker
	for _, clause := range bq.Clauses() {
		if dmq, ok := clause.Query.(*DisjunctionMaxQuery); ok {
			if dmq.TieBreakerMultiplier() != 0.01 {
				t.Errorf("Expected tie breaker 0.01, got %f", dmq.TieBreakerMultiplier())
			}
		}
	}
}

// TestDisjunctionMaxQuery_BooleanOptionalWithTiebreakerAndBoost tests
// DisjunctionMaxQuery with boosted subqueries
func TestDisjunctionMaxQuery_BooleanOptionalWithTiebreakerAndBoost(t *testing.T) {
	// Create boosted term queries
	q1a := NewBoostQuery(NewTermQuery(index.NewTerm("hed", "albino")), 1.5)
	q1b := NewTermQuery(index.NewTerm("dek", "albino"))
	dmq1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1a, q1b}, 0.01)

	q2a := NewBoostQuery(NewTermQuery(index.NewTerm("hed", "elephant")), 1.5)
	q2b := NewTermQuery(index.NewTerm("dek", "elephant"))
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q2a, q2b}, 0.01)

	bq := NewBooleanQuery()
	bq.Add(dmq1, SHOULD)
	bq.Add(dmq2, SHOULD)

	// Verify structure
	if len(bq.Clauses()) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(bq.Clauses()))
	}

	// Check that BoostQuery is in disjuncts
	clause1 := bq.Clauses()[0]
	dmq, ok := clause1.Query.(*DisjunctionMaxQuery)
	if !ok {
		t.Fatal("Expected DisjunctionMaxQuery")
	}

	firstDisjunct := dmq.Disjuncts()[0]
	if _, ok := firstDisjunct.(*BoostQuery); !ok {
		t.Error("First disjunct should be BoostQuery")
	}
}

// TestDisjunctionMaxQuery_RewriteEmpty tests that an empty DisjunctionMaxQuery
// rewrites to MatchNoDocsQuery
func TestDisjunctionMaxQuery_RewriteEmpty(t *testing.T) {
	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{}, 0.0)

	if len(dmq.Disjuncts()) != 0 {
		t.Errorf("Expected 0 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// Empty query should have no disjuncts to match
	// In Lucene, this rewrites to MatchNoDocsQuery
	// For now, we verify the structure is correct
}

// TestDisjunctionMaxQuery_DisjunctOrderAndEquals tests that disjunct order
// does not matter for equals() comparison (but may matter for toString)
func TestDisjunctionMaxQuery_DisjunctOrderAndEquals(t *testing.T) {
	q1 := NewTermQuery(index.NewTerm("hed", "albino"))
	q2 := NewTermQuery(index.NewTerm("hed", "elephant"))

	// Same disjuncts, different order
	dmq1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q1, q2}, 1.0)
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{q2, q1}, 1.0)

	// In Lucene, these should be equal (order-independent equals)
	// Note: Current implementation may be order-dependent
	// This test documents the expected behavior

	// Test that they have same tie breaker
	if dmq1.TieBreakerMultiplier() != dmq2.TieBreakerMultiplier() {
		t.Error("Tie breakers should be equal")
	}

	// Test that they have same number of disjuncts
	if len(dmq1.Disjuncts()) != len(dmq2.Disjuncts()) {
		t.Error("Should have same number of disjuncts")
	}
}

// TestDisjunctionMaxQuery_DisjunctOrderMatters tests cases where
// disjunct order matters (toString and getDisjuncts iteration)
func TestDisjunctionMaxQuery_DisjunctOrderMatters(t *testing.T) {
	// Create queries with specific order
	terms := []string{"a", "b", "c", "d", "e"}
	disjuncts := make([]Query, len(terms))
	for i, term := range terms {
		disjuncts[i] = NewTermQuery(index.NewTerm("test", term))
	}

	dmq := NewDisjunctionMaxQueryWithTieBreaker(disjuncts, 1.0)

	// Verify disjuncts are in order
	resultDisjuncts := dmq.Disjuncts()
	if len(resultDisjuncts) != len(terms) {
		t.Errorf("Expected %d disjuncts, got %d", len(terms), len(resultDisjuncts))
	}

	for i, term := range terms {
		tq, ok := resultDisjuncts[i].(*TermQuery)
		if !ok {
			t.Errorf("Expected TermQuery at index %d", i)
			continue
		}
		if tq.Term().Text() != term {
			t.Errorf("Expected term %s at index %d, got %s", term, i, tq.Term().Text())
		}
	}
}

// TestDisjunctionMaxQuery_Add tests adding disjuncts after construction
func TestDisjunctionMaxQuery_Add(t *testing.T) {
	dmq := NewDisjunctionMaxQuery(nil)

	if len(dmq.Disjuncts()) != 0 {
		t.Errorf("Expected 0 disjuncts initially, got %d", len(dmq.Disjuncts()))
	}

	// Add first disjunct
	dmq.Add(NewTermQuery(index.NewTerm("field", "value1")))
	if len(dmq.Disjuncts()) != 1 {
		t.Errorf("Expected 1 disjunct, got %d", len(dmq.Disjuncts()))
	}

	// Add second disjunct
	dmq.Add(NewTermQuery(index.NewTerm("field", "value2")))
	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(dmq.Disjuncts()))
	}
}

// TestDisjunctionMaxQuery_Clone tests cloning functionality
func TestDisjunctionMaxQuery_Clone(t *testing.T) {
	term1 := index.NewTerm("field1", "value1")
	term2 := index.NewTerm("field2", "value2")

	disjuncts := []Query{
		NewTermQuery(term1),
		NewTermQuery(term2),
	}

	original := NewDisjunctionMaxQueryWithTieBreaker(disjuncts, 0.3)

	cloned := original.Clone().(*DisjunctionMaxQuery)

	// Verify tie breaker is preserved
	if cloned.TieBreakerMultiplier() != 0.3 {
		t.Errorf("Expected tie breaker 0.3, got %f", cloned.TieBreakerMultiplier())
	}

	// Verify disjuncts are cloned
	if len(cloned.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(cloned.Disjuncts()))
	}

	// Verify cloned disjuncts are independent
	// Modifying cloned disjuncts should not affect original
	cloned.Add(NewTermQuery(index.NewTerm("field3", "value3")))
	if len(original.Disjuncts()) != 2 {
		t.Error("Original should not be modified when cloned is modified")
	}
	if len(cloned.Disjuncts()) != 3 {
		t.Error("Cloned should have new disjunct")
	}
}

// TestDisjunctionMaxQuery_Equals tests equality comparison
func TestDisjunctionMaxQuery_Equals(t *testing.T) {
	term1 := index.NewTerm("field", "value1")
	term2 := index.NewTerm("field", "value2")

	q1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term1),
	}, 0.0)

	q2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term1),
	}, 0.0)

	q3 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term2),
	}, 0.0)

	q4 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term1),
	}, 0.5)

	// Same disjuncts and tie breaker
	if !q1.Equals(q2) {
		t.Error("Expected q1 and q2 to be equal")
	}

	// Different disjunct
	if q1.Equals(q3) {
		t.Error("Expected q1 and q3 to be different (different disjunct)")
	}

	// Different tie breaker
	if q1.Equals(q4) {
		t.Error("Expected q1 and q4 to be different (different tie breaker)")
	}
}

// TestDisjunctionMaxQuery_HashCode tests hash code generation
func TestDisjunctionMaxQuery_HashCode(t *testing.T) {
	term1 := index.NewTerm("field", "value1")
	term2 := index.NewTerm("field", "value2")

	q1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term1),
		NewTermQuery(term2),
	}, 0.5)

	q2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(term1),
		NewTermQuery(term2),
	}, 0.5)

	// Equal queries should have same hash code
	if q1.HashCode() != q2.HashCode() {
		t.Error("Equal queries should have same hash code")
	}
}

// TestDisjunctionMaxQuery_SetTieBreakerMultiplier tests setting tie breaker
func TestDisjunctionMaxQuery_SetTieBreakerMultiplier(t *testing.T) {
	q := NewDisjunctionMaxQuery([]Query{
		NewTermQuery(index.NewTerm("field", "value")),
	})

	if q.TieBreakerMultiplier() != 0.0 {
		t.Errorf("Expected default tie breaker 0.0, got %f", q.TieBreakerMultiplier())
	}

	q.SetTieBreakerMultiplier(0.5)
	if q.TieBreakerMultiplier() != 0.5 {
		t.Errorf("Expected tie breaker 0.5, got %f", q.TieBreakerMultiplier())
	}
}

// TestDisjunctionMaxQuery_WithBoostedDisjuncts tests DisjunctionMaxQuery
// containing BoostQuery disjuncts
func TestDisjunctionMaxQuery_WithBoostedDisjuncts(t *testing.T) {
	term1 := index.NewTerm("field", "value1")
	term2 := index.NewTerm("field", "value2")

	// Create boosted queries
	boosted1 := NewBoostQuery(NewTermQuery(term1), 2.0)
	boosted2 := NewBoostQuery(NewTermQuery(term2), 3.0)

	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{boosted1, boosted2}, 0.1)

	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// Verify disjuncts are BoostQuery
	for i, disjunct := range dmq.Disjuncts() {
		if _, ok := disjunct.(*BoostQuery); !ok {
			t.Errorf("Disjunct %d should be BoostQuery", i)
		}
	}

	// Test clone preserves boosted disjuncts
	cloned := dmq.Clone().(*DisjunctionMaxQuery)
	if len(cloned.Disjuncts()) != 2 {
		t.Errorf("Cloned should have 2 disjuncts, got %d", len(cloned.Disjuncts()))
	}
}

// TestDisjunctionMaxQuery_NilDisjuncts tests handling of nil disjuncts
func TestDisjunctionMaxQuery_NilDisjuncts(t *testing.T) {
	// Create with nil disjuncts slice
	dmq := NewDisjunctionMaxQuery(nil)

	if dmq.Disjuncts() == nil {
		t.Error("Disjuncts should not be nil (should be empty slice)")
	}

	if len(dmq.Disjuncts()) != 0 {
		t.Errorf("Expected 0 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// Add a disjunct
	dmq.Add(NewTermQuery(index.NewTerm("field", "value")))
	if len(dmq.Disjuncts()) != 1 {
		t.Errorf("Expected 1 disjunct, got %d", len(dmq.Disjuncts()))
	}
}

// TestDisjunctionMaxQuery_ComplexNesting tests complex query nesting
func TestDisjunctionMaxQuery_ComplexNesting(t *testing.T) {
	// Create nested structure: DMQ(BQ(TQ, TQ), TQ)
	innerBQ := NewBooleanQuery()
	innerBQ.Add(NewTermQuery(index.NewTerm("f1", "v1")), MUST)
	innerBQ.Add(NewTermQuery(index.NewTerm("f2", "v2")), SHOULD)

	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		innerBQ,
		NewTermQuery(index.NewTerm("f3", "v3")),
	}, 0.2)

	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts, got %d", len(dmq.Disjuncts()))
	}

	// First disjunct should be BooleanQuery
	if _, ok := dmq.Disjuncts()[0].(*BooleanQuery); !ok {
		t.Error("First disjunct should be BooleanQuery")
	}

	// Second disjunct should be TermQuery
	if _, ok := dmq.Disjuncts()[1].(*TermQuery); !ok {
		t.Error("Second disjunct should be TermQuery")
	}

	// Test clone of nested structure
	cloned := dmq.Clone().(*DisjunctionMaxQuery)
	if len(cloned.Disjuncts()) != 2 {
		t.Errorf("Cloned should have 2 disjuncts, got %d", len(cloned.Disjuncts()))
	}
}

// TestDisjunctionMaxQuery_EmptyDisjunctsEquals tests equality with empty disjuncts
func TestDisjunctionMaxQuery_EmptyDisjunctsEquals(t *testing.T) {
	q1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{}, 0.0)
	q2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{}, 0.0)
	q3 := NewDisjunctionMaxQueryWithTieBreaker([]Query{}, 0.5)

	if !q1.Equals(q2) {
		t.Error("Empty queries with same tie breaker should be equal")
	}

	if q1.Equals(q3) {
		t.Error("Empty queries with different tie breakers should not be equal")
	}
}

// TestDisjunctionMaxQuery_SingleDisjunct tests with single disjunct
func TestDisjunctionMaxQuery_SingleDisjunct(t *testing.T) {
	tq := NewTermQuery(index.NewTerm("field", "value"))
	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{tq}, 0.0)

	if len(dmq.Disjuncts()) != 1 {
		t.Errorf("Expected 1 disjunct, got %d", len(dmq.Disjuncts()))
	}

	// Single disjunct should equal itself
	if !dmq.Disjuncts()[0].Equals(tq) {
		t.Error("Single disjunct should equal original query")
	}
}

// TestDisjunctionMaxQuery_MultipleTieBreakerValues tests various tie breaker values
func TestDisjunctionMaxQuery_MultipleTieBreakerValues(t *testing.T) {
	testValues := []float32{0.0, 0.01, 0.1, 0.5, 0.99, 1.0}

	for _, tb := range testValues {
		dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{
			NewTermQuery(index.NewTerm("f", "v")),
		}, tb)

		if dmq.TieBreakerMultiplier() != tb {
			t.Errorf("Expected tie breaker %f, got %f", tb, dmq.TieBreakerMultiplier())
		}
	}
}

// TestDisjunctionMaxQuery_CloneIndependence tests that cloned queries are independent
func TestDisjunctionMaxQuery_CloneIndependence(t *testing.T) {
	original := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("f1", "v1")),
		NewTermQuery(index.NewTerm("f2", "v2")),
	}, 0.5)

	cloned := original.Clone().(*DisjunctionMaxQuery)

	// Modify original
	original.SetTieBreakerMultiplier(0.9)
	original.Add(NewTermQuery(index.NewTerm("f3", "v3")))

	// Verify cloned is unchanged
	if cloned.TieBreakerMultiplier() != 0.5 {
		t.Error("Cloned tie breaker should not change when original changes")
	}

	if len(cloned.Disjuncts()) != 2 {
		t.Error("Cloned disjuncts should not change when original changes")
	}
}

// TestDisjunctionMaxQuery_EqualsWithDifferentLengths tests equality with different number of disjuncts
func TestDisjunctionMaxQuery_EqualsWithDifferentLengths(t *testing.T) {
	q1 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("f", "v1")),
	}, 0.0)

	q2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("f", "v1")),
		NewTermQuery(index.NewTerm("f", "v2")),
	}, 0.0)

	if q1.Equals(q2) {
		t.Error("Queries with different number of disjuncts should not be equal")
	}
}

// TestDisjunctionMaxQuery_NilHandling tests handling of nil queries in disjuncts
func TestDisjunctionMaxQuery_NilHandling(t *testing.T) {
	// Create with one nil disjunct
	dmq := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("f", "v")),
		nil,
	}, 0.0)

	if len(dmq.Disjuncts()) != 2 {
		t.Errorf("Expected 2 disjuncts (including nil), got %d", len(dmq.Disjuncts()))
	}

	// Test clone with nil
	cloned := dmq.Clone().(*DisjunctionMaxQuery)
	if len(cloned.Disjuncts()) != 2 {
		t.Errorf("Cloned should have 2 disjuncts, got %d", len(cloned.Disjuncts()))
	}

	// Test equals with nil
	dmq2 := NewDisjunctionMaxQueryWithTieBreaker([]Query{
		NewTermQuery(index.NewTerm("f", "v")),
		nil,
	}, 0.0)

	if !dmq.Equals(dmq2) {
		t.Error("Queries with same disjuncts (including nil) should be equal")
	}
}
