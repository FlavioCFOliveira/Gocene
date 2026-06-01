// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/search/TestBooleanScorerSupplier.java
// Purpose: Tests scorer selection logic and cost-based optimization for BooleanScorerSupplier

package search_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// FakeScorerSupplier is a test implementation of ScorerSupplier
type FakeScorerSupplier struct {
	cost                  int64
	leadCost              *int64
	topLevelScoringClause bool
}

// NewFakeScorerSupplier creates a new FakeScorerSupplier with just cost.
func NewFakeScorerSupplier(cost int64) *FakeScorerSupplier {
	return &FakeScorerSupplier{
		cost:     cost,
		leadCost: nil,
	}
}

// NewFakeScorerSupplierWithLeadCost creates a new FakeScorerSupplier with cost and expected leadCost.
func NewFakeScorerSupplierWithLeadCost(cost, leadCost int64) *FakeScorerSupplier {
	lc := leadCost
	return &FakeScorerSupplier{
		cost:     cost,
		leadCost: &lc,
	}
}

// Get returns a Scorer for the given leadCost.
func (s *FakeScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	if s.leadCost != nil {
		if *s.leadCost < s.cost {
			// If the expected lead cost is less than the cost, ie. another clause is leading
			// iteration, then the exact lead cost must be provided.
			if *s.leadCost != leadCost {
				panic(fmt.Sprintf("%s actual leadCost=%d, expected=%d", s.String(), leadCost, *s.leadCost))
			}
		} else {
			// Otherwise the lead cost may be provided as the cost of this very clause or as
			// Long.MAX_VALUE (typically for bulk scorers), both signaling that this clause is leading
			// iteration.
			if leadCost < *s.leadCost {
				panic(fmt.Sprintf("%s actual leadCost=%d, expected >= %d", s.String(), leadCost, *s.leadCost))
			}
		}
	}
	return NewFakeScorer(s.cost), nil
}

// Cost returns the estimated cost.
func (s *FakeScorerSupplier) Cost() int64 {
	return s.cost
}

// String returns a string representation.
func (s *FakeScorerSupplier) String() string {
	if s.leadCost != nil {
		return fmt.Sprintf("FakeLazyScorer(cost=%d,leadCost=%d)", s.cost, *s.leadCost)
	}
	return fmt.Sprintf("FakeLazyScorer(cost=%d,leadCost=nil)", s.cost)
}

// SetTopLevelScoringClause marks this as a top-level scoring clause.
func (s *FakeScorerSupplier) SetTopLevelScoringClause() {
	s.topLevelScoringClause = true
}

// IsTopLevelScoringClause returns true if this is a top-level scoring clause.
func (s *FakeScorerSupplier) IsTopLevelScoringClause() bool {
	return s.topLevelScoringClause
}

// FakeScorer is a minimal Scorer implementation for testing
type FakeScorer struct {
	*search.BaseDocIdSetIterator
	cost int64
}

// NewFakeScorer creates a new FakeScorer.
func NewFakeScorer(cost int64) *FakeScorer {
	return &FakeScorer{
		BaseDocIdSetIterator: search.NewBaseDocIdSetIterator(),
		cost:                 cost,
	}
}

// DocID returns the current document ID.
func (s *FakeScorer) DocID() int {
	return -1
}

