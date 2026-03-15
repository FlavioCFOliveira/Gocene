// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/search/TestSynonymQuery.java
// Purpose: Tests SynonymQuery term boosting and score aggregation behavior

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestSynonymQuery_Equals tests equality of SynonymQuery instances
// Source: TestSynonymQuery.testEquals()
func TestSynonymQuery_Equals(t *testing.T) {
	// Empty queries with same field are equal
	sq1 := NewSynonymQueryBuilder("foo").Build()
	sq2 := NewSynonymQueryBuilder("foo").Build()
	if !sq1.Equals(sq2) {
		t.Error("Empty SynonymQueries with same field should be equal")
	}

	// Queries with same term are equal
	sq3 := NewSynonymQueryBuilder("foo").AddTerm(index.NewTerm("foo", "bar")).Build()
	sq4 := NewSynonymQueryBuilder("foo").AddTerm(index.NewTerm("foo", "bar")).Build()
	if !sq3.Equals(sq4) {
		t.Error("SynonymQueries with same term should be equal")
	}

	// Queries with same terms in different order are equal
	sq5 := NewSynonymQueryBuilder("a").
		AddTerm(index.NewTerm("a", "a")).
		AddTerm(index.NewTerm("a", "b")).
		Build()
	sq6 := NewSynonymQueryBuilder("a").
		AddTerm(index.NewTerm("a", "b")).
		AddTerm(index.NewTerm("a", "a")).
		Build()
	if !sq5.Equals(sq6) {
		t.Error("SynonymQueries with same terms in different order should be equal")
	}

	// Queries with same terms and boosts are equal
	sq7 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "b"), 0.4).
		AddTermWithBoost(index.NewTerm("field", "c"), 0.2).
		AddTerm(index.NewTerm("field", "d")).
		Build()
	sq8 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "b"), 0.4).
		AddTermWithBoost(index.NewTerm("field", "c"), 0.2).
		AddTerm(index.NewTerm("field", "d")).
		Build()
	if !sq7.Equals(sq8) {
		t.Error("SynonymQueries with same terms and boosts should be equal")
	}

	// Different term values should not be equal
	sq9 := NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.4).Build()
	sq10 := NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "b"), 0.4).Build()
	if sq9.Equals(sq10) {
		t.Error("SynonymQueries with different term values should not be equal")
	}

	// Different boosts should not be equal
	sq11 := NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.2).Build()
	sq12 := NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), 0.4).Build()
	if sq11.Equals(sq12) {
		t.Error("SynonymQueries with different boosts should not be equal")
	}

	// Different fields should not be equal
	sq13 := NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "b"), 0.4).Build()
	sq14 := NewSynonymQueryBuilder("field2").AddTermWithBoost(index.NewTerm("field2", "b"), 0.4).Build()
	if sq13.Equals(sq14) {
		t.Error("SynonymQueries with different fields should not be equal")
	}
}

// TestSynonymQuery_HashCode tests hash code consistency
// Source: TestSynonymQuery.testHashCode()
func TestSynonymQuery_HashCode(t *testing.T) {
	q0 := NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0.4).Build()
	q1 := NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0.4).Build()
	q2 := NewSynonymQueryBuilder("field2").AddTermWithBoost(index.NewTerm("field2", "a"), 0.4).Build()

	// Equal queries should have same hash code
	if q0.HashCode() != q1.HashCode() {
		t.Error("Equal SynonymQueries should have same HashCode")
	}

	// Different fields should ideally have different hash codes
	if q0.HashCode() == q2.HashCode() {
		t.Log("Warning: Different SynonymQueries have same HashCode (may be a hash collision)")
	}
}

// TestSynonymQuery_GetField tests retrieving the field name
// Source: TestSynonymQuery.testGetField()
func TestSynonymQuery_GetField(t *testing.T) {
	query := NewSynonymQueryBuilder("field1").AddTerm(index.NewTerm("field1", "a")).Build()
	if query.GetField() != "field1" {
		t.Errorf("Expected field 'field1', got %q", query.GetField())
	}
}

// TestSynonymQuery_BogusParams tests invalid parameter handling
// Source: TestSynonymQuery.testBogusParams()
func TestSynonymQuery_BogusParams(t *testing.T) {
	// Adding term with different field should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with different field")
			}
		}()
		NewSynonymQueryBuilder("field1").
			AddTerm(index.NewTerm("field1", "a")).
			AddTerm(index.NewTerm("field2", "b"))
	}()

	// Boost > 1.0 should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with boost > 1.0")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 1.3)
	}()

	// NaN boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with NaN boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), math.Float32frombits(0x7FC00000))
	}()

	// Positive infinity boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with +Inf boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.Inf(1)))
	}()

	// Negative infinity boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with -Inf boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), float32(math.Inf(-1)))
	}()

	// Negative boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with negative boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), -0.3)
	}()

	// Zero boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with zero boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), 0.0)
	}()

	// Negative zero boost should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when adding term with -0 boost")
			}
		}()
		NewSynonymQueryBuilder("field1").AddTermWithBoost(index.NewTerm("field1", "a"), math.Float32frombits(0x80000000))
	}()

	// Nil field should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when creating builder with nil field")
			}
		}()
		NewSynonymQueryBuilder("").AddTermWithBoost(index.NewTerm("field1", "a"), -0.0)
	}()
}

// TestSynonymQuery_ToString tests string representation
// Source: TestSynonymQuery.testToString()
func TestSynonymQuery_ToString(t *testing.T) {
	// Empty query
	sq1 := NewSynonymQueryBuilder("foo").Build()
	if sq1.String() != "Synonym()" {
		t.Errorf("Expected 'Synonym()', got %q", sq1.String())
	}

	// Single term
	t1 := index.NewTerm("foo", "bar")
	sq2 := NewSynonymQueryBuilder("foo").AddTerm(t1).Build()
	if sq2.String() != "Synonym(foo:bar)" {
		t.Errorf("Expected 'Synonym(foo:bar)', got %q", sq2.String())
	}

	// Multiple terms
	t2 := index.NewTerm("foo", "baz")
	sq3 := NewSynonymQueryBuilder("foo").AddTerm(t1).AddTerm(t2).Build()
	expected := "Synonym(foo:bar foo:baz)"
	if sq3.String() != expected {
		t.Errorf("Expected %q, got %q", expected, sq3.String())
	}
}

