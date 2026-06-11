// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestDenseConjunctionBulkScorer.java
//
// Java tests use FixedBitSet/BitSetIterator, AssertingBulkScorer, and
// RandomTwoPhaseView.  Gocene ports use util.FixedBitSet/util.BitSetIterator;
// AssertingBulkScorer and RandomTwoPhaseView have no Gocene equivalents and
// the tests that rely on them are skipped.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// bitSetDISI builds a search.DocIdSetIterator from a *util.FixedBitSet.
func bitSetDISI(bs *util.FixedBitSet) search.DocIdSetIterator {
	return util.NewBitSetIterator(bs, int64(bs.Cardinality()))
}

// newDenseConjScorer is a convenience wrapper for tests.
func newDenseConjScorer(t *testing.T, iters []search.DocIdSetIterator, maxDoc int) *search.DenseConjunctionBulkScorer {
	t.Helper()
	bs, err := search.NewDenseConjunctionBulkScorer(iters, nil, maxDoc, 0)
	if err != nil {
		t.Fatalf("NewDenseConjunctionBulkScorer: %v", err)
	}
	return bs
}

// collectDense runs the scorer and returns the set of collected doc IDs.
func collectDense(t *testing.T, scorer *search.DenseConjunctionBulkScorer, maxDoc int) []int {
	t.Helper()
	lc := &batchLeafCollector{}
	if _, err := scorer.Score(lc, nil, 0, maxDoc); err != nil {
		t.Fatalf("Score: %v", err)
	}
	return lc.docs
}

// bitsFromFunc builds a util.FixedBitSet where bit i is set iff predicate(i).
func bitsFromFunc(numBits int, predicate func(int) bool) *util.FixedBitSet {
	bs, _ := util.NewFixedBitSet(numBits)
	for i := 0; i < numBits; i++ {
		if predicate(i) {
			bs.Set(i)
		}
	}
	return bs
}

// bitsRange sets bits [from, to).
func bitsRange(numBits, from, to int) *util.FixedBitSet {
	bs, _ := util.NewFixedBitSet(numBits)
	bs.SetRange(from, to)
	return bs
}

// TestDenseConjunctionBulkScorer_WindowSize verifies the exported constant.
func TestDenseConjunctionBulkScorer_WindowSize(t *testing.T) {
	if search.WindowSize != 4096 {
		t.Errorf("WindowSize=%d, want 4096", search.WindowSize)
	}
}

// TestDenseConjunctionBulkScorer_DensityThresholdInverse verifies the constant.
func TestDenseConjunctionBulkScorer_DensityThresholdInverse(t *testing.T) {
	if search.DensityThresholdInverse != 32 {
		t.Errorf("DensityThresholdInverse=%d, want 32", search.DensityThresholdInverse)
	}
}

// TestDenseConjunctionBulkScorer_EmptyIterators verifies the constructor
// returns an error when no iterators are supplied.
func TestDenseConjunctionBulkScorer_EmptyIterators(t *testing.T) {
	_, err := search.NewDenseConjunctionBulkScorer(nil, nil, 100, 0)
	if err == nil {
		t.Fatal("expected error for empty iterators, got nil")
	}
}

// TestDenseConjunctionBulkScorer_ImplementsBulkScorer checks interface.
func TestDenseConjunctionBulkScorer_ImplementsBulkScorer(t *testing.T) {
	maxDoc := 10
	bs, _ := util.NewFixedBitSet(maxDoc)
	var _ search.BulkScorer = newDenseConjScorer(t, []search.DocIdSetIterator{bitSetDISI(bs)}, maxDoc)
}

// TestDenseConjunctionBulkScorer_SameMatches mirrors testSameMatches.
// Three identical clauses (every even doc); intersection == clause.
func TestDenseConjunctionBulkScorer_SameMatches(t *testing.T) {
	maxDoc := 10_000
	clause1 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })
	clause2 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })
	clause3 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{
		bitSetDISI(clause1), bitSetDISI(clause2), bitSetDISI(clause3),
	}, maxDoc)
	docs := collectDense(t, scorer, maxDoc)

	wantCount := clause1.Cardinality()
	if len(docs) != wantCount {
		t.Errorf("got %d docs, want %d", len(docs), wantCount)
	}
	for _, d := range docs {
		if d%2 != 0 {
			t.Errorf("unexpected odd doc %d", d)
		}
	}
}

// TestDenseConjunctionBulkScorer_EmptyIntersection mirrors testEmptyIntersection.
// Two clauses with no overlap → 0 results.
func TestDenseConjunctionBulkScorer_EmptyIntersection(t *testing.T) {
	maxDoc := 10_000
	clause1 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })
	clause2 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 != 0 })

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{
		bitSetDISI(clause1), bitSetDISI(clause2),
	}, maxDoc)
	docs := collectDense(t, scorer, maxDoc)

	if len(docs) != 0 {
		t.Errorf("got %d docs, want 0 (empty intersection)", len(docs))
	}
}