// NextDoc advances to the next document.
func (s *FakeScorer) NextDoc() (int, error) {
	return search.NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (s *FakeScorer) Advance(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// Cost returns the estimated cost.
func (s *FakeScorer) Cost() int64 {
	return s.cost
}

// Score returns the score.
func (s *FakeScorer) Score() float32 {
	return 1.0
}

// GetMaxScore returns the maximum score.
func (s *FakeScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// AdvanceShallow returns search.NO_MORE_DOCS (the Scorer default).
func (s *FakeScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// String returns a string representation.
func (s *FakeScorer) String() string {
	return fmt.Sprintf("FakeScorer(cost=%d)", s.cost)
}

// BooleanScorerSupplier provides scorers for BooleanQuery.
// This is a test-local implementation that mirrors the production
// BooleanScorerSupplier logic for cost verification.
type BooleanScorerSupplier struct {
	weight         search.Weight
	subs           map[search.Occur][]search.ScorerSupplier
	scoreMode      search.ScoreMode
	minShouldMatch int
	minScore       float32
	cachedCost     int64 // -1 means not yet computed
}

// NewBooleanScorerSupplier creates a new BooleanScorerSupplier.
func NewBooleanScorerSupplier(weight search.Weight, subs map[search.Occur][]search.ScorerSupplier, scoreMode search.ScoreMode, minShouldMatch int, minScore float32) *BooleanScorerSupplier {
	return &BooleanScorerSupplier{
		weight:         weight,
		subs:           subs,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		minScore:       minScore,
		cachedCost:     -1,
	}
}

// computeShouldCost returns the cost of SHOULD clauses using CostWithMinShouldMatch.
// Mirrors BooleanScorerSupplier.computeShouldCost().
func (bss *BooleanScorerSupplier) computeShouldCost() int64 {
	optional := bss.subs[search.SHOULD]
	if len(optional) == 0 {
		return 0
	}
	costs := make([]int64, len(optional))
	for i, s := range optional {
		costs[i] = s.Cost()
	}
	return search.CostWithMinShouldMatch(costs, len(optional), bss.minShouldMatch)
}

// computeCost computes the cost following Lucene's BooleanScorerSupplier.computeCost().
func (bss *BooleanScorerSupplier) computeCost() int64 {
	// minimum required cost (MUST + FILTER)
	minRequired := int64(math.MaxInt64)
	hasRequired := false
	for _, s := range bss.subs[search.MUST] {
		if c := s.Cost(); c < minRequired {
			minRequired = c
			hasRequired = true
		}
	}
	for _, s := range bss.subs[search.FILTER] {
		if c := s.Cost(); c < minRequired {
			minRequired = c
			hasRequired = true
		}
	}

	if hasRequired && bss.minShouldMatch == 0 {
		return minRequired
	}

	shouldCost := bss.computeShouldCost()
	if hasRequired {
		if minRequired < shouldCost {
			return minRequired
		}
		return shouldCost
	}
	return shouldCost
}

// Cost returns an estimate of the number of documents this scorer will match.
func (bss *BooleanScorerSupplier) Cost() int64 {
	if bss.cachedCost == -1 {
		bss.cachedCost = bss.computeCost()
	}
	return bss.cachedCost
}

// Get returns a Scorer for the given leadCost.
// Mirrors BooleanScorerSupplier.getInternal(): first clamps leadCost to Cost().
func (bss *BooleanScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	// Mirror getInternal() first line: leadCost = min(leadCost, cost())
	if c := bss.Cost(); leadCost > c {
		leadCost = c
	}

	scorers := make([]search.Scorer, 0)

	for _, clause := range bss.subs[search.MUST] {
		scorer, err := clause.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}
	for _, clause := range bss.subs[search.FILTER] {
		scorer, err := clause.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}
	for _, clause := range bss.subs[search.SHOULD] {
		scorer, err := clause.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}
	for _, clause := range bss.subs[search.MUST_NOT] {
		scorer, err := clause.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}

	return newBooleanScorerWithCost(scorers, bss.scoreMode, bss.minShouldMatch, bss.Cost()), nil
}

// BulkScorer returns a BulkScorer for this boolean query.
func (bss *BooleanScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	scorer, err := bss.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return NewDefaultBulkScorer(scorer), nil
}

// SetTopLevelScoringClause marks this as a top-level scoring clause.
func (bss *BooleanScorerSupplier) SetTopLevelScoringClause() {
	mustCount := len(bss.subs[search.MUST])
	shouldCount := len(bss.subs[search.SHOULD])
	filterCount := len(bss.subs[search.FILTER])

	// Single MUST clause with only FILTER clauses -> mark MUST as top-level
	if mustCount == 1 && shouldCount == 0 {
		for _, clause := range bss.subs[search.MUST] {
			if sslc, ok := clause.(interface{ SetTopLevelScoringClause() }); ok {
				sslc.SetTopLevelScoringClause()
			}
		}
		return
	}

	// Single SHOULD clause with only MUST_NOT clauses -> mark SHOULD as top-level
	if shouldCount == 1 && mustCount == 0 && filterCount == 0 {
		for _, clause := range bss.subs[search.SHOULD] {
			if sslc, ok := clause.(interface{ SetTopLevelScoringClause() }); ok {
				sslc.SetTopLevelScoringClause()
			}
		}
		return
	}
}

// String returns a string representation.
func (bss *BooleanScorerSupplier) String() string {
	return fmt.Sprintf("BooleanScorerSupplier(cost=%d)", bss.Cost())
}

// BooleanScorer is a scorer for boolean queries.
// cost is pre-computed by the supplier and stored here so that Cost() is
// consistent with the supplier's Cost() — matching the Java behaviour where
// the scorer returned by get() has the same cost as supplier.cost().
type BooleanScorer struct {
	*search.BaseScorer
	scorers        []search.Scorer
	scoreMode      search.ScoreMode
	minShouldMatch int
	currentDoc     int
	supplierCost   int64
}

// NewBooleanScorer creates a new BooleanScorer.
func NewBooleanScorer(scorers []search.Scorer, scoreMode search.ScoreMode, minShouldMatch int) *BooleanScorer {
	return &BooleanScorer{
		BaseScorer:     search.NewBaseScorer(nil),
		scorers:        scorers,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		currentDoc:     -1,
		supplierCost:   -1, // set by supplier after construction
	}
}

// newBooleanScorerWithCost creates a BooleanScorer with a pre-computed cost.
func newBooleanScorerWithCost(scorers []search.Scorer, scoreMode search.ScoreMode, minShouldMatch int, cost int64) *BooleanScorer {
	return &BooleanScorer{
		BaseScorer:     search.NewBaseScorer(nil),
		scorers:        scorers,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		currentDoc:     -1,
		supplierCost:   cost,
	}
}

// DocID returns the current document ID.
func (bs *BooleanScorer) DocID() int {
	return bs.currentDoc
}

// NextDoc advances to the next document.
func (bs *BooleanScorer) NextDoc() (int, error) {
	if len(bs.scorers) == 0 {
		bs.currentDoc = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}
	bs.currentDoc = search.NO_MORE_DOCS
	return search.NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (bs *BooleanScorer) Advance(target int) (int, error) {
	bs.currentDoc = search.NO_MORE_DOCS
	return search.NO_MORE_DOCS, nil
}

// Cost returns the estimated cost. When the supplier pre-computed the cost it
// is stored in supplierCost so that get().cost() == supplier.cost().
func (bs *BooleanScorer) Cost() int64 {
	if bs.supplierCost >= 0 {
		return bs.supplierCost
	}
	var cost int64 = 0
	for _, scorer := range bs.scorers {
		cost += scorer.Cost()
	}
	return cost
}

// Score returns the score for the current document.
func (bs *BooleanScorer) Score() float32 {
	var score float32 = 0
	for _, scorer := range bs.scorers {
		score += scorer.Score()
	}
	return score
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (bs *BooleanScorer) GetMaxScore(upTo int) float32 {
	var maxScore float32 = 0
	for _, scorer := range bs.scorers {
		if sms, ok := scorer.(interface{ GetMaxScore(int) float32 }); ok {
			maxScore += sms.GetMaxScore(upTo)
		} else {
			maxScore += 1.0
		}
	}
	return maxScore
}

// AdvanceShallow returns search.NO_MORE_DOCS (the Scorer default).
func (bs *BooleanScorer) AdvanceShallow(int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (bs *BooleanScorer) DocIDRunEnd() int {
	return bs.currentDoc + 1
}

// DefaultBulkScorer is a test-local BulkScorer used only to satisfy the
// BooleanScorerSupplier.BulkScorer() side-effect path in these tests. It mirrors
// the windowed search.BulkScorer contract introduced by rmp #4777.
type DefaultBulkScorer struct {
	scorer search.Scorer
}

// NewDefaultBulkScorer creates a new DefaultBulkScorer.
func NewDefaultBulkScorer(scorer search.Scorer) *DefaultBulkScorer {
	return &DefaultBulkScorer{scorer: scorer}
}

// Score scores documents in [min, max), returning the next matching doc on or
// after max.
func (bs *DefaultBulkScorer) Score(collector search.LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if err := collector.SetScorer(bs.scorer); err != nil {
		return 0, err
	}
	doc := bs.scorer.DocID()
	if doc < min {
		var err error
		doc, err = bs.scorer.Advance(min)
		if err != nil {
			return 0, err
		}
	}
	for doc < max {
		if acceptDocs == nil || acceptDocs.Get(doc) {
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
		}
		var err error
		doc, err = bs.scorer.NextDoc()
		if err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// Cost returns the underlying scorer's cost.
func (bs *DefaultBulkScorer) Cost() int64 { return bs.scorer.Cost() }

// randomOccur returns a random Occur value from the given slice
func randomOccur(occurs []search.Occur) search.Occur {
	return occurs[rand.Intn(len(occurs))]
}

// randomScoreMode returns a random ScoreMode
func randomScoreMode() search.ScoreMode {
	modes := []search.ScoreMode{search.COMPLETE, search.COMPLETE_NO_SCORES, search.TOP_SCORES}
	return modes[rand.Intn(len(modes))]
}

// scoreModeNeedsScores returns true if the score mode needs scores
func scoreModeNeedsScores(mode search.ScoreMode) bool {
	return mode == search.COMPLETE || mode == search.TOP_SCORES
}

// TestBooleanScorerSupplier_ConjunctionCost tests cost calculation for conjunctions (MUST/FILTER clauses)
// Source: testConjunctionCost()
func TestBooleanScorerSupplier_ConjunctionCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// Add first clause
	occur := randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplier(42))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 42 {
		t.Errorf("Expected cost 42, got %d", bss.Cost())
	}

	// Add second clause
	occur = randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplier(12))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 12 {
		t.Errorf("Expected cost 12, got %d", bss.Cost())
	}

	// Add third clause
	occur = randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplier(20))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 12 {
		t.Errorf("Expected cost 12, got %d", bss.Cost())
	}
}

// TestBooleanScorerSupplier_DisjunctionCost tests cost calculation for disjunctions (SHOULD clauses)
// Source: testDisjunctionCost()
func TestBooleanScorerSupplier_DisjunctionCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(42))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 42 {
		t.Errorf("Expected cost 42, got %d", bss.Cost())
	}

	scorer, err := bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42 {
		t.Errorf("Expected scorer cost 42, got %d", scorer.Cost())
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(12))
	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 42+12 {
		t.Errorf("Expected cost %d, got %d", 42+12, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42+12 {
		t.Errorf("Expected scorer cost %d, got %d", 42+12, scorer.Cost())
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(20))
	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	if bss.Cost() != 42+12+20 {
		t.Errorf("Expected cost %d, got %d", 42+12+20, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42+12+20 {
		t.Errorf("Expected scorer cost %d, got %d", 42+12+20, scorer.Cost())
	}
}

// TestBooleanScorerSupplier_DisjunctionWithMinShouldMatchCost tests cost with minShouldMatch
// Source: testDisjunctionWithMinShouldMatchCost()
func TestBooleanScorerSupplier_DisjunctionWithMinShouldMatchCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(42))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(12))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 1, 100)
	if bss.Cost() != 42+12 {
		t.Errorf("Expected cost %d, got %d", 42+12, bss.Cost())
	}

	scorer, err := bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42+12 {
		t.Errorf("Expected scorer cost %d, got %d", 42+12, scorer.Cost())
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(20))
	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 1, 100)
	if bss.Cost() != 42+12+20 {
		t.Errorf("Expected cost %d, got %d", 42+12+20, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42+12+20 {
		t.Errorf("Expected scorer cost %d, got %d", 42+12+20, scorer.Cost())
	}

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	if bss.Cost() != 12+20 {
		t.Errorf("Expected cost %d, got %d", 12+20, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 12+20 {
		t.Errorf("Expected scorer cost %d, got %d", 12+20, scorer.Cost())
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplier(30))
	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 1, 100)
	if bss.Cost() != 42+12+20+30 {
		t.Errorf("Expected cost %d, got %d", 42+12+20+30, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 42+12+20+30 {
		t.Errorf("Expected scorer cost %d, got %d", 42+12+20+30, scorer.Cost())
	}

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	if bss.Cost() != 12+20+30 {
		t.Errorf("Expected cost %d, got %d", 12+20+30, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 12+20+30 {
		t.Errorf("Expected scorer cost %d, got %d", 12+20+30, scorer.Cost())
	}

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 3, 100)
	if bss.Cost() != 12+20 {
		t.Errorf("Expected cost %d, got %d", 12+20, bss.Cost())
	}

	scorer, err = bss.Get(int64(rand.Intn(100)))
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}
	if scorer.Cost() != 12+20 {
		t.Errorf("Expected scorer cost %d, got %d", 12+20, scorer.Cost())
	}
}

// TestBooleanScorerSupplier_DuelCost tests cost consistency between cost() and get().cost()
// Source: testDuelCost()
func TestBooleanScorerSupplier_DuelCost(t *testing.T) {
	occurs := []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER}

	for iter := 0; iter < 100; iter++ {
		subs := make(map[search.Occur][]search.ScorerSupplier)
		for _, occur := range occurs {
			subs[occur] = make([]search.ScorerSupplier, 0)
		}

		numClauses := rand.Intn(10) + 1
		numShoulds := 0
		numRequired := 0

		for j := 0; j < numClauses; j++ {
			occur := occurs[rand.Intn(len(occurs))]
			subs[occur] = append(subs[occur], NewFakeScorerSupplier(int64(rand.Intn(100))))
			if occur == search.SHOULD {
				numShoulds++
			} else if occur == search.FILTER || occur == search.MUST {
				numRequired++
			}
		}

		scoreMode := randomScoreMode()
		if !scoreModeNeedsScores(scoreMode) && numRequired > 0 {
			numClauses -= numShoulds
			numShoulds = 0
			subs[search.SHOULD] = subs[search.SHOULD][:0]
		}

		if numShoulds+numRequired == 0 {
			// only negative clauses, invalid
			continue
		}

		minShouldMatch := 0
		if numShoulds > 0 {
			minShouldMatch = rand.Intn(numShoulds)
		}

		bss := NewBooleanScorerSupplier(nil, subs, scoreMode, minShouldMatch, 100)
		cost1 := bss.Cost()

		scorer, err := bss.Get(math.MaxInt64) // MaxInt64
		if err != nil {
			t.Fatalf("Failed to get scorer: %v", err)
		}
		cost2 := scorer.Cost()

		if cost1 != cost2 {
			t.Errorf("Iteration %d: Cost mismatch: cost()=%d, get().cost()=%d, minShouldMatch=%d",
				iter, cost1, cost2, minShouldMatch)
		}
	}
}

// TestBooleanScorerSupplier_FakeScorerSupplier tests the test infrastructure
// Source: testFakeScorerSupplier()
func TestBooleanScorerSupplier_FakeScorerSupplier(t *testing.T) {
	// Test case 1: randomAccessSupplier with cost > leadCost
	// Should panic if called with wrong leadCost
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for randomAccessSupplier with wrong leadCost")
		}
	}()

	leadCost := int64(30)
	randomAccessSupplier := NewFakeScorerSupplierWithLeadCost(int64(rand.Intn(70)+31), leadCost)
	randomAccessSupplier.Get(70) // Should panic - wrong leadCost
}

// TestBooleanScorerSupplier_FakeScorerSupplier_Sequential tests sequential supplier
func TestBooleanScorerSupplier_FakeScorerSupplier_Sequential(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for sequentialSupplier with wrong leadCost")
		}
	}()

	leadCost := int64(70)
	sequentialSupplier := NewFakeScorerSupplierWithLeadCost(int64(rand.Intn(70)), leadCost)
	sequentialSupplier.Get(30) // Should panic - leadCost too low
}

// TestBooleanScorerSupplier_ConjunctionLeadCost tests lead cost propagation for conjunctions
// Source: testConjunctionLeadCost()
func TestBooleanScorerSupplier_ConjunctionLeadCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// If the clauses are less costly than the lead cost, the min cost is the new lead cost
	occur := randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplierWithLeadCost(42, 12))

	occur = randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplierWithLeadCost(12, 12))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(math.MaxInt64) // MaxInt64 - triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	// Reset
	subs = make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// If the lead cost is less than the clauses' cost, then we don't modify it
	occur = randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplierWithLeadCost(42, 7))

	occur = randomOccur([]search.Occur{search.FILTER, search.MUST})
	subs[occur] = append(subs[occur], NewFakeScorerSupplierWithLeadCost(12, 7))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(7) // triggers assertions as a side-effect
}

