// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BooleanWeight is the Weight implementation for BooleanQuery.
// This is the Go port of Lucene's org.apache.lucene.search.BooleanWeight.
type BooleanWeight struct {
	*BaseWeight
	query         *BooleanQuery
	searcher      *IndexSearcher
	needsScores   bool
	weights       []Weight
	scorerEnabled []bool
}

// NewBooleanWeight creates a new BooleanWeight.
func NewBooleanWeight(query *BooleanQuery, searcher *IndexSearcher, needsScores bool) (*BooleanWeight, error) {
	w := &BooleanWeight{
		BaseWeight:    NewBaseWeight(query),
		query:         query,
		searcher:      searcher,
		needsScores:   needsScores,
		weights:       make([]Weight, len(query.clauses)),
		scorerEnabled: make([]bool, len(query.clauses)),
	}

	// Create weights for each clause
	for i, clause := range query.clauses {
		// For FILTER clauses, we don't need scores
		clauseNeedsScores := needsScores && clause.Occur != FILTER
		weight, err := clause.Query.CreateWeight(searcher, clauseNeedsScores, 1.0)
		if err != nil {
			return nil, err
		}
		w.weights[i] = weight
		w.scorerEnabled[i] = clauseNeedsScores
	}

	return w, nil
}

// Scorer creates a scorer for this weight.
func (w *BooleanWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	var mustScorers []Scorer
	var filterScorers []Scorer
	var shouldScorers []Scorer
	var mustNotScorers []Scorer

	for i, weight := range w.weights {
		if weight == nil {
			continue
		}
		clause := w.query.clauses[i]
		scorer, err := weight.Scorer(context)
		if err != nil {
			return nil, err
		}
		if scorer == nil {
			// A nil scorer for MUST or FILTER means no documents can match.
			if clause.Occur == MUST || clause.Occur == FILTER {
				return nil, nil
			}
			continue
		}
		switch clause.Occur {
		case MUST:
			mustScorers = append(mustScorers, scorer)
		case FILTER:
			filterScorers = append(filterScorers, scorer)
		case SHOULD:
			shouldScorers = append(shouldScorers, scorer)
		case MUST_NOT:
			mustNotScorers = append(mustNotScorers, scorer)
		}
	}

	scoreMode := COMPLETE_NO_SCORES
	if w.needsScores {
		scoreMode = COMPLETE
	}

	return NewBooleanScorerWithClauses(
		mustScorers, filterScorers, shouldScorers, mustNotScorers,
		scoreMode, w.query.minShouldMatch,
	), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
//
// The returned supplier defers scorer construction so that
// SetTopLevelScoringClause can route a top-level, score-needing SHOULD-only
// disjunction to a WANDScorer (enabling block-max SetMinCompetitiveScore early
// termination), mirroring BooleanScorerSupplier in Lucene 10.4.0. All other
// shapes fall back to the eager BooleanScorer built by Scorer.
func (w *BooleanWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	// A nil scorer (no possible matches) yields a nil supplier, matching
	// Lucene's BooleanWeight.scorerSupplier returning null.
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return &booleanWeightScorerSupplier{weight: w, context: context}, nil
}

// isPureShouldDisjunction reports whether this query is a SHOULD-only
// disjunction with minShouldMatch <= 1, the shape Lucene routes to a WANDScorer
// when scores are needed and the supplier is the top-level scoring clause.
func (w *BooleanWeight) isPureShouldDisjunction() bool {
	if !w.needsScores || w.query.minShouldMatch > 1 {
		return false
	}
	hasShould := false
	for _, clause := range w.query.clauses {
		switch clause.Occur {
		case SHOULD:
			hasShould = true
		default:
			return false
		}
	}
	return hasShould
}

// wandScorer builds a WANDScorer over the SHOULD sub-scorers in TOP_SCORES mode
// when the query is a pure SHOULD disjunction. It returns (scorer, true, nil)
// on success, (nil, false, nil) when the shape does not qualify or a required
// SHOULD scorer is missing, and (nil, false, err) on error.
//
// Mirrors the WANDScorer construction in
// BooleanScorerSupplier.getInternal()/optionalBulkScorer() (Lucene 10.4.0):
// minShouldMatch is clamped to at least 1 (at least one clause must match) and
// the disjunction is scored under ScoreMode.TOP_SCORES.
func (w *BooleanWeight) wandScorer(context *index.LeafReaderContext, leadCost int64) (Scorer, bool, error) {
	if !w.isPureShouldDisjunction() {
		return nil, false, nil
	}

	var shouldScorers []Scorer
	for i, weight := range w.weights {
		if weight == nil {
			continue
		}
		if w.query.clauses[i].Occur != SHOULD {
			continue
		}
		scorer, err := weight.Scorer(context)
		if err != nil {
			return nil, false, err
		}
		if scorer == nil {
			continue
		}
		shouldScorers = append(shouldScorers, scorer)
	}

	// WANDScorer needs at least minShouldMatch+1 scorers; with a single
	// matching clause it cannot run, so fall back to the standard scorer.
	if len(shouldScorers) < 2 {
		return nil, false, nil
	}

	minShouldMatch := w.query.minShouldMatch
	if minShouldMatch < 1 {
		minShouldMatch = 1
	}

	ws, err := NewWANDScorer(shouldScorers, minShouldMatch, TOP_SCORES, leadCost)
	if err != nil {
		return nil, false, err
	}
	return ws, true, nil
}

// Explain returns an explanation of the score for the given document.
//
// It ports org.apache.lucene.search.BooleanWeight.explain: each clause's
// sub-weight is explained for doc and combined according to its occur type —
// matching scoring clauses (SHOULD/MUST) contribute their explanation, a
// matching FILTER clause is wrapped as a zero-valued required match, a matching
// MUST_NOT clause forces a non-match, and a non-matching required clause
// (MUST/FILTER) also forces a non-match. The minimum-should-match constraint is
// then enforced. On success the final value is taken from a live Scorer rather
// than summed from the sub-explanations, exactly as Lucene does, so the
// explained value matches the scored value despite intermediate float casts.
func (w *BooleanWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	minShouldMatch := w.query.minShouldMatch

	subs := make([]Explanation, 0, len(w.weights))
	fail := false
	matchCount := 0
	shouldMatchCount := 0

	for i, weight := range w.weights {
		if weight == nil {
			continue
		}
		clause := w.query.clauses[i]
		e, err := weight.Explain(context, doc)
		if err != nil {
			return nil, err
		}
		if e == nil {
			e = NoMatchExplanation("no match")
		}

		isScoring := clause.Occur == SHOULD || clause.Occur == MUST
		isRequired := clause.Occur == MUST || clause.Occur == FILTER
		isProhibited := clause.Occur == MUST_NOT

		if e.IsMatch() {
			switch {
			case isScoring:
				subs = append(subs, e)
			case isRequired:
				wrap := MatchExplanation(0, "match on required clause, product of:")
				wrap.AddDetail(MatchExplanation(0, FILTER.String()+" clause"))
				wrap.AddDetail(e)
				subs = append(subs, wrap)
			case isProhibited:
				prohibited := NoMatchExplanation(
					fmt.Sprintf("match on prohibited clause (%s)", clause.Query))
				prohibited.AddDetail(e)
				subs = append(subs, prohibited)
				fail = true
			}
			if !isProhibited {
				matchCount++
			}
			if clause.Occur == SHOULD {
				shouldMatchCount++
			}
		} else if isRequired {
			noMatch := NoMatchExplanation(
				fmt.Sprintf("no match on required clause (%s)", clause.Query))
			noMatch.AddDetail(e)
			subs = append(subs, noMatch)
			fail = true
		}
	}

	switch {
	case fail:
		result := NoMatchExplanation(
			"Failure to meet condition(s) of required/prohibited clause(s)")
		for _, s := range subs {
			result.AddDetail(s)
		}
		return result, nil
	case matchCount == 0:
		result := NoMatchExplanation("No matching clauses")
		for _, s := range subs {
			result.AddDetail(s)
		}
		return result, nil
	case shouldMatchCount < minShouldMatch:
		result := NoMatchExplanation(
			fmt.Sprintf("Failure to match minimum number of optional clauses: %d", minShouldMatch))
		for _, s := range subs {
			result.AddDetail(s)
		}
		return result, nil
	default:
		// Pull a Scorer and use it to compute the score so the explained value
		// matches the scored value (Lucene replicates the same float casts via
		// the scorer rather than re-summing the sub-explanations).
		matched, score, err := scorerMatch(w, context, doc)
		if err != nil {
			return nil, err
		}
		if !matched {
			// Should not happen: the clause analysis above already established a
			// match. Fall back to the summed contributions to remain robust.
			var sum float32
			for _, s := range subs {
				if s.IsMatch() {
					sum += s.GetValue()
				}
			}
			score = sum
		}
		result := MatchExplanation(score, "sum of:")
		for _, s := range subs {
			result.AddDetail(s)
		}
		return result, nil
	}
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *BooleanWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *BooleanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, weight := range w.weights {
		if weight != nil && !weight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *BooleanWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *BooleanWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure BooleanWeight implements Weight
var _ Weight = (*BooleanWeight)(nil)
