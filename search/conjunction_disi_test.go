// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: conjunction_disi_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestConjunctionDISI.java
// Purpose: Tests for ConjunctionDISI - AND operation on multiple DocIdSetIterators
//
// ConjunctionDISI performs a conjunction (AND) of multiple DocIdSetIterators,
// returning only documents that match ALL iterators. It supports two-phase
// iteration for efficient approximation and confirmation.

package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestConjunctionDISI_Conjunction tests basic conjunction correctness
// Source: TestConjunctionDISI.testConjunction()
// Purpose: Tests that the conjunction iterator returns correct intersection of documents
func TestConjunctionDISI_Conjunction(t *testing.T) {
	// Number of iterations for randomized testing
	numIters := atLeast(100)

	for iter := 0; iter < numIters; iter++ {
		maxDoc := nextInt(100, 10000)
		numIterators := nextInt(2, 5)

		sets := make([]*util.FixedBitSet, numIterators)
		iterators := make([]search.DocIdSetIterator, numIterators)

		for i := 0; i < numIterators; i++ {
			set := randomSet(maxDoc)
			sets[i] = set

			// Create different types of iterators based on random choice
			choice := rand.Intn(3)
			switch choice {
			case 0:
				// Simple anonymized iterator
				iterators[i] = anonymizeIterator(newBitSetIterator(set))
			case 1:
				// BitSet iterator
				iterators[i] = newBitSetIterator(set)
			default:
				// Scorer with approximation (TwoPhaseIterator)
				confirmed := clearRandomBits(set)
				sets[i] = confirmed // Use confirmed set for expected results
				approximation := createApproximation(newBitSetIterator(set), confirmed)
				iterators[i] = createScorerWithTwoPhase(approximation)
			}
		}

		// Create conjunction
		conjunction, err := search.IntersectIterators(iterators)
		if err != nil {
			t.Fatalf("Failed to create conjunction: %v", err)
		}

		// Verify results match expected intersection
		expected := intersect(sets)
		actual := toBitSet(maxDoc, conjunction)

		if !expected.Equals(actual) {
			t.Errorf("Iteration %d: Conjunction result mismatch. Expected cardinality %d, got %d",
				iter, expected.Cardinality(), actual.Cardinality())
		}
	}
}

// TestConjunctionDISI_ConjunctionApproximation tests two-phase iteration approximation
// Source: TestConjunctionDISI.testConjunctionApproximation()
// Purpose: Tests that the conjunction correctly uses TwoPhaseIterator for approximation
func TestConjunctionDISI_ConjunctionApproximation(t *testing.T) {
	numIters := atLeast(100)

	for iter := 0; iter < numIters; iter++ {
		maxDoc := nextInt(100, 10000)
		numIterators := nextInt(2, 5)

		sets := make([]*util.FixedBitSet, numIterators)
		scorers := make([]search.Scorer, numIterators)
		hasApproximation := false

		for i := 0; i < numIterators; i++ {
			set := randomSet(maxDoc)

			if rand.Intn(2) == 0 {
				// Simple iterator
				sets[i] = set
				scorers[i] = createConstantScoreScorer(newBitSetIterator(set))
			} else {
				// Scorer with approximation
				confirmed := clearRandomBits(set)
				sets[i] = confirmed
				approximation := createApproximation(newBitSetIterator(set), confirmed)
				scorers[i] = createScorerWithTwoPhase(approximation)
				hasApproximation = true
			}
		}

		// Create conjunction from scorers
		conjunction, err := search.IntersectScorers(scorers)
		if err != nil {
			t.Fatalf("Failed to create conjunction: %v", err)
		}

		// Check if TwoPhaseIterator is available
		twoPhase := search.UnwrapTwoPhaseIterator(conjunction)
		if hasApproximation && twoPhase == nil {
			t.Error("Expected TwoPhaseIterator when approximation is present")
		}
		if !hasApproximation && twoPhase != nil {
			t.Error("Did not expect TwoPhaseIterator when no approximation is present")
		}

		// Verify results
		expected := intersect(sets)
		var actual *util.FixedBitSet
		if twoPhase != nil {
			actual = toBitSet(maxDoc, search.AsDocIdSetIterator(twoPhase))
		} else {
			actual = toBitSet(maxDoc, conjunction)
		}

		if !expected.Equals(actual) {
			t.Errorf("Iteration %d: Conjunction approximation result mismatch", iter)
		}
	}
}

