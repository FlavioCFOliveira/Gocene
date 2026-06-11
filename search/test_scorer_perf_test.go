// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestScorerPerf.java
//
// TestScorerPerf validates the BooleanScorer conjunction logic by building
// random FixedBitSet "documents" and wrapping each in a constant-score
// BitSetQuery, then asserting that a conjunction (MUST) of those queries collects
// exactly the documents in the intersection of the corresponding bitsets. The
// index itself carries a single empty document — the bitsets ARE the document
// sets the scorers iterate, so the test exercises the scorer/collector plumbing
// directly with a known-correct reference (the bitset AND).
package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestScorerPerf_Perf mirrors TestScorerPerf.testConjunctions.
func TestScorerPerf_Perf(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addDoc(document.NewDocument()) // single empty document, as upstream
	searcher, cleanup := ix.searcher()
	defer cleanup()

	rng := rand.New(rand.NewSource(1234))
	const (
		numSets    = 1000
		setSize    = 30
		iterations = 200
	)
	sets := randBitSets(t, rng, numSets, setSize)

	doConjunctions(t, searcher, rng, sets, iterations, 5)
	doNestedConjunctions(t, searcher, rng, sets, iterations, 3, 3)
}

// randBitSet builds a FixedBitSet of sz bits with numBitsToSet random bits set.
func randBitSet(t *testing.T, rng *rand.Rand, sz, numBitsToSet int) *util.FixedBitSet {
	t.Helper()
	set, err := util.NewFixedBitSet(sz)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	for i := 0; i < numBitsToSet; i++ {
		set.Set(rng.Intn(sz))
	}
	return set
}

func randBitSets(t *testing.T, rng *rand.Rand, numSets, setSize int) []*util.FixedBitSet {
	t.Helper()
	sets := make([]*util.FixedBitSet, numSets)
	for i := range sets {
		sets[i] = randBitSet(t, rng, setSize, rng.Intn(setSize))
	}
	return sets
}

// addClause adds a random bitset query as a MUST clause and folds its bitset
// into the running expected-intersection result.
func addClause(t *testing.T, rng *rand.Rand, sets []*util.FixedBitSet, bq *search.BooleanQuery, result *util.FixedBitSet) *util.FixedBitSet {
	t.Helper()
	rnd := sets[rng.Intn(len(sets))]
	bq.Add(newBitSetQuery(rnd), search.MUST)
	if result == nil {
		result = rnd.Clone()
	} else {
		if err := result.And(rnd); err != nil {
			t.Fatalf("FixedBitSet.And: %v", err)
		}
	}
	return result
}

func doConjunctions(t *testing.T, s *search.IndexSearcher, rng *rand.Rand, sets []*util.FixedBitSet, iter, maxClauses int) {
	t.Helper()
	for i := 0; i < iter; i++ {
		nClauses := rng.Intn(maxClauses-1) + 2 // min 2 clauses
		bq := search.NewBooleanQuery()
		var result *util.FixedBitSet
		for j := 0; j < nClauses; j++ {
			result = addClause(t, rng, sets, bq, result)
		}
		count := countHits(t, s, bq)
		if got, want := count, result.Cardinality(); got != want {
			t.Fatalf("conjunction iter %d: collected %d, want intersection cardinality %d", i, got, want)
		}
	}
}

func doNestedConjunctions(t *testing.T, s *search.IndexSearcher, rng *rand.Rand, sets []*util.FixedBitSet, iter, maxOuterClauses, maxClauses int) {
	t.Helper()
	for i := 0; i < iter; i++ {
		oClauses := rng.Intn(maxOuterClauses-1) + 2
		oq := search.NewBooleanQuery()
		var result *util.FixedBitSet
		for o := 0; o < oClauses; o++ {
			nClauses := rng.Intn(maxClauses-1) + 2 // min 2 clauses
			bq := search.NewBooleanQuery()
			for j := 0; j < nClauses; j++ {
				result = addClause(t, rng, sets, bq, result)
			}
			oq.Add(bq, search.MUST)
		}
		count := countHits(t, s, oq)
		if got, want := count, result.Cardinality(); got != want {
			t.Fatalf("nested conjunction iter %d: collected %d, want intersection cardinality %d", i, got, want)
		}
	}

// countHits runs the query with a counting collector that, like the upstream
// CountingHitCollector, sums docBase+doc to defeat any dead-code elimination
// while reporting the number of collected documents.
func countHits(t *testing.T, s *search.IndexSearcher, q search.Query) int {
	t.Helper()
	c := &countingHitCollector{}
	if err := s.SearchWithCollector(q, c); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
	return c.count
}

// countingHitCollector mirrors TestScorerPerf.CountingHitCollector. ScoreMode is
// COMPLETE_NO_SCORES.
type countingHitCollector struct {
	count   int
	sum     int
	docBase int
}

func (c *countingHitCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *countingHitCollector) GetLeafCollector(ctx *index.LeafReaderContext) (search.LeafCollector, error) {
	if ctx != nil {
		c.docBase = ctx.DocBase()
	}
	return c, nil
}

func (c *countingHitCollector) SetScorer(_ search.Scorer) error { return nil }

func (c *countingHitCollector) Collect(doc int) error {
	c.count++
	c.sum += c.docBase + doc
	return nil
}

// newBitSetQuery builds a constant-score query whose scorer iterates the set
// bits of docs, mirroring TestScorerPerf.BitSetQuery.
func newBitSetQuery(docs *util.FixedBitSet) *bitSetQuery {
	return &bitSetQuery{docs: docs}
}

// bitSetQuery is a faithful port of TestScorerPerf.BitSetQuery: a Query whose
// Weight is a ConstantScoreWeight returning a ConstantScoreScorer over a
// BitSetIterator built from the bitset.
type bitSetQuery struct {
	docs *util.FixedBitSet
}

func (q *bitSetQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	scoreMode := search.COMPLETE_NO_SCORES
	if needsScores {
		scoreMode = search.COMPLETE
	}
	supplier := func(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
		iter := util.NewBitSetIterator(q.docs, int64(q.docs.Cardinality()))
		return search.NewConstantScoreScorerSupplierFromIterator(boost, scoreMode, iter), nil
	}
	cacheable := func(_ *index.LeafReaderContext) bool { return false }
	return search.NewConstantScoreWeight(q, boost, supplier, cacheable), nil
}

func (q *bitSetQuery) Rewrite(_ search.IndexReader) (search.Query, error) { return q, nil }

func (q *bitSetQuery) Clone() search.Query { return &bitSetQuery{docs: q.docs} }

func (q *bitSetQuery) Equals(other search.Query) bool {
	o, ok := other.(*bitSetQuery)
	return ok && o.docs == q.docs
}

// HashCode hashes the bitset contents, mirroring Lucene's
// FixedBitSet.hashCode so that BooleanQuery.rewrite's clause-dedup (keyed by
// type+hashCode) does not collapse distinct bitset clauses into one (which it
// would if every bitset hashed to the same value). This matches the upstream
// BitSetQuery.hashCode that mixes docs.hashCode() into the result.
func (q *bitSetQuery) HashCode() int {
	var h uint64 = 0
	for _, word := range q.docs.GetBits() {
		h = (h << 1) | (h >> 63) // rotate left 1, per FixedBitSet.hashCode
		h += word
	}
	return int((h>>32)^h) + 0x98761234
}

func (q *bitSetQuery) String() string { return "randomBitSetFilter" }

var _ search.Query = (*bitSetQuery)(nil)