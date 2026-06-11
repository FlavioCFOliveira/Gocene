// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math/rand"
	"testing"
)

// TestDisjunctionDISIApproximation_DocIDRunEnd tests the docIDRunEnd method.
// This is the Go port of Lucene's TestDisjunctionDISIApproximation.testDocIDRunEnd().
func TestDisjunctionDISIApproximation_DocIDRunEnd(t *testing.T) {
	// Create three range iterators
	clause1 := NewRangeDocIdSetIterator(10000, 30000)
	clause2 := NewRangeDocIdSetIterator(20000, 50000)
	clause3 := NewRangeDocIdSetIterator(60000, 60001)

	// Generate a random lead cost (simulating TestUtil.nextLong(random(), 1, 100_000))
	leadCost := int64(rand.Intn(99999) + 1)

	// Create ConstantScoreScorers for each clause
	scorer1 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause1)
	scorer2 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause2)
	scorer3 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause3)

	// Create DisiWrappers
	wrapper1 := NewDisiWrapper(scorer1, false)
	wrapper2 := NewDisiWrapper(scorer2, false)
	wrapper3 := NewDisiWrapper(scorer3, false)

	// Create the DisjunctionDISIApproximation
	iterator := NewDisjunctionDISIApproximation(
		[]*DisiWrapper{wrapper1, wrapper2, wrapper3},
		leadCost,
	)

	// Test: nextDoc() should return 10000
	doc, err := iterator.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc() error: %v", err)
	}
	if doc != 10000 {
		t.Errorf("Expected nextDoc() = 10000, got %d", doc)
	}

	// Test: docIDRunEnd() should return 30000 (end of clause1's range)
	runEnd := iterator.DocIDRunEnd()
	if runEnd != 30000 {
		t.Errorf("Expected docIDRunEnd() = 30000, got %d", runEnd)
	}

	// Test: advance(25000) should return 25000
	advanced, err := iterator.Advance(25000)
	if err != nil {
		t.Fatalf("Advance(25000) error: %v", err)
	}
	if advanced != 25000 {
		t.Errorf("Expected advance(25000) = 25000, got %d", advanced)
	}

	// Test: docIDRunEnd() should return 50000 (end of clause2's range)
	runEnd = iterator.DocIDRunEnd()
	if runEnd != 50000 {
		t.Errorf("Expected docIDRunEnd() = 50000, got %d", runEnd)
	}

	// Test: advance(50000) should return 60000
	advanced, err = iterator.Advance(50000)
	if err != nil {
		t.Fatalf("Advance(50000) error: %v", err)
	}
	if advanced != 60000 {
		t.Errorf("Expected advance(50000) = 60000, got %d", advanced)
	}

	// Test: docIDRunEnd() should return 60001 (end of clause3's range)
	runEnd = iterator.DocIDRunEnd()
	if runEnd != 60001 {
		t.Errorf("Expected docIDRunEnd() = 60001, got %d", runEnd)
	}
}

// TestDisjunctionDISIApproximation_BasicOperations tests basic operations.
func TestDisjunctionDISIApproximation_BasicOperations(t *testing.T) {
	// Create simple range iterators
	clause1 := NewRangeDocIdSetIterator(0, 10)
	clause2 := NewRangeDocIdSetIterator(5, 15)

	scorer1 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause1)
	scorer2 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause2)

	wrapper1 := NewDisiWrapper(scorer1, false)
	wrapper2 := NewDisiWrapper(scorer2, false)

	iterator := NewDisjunctionDISIApproximation(
		[]*DisiWrapper{wrapper1, wrapper2},
		100,
	)

	// Test initial state
	if iterator.DocID() != -1 {
		t.Errorf("Expected initial docID = -1, got %d", iterator.DocID())
	}

	// Test NextDoc
	doc, err := iterator.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc() error: %v", err)
	}
	if doc != 0 {
		t.Errorf("Expected first doc = 0, got %d", doc)
	}

	// Test Advance
	advanced, err := iterator.Advance(7)
	if err != nil {
		t.Fatalf("Advance(7) error: %v", err)
	}
	if advanced != 7 {
		t.Errorf("Expected advance(7) = 7, got %d", advanced)
	}

	// Test Cost
	cost := iterator.Cost()
	if cost <= 0 {
		t.Errorf("Expected positive cost, got %d", cost)
	}

// TestDisjunctionDISIApproximation_Empty tests behavior with adjacent ranges.
}
func TestDisjunctionDISIApproximation_Empty(t *testing.T) {
	// Create iterators with adjacent ranges
	clause1 := NewRangeDocIdSetIterator(0, 5)
	clause2 := NewRangeDocIdSetIterator(5, 10)

	scorer1 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause1)
	scorer2 := NewConstantScoreScorer(1.0, ScoreMode(COMPLETE_NO_SCORES), clause2)

	wrapper1 := NewDisiWrapper(scorer1, false)
	wrapper2 := NewDisiWrapper(scorer2, false)

	iterator := NewDisjunctionDISIApproximation(
		[]*DisiWrapper{wrapper1, wrapper2},
		100,
	)

	// Advance to end of first range
	doc, err := iterator.Advance(4)
	if err != nil {
		t.Fatalf("Advance(4) error: %v", err)
	}
	if doc != 4 {
		t.Errorf("Expected doc = 4, got %d", doc)
	}

	// Check run end at boundary
	runEnd := iterator.DocIDRunEnd()
	if runEnd != 5 {
		t.Errorf("Expected run end = 5, got %d", runEnd)
	}
}