// TestConjunctionDISI_RecursiveConjunctionApproximation tests nested conjunctions
// Source: TestConjunctionDISI.testRecursiveConjunctionApproximation()
// Purpose: Tests that when nesting scorers with ConjunctionDISI, confirmations are pushed to the root
func TestConjunctionDISI_RecursiveConjunctionApproximation(t *testing.T) {
	numIters := atLeast(100)

	for iter := 0; iter < numIters; iter++ {
		maxDoc := nextInt(100, 10000)
		numIterators := nextInt(2, 5)

		sets := make([]*util.FixedBitSet, numIterators)
		var conjunction search.Scorer
		hasApproximation := false

		for i := 0; i < numIterators; i++ {
			set := randomSet(maxDoc)
			var newScorer search.Scorer

			choice := rand.Intn(3)
			switch choice {
			case 0:
				// Simple iterator
				sets[i] = set
				newScorer = createConstantScoreScorer(anonymizeIterator(newBitSetIterator(set)))
			case 1:
				// BitSet iterator
				sets[i] = set
				newScorer = createConstantScoreScorer(newBitSetIterator(set))
			default:
				// Scorer with approximation
				confirmed := clearRandomBits(set)
				sets[i] = confirmed
				approximation := createApproximation(newBitSetIterator(set), confirmed)
				newScorer = createScorerWithTwoPhase(approximation)
				hasApproximation = true
			}

			if conjunction == nil {
				conjunction = newScorer
			} else {
				conjIter, err := search.IntersectScorers([]search.Scorer{conjunction, newScorer})
				if err != nil {
					t.Fatalf("Failed to create conjunction: %v", err)
				}
				conjScorer := createScorerFromIterator(conjIter)
				conjunction = conjScorer
			}
		}

		// Check TwoPhaseIterator at root
		twoPhase := conjunction.GetTwoPhaseIterator()
		if hasApproximation && twoPhase == nil {
			t.Error("Expected TwoPhaseIterator at root when nested approximations exist")
		}
		if !hasApproximation && twoPhase != nil {
			t.Error("Did not expect TwoPhaseIterator at root when no approximations exist")
		}

		// Verify results
		expected := intersect(sets)
		var actual *util.FixedBitSet
		if twoPhase != nil {
			actual = toBitSet(maxDoc, search.AsDocIdSetIterator(twoPhase))
		} else {
			actual = toBitSet(maxDoc, conjunction.Iterator())
		}

		if !expected.Equals(actual) {
			t.Errorf("Iteration %d: Recursive conjunction result mismatch", iter)
		}
	}
}

// TestConjunctionDISI_CollapseSubConjunctionDISIs tests collapsing sub-conjunctions (iterators)
// Source: TestConjunctionDISI.testCollapseSubConjunctionDISIs()
// Purpose: Tests that nested ConjunctionDISI iterators are properly collapsed
func TestConjunctionDISI_CollapseSubConjunctionDISIs(t *testing.T) {
	testCollapseSubConjunctions(t, false)
}

// TestConjunctionDISI_CollapseSubConjunctionScorers tests collapsing sub-conjunctions (scorers)
// Source: TestConjunctionDISI.testCollapseSubConjunctionScorers()
// Purpose: Tests that nested ConjunctionScorer instances are properly collapsed
func TestConjunctionDISI_CollapseSubConjunctionScorers(t *testing.T) {
	testCollapseSubConjunctions(t, true)
}

