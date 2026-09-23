// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBooleanScorerSupplier.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// bssScoreModeValues renders ScoreMode.values().
var bssScoreModeValues = []search.ScoreMode{
	search.COMPLETE, search.COMPLETE_NO_SCORES, search.TOP_SCORES, search.TOP_DOCS, search.TOP_DOCS_WITH_SCORES,
}

// bssRandomScoreMode renders RandomPicks.randomFrom(random(), ScoreMode.values()).
func bssRandomScoreMode() search.ScoreMode {
	return bssScoreModeValues[random().Intn(len(bssScoreModeValues))]
}

// bssRandomRequiredOccur renders
// RandomPicks.randomFrom(random(), Arrays.asList(Occur.FILTER, Occur.MUST)).
func bssRandomRequiredOccur() search.Occur {
	if random().Intn(2) == 0 {
		return search.FILTER
	}
	return search.MUST
}

// bssFakeWeight renders the private static FakeWeight.
type bssFakeWeight struct {
	*search.BaseWeight
}

func newBSSFakeWeight() *bssFakeWeight {
	return &bssFakeWeight{BaseWeight: search.NewBaseWeight(search.MatchNoDocsQueryInstance)}
}

func (w *bssFakeWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return nil, nil
}

func (w *bssFakeWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

func (w *bssFakeWeight) IsCacheable(ctx *index.LeafReaderContext) bool { return false }

// bssFakeScorer renders the private static FakeScorer.
type bssFakeScorer struct {
	search.BaseScorer
	it search.DocIdSetIterator
}

func newBSSFakeScorer(cost int64) *bssFakeScorer {
	if cost > math.MaxInt32 || cost < math.MinInt32 {
		panic("ArithmeticException: integer overflow") // Math.toIntExact
	}
	return &bssFakeScorer{it: search.All(int(cost))}
}

func (s *bssFakeScorer) DocID() int { return s.it.DocID() }

func (s *bssFakeScorer) Score() (float32, error) { return 1, nil }

func (s *bssFakeScorer) GetMaxScore(upTo int) (float32, error) { return 1, nil }

func (s *bssFakeScorer) Iterator() search.DocIdSetIterator { return s.it }

// NextDocsAndScores keeps the concrete body of Scorer.nextDocsAndScores.
func (s *bssFakeScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

func (s *bssFakeScorer) String() string {
	return fmt.Sprintf("FakeScorer(cost=%d)", s.it.Cost())
}

// bssFakeScorerSupplier renders the private static FakeScorerSupplier.
type bssFakeScorerSupplier struct {
	search.BaseScorerSupplier
	cost                  int64
	leadCost              *int64
	topLevelScoringClause bool
}

// newBSSFakeScorerSupplier renders FakeScorerSupplier(long cost).
func newBSSFakeScorerSupplier(cost int64) *bssFakeScorerSupplier {
	return &bssFakeScorerSupplier{cost: cost}
}

// newBSSFakeScorerSupplierWithLeadCost renders FakeScorerSupplier(long cost, long leadCost).
func newBSSFakeScorerSupplierWithLeadCost(cost, leadCost int64) *bssFakeScorerSupplier {
	return &bssFakeScorerSupplier{cost: cost, leadCost: &leadCost}
}

// Get renders get(long leadCost). Its JUnit assertions throw AssertionError,
// rendered as a panic with util.NewAssertionError.
func (s *bssFakeScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	if s.leadCost != nil {
		if *s.leadCost < s.cost {
			// If the expected lead cost is less than the cost, ie. another clause is leading
			// iteration, then the exact lead cost must be provided.
			if *s.leadCost != leadCost {
				panic(util.NewAssertionError(fmt.Sprintf("%v actual leadCost=%d expected:<%d> but was:<%d>",
					s, leadCost, *s.leadCost, leadCost)))
			}
		} else {
			// Otherwise the lead cost may be provided as the cost of this very clause or as
			// Long.MAX_VALUE (typically for bulk scorers), both signaling that this clause is leading
			// iteration.
			if !(leadCost >= *s.leadCost) {
				panic(util.NewAssertionError(fmt.Sprintf("%v actual leadCost=%d", s, leadCost)))
			}
		}
	}
	return newBSSFakeScorer(s.cost), nil
}

func (s *bssFakeScorerSupplier) Cost() int64 { return s.cost }

func (s *bssFakeScorerSupplier) String() string {
	leadCost := "null"
	if s.leadCost != nil {
		leadCost = fmt.Sprint(*s.leadCost)
	}
	return fmt.Sprintf("FakeLazyScorer(cost=%d,leadCost=%s)", s.cost, leadCost)
}

func (s *bssFakeScorerSupplier) SetTopLevelScoringClause() error {
	s.topLevelScoringClause = true
	return nil
}

// BulkScorer keeps ScorerSupplier.bulkScorer()'s concrete body.
func (s *bssFakeScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.DefaultScorerSupplierBulkScorer(s)
}

// newBSSSubs renders the EnumMap<Occur, Collection<ScorerSupplier>> with an
// empty list per Occur.
func newBSSSubs() map[search.Occur][]search.ScorerSupplier {
	subs := map[search.Occur][]search.ScorerSupplier{}
	for _, occur := range bqOccurValues {
		subs[occur] = []search.ScorerSupplier{}
	}
	return subs
}

func bssMustGet(t *testing.T, s search.ScorerSupplier, leadCost int64) search.Scorer {
	t.Helper()
	scorer, err := s.Get(leadCost)
	if err != nil {
		t.Fatalf("get(%d): %v", leadCost, err)
	}
	return scorer
}

func bssMustBulkScorer(t *testing.T, s search.ScorerSupplier) search.BulkScorer {
	t.Helper()
	bs, err := s.BulkScorer()
	if err != nil {
		t.Fatalf("bulkScorer: %v", err)
	}
	return bs
}

func assertInt64Equals(t *testing.T, expected, actual int64) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected:<%d> but was:<%d>", expected, actual)
	}
}

