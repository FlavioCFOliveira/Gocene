// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math/rand"
	"testing"
)

// ConstantScoreScorer is a Scorer that returns a constant score for all documents.
// This is the Go port of Lucene's ConstantScoreScorer.
type ConstantScoreScorer struct {
	score         float32
	scoreMode     ScoreMode
	approximation DocIdSetIterator
	iterator      DocIdSetIterator
}

// NewConstantScoreScorer creates a new ConstantScoreScorer.
func NewConstantScoreScorer(score float32, scoreMode ScoreMode, disi DocIdSetIterator) *ConstantScoreScorer {
	return &ConstantScoreScorer{
		score:         score,
		scoreMode:     scoreMode,
		approximation: disi,
		iterator:      disi,
	}
}

// DocID returns the current document ID.
func (s *ConstantScoreScorer) DocID() int {
	return s.iterator.DocID()
}

// NextDoc advances to the next document.
func (s *ConstantScoreScorer) NextDoc() (int, error) {
	return s.iterator.NextDoc()
}

// Advance advances to the target document.
func (s *ConstantScoreScorer) Advance(target int) (int, error) {
	return s.iterator.Advance(target)
}

// Cost returns the estimated cost.
func (s *ConstantScoreScorer) Cost() int64 {
	return s.iterator.Cost()
}

// Score returns the constant score.
func (s *ConstantScoreScorer) Score() float32 {
	return s.score
}

// DocIDRunEnd returns the end of the current run.
func (s *ConstantScoreScorer) DocIDRunEnd() int {
	return s.iterator.DocIDRunEnd()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *ConstantScoreScorer) GetMaxScore(upTo int) float32 {
	return s.score
}

// Ensure ConstantScoreScorer implements Scorer
var _ Scorer = (*ConstantScoreScorer)(nil)

// DisiWrapper wraps a Scorer for use in disjunctions.
// This is the Go port of Lucene's DisiWrapper.
type DisiWrapper struct {
	scorer        Scorer
	iterator      DocIdSetIterator
	approximation DocIdSetIterator
	cost          int64
	doc           int
	next          *DisiWrapper
}

// NewDisiWrapper creates a new DisiWrapper.
func NewDisiWrapper(scorer Scorer, impacts bool) *DisiWrapper {
	iterator := scorer
	w := &DisiWrapper{
		scorer:   scorer,
		iterator: iterator,
		cost:     iterator.Cost(),
		doc:      -1,
	}
	// For simplicity, approximation is the iterator itself
	// In full Lucene, this would handle two-phase iteration
	w.approximation = iterator
	return w
}

// DisiPriorityQueue is a priority queue for DisiWrapper instances.
type DisiPriorityQueue struct {
	heap []*DisiWrapper
	size int
}

// NewDisiPriorityQueue creates a new DisiPriorityQueue with the given max size.
func NewDisiPriorityQueue(maxSize int) *DisiPriorityQueue {
	return &DisiPriorityQueue{
		heap: make([]*DisiWrapper, maxSize+1),
	}
}

// Add adds a DisiWrapper to the queue.
func (pq *DisiPriorityQueue) Add(w *DisiWrapper) {
	pq.size++
	pq.heap[pq.size] = w
	pq.heapUp(pq.size)
}

// AddAll adds multiple wrappers starting from the given index.
func (pq *DisiPriorityQueue) AddAll(wrappers []*DisiWrapper, start, length int) {
	for i := 0; i < length && start+i < len(wrappers); i++ {
		pq.Add(wrappers[start+i])
	}
}

// Top returns the top element.
func (pq *DisiPriorityQueue) Top() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	return pq.heap[1]
}

// UpdateTop updates the top element after modification.
func (pq *DisiPriorityQueue) UpdateTop() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	pq.heapDown(1)
	return pq.heap[1]
}

// TopList returns a linked list of all wrappers at the current doc.
func (pq *DisiPriorityQueue) TopList() *DisiWrapper {
	if pq.size == 0 {
		return nil
	}
	topDoc := pq.heap[1].doc
	var list *DisiWrapper
	for i := 1; i <= pq.size; i++ {
		if pq.heap[i].doc == topDoc {
			pq.heap[i].next = list
			list = pq.heap[i]
		}
	}
	return list
}

func (pq *DisiPriorityQueue) heapUp(i int) {
	for i > 1 {
		parent := i / 2
		if pq.heap[parent].doc <= pq.heap[i].doc {
			break
		}
		pq.heap[parent], pq.heap[i] = pq.heap[i], pq.heap[parent]
		i = parent
	}
}