// testCollapseSubConjunctions is the helper for collapse tests
func testCollapseSubConjunctions(t *testing.T, wrapWithScorer bool) {
	numIters := atLeast(100)

	for iter := 0; iter < numIters; iter++ {
		maxDoc := nextInt(100, 10000)
		numIterators := nextInt(5, 10)

		sets := make([]*util.FixedBitSet, numIterators)
		scorers := make([]search.Scorer, 0, numIterators)

		for i := 0; i < numIterators; i++ {
			set := randomSet(maxDoc)
			if rand.Intn(2) == 0 {
				// Simple iterator
				sets[i] = set
				scorers = append(scorers, createConstantScoreScorer(newBitSetIterator(set)))
			} else {
				// Scorer with approximation
				confirmed := clearRandomBits(set)
				sets[i] = confirmed
				approximation := createApproximation(newBitSetIterator(set), confirmed)
				scorers = append(scorers, createScorerWithTwoPhase(approximation))
			}
		}

		// Create some sub-conjunctions
		subIters := atLeast(3)
		for subIter := 0; subIter < subIters && len(scorers) > 3; subIter++ {
			subSeqStart := nextInt(0, len(scorers)-2)
			subSeqEnd := nextInt(subSeqStart+2, len(scorers))
			subScorers := scorers[subSeqStart:subSeqEnd]

			var subConjunction search.Scorer
			if wrapWithScorer {
				subConjunction = createConjunctionScorer(subScorers)
			} else {
				subIter, err := search.IntersectScorers(subScorers)
				if err != nil {
					t.Fatalf("Failed to create sub-conjunction: %v", err)
				}
				subConjunction = createConstantScoreScorer(subIter)
			}

			// Replace sub-sequence with sub-conjunction
			scorers[subSeqStart] = subConjunction
			toRemove := subSeqEnd - subSeqStart - 1
			for toRemove > 0 {
				if subSeqStart+1 < len(scorers) {
					scorers = append(scorers[:subSeqStart+1], scorers[subSeqStart+2:]...)
				}
				toRemove--
			}
		}

		// Ensure at least 2 scorers for conjunction
		if len(scorers) == 1 {
			scorers = append(scorers, createConstantScoreScorer(search.DocIdSetIteratorAll(maxDoc)))
		}

		// Create final conjunction
		conjunction, err := search.IntersectScorers(scorers)
		if err != nil {
			t.Fatalf("Failed to create conjunction: %v", err)
		}

		// Verify results
		expected := intersect(sets)
		actual := toBitSet(maxDoc, conjunction)

		if !expected.Equals(actual) {
			t.Errorf("Iteration %d: Collapse sub-conjunction result mismatch", iter)
		}
	}
}

// TestConjunctionDISI_IllegalAdvancementOfSubIterators tests assertion on illegal advancement
// Source: TestConjunctionDISI.testIllegalAdvancementOfSubIteratorsTripsAssertion()
// Purpose: Tests that illegally advancing sub-iterators outside the conjunction triggers assertion
func TestConjunctionDISI_IllegalAdvancementOfSubIterators(t *testing.T) {
	if !search.TestAssertsEnabled() {
		t.Skip("Assertions must be enabled for this test")
	}

	maxDoc := 100
	numIterators := nextInt(2, 5)
	set := randomSet(maxDoc)

	iterators := make([]search.DocIdSetIterator, numIterators)
	for i := 0; i < numIterators; i++ {
		iterators[i] = newBitSetIterator(set)
	}

	conjunction, err := search.IntersectIterators(iterators)
	if err != nil {
		t.Fatalf("Failed to create conjunction: %v", err)
	}

	// Illegally advance one of the sub-iterators
	idx := nextInt(0, numIterators-1)
	iterators[idx].NextDoc()

	// This should trigger an assertion error
	defer func() {
		if r := recover(); r != nil {
			// Expected assertion error
			if r != "Sub-iterators of ConjunctionDISI are not on the same document!" {
				t.Errorf("Unexpected panic message: %v", r)
			}
		} else {
			t.Error("Expected assertion error when illegally advancing sub-iterator")
		}
	}()

	conjunction.NextDoc()
}