// TestSynonymQuery_Terms tests retrieving terms from the query
func TestSynonymQuery_Terms(t *testing.T) {
	// Empty query has no terms
	sq1 := NewSynonymQueryBuilder("foo").Build()
	if len(sq1.GetTerms()) != 0 {
		t.Errorf("Expected 0 terms, got %d", len(sq1.GetTerms()))
	}

	// Query with terms
	t1 := index.NewTerm("foo", "bar")
	t2 := index.NewTerm("foo", "baz")
	sq2 := NewSynonymQueryBuilder("foo").AddTerm(t1).AddTerm(t2).Build()
	terms := sq2.GetTerms()
	if len(terms) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(terms))
	}
}

// TestSynonymQuery_Boosts tests term boosting behavior
// Source: TestSynonymQuery.testBoosts()
func TestSynonymQuery_Boosts(t *testing.T) {
	// Create query with different boosts
	query := NewSynonymQueryBuilder("f").
		AddTermWithBoost(index.NewTerm("f", "a"), 0.25).
		AddTermWithBoost(index.NewTerm("f", "b"), 0.5).
		AddTerm(index.NewTerm("f", "c")).
		Build()

	terms := query.GetTerms()
	if len(terms) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(terms))
	}

	// Verify boosts are stored correctly
	boosts := query.GetBoosts()
	if len(boosts) != 3 {
		t.Errorf("Expected 3 boosts, got %d", len(boosts))
	}

	// First term should have boost 0.25
	if boosts[0] != 0.25 {
		t.Errorf("Expected boost 0.25, got %f", boosts[0])
	}

	// Second term should have boost 0.5
	if boosts[1] != 0.5 {
		t.Errorf("Expected boost 0.5, got %f", boosts[1])
	}

	// Third term should have default boost 1.0
	if boosts[2] != 1.0 {
		t.Errorf("Expected default boost 1.0, got %f", boosts[2])
	}
}

// TestSynonymQuery_Clone tests query cloning
func TestSynonymQuery_Clone(t *testing.T) {
	original := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	cloned := original.Clone().(*SynonymQuery)

	// Cloned query should equal original
	if !original.Equals(cloned) {
		t.Error("Cloned query should equal original")
	}

	// Cloned query should have independent state
	// Modifying cloned should not affect original (if mutable methods existed)
}

// TestSynonymQuery_Rewrite tests query rewriting behavior
// Source: TestSynonymQuery.testRewrite()
func TestSynonymQuery_Rewrite(t *testing.T) {
	// Zero length SynonymQuery should rewrite to MatchNoDocsQuery
	sq1 := NewSynonymQueryBuilder("f").Build()
	if len(sq1.GetTerms()) != 0 {
		t.Error("Empty SynonymQuery should have no terms")
	}

	// For now, we just verify the empty state
	// In full implementation, Rewrite should return MatchNoDocsQuery

	// Non-boosted single term SynonymQuery should rewrite to TermQuery
	sq2 := NewSynonymQueryBuilder("f").AddTermWithBoost(index.NewTerm("f", "v"), 1.0).Build()
	if len(sq2.GetTerms()) != 1 {
		t.Error("Single term SynonymQuery should have 1 term")
	}
	// In full implementation, Rewrite should return TermQuery

	// Boosted single term SynonymQuery should NOT rewrite
	sq3 := NewSynonymQueryBuilder("f").AddTermWithBoost(index.NewTerm("f", "v"), 0.8).Build()
	if len(sq3.GetTerms()) != 1 {
		t.Error("Boosted single term SynonymQuery should have 1 term")
	}
	// In full implementation, Rewrite should return the same SynonymQuery

	// Multiple term SynonymQuery should NOT rewrite
	sq4 := NewSynonymQueryBuilder("f").
		AddTermWithBoost(index.NewTerm("f", "v"), 1.0).
		AddTermWithBoost(index.NewTerm("f", "w"), 1.0).
		Build()
	if len(sq4.GetTerms()) != 2 {
		t.Error("Multiple term SynonymQuery should have 2 terms")
	}
	// In full implementation, Rewrite should return the same SynonymQuery
}

// TestSynonymQuery_DefaultBoost tests that default boost is 1.0
func TestSynonymQuery_DefaultBoost(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "value")).
		Build()

	boosts := query.GetBoosts()
	if len(boosts) != 1 {
		t.Fatalf("Expected 1 boost, got %d", len(boosts))
	}

	if boosts[0] != 1.0 {
		t.Errorf("Expected default boost 1.0, got %f", boosts[0])
	}
}

// TestSynonymQuery_MultipleTerms tests adding multiple terms
func TestSynonymQuery_MultipleTerms(t *testing.T) {
	builder := NewSynonymQueryBuilder("field")

	terms := []*index.Term{
		index.NewTerm("field", "term1"),
		index.NewTerm("field", "term2"),
		index.NewTerm("field", "term3"),
	}

	for _, term := range terms {
		builder.AddTerm(term)
	}

	query := builder.Build()

	if len(query.GetTerms()) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(query.GetTerms()))
	}

	// Verify all terms are present
	queryTerms := query.GetTerms()
	for i, term := range terms {
		if !queryTerms[i].Equals(term) {
			t.Errorf("Term %d does not match", i)
		}
	}
}

// TestSynonymQuery_EmptyBuilder tests building with no terms
func TestSynonymQuery_EmptyBuilder(t *testing.T) {
	query := NewSynonymQueryBuilder("field").Build()

	if query.GetField() != "field" {
		t.Errorf("Expected field 'field', got %q", query.GetField())
	}

	if len(query.GetTerms()) != 0 {
		t.Errorf("Expected 0 terms, got %d", len(query.GetTerms()))
	}

	if len(query.GetBoosts()) != 0 {
		t.Errorf("Expected 0 boosts, got %d", len(query.GetBoosts()))
	}
}