func TestBooleanScorerSupplierConjunctionCost(t *testing.T) {
	subs := newBSSSubs()

	o := bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplier(42))
	assertInt64Equals(t, 42, search.NewBooleanScorerSupplier(nil, subs, bssRandomScoreMode(), 0, 100).Cost())

	o = bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplier(12))
	assertInt64Equals(t, 12, search.NewBooleanScorerSupplier(nil, subs, bssRandomScoreMode(), 0, 100).Cost())

	o = bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplier(20))
	assertInt64Equals(t, 12, search.NewBooleanScorerSupplier(nil, subs, bssRandomScoreMode(), 0, 100).Cost())
}

func TestBooleanScorerSupplierDisjunctionCost(t *testing.T) {
	subs := newBSSSubs()

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(42))
	var s search.ScorerSupplier = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)
	assertInt64Equals(t, 42, s.Cost())
	assertInt64Equals(t, 42, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(12))
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)
	assertInt64Equals(t, 42+12, s.Cost())
	assertInt64Equals(t, 42+12, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(20))
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)
	assertInt64Equals(t, 42+12+20, s.Cost())
	assertInt64Equals(t, 42+12+20, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())
}

func TestBooleanScorerSupplierDisjunctionWithMinShouldMatchCost(t *testing.T) {
	subs := newBSSSubs()

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(42))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(12))
	var s search.ScorerSupplier = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 1, 100)
	assertInt64Equals(t, 42+12, s.Cost())
	assertInt64Equals(t, 42+12, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(20))
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 1, 100)
	assertInt64Equals(t, 42+12+20, s.Cost())
	assertInt64Equals(t, 42+12+20, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100)
	assertInt64Equals(t, 12+20, s.Cost())
	assertInt64Equals(t, 12+20, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplier(30))
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 1, 100)
	assertInt64Equals(t, 42+12+20+30, s.Cost())
	assertInt64Equals(t, 42+12+20+30, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100)
	assertInt64Equals(t, 12+20+30, s.Cost())
	assertInt64Equals(t, 12+20+30, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())
	s = search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 3, 100)
	assertInt64Equals(t, 12+20, s.Cost())
	assertInt64Equals(t, 12+20, bssMustGet(t, s, int64(random().Intn(100))).Iterator().Cost())
}