func (pq *DisiPriorityQueue) heapDown(i int) {
	for {
		left := i * 2
		right := left + 1
		smallest := i

		if left <= pq.size && pq.heap[left].doc < pq.heap[smallest].doc {
			smallest = left
		}
		if right <= pq.size && pq.heap[right].doc < pq.heap[smallest].doc {
			smallest = right
		}

		if smallest == i {
			break
		}
		pq.heap[i], pq.heap[smallest] = pq.heap[smallest], pq.heap[i]
		i = smallest
	}
}

// DisjunctionDISIApproximation is a DocIdSetIterator that is a disjunction of approximations.
// This is the Go port of Lucene's DisjunctionDISIApproximation.
type DisjunctionDISIApproximation struct {
	leadIterators  *DisiPriorityQueue
	otherIterators []*DisiWrapper
	cost           int64
	leadTop        *DisiWrapper
	minOtherDoc    int
	doc            int
}

// NewDisjunctionDISIApproximation creates a new DisjunctionDISIApproximation.
func NewDisjunctionDISIApproximation(subIterators []*DisiWrapper, leadCost int64) *DisjunctionDISIApproximation {
	// Sort by descending cost (simplified - just use as-is for now)
	wrappers := make([]*DisiWrapper, len(subIterators))
	copy(wrappers, subIterators)

	// Simple heuristic: put all in lead iterators for now
	// In full Lucene, this would split based on cost
	pq := NewDisiPriorityQueue(len(wrappers))
	for _, w := range wrappers {
		pq.Add(w)
	}

	var totalCost int64
	for _, w := range wrappers {
		totalCost += w.cost
	}

	return &DisjunctionDISIApproximation{
		leadIterators:  pq,
		otherIterators: make([]*DisiWrapper, 0),
		cost:           totalCost,
		leadTop:        pq.Top(),
		minOtherDoc:    NO_MORE_DOCS,
		doc:            -1,
	}
}

// DocID returns the current document ID.
func (it *DisjunctionDISIApproximation) DocID() int {
	return it.doc
}

// NextDoc advances to the next document.
func (it *DisjunctionDISIApproximation) NextDoc() (int, error) {
	if it.leadTop.doc < it.minOtherDoc {
		curDoc := it.leadTop.doc
		for {
			it.leadTop.doc, _ = it.leadTop.approximation.NextDoc()
			it.leadTop = it.leadIterators.UpdateTop()
			if it.leadTop.doc != curDoc {
				break
			}
		}
		if it.leadTop.doc < it.minOtherDoc {
			it.doc = it.leadTop.doc
		} else {
			it.doc = it.minOtherDoc
		}
		return it.doc, nil
	}
	return it.Advance(it.minOtherDoc + 1)
}

// Advance advances to the target document.
func (it *DisjunctionDISIApproximation) Advance(target int) (int, error) {
	for it.leadTop != nil && it.leadTop.doc < target {
		var err error
		it.leadTop.doc, err = it.leadTop.approximation.Advance(target)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		it.leadTop = it.leadIterators.UpdateTop()
	}

	it.minOtherDoc = NO_MORE_DOCS
	for _, w := range it.otherIterators {
		if w.doc < target {
			var err error
			w.doc, err = w.approximation.Advance(target)
			if err != nil {
				return NO_MORE_DOCS, err
			}
		}
		if w.doc < it.minOtherDoc {
			it.minOtherDoc = w.doc
		}
	}

	if it.leadTop != nil && it.leadTop.doc < it.minOtherDoc {
		it.doc = it.leadTop.doc
	} else {
		it.doc = it.minOtherDoc
	}
	return it.doc, nil
}

// Cost returns the estimated cost.
func (it *DisjunctionDISIApproximation) Cost() int64 {
	return it.cost
}

// topList returns the linked list of iterators positioned on the current doc.
func (it *DisjunctionDISIApproximation) topList() *DisiWrapper {
	if it.leadTop == nil {
		return nil
	}
	if it.leadTop.doc < it.minOtherDoc {
		return it.leadIterators.TopList()
	}
	return it.computeTopList()
}

func (it *DisjunctionDISIApproximation) computeTopList() *DisiWrapper {
	var topList *DisiWrapper
	if it.leadTop != nil && it.leadTop.doc == it.minOtherDoc {
		topList = it.leadIterators.TopList()
	}
	for _, w := range it.otherIterators {
		if w.doc == it.minOtherDoc {
			w.next = topList
			topList = w
		}
	}
	return topList
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (it *DisjunctionDISIApproximation) DocIDRunEnd() int {
	// Default implementation from AbstractDocIdSetIterator
	maxDocIDRunEnd := it.doc + 1
	for w := it.topList(); w != nil; w = w.next {
		runEnd := w.approximation.DocIDRunEnd()
		if runEnd > maxDocIDRunEnd {
			maxDocIDRunEnd = runEnd
		}
	}
	return maxDocIDRunEnd
}

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
}

// TestDisjunctionDISIApproximation_Empty tests behavior with empty ranges.
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