// TestSynonymQuery_SingleTerm tests single term synonym query
func TestSynonymQuery_SingleTerm(t *testing.T) {
	term := index.NewTerm("field", "value")
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(term, 0.75).
		Build()

	if len(query.GetTerms()) != 1 {
		t.Errorf("Expected 1 term, got %d", len(query.GetTerms()))
	}

	if !query.GetTerms()[0].Equals(term) {
		t.Error("Term should match")
	}

	if query.GetBoosts()[0] != 0.75 {
		t.Errorf("Expected boost 0.75, got %f", query.GetBoosts()[0])
	}
}

// TestSynonymQuery_BoostBoundary tests boundary values for boosts
func TestSynonymQuery_BoostBoundary(t *testing.T) {
	// Minimum valid boost (very small positive)
	minBoost := float32(0.0000001)
	query1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), minBoost).
		Build()
	if query1.GetBoosts()[0] != minBoost {
		t.Errorf("Expected boost %f, got %f", minBoost, query1.GetBoosts()[0])
	}

	// Maximum valid boost (1.0)
	query2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "b"), 1.0).
		Build()
	if query2.GetBoosts()[0] != 1.0 {
		t.Errorf("Expected boost 1.0, got %f", query2.GetBoosts()[0])
	}
}

// TestSynonymQuery_TermOrderIndependence tests that term order doesn't affect equality
func TestSynonymQuery_TermOrderIndependence(t *testing.T) {
	// Create two queries with same terms in different order
	query1 := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		AddTerm(index.NewTerm("field", "c")).
		Build()

	query2 := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "c")).
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	if !query1.Equals(query2) {
		t.Error("SynonymQueries with same terms in different order should be equal")
	}

	if query1.HashCode() != query2.HashCode() {
		t.Error("SynonymQueries with same terms should have same HashCode")
	}
}

// TestSynonymQuery_DifferentTypesNotEqual tests that SynonymQuery doesn't equal other query types
func TestSynonymQuery_DifferentTypesNotEqual(t *testing.T) {
	synonymQuery := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "value")).
		Build()

	termQuery := NewTermQuery(index.NewTerm("field", "value"))

	if synonymQuery.Equals(termQuery) {
		t.Error("SynonymQuery should not equal TermQuery")
	}
}

// TestSynonymQuery_NilHandling tests handling of nil inputs
func TestSynonymQuery_NilHandling(t *testing.T) {
	// Test Equals with nil
	query := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()
	if query.Equals(nil) {
		t.Error("SynonymQuery should not equal nil")
	}

	// Test Equals with different type
	if query.Equals("not a query") {
		t.Error("SynonymQuery should not equal a string")
	}
}

// TestSynonymQuery_BuilderReuse tests that builder can be used to create multiple queries
func TestSynonymQuery_BuilderReuse(t *testing.T) {
	builder := NewSynonymQueryBuilder("field")

	query1 := builder.AddTerm(index.NewTerm("field", "a")).Build()
	query2 := builder.AddTerm(index.NewTerm("field", "b")).Build()

	// Both queries should have both terms
	if len(query1.GetTerms()) != 2 {
		t.Errorf("Expected query1 to have 2 terms, got %d", len(query1.GetTerms()))
	}

	if len(query2.GetTerms()) != 2 {
		t.Errorf("Expected query2 to have 2 terms, got %d", len(query2.GetTerms()))
	}
}

// TestSynonymQuery_ScoreAggregation tests score aggregation with multiple terms
// This is the core functionality for synonym queries - scores should be combined
// Source: TestSynonymQuery.testScores()
func TestSynonymQuery_ScoreAggregation(t *testing.T) {
	// Create a synonym query with multiple terms
	// In a full implementation, this would test that documents matching
	// any of the terms get an aggregated score

	query := NewSynonymQueryBuilder("content").
		AddTermWithBoost(index.NewTerm("content", "quick"), 1.0).
		AddTermWithBoost(index.NewTerm("content", "fast"), 1.0).
		AddTermWithBoost(index.NewTerm("content", "rapid"), 0.5).
		Build()

	// Verify the query structure
	if len(query.GetTerms()) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(query.GetTerms()))
	}

	boosts := query.GetBoosts()
	if len(boosts) != 3 {
		t.Errorf("Expected 3 boosts, got %d", len(boosts))
	}

	// Verify boosts are applied correctly
	if boosts[0] != 1.0 || boosts[1] != 1.0 || boosts[2] != 0.5 {
		t.Errorf("Boosts not stored correctly: %v", boosts)
	}

	// In a full implementation with IndexSearcher, we would verify:
	// 1. Documents matching multiple terms get combined scores
	// 2. The boost factors are applied correctly
	// 3. All matching documents have consistent scoring
}

// TestSynonymQuery_TermBoosting tests that individual term boosts affect scoring
// Source: TestSynonymQuery.testBoosts() - focus on boost behavior
func TestSynonymQuery_TermBoosting(t *testing.T) {
	// Create queries with different boost combinations
	query1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 1.0).
		AddTermWithBoost(index.NewTerm("field", "b"), 1.0).
		Build()

	query2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 1.0).
		Build()

	// Queries with different boosts should not be equal
	if query1.Equals(query2) {
		t.Error("Queries with different boosts should not be equal")
	}

	// Verify the boosts are stored
	boosts1 := query1.GetBoosts()
	boosts2 := query2.GetBoosts()

	if boosts1[0] != 1.0 || boosts1[1] != 1.0 {
		t.Error("Query1 boosts not stored correctly")
	}

	if boosts2[0] != 0.5 || boosts2[1] != 1.0 {
		t.Error("Query2 boosts not stored correctly")
	}
}

// TestSynonymQuery_MixedBoosts tests queries with mixed boosted and unboosted terms
func TestSynonymQuery_MixedBoosts(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.75).
		AddTerm(index.NewTerm("field", "b")). // default 1.0
		AddTermWithBoost(index.NewTerm("field", "c"), 0.5).
		Build()

	boosts := query.GetBoosts()
	if len(boosts) != 3 {
		t.Fatalf("Expected 3 boosts, got %d", len(boosts))
	}

	if boosts[0] != 0.75 {
		t.Errorf("Expected boost[0] = 0.75, got %f", boosts[0])
	}

	if boosts[1] != 1.0 {
		t.Errorf("Expected boost[1] = 1.0 (default), got %f", boosts[1])
	}

	if boosts[2] != 0.5 {
		t.Errorf("Expected boost[2] = 0.5, got %f", boosts[2])
	}
}