// TestBooleanScorerSupplier_DisjunctionLeadCost tests lead cost propagation for disjunctions
// Source: testDisjunctionLeadCost()
func TestBooleanScorerSupplier_DisjunctionLeadCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(42, 54))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 54))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 20))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(20) // triggers assertions as a side-effect
}

// TestBooleanScorerSupplier_DisjunctionWithMinShouldMatchLeadCost tests lead cost with MSM
// Source: testDisjunctionWithMinShouldMatchLeadCost()
func TestBooleanScorerSupplier_DisjunctionWithMinShouldMatchLeadCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// minShouldMatch is 2 so the 2 least costly clauses will lead iteration
	// and their cost will be 30+12=42
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(50, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(30, 42))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	// Reset
	subs = make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// If the leadCost is less than the msm cost, then it wins
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(30, 20))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	bss.Get(20) // triggers assertions as a side-effect

	// Reset
	subs = make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(42, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(30, 62))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(20, 62))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 2, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	// Reset
	subs = make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(42, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(12, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(30, 32))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(20, 32))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 3, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 3, 100)
	bss.BulkScorer() // triggers assertions as a side-effect
}

// TestBooleanScorerSupplier_ProhibitedLeadCost tests MUST_NOT clause lead cost
// Source: testProhibitedLeadCost()
func TestBooleanScorerSupplier_ProhibitedLeadCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// The MUST_NOT clause is called with the same lead cost as the MUST clause
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], NewFakeScorerSupplierWithLeadCost(30, 42))

	bss := NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.MUST_NOT] = subs[search.MUST_NOT][:0]
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], NewFakeScorerSupplierWithLeadCost(80, 42))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.MUST_NOT] = subs[search.MUST_NOT][:0]
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], NewFakeScorerSupplierWithLeadCost(30, 20))

	bss = NewBooleanScorerSupplier(nil, subs, randomScoreMode(), 0, 100)
	bss.Get(20) // triggers assertions as a side-effect
}

