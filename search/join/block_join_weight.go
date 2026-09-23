// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ToParentBlockJoinWeight is the Weight implementation for ToParentBlockJoinQuery.
// It handles the parent document scoring based on matching child documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.ToParentBlockJoinWeight.
type ToParentBlockJoinWeight struct {
	// query is the parent ToParentBlockJoinQuery
	query *ToParentBlockJoinQuery

	// childWeight is the weight of the child query
	childWeight search.Weight

	// parentsFilter identifies parent documents
	parentsFilter BitSetProducer

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode

	// boost is the query boost
	boost float32
}

// NewToParentBlockJoinWeight creates a new ToParentBlockJoinWeight.
func NewToParentBlockJoinWeight(query *ToParentBlockJoinQuery, childWeight search.Weight, parentsFilter BitSetProducer, scoreMode ScoreMode, boost float32) *ToParentBlockJoinWeight {
	return &ToParentBlockJoinWeight{
		query:         query,
		childWeight:   childWeight,
		parentsFilter: parentsFilter,
		scoreMode:     scoreMode,
		boost:         boost,
	}
}

// GetQuery returns the parent query.
func (w *ToParentBlockJoinWeight) GetQuery() search.Query {
	return w.query
}

// GetChildWeight returns the child weight.
func (w *ToParentBlockJoinWeight) GetChildWeight() search.Weight {
	return w.childWeight
}

// GetParentsFilter returns the parents filter.
func (w *ToParentBlockJoinWeight) GetParentsFilter() BitSetProducer {
	return w.parentsFilter
}

// GetScoreMode returns the score mode.
func (w *ToParentBlockJoinWeight) GetScoreMode() ScoreMode {
	return w.scoreMode
}

// Scorer creates a scorer for this weight.
func (w *ToParentBlockJoinWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	// A nil child weight means the child query produced no runnable weight,
	// hence no child (and therefore no parent) matches. Mirrors the "no matches"
	// short-circuit in Lucene's BlockJoinWeight.scorerSupplier, which returns
	// null when the child scorer supplier is null.
	if w.childWeight == nil {
		return nil, nil
	}

	// Get the parents BitSet for this context
	parentsBits, err := w.parentsFilter.GetBitSet(context)
	if err != nil {
		return nil, fmt.Errorf("failed to get parents bitset: %w", err)
	}

	if parentsBits == nil {
		return nil, nil
	}

	// Create the child scorer
	childScorer, err := w.childWeight.Scorer(context)
	if err != nil {
		return nil, fmt.Errorf("failed to create child scorer: %w", err)
	}

	if childScorer == nil {
		return nil, nil
	}

	// ScoreMode.None: Lucene wraps the child query in a ConstantScoreQuery with
	// boost 0 created in the top-level scoreMode, so the child produces score 0
	// and — under TOP_SCORES — can early-terminate once the minimum competitive
	// score exceeds 0. We reproduce that here by wrapping the child scorer's
	// iterator in a TOP_SCORES ConstantScoreScorer(score=0). This keeps the
	// None-mode parent score at 0 (the block-join scorer already returns 0 for
	// None) while enabling SetMinCompetitiveScore to skip non-competitive parents
	// (Lucene ToParentBlockJoinQuery.createWeight, ScoreMode.None branch).
	if w.scoreMode == None {
		childScorer = search.NewConstantScoreScorer(0, search.TOP_SCORES, childScorer.Iterator())
	}

	// Create and return the ToParentBlockJoinScorer
	return NewToParentBlockJoinScorer(w, childScorer, parentsBits, w.scoreMode, w.boost), nil
}

// ScorerSupplier creates a ScorerSupplier for this weight.
//
// Faithful port of ToParentBlockJoinQuery.ToParentBlockJoinWeight.scorerSupplier:
// the supplier defers child-scorer creation and forwards
// SetTopLevelScoringClause to the child supplier only when scoreMode is Max, so
// a TOP_SCORES + Max block join can route its SHOULD-disjunction child to a
// WANDScorer for block-max SetMinCompetitiveScore early termination.
func (w *ToParentBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	if w.childWeight == nil {
		return nil, nil
	}

	childSupplier, err := w.childWeight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if childSupplier == nil {
		return nil, nil
	}

	parentsBits, err := w.parentsFilter.GetBitSet(context)
	if err != nil {
		return nil, fmt.Errorf("failed to get parents bitset: %w", err)
	}
	if parentsBits == nil {
		return nil, nil
	}

	return &toParentBlockJoinScorerSupplier{
		weight:        w,
		childSupplier: childSupplier,
		parentsBits:   parentsBits,
	}, nil
}

// toParentBlockJoinScorerSupplier is the ScorerSupplier for
// ToParentBlockJoinWeight. It mirrors the anonymous ScorerSupplier returned by
// ToParentBlockJoinQuery.ToParentBlockJoinWeight.scorerSupplier in Lucene
// 10.4.0.
type toParentBlockJoinScorerSupplier struct {
	weight        *ToParentBlockJoinWeight
	childSupplier search.ScorerSupplier
	parentsBits   util.BitSet
}

