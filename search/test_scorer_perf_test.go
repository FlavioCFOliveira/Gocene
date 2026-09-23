// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestScorerPerf.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// scorerPerfValidate renders `private final boolean validate = true; // set to
// false when doing performance testing`.
const scorerPerfValidate = true

// spRandBitSet renders the static randBitSet(int, int).
func spRandBitSet(t *testing.T, sz, numBitsToSet int) *util.FixedBitSet {
	t.Helper()
	set, err := util.NewFixedBitSet(sz)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < numBitsToSet; i++ {
		set.Set(random().Intn(sz))
	}
	return set
}

// spRandBitSets renders the static randBitSets(int, int).
func spRandBitSets(t *testing.T, numSets, setSize int) []*util.FixedBitSet {
	t.Helper()
	sets := make([]*util.FixedBitSet, numSets)
	for i := 0; i < len(sets); i++ {
		sets[i] = spRandBitSet(t, setSize, random().Intn(setSize))
	}
	return sets
}

// countingHitCollectorManager renders the private record
// CountingHitCollectorManager.
type countingHitCollectorManager struct{}

func (countingHitCollectorManager) NewCollector() (*countingHitCollector, error) {
	return newCountingHitCollector(), nil
}

func (countingHitCollectorManager) Reduce(collectors []*countingHitCollector) (*countingHitCollector, error) {
	result := newCountingHitCollector()
	for _, collector := range collectors {
		result.count += collector.count
		result.sum += collector.sum
	}
	return result, nil
}

// countingHitCollector renders the private static class CountingHitCollector.
type countingHitCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	count   int
	sum     int
	docBase int
}

func newCountingHitCollector() *countingHitCollector {
	c := &countingHitCollector{}
	c.Outer = c
	return c
}

func (c *countingHitCollector) Collect(doc int) error {
	c.count++
	c.sum += c.docBase + doc // use it to avoid any possibility of being eliminated by hotspot
	return nil
}

func (c *countingHitCollector) getCount() int { return c.count }

func (c *countingHitCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	c.docBase = context.DocBase
	return nil
}

func (c *countingHitCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *countingHitCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *countingHitCollector) SetScorer(scorer search.Scorable) error { return nil }

func (c *countingHitCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *countingHitCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *countingHitCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.BaseLeafCollector.CompetitiveIterator()
}

func (c *countingHitCollector) Finish() error { return c.BaseLeafCollector.Finish() }

// bitSetQuery renders the private static class BitSetQuery.
type bitSetQuery struct {
	search.BaseQuery
	docs *util.FixedBitSet
}

// CreateWeight renders createWeight: an anonymous ConstantScoreWeight.
func (q *bitSetQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	var w *search.ConstantScoreWeight
	w = search.NewConstantScoreWeight(q, boost,
		func(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
			scorer := search.NewConstantScoreScorer(
				w.Score(), scoreMode, util.NewBitSetIterator(q.docs, int64(q.docs.ApproximateCardinality())))
			return search.NewDefaultScorerSupplier(scorer), nil
		},
		func(ctx *index.LeafReaderContext) bool {
			return false
		})
	return w, nil
}

func (q *bitSetQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) { return q, nil }

// Visit renders visit(QueryVisitor), which is empty.
func (q *bitSetQuery) Visit(visitor search.QueryVisitor) {}

// ToString renders toString(String).
func (q *bitSetQuery) ToString(field string) string { return "randomBitSetFilter" }

func (q *bitSetQuery) String() string { return q.ToString("") }

// Equals renders equals(Object).
func (q *bitSetQuery) Equals(other spi.Query) bool {
	o, ok := other.(*bitSetQuery)
	return ok && q.docs.Equals(o.docs)
}

// HashCode renders hashCode(): 31 * classHash() + docs.hashCode().
func (q *bitSetQuery) HashCode() int {
	return int(int32(31*javaStringHashCode("org.apache.lucene.search.TestScorerPerf$BitSetQuery")) + int32(q.docs.HashCode()))
}

// spAddClause renders the private addClause(FixedBitSet[], BooleanQuery.Builder, FixedBitSet).
func spAddClause(t *testing.T, sets []*util.FixedBitSet, bq *search.BooleanQueryBuilder, result *util.FixedBitSet) *util.FixedBitSet {
	t.Helper()
	rnd := sets[random().Intn(len(sets))]
	q := &bitSetQuery{docs: rnd}
	bq.Add(q, search.MUST)
	if scorerPerfValidate {
		if result == nil {
			result = rnd.Clone()
		} else if err := result.And(rnd); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

// spDoConjunctions renders the private doConjunctions(IndexSearcher, FixedBitSet[], int, int).
func spDoConjunctions(t *testing.T, s *search.IndexSearcher, sets []*util.FixedBitSet, iter, maxClauses int) {
	t.Helper()
	for i := 0; i < iter; i++ {
		nClauses := random().Intn(maxClauses-1) + 2 // min 2 clauses
		bq := search.NewBooleanQueryBuilder()
		var result *util.FixedBitSet
		for j := 0; j < nClauses; j++ {
			result = spAddClause(t, sets, bq, result)
		}
		hc, err := search.SearchWithCollectorManager[*countingHitCollector, *countingHitCollector](s, bq.Build(), countingHitCollectorManager{})
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		if scorerPerfValidate {
			assertIntEquals(t, result.Cardinality(), hc.getCount())
		}
	}
}

// spDoNestedConjunctions renders the private doNestedConjunctions(IndexSearcher,
// FixedBitSet[], int, int, int).
func spDoNestedConjunctions(t *testing.T, s *search.IndexSearcher, sets []*util.FixedBitSet, iter, maxOuterClauses, maxClauses int) {
	t.Helper()
	nMatches := int64(0)

	for i := 0; i < iter; i++ {
		oClauses := random().Intn(maxOuterClauses-1) + 2
		oq := search.NewBooleanQueryBuilder()
		var result *util.FixedBitSet

		for o := 0; o < oClauses; o++ {
			nClauses := random().Intn(maxClauses-1) + 2 // min 2 clauses
			bq := search.NewBooleanQueryBuilder()
			for j := 0; j < nClauses; j++ {
				result = spAddClause(t, sets, bq, result)
			}

			oq.Add(bq.Build(), search.MUST)
		} // outer

		hc, err := search.SearchWithCollectorManager[*countingHitCollector, *countingHitCollector](s, oq.Build(), countingHitCollectorManager{})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		nMatches += int64(hc.getCount())
		if scorerPerfValidate {
			assertIntEquals(t, result.Cardinality(), hc.getCount())
		}
	}
	if testing.Verbose() {
		t.Logf("Average number of matches=%d", nMatches/int64(iter))
	}
}

func TestScorerPerfConjunctions(t *testing.T) {
	// test many small sets... the bugs will be found on boundary conditions
	d := newDirectory()
	defer mustClose(t, d)
	iw := mustNewIndexWriter(t, d, newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random())))
	mustAddDocument(t, iw, document.NewDocument())
	mustClose(t, iw)

	r := mustOpenDirectoryReader(t, d)
	defer mustClose(t, r)
	s := newSearcher(t, r)
	s.SetQueryCache(nil)
	sets := spRandBitSets(t, atLeast(1000), atLeast(10))
	iterations := atLeast(500) // TEST_NIGHTLY ? atLeast(10000) : atLeast(500)
	spDoConjunctions(t, s, sets, iterations, atLeast(5))
	spDoNestedConjunctions(t, s, sets, iterations, atLeast(3), atLeast(3))
}