// TestBooleanScorerSupplier_MixedLeadCost tests mixed clause lead costs
// Source: testMixedLeadCost()
func TestBooleanScorerSupplier_MixedLeadCost(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	// The SHOULD clause is always called with the same lead cost as the MUST clause
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(30, 42))

	bss := NewBooleanScorerSupplier(nil, subs, search.COMPLETE, 0, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, search.COMPLETE, 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 42))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(80, 42))

	bss = NewBooleanScorerSupplier(nil, subs, search.COMPLETE, 0, 100)
	bss.Get(100) // triggers assertions as a side-effect

	bss = NewBooleanScorerSupplier(nil, subs, search.COMPLETE, 0, 100)
	bss.BulkScorer() // triggers assertions as a side-effect

	subs[search.MUST] = subs[search.MUST][:0]
	subs[search.SHOULD] = subs[search.SHOULD][:0]
	subs[search.MUST] = append(subs[search.MUST], NewFakeScorerSupplierWithLeadCost(42, 20))
	subs[search.SHOULD] = append(subs[search.SHOULD], NewFakeScorerSupplierWithLeadCost(80, 20))

	bss = NewBooleanScorerSupplier(nil, subs, search.COMPLETE, 0, 100)
	bss.Get(20) // triggers assertions as a side-effect
}