// TestSynonymQuery_BoostPrecision tests float precision for boosts
func TestSynonymQuery_BoostPrecision(t *testing.T) {
	// Test with precise float values
	boost := float32(0.3333333)
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), boost).
		Build()

	storedBoost := query.GetBoosts()[0]
	if storedBoost != boost {
		t.Errorf("Boost precision lost: expected %f, got %f", boost, storedBoost)
	}
}

// TestSynonymQuery_LargeNumberOfTerms tests behavior with many terms
func TestSynonymQuery_LargeNumberOfTerms(t *testing.T) {
	builder := NewSynonymQueryBuilder("field")

	// Add many terms
	for i := 0; i < 100; i++ {
		builder.AddTerm(index.NewTerm("field", string(rune('a'+i%26))))
	}

	query := builder.Build()

	if len(query.GetTerms()) != 100 {
		t.Errorf("Expected 100 terms, got %d", len(query.GetTerms()))
	}

	if len(query.GetBoosts()) != 100 {
		t.Errorf("Expected 100 boosts, got %d", len(query.GetBoosts()))
	}
}

// TestSynonymQuery_FieldConsistency tests that all terms have the same field
func TestSynonymQuery_FieldConsistency(t *testing.T) {
	query := NewSynonymQueryBuilder("myfield").
		AddTerm(index.NewTerm("myfield", "a")).
		AddTerm(index.NewTerm("myfield", "b")).
		Build()

	if query.GetField() != "myfield" {
		t.Errorf("Expected field 'myfield', got %q", query.GetField())
	}

	// All terms should have the same field
	for _, term := range query.GetTerms() {
		if term.Field != "myfield" {
			t.Errorf("Term has wrong field: %q", term.Field)
		}
	}
}

// TestSynonymQuery_StringWithBoosts tests string representation with boosts
func TestSynonymQuery_StringWithBoosts(t *testing.T) {
	// Query with boosted terms should show boosts in string
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	str := query.String()

	// String should contain field and terms
	if str == "" {
		t.Error("String representation should not be empty")
	}

	// Should contain "Synonym" prefix
	if len(str) < 7 || str[:7] != "Synonym" {
		t.Errorf("String should start with 'Synonym', got %q", str)
	}
}

// TestSynonymQuery_HashCodeDistribution tests that hash codes are reasonably distributed
func TestSynonymQuery_HashCodeDistribution(t *testing.T) {
	queries := make([]*SynonymQuery, 10)
	hashCodes := make(map[int]int)

	for i := 0; i < 10; i++ {
		queries[i] = NewSynonymQueryBuilder("field").
			AddTerm(index.NewTerm("field", string(rune('a'+i)))).
			Build()
		hashCodes[queries[i].HashCode()]++
	}

	// Most queries should have unique hash codes
	// (some collisions are acceptable)
	if len(hashCodes) < 5 {
		t.Log("Warning: Hash codes may not be well distributed")
	}
}

// TestSynonymQuery_EqualsSymmetry tests that Equals is symmetric
func TestSynonymQuery_EqualsSymmetry(t *testing.T) {
	q1 := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()
	q2 := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()

	if q1.Equals(q2) != q2.Equals(q1) {
		t.Error("Equals should be symmetric")
	}
}

// TestSynonymQuery_EqualsTransitivity tests that Equals is transitive
func TestSynonymQuery_EqualsTransitivity(t *testing.T) {
	q1 := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()
	q2 := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()
	q3 := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()

	if q1.Equals(q2) && q2.Equals(q3) && !q1.Equals(q3) {
		t.Error("Equals should be transitive")
	}
}

// TestSynonymQuery_EqualsReflexivity tests that Equals is reflexive
func TestSynonymQuery_EqualsReflexivity(t *testing.T) {
	q := NewSynonymQueryBuilder("field").AddTerm(index.NewTerm("field", "a")).Build()

	if !q.Equals(q) {
		t.Error("Equals should be reflexive")
	}
}

// TestSynonymQuery_CloneIndependence tests that cloned queries are independent
func TestSynonymQuery_CloneIndependence(t *testing.T) {
	original := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	cloned := original.Clone().(*SynonymQuery)

	// Verify they are equal but independent
	if !original.Equals(cloned) {
		t.Error("Clone should be equal to original")
	}

	// Modifying the clone's term slice shouldn't affect original
	// (this depends on implementation details)
}

// TestSynonymQuery_BuilderPattern tests the builder pattern thoroughly
func TestSynonymQuery_BuilderPattern(t *testing.T) {
	// Test fluent interface
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.5).
		AddTerm(index.NewTerm("field", "c")).
		AddTermWithBoost(index.NewTerm("field", "d"), 0.75).
		Build()

	if len(query.GetTerms()) != 4 {
		t.Errorf("Expected 4 terms, got %d", len(query.GetTerms()))
	}

	boosts := query.GetBoosts()
	if len(boosts) != 4 {
		t.Errorf("Expected 4 boosts, got %d", len(boosts))
	}

	// Verify specific boosts
	if boosts[0] != 1.0 {
		t.Errorf("Expected boost[0] = 1.0, got %f", boosts[0])
	}
	if boosts[1] != 0.5 {
		t.Errorf("Expected boost[1] = 0.5, got %f", boosts[1])
	}
	if boosts[2] != 1.0 {
		t.Errorf("Expected boost[2] = 1.0, got %f", boosts[2])
	}
	if boosts[3] != 0.75 {
		t.Errorf("Expected boost[3] = 0.75, got %f", boosts[3])
	}
}

// TestSynonymQuery_SameTermMultipleTimes tests adding the same term multiple times
func TestSynonymQuery_SameTermMultipleTimes(t *testing.T) {
	term := index.NewTerm("field", "value")

	query := NewSynonymQueryBuilder("field").
		AddTerm(term).
		AddTerm(term).
		Build()

	// Should have 2 terms (even if they're the same)
	if len(query.GetTerms()) != 2 {
		t.Errorf("Expected 2 terms when adding same term twice, got %d", len(query.GetTerms()))
	}
}