// TestDenseConjunctionBulkScorer_Clustered mirrors testClustered.
// Three range clauses with a known overlap window.
func TestDenseConjunctionBulkScorer_Clustered(t *testing.T) {
	maxDoc := 10_000
	// clause1: [1000, 9000), clause2: [0, 8000), clause3: [2000, 10000)
	// intersection: [2000, 8000)
	clause1 := bitsRange(maxDoc, 1000, 9000)
	clause2 := bitsRange(maxDoc, 0, 8000)
	clause3 := bitsRange(maxDoc, 2000, 10000)

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{
		bitSetDISI(clause1), bitSetDISI(clause2), bitSetDISI(clause3),
	}, maxDoc)
	docs := collectDense(t, scorer, maxDoc)

	// Expect exactly [2000, 8000)
	wantCount := 8000 - 2000
	if len(docs) != wantCount {
		t.Errorf("got %d docs, want %d", len(docs), wantCount)
	}
	if len(docs) > 0 && (docs[0] != 2000 || docs[len(docs)-1] != 7999) {
		t.Errorf("first=%d last=%d, want first=2000 last=7999", docs[0], docs[len(docs)-1])
	}
}

// TestDenseConjunctionBulkScorer_SparseAfter2ndClause mirrors testSparseAfter2ndClause.
// Three prime-step clauses: intersection is sparse (multiples of 13*17*19).
func TestDenseConjunctionBulkScorer_SparseAfter2ndClause(t *testing.T) {
	maxDoc := 100_000
	clause1 := bitsFromFunc(maxDoc, func(i int) bool { return i%13 == 0 })
	clause2 := bitsFromFunc(maxDoc, func(i int) bool { return i%17 == 0 })
	clause3 := bitsFromFunc(maxDoc, func(i int) bool { return i%19 == 0 })

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{
		bitSetDISI(clause1), bitSetDISI(clause2), bitSetDISI(clause3),
	}, maxDoc)
	docs := collectDense(t, scorer, maxDoc)

	// Expected: multiples of 13*17*19 = 4199
	lcm := 13 * 17 * 19
	var want []int
	for i := 0; i < maxDoc; i += lcm {
		want = append(want, i)
	}
	if len(docs) != len(want) {
		t.Fatalf("got %d docs, want %d", len(docs), len(want))
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, docs[i], want[i])
		}
	}
}

// TestDenseConjunctionBulkScorer_MatchAllNoLiveDocs mirrors testMatchAllNoLiveDocs.
// Single all-docs clause → every doc collected.
func TestDenseConjunctionBulkScorer_MatchAllNoLiveDocs(t *testing.T) {
	maxDoc := 10_000
	allBits, _ := util.NewFixedBitSet(maxDoc)
	allBits.SetRange(0, maxDoc)

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{bitSetDISI(allBits)}, maxDoc)
	docs := collectDense(t, scorer, maxDoc)

	if len(docs) != maxDoc {
		t.Errorf("got %d docs, want %d", len(docs), maxDoc)
	}
}

// TestDenseConjunctionBulkScorer_ApplyAcceptDocs mirrors testApplyAcceptDocs.
// Two all-doc clauses + acceptDocs filter → only even docs.
func TestDenseConjunctionBulkScorer_ApplyAcceptDocs(t *testing.T) {
	maxDoc := 10_000
	allBits, _ := util.NewFixedBitSet(maxDoc)
	allBits.SetRange(0, maxDoc)

	evenBits := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })

	scorer, err := search.NewDenseConjunctionBulkScorer(
		[]search.DocIdSetIterator{bitSetDISI(allBits), bitSetDISI(allBits)},
		nil, maxDoc, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	lc := &batchLeafCollector{}
	if _, err := scorer.Score(lc, evenBits, 0, maxDoc); err != nil {
		t.Fatalf("Score: %v", err)
	}

	wantCount := evenBits.Cardinality()
	if len(lc.docs) != wantCount {
		t.Errorf("got %d docs, want %d", len(lc.docs), wantCount)
	}
	for _, d := range lc.docs {
		if d%2 != 0 {
			t.Errorf("unexpected odd doc %d after acceptDocs filter", d)
		}
	}
}

// TestDenseConjunctionBulkScorer_Cost verifies Cost() == cost of the cheapest clause.
func TestDenseConjunctionBulkScorer_Cost(t *testing.T) {
	maxDoc := 100
	// clause1 cost=50 (every even), clause2 cost=34 (every third)
	c1 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })
	c2 := bitsFromFunc(maxDoc, func(i int) bool { return i%3 == 0 })

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{bitSetDISI(c1), bitSetDISI(c2)}, maxDoc)
	// Cost must equal min of the two cardinalities.
	cost := scorer.Cost()
	minCard := int64(c1.Cardinality())
	if int64(c2.Cardinality()) < minCard {
		minCard = int64(c2.Cardinality())
	}
	if cost != minCard {
		t.Errorf("Cost()=%d, want %d (min cardinality)", cost, minCard)
	}
}