// TestBooleanScorerSupplier_DisjunctionTopLevelScoringClause tests top-level scoring for disjunctions
// Source: testDisjunctionTopLevelScoringClause()
func TestBooleanScorerSupplier_DisjunctionTopLevelScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.SHOULD] = append(subs[search.SHOULD], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.SHOULD] = append(subs[search.SHOULD], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	bss.SetTopLevelScoringClause()

	if clause1.IsTopLevelScoringClause() {
		t.Error("clause1 should NOT be topLevelScoringClause for disjunction")
	}
	if clause2.IsTopLevelScoringClause() {
		t.Error("clause2 should NOT be topLevelScoringClause for disjunction")
	}
}

// TestBooleanScorerSupplier_ConjunctionTopLevelScoringClause tests top-level scoring for conjunctions
// Source: testConjunctionTopLevelScoringClause()
func TestBooleanScorerSupplier_ConjunctionTopLevelScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	bss.SetTopLevelScoringClause()

	if clause1.IsTopLevelScoringClause() {
		t.Error("clause1 should NOT be topLevelScoringClause for conjunction")
	}
	if clause2.IsTopLevelScoringClause() {
		t.Error("clause2 should NOT be topLevelScoringClause for conjunction")
	}
}

// TestBooleanScorerSupplier_FilterTopLevelScoringClause tests top-level scoring for filters
// Source: testFilterTopLevelScoringClause()
func TestBooleanScorerSupplier_FilterTopLevelScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.FILTER] = append(subs[search.FILTER], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.FILTER] = append(subs[search.FILTER], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	bss.SetTopLevelScoringClause()

	if clause1.IsTopLevelScoringClause() {
		t.Error("clause1 should NOT be topLevelScoringClause for filter")
	}
	if clause2.IsTopLevelScoringClause() {
		t.Error("clause2 should NOT be topLevelScoringClause for filter")
	}
}