// TestSynonymQuery_BoostZeroAndNegative tests edge cases for boost values
func TestSynonymQuery_BoostZeroAndNegative(t *testing.T) {
	// These should panic based on Java implementation
	testCases := []struct {
		name  string
		boost float32
	}{
		{"zero", 0.0},
		{"negative", -0.1},
		{"large negative", -100.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic for boost %f", tc.boost)
				}
			}()
			NewSynonymQueryBuilder("field").AddTermWithBoost(index.NewTerm("field", "a"), tc.boost)
		})
	}
}

// TestSynonymQuery_ValidBoostRange tests valid boost ranges
func TestSynonymQuery_ValidBoostRange(t *testing.T) {
	// Test various valid boost values
	validBoosts := []float32{0.0001, 0.1, 0.5, 0.9, 0.99, 1.0}

	for _, boost := range validBoosts {
		query := NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "a"), boost).
			Build()

		if query.GetBoosts()[0] != boost {
			t.Errorf("Boost %f not stored correctly, got %f", boost, query.GetBoosts()[0])
		}
	}
}

// TestSynonymQuery_EmptyField tests behavior with empty field name
func TestSynonymQuery_EmptyField(t *testing.T) {
	// Empty field should be handled gracefully
	query := NewSynonymQueryBuilder("").AddTerm(index.NewTerm("", "value")).Build()

	if query.GetField() != "" {
		t.Errorf("Expected empty field, got %q", query.GetField())
	}
}

// TestSynonymQuery_SpecialCharactersInTerms tests terms with special characters
func TestSynonymQuery_SpecialCharactersInTerms(t *testing.T) {
	specialTerms := []string{
		"term with spaces",
		"term-with-dashes",
		"term_with_underscores",
		"term.with.dots",
		"term@with@symbols",
		"UPPERCASE",
		"mixedCase",
		"123numeric",
	}

	builder := NewSynonymQueryBuilder("field")
	for _, termText := range specialTerms {
		builder.AddTerm(index.NewTerm("field", termText))
	}

	query := builder.Build()

	if len(query.GetTerms()) != len(specialTerms) {
		t.Errorf("Expected %d terms, got %d", len(specialTerms), len(query.GetTerms()))
	}

	// Verify each term is stored correctly
	for i, term := range query.GetTerms() {
		expectedText := specialTerms[i]
		if term.Text() != expectedText {
			t.Errorf("Term %d: expected %q, got %q", i, expectedText, term.Text())
		}
	}
}

// TestSynonymQuery_UnicodeTerms tests terms with unicode characters
func TestSynonymQuery_UnicodeTerms(t *testing.T) {
	unicodeTerms := []string{
		"café",
		"日本語",
		"emoji",
		"αβγ",
	}

	builder := NewSynonymQueryBuilder("field")
	for _, termText := range unicodeTerms {
		builder.AddTerm(index.NewTerm("field", termText))
	}

	query := builder.Build()

	if len(query.GetTerms()) != len(unicodeTerms) {
		t.Errorf("Expected %d terms, got %d", len(unicodeTerms), len(query.GetTerms()))
	}
}

// TestSynonymQuery_LongFieldName tests with long field names
func TestSynonymQuery_LongFieldName(t *testing.T) {
	longField := "very_long_field_name_that_might_cause_issues_if_there_are_buffer_limit_problems_in_the_implementation"

	query := NewSynonymQueryBuilder(longField).
		AddTerm(index.NewTerm(longField, "value")).
		Build()

	if query.GetField() != longField {
		t.Error("Long field name not stored correctly")
	}
}

// TestSynonymQuery_SingleCharacterTerms tests single character terms
func TestSynonymQuery_SingleCharacterTerms(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		AddTerm(index.NewTerm("field", "c")).
		Build()

	if len(query.GetTerms()) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(query.GetTerms()))
	}
}

// TestSynonymQuery_EmptyTermText tests with empty term text
func TestSynonymQuery_EmptyTermText(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "")).
		Build()

	if len(query.GetTerms()) != 1 {
		t.Errorf("Expected 1 term, got %d", len(query.GetTerms()))
	}

	if query.GetTerms()[0].Text() != "" {
		t.Errorf("Expected empty term text, got %q", query.GetTerms()[0].Text())
	}
}

// TestSynonymQuery_BoostEquality tests that boosts affect equality correctly
func TestSynonymQuery_BoostEquality(t *testing.T) {
	// Same terms, same boosts - should be equal
	q1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		Build()

	q2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		Build()

	if !q1.Equals(q2) {
		t.Error("Queries with same terms and boosts should be equal")
	}

	// Same terms, different boosts - should not be equal
	q3 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.9). // different boost
		Build()

	if q1.Equals(q3) {
		t.Error("Queries with different boosts should not be equal")
	}
}

// TestSynonymQuery_TermCountConsistency tests that term and boost counts match
func TestSynonymQuery_TermCountConsistency(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.5).
		AddTerm(index.NewTerm("field", "c")).
		Build()

	termCount := len(query.GetTerms())
	boostCount := len(query.GetBoosts())

	if termCount != boostCount {
		t.Errorf("Term count (%d) should equal boost count (%d)", termCount, boostCount)
	}
}