// TestConjunctionDISI_BitSetConjunctionDISIDocIDOnExhaust tests docID on exhaustion
// Source: TestConjunctionDISI.testBitSetConjunctionDISIDocIDOnExhaust()
// Purpose: Tests that docID returns NO_MORE_DOCS when iterator is exhausted
func TestConjunctionDISI_BitSetConjunctionDISIDocIDOnExhaust(t *testing.T) {
	numBitSetIterators := nextInt(2, 5)
	iterators := make([]search.DocIdSetIterator, numBitSetIterators+1)

	// Create sparse iterator with single match greater than bitset lengths
	maxBitSetLength := 1000
	minBitSetLength := 2
	leadMaxDoc := maxBitSetLength + 1
	iterators[numBitSetIterators] = search.DocIdSetIteratorRange(leadMaxDoc, leadMaxDoc+1)

	for i := 0; i < numBitSetIterators; i++ {
		bitSetLength := nextInt(minBitSetLength, maxBitSetLength)
		bitSet, _ := util.NewFixedBitSet(bitSetLength)
		for j := 0; j < bitSetLength-1; j++ {
			bitSet.Set(j)
		}
		iterators[i] = newBitSetIterator(bitSet)
	}

	conjunction, err := search.IntersectIterators(iterators)
	if err != nil {
		t.Fatalf("Failed to create conjunction: %v", err)
	}

	doc, err := conjunction.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", doc)
	}

	if conjunction.DocID() != search.NO_MORE_DOCS {
		t.Errorf("Expected DocID() to be NO_MORE_DOCS, got %d", conjunction.DocID())
	}
}

// TestConjunctionDISI_Cost tests cost computation
// Source: TestConjunctionDISI (cost-related assertions)
// Purpose: Tests that the conjunction correctly computes the minimum cost
func TestConjunctionDISI_Cost(t *testing.T) {
	numIters := atLeast(50)

	for iter := 0; iter < numIters; iter++ {
		maxDoc := nextInt(100, 1000)
		numIterators := nextInt(2, 5)

		iterators := make([]search.DocIdSetIterator, numIterators)
		minCost := int64(maxDoc)

		for i := 0; i < numIterators; i++ {
			set := randomSet(maxDoc)
			iterators[i] = newBitSetIterator(set)
			cost := int64(set.Cardinality())
			if cost < minCost {
				minCost = cost
			}
		}

		conjunction, err := search.IntersectIterators(iterators)
		if err != nil {
			t.Fatalf("Failed to create conjunction: %v", err)
		}

		// Cost should be the minimum of all iterator costs
		if conjunction.Cost() != minCost {
			t.Errorf("Iteration %d: Expected cost %d, got %d", iter, minCost, conjunction.Cost())
		}
	}
}

// TestConjunctionDISI_EmptyIterator tests conjunction with empty iterators
// Source: Derived from Lucene test patterns
// Purpose: Tests that conjunction with empty iterator returns no documents
func TestConjunctionDISI_EmptyIterator(t *testing.T) {
	maxDoc := 100

	// Create one empty and one non-empty iterator
	emptySet, _ := util.NewFixedBitSet(maxDoc)
	nonEmptySet := randomSet(maxDoc)

	iterators := []search.DocIdSetIterator{
		newBitSetIterator(emptySet),
		newBitSetIterator(nonEmptySet),
	}

	conjunction, err := search.IntersectIterators(iterators)
	if err != nil {
		t.Fatalf("Failed to create conjunction: %v", err)
	}

	doc, err := conjunction.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc error: %v", err)
	}
	if doc != search.NO_MORE_DOCS {
		t.Error("Expected NO_MORE_DOCS for conjunction with empty iterator")
	}
}

// TestConjunctionDISI_SingleIterator tests conjunction with single iterator
// Source: Derived from Lucene test patterns
// Purpose: Tests that single iterator conjunction returns same documents
func TestConjunctionDISI_SingleIterator(t *testing.T) {
	maxDoc := 100
	set := randomSet(maxDoc)

	iterators := []search.DocIdSetIterator{
		newBitSetIterator(set),
	}

	conjunction, err := search.IntersectIterators(iterators)
	if err != nil {
		t.Fatalf("Failed to create conjunction: %v", err)
	}

	actual := toBitSet(maxDoc, conjunction)
	if !set.Equals(actual) {
		t.Error("Single iterator conjunction should return same documents")
	}
}