// TestBooleanScorerSupplier_SingleMustScoringClause tests single MUST as top-level scoring
// Source: testSingleMustScoringClause()
func TestBooleanScorerSupplier_SingleMustScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.FILTER] = append(subs[search.FILTER], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	bss.SetTopLevelScoringClause()

	if !clause1.IsTopLevelScoringClause() {
		t.Error("clause1 SHOULD be topLevelScoringClause for single MUST")
	}
	if clause2.IsTopLevelScoringClause() {
		t.Error("clause2 should NOT be topLevelScoringClause for FILTER")
	}
}

// TestBooleanScorerSupplier_SingleShouldScoringClause tests single SHOULD as top-level scoring
// Source: testSingleShouldScoringClause()
func TestBooleanScorerSupplier_SingleShouldScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.SHOULD] = append(subs[search.SHOULD], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST_NOT] = append(subs[search.MUST_NOT], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	bss.SetTopLevelScoringClause()

	if !clause1.IsTopLevelScoringClause() {
		t.Error("clause1 SHOULD be topLevelScoringClause for single SHOULD")
	}
	if clause2.IsTopLevelScoringClause() {
		t.Error("clause2 should NOT be topLevelScoringClause for MUST_NOT")
	}
}

// TestBooleanScorerSupplier_MaxScoreNonTopLevelScoringClause tests max score calculation
// Source: testMaxScoreNonTopLevelScoringClause()
func TestBooleanScorerSupplier_MaxScoreNonTopLevelScoringClause(t *testing.T) {
	subs := make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	clause1 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause1)

	clause2 := NewFakeScorerSupplierWithLeadCost(10, 10)
	subs[search.MUST] = append(subs[search.MUST], clause2)

	bss := NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	scorer, err := bss.Get(10)
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}

	maxScore := scorer.(interface{ GetMaxScore(int) float32 }).GetMaxScore(search.NO_MORE_DOCS)
	if maxScore != 2.0 {
		t.Errorf("Expected max score 2.0, got %f", maxScore)
	}

	// Reset
	subs = make(map[search.Occur][]search.ScorerSupplier)
	for _, occur := range []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT, search.FILTER} {
		subs[occur] = make([]search.ScorerSupplier, 0)
	}

	subs[search.SHOULD] = append(subs[search.SHOULD], clause1)
	subs[search.SHOULD] = append(subs[search.SHOULD], clause2)

	bss = NewBooleanScorerSupplier(nil, subs, search.TOP_SCORES, 0, 100)
	scorer, err = bss.Get(10)
	if err != nil {
		t.Fatalf("Failed to get scorer: %v", err)
	}

	maxScore = scorer.(interface{ GetMaxScore(int) float32 }).GetMaxScore(search.NO_MORE_DOCS)
	if maxScore != 2.0 {
		t.Errorf("Expected max score 2.0, got %f", maxScore)
	}
}