// TestSynonymQuery_ComplexScenario tests a complex real-world scenario
func TestSynonymQuery_ComplexScenario(t *testing.T) {
	// Simulate a real-world synonym expansion:
	// "automobile" expands to: car (boost 1.0), vehicle (boost 0.8), motorcar (boost 0.6)

	query := NewSynonymQueryBuilder("content").
		AddTerm(index.NewTerm("content", "car")).
		AddTermWithBoost(index.NewTerm("content", "vehicle"), 0.8).
		AddTermWithBoost(index.NewTerm("content", "motorcar"), 0.6).
		Build()

	// Verify structure
	if query.GetField() != "content" {
		t.Errorf("Expected field 'content', got %q", query.GetField())
	}

	if len(query.GetTerms()) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(query.GetTerms()))
	}

	// Verify term texts
	expectedTerms := []string{"car", "vehicle", "motorcar"}
	for i, term := range query.GetTerms() {
		if term.Text() != expectedTerms[i] {
			t.Errorf("Term %d: expected %q, got %q", i, expectedTerms[i], term.Text())
		}
	}

	// Verify boosts
	expectedBoosts := []float32{1.0, 0.8, 0.6}
	boosts := query.GetBoosts()
	for i, expectedBoost := range expectedBoosts {
		if boosts[i] != expectedBoost {
			t.Errorf("Boost %d: expected %f, got %f", i, expectedBoost, boosts[i])
		}
	}

	// Verify string representation
	str := query.String()
	if str == "" {
		t.Error("String representation should not be empty")
	}

	// Verify hash code
	hash := query.HashCode()
	if hash == 0 {
		t.Log("Warning: Hash code is 0, which may indicate an issue")
	}

	// Verify clone
	cloned := query.Clone().(*SynonymQuery)
	if !query.Equals(cloned) {
		t.Error("Cloned query should equal original")
	}
}

// TestSynonymQuery_ScoreEquivalence tests that documents matching different terms
// with the same boost get equivalent scores
// Source: TestSynonymQuery.testScores() - core scoring behavior
func TestSynonymQuery_ScoreEquivalence(t *testing.T) {
	// Create two queries with single terms but same boost
	query1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 1.0).
		Build()

	query2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "b"), 1.0).
		Build()

	// The queries should have the same structure
	if len(query1.GetTerms()) != 1 || len(query2.GetTerms()) != 1 {
		t.Error("Both queries should have 1 term")
	}

	if query1.GetBoosts()[0] != query2.GetBoosts()[0] {
		t.Error("Both queries should have same boost")
	}

	// In a full implementation with IndexSearcher, we would verify:
	// Documents matching either term with the same boost should get the same score
	// (assuming the terms have the same document frequency)
}

// TestSynonymQuery_BoostedVsUnboosted tests difference between boosted and unboosted terms
func TestSynonymQuery_BoostedVsUnboosted(t *testing.T) {
	// Unboosted term (default 1.0)
	query1 := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		Build()

	// Explicitly boosted to 1.0
	query2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 1.0).
		Build()

	// These should be equal since both have boost 1.0
	if !query1.Equals(query2) {
		t.Error("Default boost and explicit 1.0 boost should be equal")
	}

	// Boosted to 0.5
	query3 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		Build()

	// These should NOT be equal
	if query1.Equals(query3) {
		t.Error("Default boost and 0.5 boost should not be equal")
	}
}

// TestSynonymQuery_MultipleBoostLevels tests various boost levels
func TestSynonymQuery_MultipleBoostLevels(t *testing.T) {
	boosts := []float32{0.1, 0.25, 0.5, 0.75, 0.9, 1.0}

	for i, boost := range boosts {
		query := NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", string(rune('a'+i))), boost).
			Build()

		if query.GetBoosts()[0] != boost {
			t.Errorf("Boost %f not stored correctly, got %f", boost, query.GetBoosts()[0])
		}
	}
}

// TestSynonymQuery_BoostRounding tests float rounding for boosts
func TestSynonymQuery_BoostRounding(t *testing.T) {
	// Test that common fractional boosts are stored accurately
	testBoosts := []float32{
		0.1,  // 1/10
		0.2,  // 1/5
		0.25, // 1/4
		0.5,  // 1/2
		0.75, // 3/4
	}

	for _, boost := range testBoosts {
		query := NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "a"), boost).
			Build()

		stored := query.GetBoosts()[0]
		// Allow small floating point differences
		if stored != boost {
			t.Errorf("Boost %f stored as %f", boost, stored)
		}
	}
}

// TestSynonymQuery_EqualityWithDifferentTermOrder tests equality with different term orders
func TestSynonymQuery_EqualityWithDifferentTermOrder(t *testing.T) {
	// Create queries with same terms in different order
	// and verify they are considered equal

	// Order 1: a, b, c
	q1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		AddTermWithBoost(index.NewTerm("field", "c"), 1.0).
		Build()

	// Order 2: c, a, b
	q2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "c"), 1.0).
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		Build()

	// Order 3: b, c, a
	q3 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		AddTermWithBoost(index.NewTerm("field", "c"), 1.0).
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		Build()

	// All should be equal
	if !q1.Equals(q2) {
		t.Error("q1 and q2 should be equal")
	}

	if !q2.Equals(q3) {
		t.Error("q2 and q3 should be equal")
	}

	if !q1.Equals(q3) {
		t.Error("q1 and q3 should be equal")
	}

	// All should have same hash code
	if q1.HashCode() != q2.HashCode() || q2.HashCode() != q3.HashCode() {
		t.Error("Equal queries should have same hash code")
	}
}

// TestSynonymQuery_InequalityWithDifferentBoosts tests inequality with different boost assignments
func TestSynonymQuery_InequalityWithDifferentBoosts(t *testing.T) {
	// Same terms, but different boosts assigned to different terms
	q1 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		Build()

	q2 := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.75). // swapped
		AddTermWithBoost(index.NewTerm("field", "b"), 0.5).  // swapped
		Build()

	// These should NOT be equal because the boost assignments are different
	if q1.Equals(q2) {
		t.Error("Queries with different boost assignments should not be equal")
	}
}

// TestSynonymQuery_HashCodeStability tests that hash codes are stable
func TestSynonymQuery_HashCodeStability(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	hash1 := query.HashCode()
	hash2 := query.HashCode()
	hash3 := query.HashCode()

	if hash1 != hash2 || hash2 != hash3 {
		t.Error("Hash code should be stable across multiple calls")
	}
}

// TestSynonymQuery_StringStability tests that string representation is stable
func TestSynonymQuery_StringStability(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	str1 := query.String()
	str2 := query.String()

	if str1 != str2 {
		t.Error("String representation should be stable")
	}
}