// TestConjunctionDISI_Advance tests advance operation
// Source: Derived from Lucene test patterns
// Purpose: Tests that advance correctly skips to target document
func TestConjunctionDISI_Advance(t *testing.T) {
	maxDoc := 1000
	numIters := atLeast(20)

	for iter := 0; iter < numIters; iter++ {
		numIterators := nextInt(2, 4)
		iterators := make([]search.DocIdSetIterator, numIterators)
		sets := make([]*util.FixedBitSet, numIterators)

		for i := 0; i < numIterators; i++ {
			sets[i] = randomSet(maxDoc)
			iterators[i] = newBitSetIterator(sets[i])
		}

		conjunction, err := search.IntersectIterators(iterators)
		if err != nil {
			t.Fatalf("Failed to create conjunction: %v", err)
		}

		// Test advance to random targets
		for target := 0; target < maxDoc; target += nextInt(1, 50) {
			doc, err := conjunction.Advance(target)
			if err != nil {
				t.Fatalf("Advance error: %v", err)
			}

			// Calculate expected result
			expected := intersect(sets)
			expectedDoc := expected.NextSetBit(target)
			if expectedDoc < 0 {
				expectedDoc = search.NO_MORE_DOCS
			}

			if doc != expectedDoc {
				t.Errorf("Advance(%d): expected %d, got %d", target, expectedDoc, doc)
			}
		}
	}
}

// Helper functions

// anonymizeIterator wraps an iterator in an anonymous type to prevent optimizations
func anonymizeIterator(it search.DocIdSetIterator) search.DocIdSetIterator {
	return &anonymousIterator{delegate: it}
}

type anonymousIterator struct {
	delegate search.DocIdSetIterator
}

func (a *anonymousIterator) DocID() int {
	return a.delegate.DocID()
}

func (a *anonymousIterator) NextDoc() (int, error) {
	return a.delegate.NextDoc()
}

func (a *anonymousIterator) Advance(target int) (int, error) {
	return a.delegate.Advance(target)
}

func (a *anonymousIterator) Cost() int64 {
	// Return docID as cost to prevent optimization
	return int64(a.delegate.DocID())
}

// createApproximation creates a TwoPhaseIterator with approximation and confirmation
func createApproximation(approximation search.DocIdSetIterator, confirmed *util.FixedBitSet) search.TwoPhaseIterator {
	return &testTwoPhaseIterator{
		approximation: approximation,
		confirmed:     confirmed,
		matchCostVal:  5, // #operations in FixedBitSet#get()
	}
}

type testTwoPhaseIterator struct {
	approximation search.DocIdSetIterator
	confirmed     *util.FixedBitSet
	matchCostVal  float32
}

func (t *testTwoPhaseIterator) Approximation() search.DocIdSetIterator {
	return t.approximation
}

func (t *testTwoPhaseIterator) Matches() bool {
	return t.confirmed.Get(t.approximation.DocID())
}

func (t *testTwoPhaseIterator) MatchCost() float32 {
	return t.matchCostVal
}

// createScorerWithTwoPhase creates a Scorer that uses TwoPhaseIterator
func createScorerWithTwoPhase(twoPhase search.TwoPhaseIterator) search.Scorer {
	return &testScorer{
		iterator:     search.AsDocIdSetIterator(twoPhase),
		twoPhaseIter: twoPhase,
	}
}

// createScorerFromIterator creates a Scorer from a DocIdSetIterator
func createScorerFromIterator(it search.DocIdSetIterator) search.Scorer {
	return &testScorer{iterator: it}
}

// createConstantScoreScorer creates a Scorer with constant score
func createConstantScoreScorer(it search.DocIdSetIterator) search.Scorer {
	return &testScorer{iterator: it}
}

// createConjunctionScorer creates a ConjunctionScorer
func createConjunctionScorer(scorers []search.Scorer) search.Scorer {
	// This would be implemented by the actual ConjunctionScorer
	it, _ := search.IntersectScorers(scorers)
	return &testScorer{iterator: it}
}

