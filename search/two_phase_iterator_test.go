// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

// TestTwoPhaseIterator_Basic tests basic TwoPhaseIterator functionality.
func TestTwoPhaseIterator_Basic(t *testing.T) {
	// Create an approximation iterator over docs 0-9
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create a matches function that only matches even documents
	matchesFunc := func() (bool, error) {
		return approx.DocID()%2 == 0, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Test that approximation is returned
	if tpi.Approximation() != approx {
		t.Error("Expected approximation to be returned")
	}

	// Test iteration
	var matchedDocs []int
	for {
		doc, err := approx.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matches, err := tpi.Matches()
		if err != nil {
			t.Fatalf("Matches error: %v", err)
		}
		if matches {
			matchedDocs = append(matchedDocs, doc)
		}
	}

	// Should match even docs: 0, 2, 4, 6, 8
	expected := []int{0, 2, 4, 6, 8}
	if len(matchedDocs) != len(expected) {
		t.Errorf("Expected %d matches, got %d: %v", len(expected), len(matchedDocs), matchedDocs)
	}
	for i, doc := range matchedDocs {
		if doc != expected[i] {
			t.Errorf("Expected doc %d at position %d, got %d", expected[i], i, doc)
		}
	}
}

// TestTwoPhaseIterator_AsDocIdSetIterator tests wrapping as DocIdSetIterator.
func TestTwoPhaseIterator_AsDocIdSetIterator(t *testing.T) {
	// Create an approximation iterator over docs 0-9
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create a matches function that only matches multiples of 3
	matchesFunc := func() (bool, error) {
		return approx.DocID()%3 == 0, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Test iteration
	var matchedDocs []int
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matchedDocs = append(matchedDocs, doc)
	}

	// Should match multiples of 3: 0, 3, 6, 9
	expected := []int{0, 3, 6, 9}
	if len(matchedDocs) != len(expected) {
		t.Errorf("Expected %d matches, got %d: %v", len(expected), len(matchedDocs), matchedDocs)
	}
	for i, doc := range matchedDocs {
		if doc != expected[i] {
			t.Errorf("Expected doc %d at position %d, got %d", expected[i], i, doc)
		}
	}
}

// TestTwoPhaseIterator_Advance tests the Advance method.
func TestTwoPhaseIterator_Advance(t *testing.T) {
	// Create an approximation iterator over docs 0-19
	approx := NewRangeDocIdSetIterator(0, 20)

	// Create a matches function that only matches even documents
	matchesFunc := func() (bool, error) {
		return approx.DocID()%2 == 0, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Advance to doc 5 - should land on 6 (first even >= 5)
	doc, err := it.Advance(5)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if doc != 6 {
		t.Errorf("Expected doc 6 after Advance(5), got %d", doc)
	}

	// Advance to doc 15 - should land on 16
	doc, err = it.Advance(15)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if doc != 16 {
		t.Errorf("Expected doc 16 after Advance(15), got %d", doc)
	}

	// Advance beyond range
	doc, err = it.Advance(20)
	if err != nil {
		t.Fatalf("Advance error: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS after Advance(20), got %d", doc)
	}
}

// TestTwoPhaseIterator_Cost tests the Cost method.
func TestTwoPhaseIterator_Cost(t *testing.T) {
	// Create an approximation iterator over docs 0-99
	approx := NewRangeDocIdSetIterator(0, 100)

	// Create a matches function
	matchesFunc := func() (bool, error) {
		return true, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Cost should be based on approximation
	if it.Cost() != 100 {
		t.Errorf("Expected cost 100, got %d", it.Cost())
	}
}

// TestConjunctionTwoPhaseIterator tests conjunction (AND) of two-phase iterators.
func TestConjunctionTwoPhaseIterator(t *testing.T) {
	// Create conjunction approximation (intersection of evens and multiples of 3)
	// For simplicity, use range 0-20 and filter
	approx := NewRangeDocIdSetIterator(0, 20)

	// Create conjunction two-phase iterator
	matches := []func() (bool, error){
		func() (bool, error) {
			return approx.DocID()%2 == 0, nil
		},
		func() (bool, error) {
			return approx.DocID()%3 == 0, nil
		},
	}

	tpi := NewConjunctionTwoPhaseIterator(approx, matches)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Test iteration
	var matchedDocs []int
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matchedDocs = append(matchedDocs, doc)
	}

	// Should match docs divisible by both 2 and 3: 0, 6, 12, 18
	expected := []int{0, 6, 12, 18}
	if len(matchedDocs) != len(expected) {
		t.Errorf("Expected %d matches, got %d: %v", len(expected), len(matchedDocs), matchedDocs)
	}
	for i, doc := range matchedDocs {
		if doc != expected[i] {
			t.Errorf("Expected doc %d at position %d, got %d", expected[i], i, doc)
		}
	}
}

// TestDisjunctionTwoPhaseIterator tests disjunction (OR) of two-phase iterators.
func TestDisjunctionTwoPhaseIterator(t *testing.T) {
	// Create disjunction approximation
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create matches functions
	// First matches even docs
	// Second matches multiples of 5
	matches := []func() (bool, error){
		func() (bool, error) {
			return approx.DocID()%2 == 0, nil
		},
		func() (bool, error) {
			return approx.DocID()%5 == 0, nil
		},
	}

	tpi := NewDisjunctionTwoPhaseIterator(approx, matches)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Test iteration
	var matchedDocs []int
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matchedDocs = append(matchedDocs, doc)
	}

	// Should match even docs OR multiples of 5: 0, 2, 4, 5, 6, 8
	expected := []int{0, 2, 4, 5, 6, 8}
	if len(matchedDocs) != len(expected) {
		t.Errorf("Expected %d matches, got %d: %v", len(expected), len(matchedDocs), matchedDocs)
	}
	for i, doc := range matchedDocs {
		if doc != expected[i] {
			t.Errorf("Expected doc %d at position %d, got %d", expected[i], i, doc)
		}
	}
}

// TestHasTwoPhaseIterator tests the HasTwoPhaseIterator function.
func TestHasTwoPhaseIterator(t *testing.T) {
	// Create a TwoPhaseIterator
	approx := NewRangeDocIdSetIterator(0, 10)
	matchesFunc := func() (bool, error) { return true, nil }
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Should be able to extract the TwoPhaseIterator
	extracted := HasTwoPhaseIterator(it)
	if extracted == nil {
		t.Error("Expected to extract TwoPhaseIterator from wrapper")
	}
	if extracted != tpi {
		t.Error("Extracted iterator should be the original")
	}

	// Regular iterator should return nil
	regularIt := NewRangeDocIdSetIterator(0, 10)
	if HasTwoPhaseIterator(regularIt) != nil {
		t.Error("Regular iterator should not have TwoPhaseIterator")
	}
}

// TestTwoPhaseIteratorScorer tests the TwoPhaseIteratorScorer.
func TestTwoPhaseIteratorScorer(t *testing.T) {
	// Create a TwoPhaseIterator
	approx := NewRangeDocIdSetIterator(0, 10)
	matchesFunc := func() (bool, error) {
		return approx.DocID()%2 == 0, nil
	}
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Create a mock weight using BaseWeight
	weight := NewBaseWeight(nil)

	// Create scorer
	scorer := NewTwoPhaseIteratorScorer(tpi, weight)

	// Test iteration
	var matchedDocs []int
	for {
		doc, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matchedDocs = append(matchedDocs, doc)

		// Check score
		score := scorer.Score()
		if score != 1.0 {
			t.Errorf("Expected score 1.0, got %f", score)
		}
	}

	// Should match even docs: 0, 2, 4, 6, 8
	expected := []int{0, 2, 4, 6, 8}
	if len(matchedDocs) != len(expected) {
		t.Errorf("Expected %d matches, got %d: %v", len(expected), len(matchedDocs), matchedDocs)
	}
}

// TestTwoPhaseIterator_EmptyApproximation tests with empty approximation.
func TestTwoPhaseIterator_EmptyApproximation(t *testing.T) {
	// Create an empty approximation
	approx := NewEmptyDocIdSetIterator()

	// Create a matches function
	matchesFunc := func() (bool, error) {
		return true, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Should immediately return NO_MORE_DOCS
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", doc)
	}
}

// TestTwoPhaseIterator_NoMatches tests when nothing matches.
func TestTwoPhaseIterator_NoMatches(t *testing.T) {
	// Create an approximation iterator over docs 0-9
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create a matches function that never matches
	matchesFunc := func() (bool, error) {
		return false, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Should immediately return NO_MORE_DOCS
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", doc)
	}
}

// TestTwoPhaseIterator_AllMatch tests when everything matches.
func TestTwoPhaseIterator_AllMatch(t *testing.T) {
	// Create an approximation iterator over docs 0-9
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create a matches function that always matches
	matchesFunc := func() (bool, error) {
		return true, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Should return all docs
	var matchedDocs []int
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc error: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matchedDocs = append(matchedDocs, doc)
	}

	// Should match all docs: 0-9
	if len(matchedDocs) != 10 {
		t.Errorf("Expected 10 matches, got %d", len(matchedDocs))
	}
	for i, doc := range matchedDocs {
		if doc != i {
			t.Errorf("Expected doc %d at position %d, got %d", i, i, doc)
		}
	}
}

// TestTwoPhaseIterator_DocIDRunEnd tests DocIDRunEnd behavior.
func TestTwoPhaseIterator_DocIDRunEnd(t *testing.T) {
	// Create an approximation iterator over docs 0-9
	approx := NewRangeDocIdSetIterator(0, 10)

	// Create a matches function
	matchesFunc := func() (bool, error) {
		return approx.DocID()%2 == 0, nil
	}

	// Create the two-phase iterator
	tpi := NewTwoPhaseIterator(approx, matchesFunc)

	// Wrap as DocIdSetIterator
	it := tpi.AsDocIdSetIterator()

	// Move to first doc
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != 0 {
		t.Errorf("Expected doc 0, got %d", doc)
	}

	// DocIDRunEnd should return doc + 1 for sparse matches
	runEnd := it.DocIDRunEnd()
	if runEnd != 1 {
		t.Errorf("Expected DocIDRunEnd 1, got %d", runEnd)
	}
}