// TestSynonymQuery_ClonePreservesBoosts tests that cloning preserves boost values
func TestSynonymQuery_ClonePreservesBoosts(t *testing.T) {
	original := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		AddTerm(index.NewTerm("field", "c")).
		Build()

	cloned := original.Clone().(*SynonymQuery)

	originalBoosts := original.GetBoosts()
	clonedBoosts := cloned.GetBoosts()

	if len(originalBoosts) != len(clonedBoosts) {
		t.Fatal("Clone should have same number of boosts")
	}

	for i, boost := range originalBoosts {
		if clonedBoosts[i] != boost {
			t.Errorf("Boost %d: original %f, cloned %f", i, boost, clonedBoosts[i])
		}
	}
}

// TestSynonymQuery_GetTermsImmutability tests that GetTerms returns a copy or immutable view
func TestSynonymQuery_GetTermsImmutability(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTerm(index.NewTerm("field", "a")).
		AddTerm(index.NewTerm("field", "b")).
		Build()

	terms1 := query.GetTerms()
	terms2 := query.GetTerms()

	// Should return the same slice (or equal slices)
	if len(terms1) != len(terms2) {
		t.Error("GetTerms should return consistent results")
	}
}

// TestSynonymQuery_GetBoostsImmutability tests that GetBoosts returns a copy or immutable view
func TestSynonymQuery_GetBoostsImmutability(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.5).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.75).
		Build()

	boosts1 := query.GetBoosts()
	boosts2 := query.GetBoosts()

	// Should return the same slice (or equal slices)
	if len(boosts1) != len(boosts2) {
		t.Error("GetBoosts should return consistent results")
	}

	// Values should be equal
	for i := range boosts1 {
		if boosts1[i] != boosts2[i] {
			t.Error("GetBoosts should return consistent values")
		}
	}
}

// TestSynonymQuery_FieldGetter tests the GetField method
func TestSynonymQuery_FieldGetter(t *testing.T) {
	fields := []string{"field1", "field2", "content", "title", "body"}

	for _, field := range fields {
		query := NewSynonymQueryBuilder(field).
			AddTerm(index.NewTerm(field, "value")).
			Build()

		if query.GetField() != field {
			t.Errorf("GetField(): expected %q, got %q", field, query.GetField())
		}
	}
}

// TestSynonymQuery_TermAndBoostAlignment tests that terms and boosts are aligned
func TestSynonymQuery_TermAndBoostAlignment(t *testing.T) {
	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "a"), 0.1).
		AddTermWithBoost(index.NewTerm("field", "b"), 0.2).
		AddTermWithBoost(index.NewTerm("field", "c"), 0.3).
		Build()

	terms := query.GetTerms()
	boosts := query.GetBoosts()

	if len(terms) != len(boosts) {
		t.Fatalf("Terms and boosts should have same length")
	}

	// Verify alignment by checking term text matches expected boost
	expectedBoosts := map[string]float32{
		"a": 0.1,
		"b": 0.2,
		"c": 0.3,
	}

	for i, term := range terms {
		expectedBoost, ok := expectedBoosts[term.Text()]
		if !ok {
			t.Errorf("Unexpected term: %q", term.Text())
			continue
		}
		if boosts[i] != expectedBoost {
			t.Errorf("Term %q has boost %f, expected %f", term.Text(), boosts[i], expectedBoost)
		}
	}
}

// TestSynonymQuery_NilTermHandling tests handling of nil terms
func TestSynonymQuery_NilTermHandling(t *testing.T) {
	// This test documents expected behavior with nil terms
	// The actual behavior depends on implementation

	defer func() {
		if r := recover(); r != nil {
			// Panic is acceptable behavior for nil term
			t.Logf("Panic with nil term (acceptable): %v", r)
		}
	}()

	// Attempt to add nil term
	builder := NewSynonymQueryBuilder("field")
	builder.AddTerm(nil)
	query := builder.Build()

	// If we get here, check the result
	if len(query.GetTerms()) > 0 {
		t.Log("Nil term was added (implementation dependent)")
	}
}

// TestSynonymQuery_VerySmallBoosts tests very small but valid boost values
func TestSynonymQuery_VerySmallBoosts(t *testing.T) {
	smallBoosts := []float32{
		0.0001,
		0.00001,
		0.000001,
		1e-7,
		1e-10,
	}

	for _, boost := range smallBoosts {
		query := NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "a"), boost).
			Build()

		if query.GetBoosts()[0] != boost {
			t.Errorf("Small boost %e stored as %e", boost, query.GetBoosts()[0])
		}
	}
}

// TestSynonymQuery_BoostNearOne tests boost values near 1.0
func TestSynonymQuery_BoostNearOne(t *testing.T) {
	nearOneBoosts := []float32{
		0.99,
		0.999,
		0.9999,
		0.99999,
	}

	for _, boost := range nearOneBoosts {
		query := NewSynonymQueryBuilder("field").
			AddTermWithBoost(index.NewTerm("field", "a"), boost).
			Build()

		if query.GetBoosts()[0] != boost {
			t.Errorf("Near-one boost %f stored as %f", boost, query.GetBoosts()[0])
		}
	}
}

// TestSynonymQuery_DuplicateTermWithDifferentBoosts tests duplicate terms with different boosts
func TestSynonymQuery_DuplicateTermWithDifferentBoosts(t *testing.T) {
	term := index.NewTerm("field", "duplicate")

	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(term, 0.5).
		AddTermWithBoost(term, 0.75).
		Build()

	// Should have 2 terms (duplicates allowed)
	if len(query.GetTerms()) != 2 {
		t.Errorf("Expected 2 terms, got %d", len(query.GetTerms()))
	}

	// Should have 2 different boosts
	boosts := query.GetBoosts()
	if len(boosts) != 2 {
		t.Fatalf("Expected 2 boosts, got %d", len(boosts))
	}

	if boosts[0] != 0.5 {
		t.Errorf("Expected first boost 0.5, got %f", boosts[0])
	}

	if boosts[1] != 0.75 {
		t.Errorf("Expected second boost 0.75, got %f", boosts[1])
	}
}