type testScorer struct {
	iterator     search.DocIdSetIterator
	twoPhaseIter search.TwoPhaseIterator
}

func (s *testScorer) DocID() int {
	if s.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return s.iterator.DocID()
}

func (s *testScorer) NextDoc() (int, error) {
	if s.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return s.iterator.NextDoc()
}

func (s *testScorer) Advance(target int) (int, error) {
	if s.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return s.iterator.Advance(target)
}

func (s *testScorer) Cost() int64 {
	if s.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return s.iterator.Cost()
}

func (s *testScorer) Score() float32 {
	return 0
}

func (s *testScorer) Iterator() search.DocIdSetIterator {
	return &testScorerIterator{scorer: s}
}

func (s *testScorer) GetTwoPhaseIterator() search.TwoPhaseIterator {
	return s.twoPhaseIter
}

type testScorerIterator struct {
	scorer *testScorer
}

func (t *testScorerIterator) DocID() int {
	return t.scorer.iterator.DocID()
}

func (t *testScorerIterator) NextDoc() (int, error) {
	if t.scorer.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return t.scorer.iterator.NextDoc()
}

func (t *testScorerIterator) Advance(target int) (int, error) {
	if t.scorer.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return t.scorer.iterator.Advance(target)
}

func (t *testScorerIterator) Cost() int64 {
	if t.scorer.twoPhaseIter != nil {
		panic("ConjunctionDISI should call the two-phase iterator")
	}
	return t.scorer.iterator.Cost()
}

// newBitSetIterator creates a DocIdSetIterator from a FixedBitSet
func newBitSetIterator(set *util.FixedBitSet) search.DocIdSetIterator {
	return &bitSetIterator{
		set:     set,
		current: -1,
	}
}

type bitSetIterator struct {
	set     *util.FixedBitSet
	current int
}

func (b *bitSetIterator) DocID() int {
	return b.current
}

func (b *bitSetIterator) NextDoc() (int, error) {
	next := b.set.NextSetBit(b.current + 1)
	if next < 0 {
		b.current = search.NO_MORE_DOCS
	} else {
		b.current = next
	}
	return b.current, nil
}

func (b *bitSetIterator) Advance(target int) (int, error) {
	next := b.set.NextSetBit(target)
	if next < 0 {
		b.current = search.NO_MORE_DOCS
	} else {
		b.current = next
	}
	return b.current, nil
}

func (b *bitSetIterator) Cost() int64 {
	return int64(b.set.Cardinality())
}

// randomSet creates a random FixedBitSet
func randomSet(maxDoc int) *util.FixedBitSet {
	set, _ := util.NewFixedBitSet(maxDoc)
	step := nextInt(1, 10)
	for doc := rand.Intn(step); doc < maxDoc; doc += nextInt(1, step) {
		set.Set(doc)
	}
	return set
}

// clearRandomBits clears random bits from a set
func clearRandomBits(other *util.FixedBitSet) *util.FixedBitSet {
	set := other.Clone()
	for i := 0; i < set.Length(); i++ {
		if rand.Intn(2) == 0 {
			set.Clear(i)
		}
	}
	return set
}

// intersect returns the intersection of multiple bitsets
func intersect(sets []*util.FixedBitSet) *util.FixedBitSet {
	if len(sets) == 0 {
		return nil
	}
	result := sets[0].Clone()
	for i := 1; i < len(sets); i++ {
		result.And(sets[i])
	}
	return result
}

// toBitSet converts a DocIdSetIterator to a FixedBitSet
func toBitSet(maxDoc int, iterator search.DocIdSetIterator) *util.FixedBitSet {
	set, _ := util.NewFixedBitSet(maxDoc)
	for {
		doc, err := iterator.NextDoc()
		if err != nil {
			break
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		set.Set(doc)
	}
	return set
}

// nextInt returns a random integer between min and max (inclusive)
func nextInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min)
}

// atLeast returns at least n iterations (scaled for testing)
func atLeast(n int) int {
	// In Lucene tests, this scales with the test multiplier
	// For now, just return n
	return n
}