// TestDenseConjunctionBulkScorer_TwoPhaseIterators mirrors testTwoPhaseIterators
// but uses simple deterministic two-phase views instead of RandomTwoPhaseView.
// prime-step clauses 3, 5, 7 → intersection on multiples of 105.
func TestDenseConjunctionBulkScorer_TwoPhaseIterators(t *testing.T) {
	maxDoc := 10_000
	// Build two-phase iterators where the approximation is the full set and
	// matches() filters to the actual step.
	makeTwoPhase := func(step int) *search.TwoPhaseIterator {
		all, _ := util.NewFixedBitSet(maxDoc)
		all.SetRange(0, maxDoc)
		approx := util.NewBitSetIterator(all, int64(maxDoc))
		return search.NewTwoPhaseIterator(approx, func() (bool, error) {
			return approx.DocID()%step == 0, nil
		})
	}

	tp1 := makeTwoPhase(3)
	tp2 := makeTwoPhase(5)
	tp3 := makeTwoPhase(7)

	scorer, err := search.NewDenseConjunctionBulkScorer(nil, []*search.TwoPhaseIterator{tp1, tp2, tp3}, maxDoc, 0)
	if err != nil {
		t.Fatal(err)
	}
	lc := &batchLeafCollector{}
	if _, err := scorer.Score(lc, nil, 0, maxDoc); err != nil {
		t.Fatalf("Score: %v", err)
	}

	lcm := 3 * 5 * 7
	var want []int
	for i := 0; i < maxDoc; i += lcm {
		want = append(want, i)
	}
	if len(lc.docs) != len(want) {
		t.Fatalf("got %d docs, want %d", len(lc.docs), len(want))
	}
	for i := range want {
		if lc.docs[i] != want[i] {
			t.Errorf("docs[%d]=%d, want %d", i, lc.docs[i], want[i])
		}
	}
}

// TestDenseConjunctionBulkScorer_StopOnMinCompetitiveScore mirrors
// testStopOnMinCompetitiveScore: after setting minCompetitiveScore > constantScore
// the scorer should not collect any more documents.
func TestDenseConjunctionBulkScorer_StopOnMinCompetitiveScore(t *testing.T) {
	maxDoc := 10_000
	// Every-2nd doc clauses.
	c1 := bitsFromFunc(maxDoc, func(i int) bool { return i%2 == 0 })
	c2 := bitsFromFunc(maxDoc, func(i int) bool { return i%5 == 0 })

	scorer := newDenseConjScorer(t, []search.DocIdSetIterator{bitSetDISI(c1), bitSetDISI(c2)}, maxDoc)

	stopDoc := 200
	var stopScorer search.Scorer
	lc := &earlyTermLeafCollector{
		stopDoc: stopDoc,
		onSetScorer: func(s search.Scorer) {
			stopScorer = s
		},
		onCollect: func(doc int) error {
			if doc >= stopDoc && stopScorer != nil {
				// Raise minCompetitiveScore above constant 0.
				if sa, ok := stopScorer.(interface {
					SetMinCompetitiveScore(float32) error
				}); ok {
					_ = sa.SetMinCompetitiveScore(1.0)
				}
			}
			return nil
		},
	}
	if _, err := scorer.Score(lc, nil, 0, maxDoc); err != nil {
		t.Fatalf("Score: %v", err)
	}
	// All docs must be ≤ stopDoc + WindowSize (can overshoot by at most one window).
	if len(lc.docs) > 0 {
		last := lc.docs[len(lc.docs)-1]
		if last > stopDoc+search.WindowSize {
			t.Errorf("last doc %d too far past stop %d (+WindowSize=%d)",
				last, stopDoc, search.WindowSize)
		}
	}

// earlyTermLeafCollector is a LeafCollector that can stop collection early.
type earlyTermLeafCollector struct {
	docs        []int
	scores      []float32
	stopDoc     int
	onSetScorer func(search.Scorer)
	onCollect   func(int) error
}

}
func (c *earlyTermLeafCollector) SetScorer(s search.Scorer) error {
	if c.onSetScorer != nil {
		c.onSetScorer(s)
	}
	return nil
}

func (c *earlyTermLeafCollector) Collect(doc int) error {
	c.docs = append(c.docs, doc)
	if c.onCollect != nil {
		return c.onCollect(doc)
	}
	return nil
}

func (c *earlyTermLeafCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *earlyTermLeafCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return c, nil
}