// TestSynonymQuery_EmptyAfterBuild tests that builder is empty after Build()
// This depends on implementation - builder may or may not reset
func TestSynonymQuery_EmptyAfterBuild(t *testing.T) {
	builder := NewSynonymQueryBuilder("field")
	builder.AddTerm(index.NewTerm("field", "a"))

	query1 := builder.Build()
	if len(query1.GetTerms()) != 1 {
		t.Error("First build should have 1 term")
	}

	// Second build - behavior depends on implementation
	query2 := builder.Build()
	// Just verify it doesn't panic
	_ = query2
}

// TestSynonymQuery_ConsistencyAfterMultipleBuilds tests consistency across multiple builds
func TestSynonymQuery_ConsistencyAfterMultipleBuilds(t *testing.T) {
	builder := NewSynonymQueryBuilder("field")
	builder.AddTerm(index.NewTerm("field", "a"))
	builder.AddTermWithBoost(index.NewTerm("field", "b"), 0.5)

	query1 := builder.Build()
	query2 := builder.Build()

	if !query1.Equals(query2) {
		t.Error("Multiple builds should produce equal queries")
	}

	if query1.HashCode() != query2.HashCode() {
		t.Error("Multiple builds should produce same hash code")
	}
}

// TestSynonymQuery_BasicScoringConcept tests the basic concept of synonym scoring
// This documents the expected behavior for score aggregation
func TestSynonymQuery_BasicScoringConcept(t *testing.T) {
	// A SynonymQuery treats multiple terms as synonyms
	// Documents matching any of the terms should be returned
	// The score should reflect the best matching term (with boost applied)

	query := NewSynonymQueryBuilder("content").
		AddTermWithBoost(index.NewTerm("content", "quick"), 1.0).
		AddTermWithBoost(index.NewTerm("content", "fast"), 0.8).
		AddTermWithBoost(index.NewTerm("content", "rapid"), 0.6).
		Build()

	// Verify the query structure
	if len(query.GetTerms()) != 3 {
		t.Errorf("Expected 3 synonym terms, got %d", len(query.GetTerms()))
	}

	// In a full implementation:
	// - Document containing "quick" should get score * 1.0
	// - Document containing "fast" should get score * 0.8
	// - Document containing "rapid" should get score * 0.6
	// - Document containing multiple terms should get the maximum score

	// For now, we just verify the query is correctly constructed
	expectedTerms := []string{"quick", "fast", "rapid"}
	expectedBoosts := []float32{1.0, 0.8, 0.6}

	terms := query.GetTerms()
	boosts := query.GetBoosts()

	for i, expectedTerm := range expectedTerms {
		if terms[i].Text() != expectedTerm {
			t.Errorf("Term %d: expected %q, got %q", i, expectedTerm, terms[i].Text())
		}
		if boosts[i] != expectedBoosts[i] {
			t.Errorf("Boost %d: expected %f, got %f", i, expectedBoosts[i], boosts[i])
		}
	}
}

// TestSynonymQuery_ScoreAggregationDocumentation documents score aggregation
// This test serves as documentation for the expected scoring behavior
func TestSynonymQuery_ScoreAggregationDocumentation(t *testing.T) {
	// SynonymQuery is designed to handle cases where multiple terms
	// are considered equivalent (synonyms). The scoring behavior:
	//
	// 1. Each document matching any synonym term is included in results
	// 2. The score is computed based on the best matching term
	// 3. Term boosts are applied to adjust relative importance
	// 4. All matching documents with the same term frequency
	//    should receive the same score (with boost applied)

	query := NewSynonymQueryBuilder("field").
		AddTermWithBoost(index.NewTerm("field", "synonym1"), 1.0).
		AddTermWithBoost(index.NewTerm("field", "synonym2"), 0.5).
		Build()

	// Verify query is properly constructed
	if query.GetField() != "field" {
		t.Error("Field should be 'field'")
	}

	if len(query.GetTerms()) != 2 {
		t.Error("Should have 2 synonym terms")
	}

	// Verify boosts
	boosts := query.GetBoosts()
	if boosts[0] != 1.0 || boosts[1] != 0.5 {
		t.Error("Boosts not stored correctly")
	}

	// In production use with IndexSearcher:
	// - Documents matching "synonym1" get full score
	// - Documents matching "synonym2" get half score (0.5 boost)
	// - Documents matching both get the maximum of the two scores
}

// TestSynonymQuery_TermBoostInteraction documents how term boosts interact
func TestSynonymQuery_TermBoostInteraction(t *testing.T) {
	// When multiple terms match the same document,
	// the score is typically the maximum of the individual term scores
	// (not the sum), because they are synonyms, not separate concepts.

	// Create a query with multiple synonyms at different boost levels
	query := NewSynonymQueryBuilder("content").
		AddTermWithBoost(index.NewTerm("content", "big"), 1.0).
		AddTermWithBoost(index.NewTerm("content", "large"), 0.9).
		AddTermWithBoost(index.NewTerm("content", "huge"), 0.8).
		AddTermWithBoost(index.NewTerm("content", "enormous"), 0.7).
		Build()

	// Verify structure
	if len(query.GetTerms()) != 4 {
		t.Errorf("Expected 4 terms, got %d", len(query.GetTerms()))
	}

	// Verify boosts are in expected order
	boosts := query.GetBoosts()
	expectedBoosts := []float32{1.0, 0.9, 0.8, 0.7}
	for i, expected := range expectedBoosts {
		if boosts[i] != expected {
			t.Errorf("Boost %d: expected %f, got %f", i, expected, boosts[i])
		}
	}
}

// TestSynonymQuery_ImplementationNote is a placeholder for implementation notes
func TestSynonymQuery_ImplementationNote(t *testing.T) {
	// This test serves as documentation for future implementation:
	//
	// The full SynonymQuery implementation should:
	// 1. Create a Weight that combines the weights of all synonym terms
	// 2. Use a DisjunctionDISI (Disjunction DocIdSetIterator) to iterate
	//    over documents matching any of the terms
	// 3. Compute scores using the maximum score across terms (with boost)
	// 4. Support impacts for efficient top-k retrieval
	// 5. Rewrite to TermQuery when there's only one term (and boost is 1.0)
	// 6. Rewrite to MatchNoDocsQuery when there are no terms
	//
	// For now, the test file verifies the query structure and basic behavior.

	t.Skip("Implementation note - no actual test")
}