// Get builds the block-join scorer over the child scorer obtained for leadCost.
// For ScoreMode.None the child is wrapped in a TOP_SCORES ConstantScoreScorer
// (score 0) so SetMinCompetitiveScore can skip non-competitive parents, exactly
// as ToParentBlockJoinWeight.scorer does.
func (s *toParentBlockJoinScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	childScorer, err := s.childSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	if childScorer == nil {
		return nil, nil
	}
	if s.weight.scoreMode == None {
		childScorer = search.NewConstantScoreScorer(0, search.TOP_SCORES, childScorer.Iterator())
	}
	return NewToParentBlockJoinScorer(s.weight, childScorer, s.parentsBits, s.weight.scoreMode, s.weight.boost), nil
}

// Cost delegates to the child supplier's cost estimate.
func (s *toParentBlockJoinScorerSupplier) Cost() int64 {
	return s.childSupplier.Cost()
}

// SetTopLevelScoringClause forwards to the child supplier only for ScoreMode.Max,
// mirroring the setTopLevelScoringClause override in Lucene's
// ToParentBlockJoinWeight.scorerSupplier (where None/Avg/Min/Total do not
// forward because they cannot early-terminate on a per-child threshold).
func (s *toParentBlockJoinScorerSupplier) SetTopLevelScoringClause() error {
	if s.weight.scoreMode == Max {
		return s.childSupplier.SetTopLevelScoringClause()
	}
	return nil
}

// BulkScorer returns the bulk-scoring path for this supplier.
//
// Faithful port of the bulkScorer() override in
// ToParentBlockJoinWeight.scorerSupplier: ScoreMode.None falls back to the
// default bulk scorer (which drives the BlockJoinScorer one child per parent),
// while every other mode returns a BlockJoinBulkScorer over the child
// supplier's bulk scorer, evaluating all child hits per parent exhaustively.
func (s *toParentBlockJoinScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	if s.weight.scoreMode == None {
		scorer, err := s.Get(int64(1)<<62 - 1)
		if err != nil {
			return nil, err
		}
		if scorer == nil {
			return nil, nil
		}
		return search.NewDefaultBulkScorer(scorer), nil
	}

	childBulkScorer, err := supplierBulkScorer(s.childSupplier)
	if err != nil {
		return nil, err
	}
	if childBulkScorer == nil {
		return nil, nil
	}
	return NewBlockJoinBulkScorer(childBulkScorer, s.parentsBits, s.weight.scoreMode), nil
}

// supplierBulkScorer returns a BulkScorer from a ScorerSupplier, preferring the
// supplier's own BulkScorer() when it exposes one (mirroring
// ScorerSupplier.bulkScorer()), and otherwise wrapping its Scorer in a
// DefaultBulkScorer (the Lucene ScorerSupplier.bulkScorer() default).
func supplierBulkScorer(supplier search.ScorerSupplier) (search.BulkScorer, error) {
	if bsp, ok := supplier.(interface {
		BulkScorer() (search.BulkScorer, error)
	}); ok {
		return bsp.BulkScorer()
	}
	scorer, err := supplier.Get(int64(1)<<62 - 1)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

var _ search.ScorerSupplier = (*toParentBlockJoinScorerSupplier)(nil)

// Explain returns an explanation of the score for the given document.
func (w *ToParentBlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}

	if scorer == nil {
		return search.NewExplanation(false, 0, "no matching documents"), nil
	}

	actualDoc, err := scorer.Iterator().Advance(doc)
	if err != nil {
		return nil, err
	}

	if actualDoc != doc {
		return search.NewExplanation(false, 0, fmt.Sprintf("document %d does not match", doc)), nil
	}

	score, err := scorer.Score()
	if err != nil {
		return nil, err
	}
	return search.NewExplanation(true, score, fmt.Sprintf("ToParentBlockJoinQuery, score mode: %s", w.scoreMode)), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
//
// It delegates to the ScorerSupplier so that ScoreMode != None uses the
// exhaustive BlockJoinBulkScorer, matching Lucene's
// ToParentBlockJoinWeight.scorerSupplier().bulkScorer().
func (w *ToParentBlockJoinWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.(*toParentBlockJoinScorerSupplier).BulkScorer()
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *ToParentBlockJoinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return false
}

// Count returns the count of matching documents in sub-linear time.
func (w *ToParentBlockJoinWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
//
// Mirrors ToParentBlockJoinWeight.matches(LeafReaderContext, int): the default
// Weight implementation would delegate to the join query's weight, which
// matches on children, so the parent scorer is advanced here instead and a
// bare MATCH_WITH_NO_TERMS is reported.
func (w *ToParentBlockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}

	if scorer == nil {
		return nil, nil
	}

	actualDoc, err := scorer.Iterator().Advance(doc)
	if err != nil {
		return nil, err
	}

	if actualDoc != doc {
		return nil, nil
	}

	return search.MatchWithNoTerms, nil
}

// Ensure ToParentBlockJoinWeight implements Weight
var _ search.Weight = (*ToParentBlockJoinWeight)(nil)