func TestBooleanScorerSupplierDuelCost(t *testing.T) {
	iters := atLeast(1000)
	for iter := 0; iter < iters; iter++ {
		subs := newBSSSubs()
		numClauses := nextInt(1, 10)
		numShoulds := 0
		numRequired := 0
		for j := 0; j < numClauses; j++ {
			occur := bqOccurValues[random().Intn(len(bqOccurValues))]
			subs[occur] = append(subs[occur], newBSSFakeScorerSupplier(int64(random().Intn(100))))
			if occur == search.SHOULD {
				numShoulds++
			} else if occur == search.FILTER || occur == search.MUST {
				numRequired++
			}
		}
		scoreMode := bssRandomScoreMode()
		if !scoreMode.NeedsScores() && numRequired > 0 {
			numClauses -= numShoulds
			numShoulds = 0
			subs[search.SHOULD] = subs[search.SHOULD][:0]
		}
		if numShoulds+numRequired == 0 {
			// only negative clauses, invalid
			continue
		}
		minShouldMatch := 0
		if numShoulds != 0 {
			minShouldMatch = nextInt(0, numShoulds-1)
		}
		supplier := search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, scoreMode, minShouldMatch, 100)
		cost1 := supplier.Cost()
		cost2 := bssMustGet(t, supplier, math.MaxInt64).Iterator().Cost()
		if cost1 != cost2 {
			t.Fatalf("clauses=%v, minShouldMatch=%d expected:<%d> but was:<%d>", subs, minShouldMatch, cost1, cost2)
		}
	}
}

// expectAssertionError renders expectThrows(AssertionError.class, ...).
func expectAssertionError(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if _, ok := r.(*util.AssertionError); !ok {
			t.Fatalf("expected AssertionError, got %v", r)
		}
	}()
	fn()
}

// test the tester...
func TestBooleanScorerSupplierFakeScorerSupplier(t *testing.T) {
	randomAccessSupplier := newBSSFakeScorerSupplierWithLeadCost(int64(nextInt(31, 100)), 30)
	expectAssertionError(t, func() { bssMustGet(t, randomAccessSupplier, 70) })
	sequentialSupplier := newBSSFakeScorerSupplierWithLeadCost(int64(random().Intn(70)), 70)
	expectAssertionError(t, func() { bssMustGet(t, sequentialSupplier, 30) })
}

func TestBooleanScorerSupplierConjunctionLeadCost(t *testing.T) {
	subs := newBSSSubs()

	// If the clauses are less costly than the lead cost, the min cost is the new lead cost
	o := bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplierWithLeadCost(42, 12))
	o = bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplierWithLeadCost(12, 12))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		math.MaxInt64) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)) // triggers assertions as a side-effect

	subs = newBSSSubs()

	// If the lead cost is less that the clauses' cost, then we don't modify it
	o = bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplierWithLeadCost(42, 7))
	o = bssRandomRequiredOccur()
	subs[o] = append(subs[o], newBSSFakeScorerSupplierWithLeadCost(12, 7))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		7) // triggers assertions as a side-effect
}

func TestBooleanScorerSupplierDisjunctionLeadCost(t *testing.T) {
	subs := newBSSSubs()
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(42, 54))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 54))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)) // triggers assertions as a side-effect

	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 20))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		20) // triggers assertions as a side-effect
}

func TestBooleanScorerSupplierDisjunctionWithMinShouldMatchLeadCost(t *testing.T) {
	subs := newBSSSubs()

	// minShouldMatch is 2 so the 2 least costly clauses will lead iteration
	// and their cost will be 30+12=42
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(50, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(30, 42))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100)) // triggers assertions as a side-effect

	subs = newBSSSubs()

	// If the leadCost is less than the msm cost, then it wins
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(30, 20))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100),
		20) // triggers assertions as a side-effect

	subs = newBSSSubs()

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(42, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(30, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(20, 62))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 2, 100)) // triggers assertions as a side-effect

	subs = newBSSSubs()

	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(42, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(12, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(30, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(20, 32))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 3, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 3, 100)) // triggers assertions as a side-effect
}

func TestBooleanScorerSupplierProhibitedLeadCost(t *testing.T) {
	subs := newBSSSubs()

	// The MUST_NOT clause is called with the same lead cost as the MUST clause
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], newBSSFakeScorerSupplierWithLeadCost(30, 42))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)) // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.MUST_NOT] = subs[search.MUST_NOT][:0]
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], newBSSFakeScorerSupplierWithLeadCost(80, 42))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100)) // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.MUST_NOT] = subs[search.MUST_NOT][:0]
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], newBSSFakeScorerSupplierWithLeadCost(30, 20))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, bssRandomScoreMode(), 0, 100),
		20) // triggers assertions as a side-effect
}

func TestBooleanScorerSupplierMixedLeadCost(t *testing.T) {
	subs := newBSSSubs()

	// The SHOULD clause is always called with the same lead cost as the MUST clause
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(30, 42))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.COMPLETE, 0, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.COMPLETE, 0, 100)) // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(80, 42))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.COMPLETE, 0, 100),
		100) // triggers assertions as a side-effect
	bssMustBulkScorer(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.COMPLETE, 0, 100)) // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.MUST] = append(subs[search.MUST], newBSSFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], newBSSFakeScorerSupplierWithLeadCost(80, 20))
	bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.COMPLETE, 0, 100),
		20) // triggers assertions as a side-effect
}

// bssTopLevelScoringClause renders the shared body of the
// *TopLevelScoringClause tests: two FakeScorerSupplier(10, 10) clauses under
// occur1 and occur2, setTopLevelScoringClause() on a TOP_SCORES supplier, then
// the expected topLevelScoringClause flag of each clause.
func bssTopLevelScoringClause(t *testing.T, occur1, occur2 search.Occur, want1, want2 bool) {
	t.Helper()
	subs := newBSSSubs()

	clause1 := newBSSFakeScorerSupplierWithLeadCost(10, 10)
	subs[occur1] = append(subs[occur1], clause1)
	clause2 := newBSSFakeScorerSupplierWithLeadCost(10, 10)
	subs[occur2] = append(subs[occur2], clause2)

	if err := search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.TOP_SCORES, 0, 100).SetTopLevelScoringClause(); err != nil {
		t.Fatal(err)
	}
	if clause1.topLevelScoringClause != want1 {
		t.Fatalf("clause1.topLevelScoringClause = %v, want %v", clause1.topLevelScoringClause, want1)
	}
	if clause2.topLevelScoringClause != want2 {
		t.Fatalf("clause2.topLevelScoringClause = %v, want %v", clause2.topLevelScoringClause, want2)
	}
}

func TestBooleanScorerSupplierDisjunctionTopLevelScoringClause(t *testing.T) {
	bssTopLevelScoringClause(t, search.SHOULD, search.SHOULD, false, false)
}

func TestBooleanScorerSupplierConjunctionTopLevelScoringClause(t *testing.T) {
	bssTopLevelScoringClause(t, search.MUST, search.MUST, false, false)
}

func TestBooleanScorerSupplierFilterTopLevelScoringClause(t *testing.T) {
	bssTopLevelScoringClause(t, search.FILTER, search.FILTER, false, false)
}

func TestBooleanScorerSupplierSingleMustScoringClause(t *testing.T) {
	bssTopLevelScoringClause(t, search.MUST, search.FILTER, true, false)
}

func TestBooleanScorerSupplierSingleShouldScoringClause(t *testing.T) {
	bssTopLevelScoringClause(t, search.SHOULD, search.MUST_NOT, true, false)
}

func TestBooleanScorerSupplierMaxScoreNonTopLevelScoringClause(t *testing.T) {
	subs := newBSSSubs()

	clause1 := newBSSFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause1)
	clause2 := newBSSFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause2)

	scorer := bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.TOP_SCORES, 0, 100), 10)
	maxScore, err := scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatal(err)
	}
	if maxScore != 2.0 {
		t.Fatalf("maxScore = %v, want 2.0", maxScore)
	}

	subs = newBSSSubs()

	subs[search.SHOULD] = append(subs[search.SHOULD], clause1)
	subs[search.SHOULD] = append(subs[search.SHOULD], clause2)

	scorer = bssMustGet(t, search.NewBooleanScorerSupplier(newBSSFakeWeight(), subs, search.TOP_SCORES, 0, 100), 10)
	maxScore, err = scorer.GetMaxScore(search.NO_MORE_DOCS)
	if err != nil {
		t.Fatal(err)
	}
	if maxScore != 2.0 {
		t.Fatalf("maxScore = %v, want 2.0", maxScore)
	}
}
