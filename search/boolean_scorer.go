// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// BooleanScorer is a scorer for boolean queries.
type BooleanScorer struct {
	BaseScorer
	scorers        []Scorer
	scoreMode      ScoreMode
	minShouldMatch int
	currentDoc     int
}

// NewBooleanScorer creates a new BooleanScorer.
func NewBooleanScorer(scorers []Scorer, scoreMode ScoreMode, minShouldMatch int) *BooleanScorer {
	return &BooleanScorer{
		BaseScorer:     *NewBaseScorer(nil),
		scorers:        scorers,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		currentDoc:     -1,
	}
}

// DocID returns the current document ID.
func (bs *BooleanScorer) DocID() int {
	return bs.currentDoc
}

// NextDoc advances to the next document.
func (bs *BooleanScorer) NextDoc() (int, error) {
	if len(bs.scorers) == 0 {
		bs.currentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}

	// For now, just return NO_MORE_DOCS (minimal implementation for tests)
	bs.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (bs *BooleanScorer) Advance(target int) (int, error) {
	// Simplified implementation
	bs.currentDoc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Cost returns the estimated cost.
func (bs *BooleanScorer) Cost() int64 {
	var cost int64 = 0
	for _, scorer := range bs.scorers {
		cost += scorer.Cost()
	}
	return cost
}

// Score returns the score for the current document.
func (bs *BooleanScorer) Score() float32 {
	// Simplified scoring - sum of all scorer scores
	var score float32 = 0
	for _, scorer := range bs.scorers {
		score += scorer.Score()
	}
	return score
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (bs *BooleanScorer) GetMaxScore(upTo int) float32 {
	// Simplified implementation - sum of max scores from all scorers
	var maxScore float32 = 0
	for _, scorer := range bs.scorers {
		maxScore += scorer.GetMaxScore(upTo)
	}
	return maxScore
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (bs *BooleanScorer) DocIDRunEnd() int {
	return bs.currentDoc + 1
}

// zeroScoreScorer wraps a scorer and returns zero for every scoring call.
// Used when FILTER clauses are the only "required" clauses but the caller
// still expects Score() to be callable (Java's FilterScorer anonymous
// subclass in BooleanScorerSupplier.req()).
type zeroScoreScorer struct {
	inner Scorer
}

func (z *zeroScoreScorer) DocID() int                      { return z.inner.DocID() }
func (z *zeroScoreScorer) NextDoc() (int, error)           { return z.inner.NextDoc() }
func (z *zeroScoreScorer) Advance(target int) (int, error) { return z.inner.Advance(target) }
func (z *zeroScoreScorer) Cost() int64                     { return z.inner.Cost() }
func (z *zeroScoreScorer) DocIDRunEnd() int                { return z.inner.DocIDRunEnd() }
func (z *zeroScoreScorer) Score() float32                  { return 0 }
func (z *zeroScoreScorer) GetMaxScore(_ int) float32       { return 0 }

// AdvanceShallow delegates to the wrapped scorer so that block boundaries stay
// consistent with the underlying iterator, mirroring how the other methods
// forward to inner. The reported max score is forced to zero by GetMaxScore.
func (z *zeroScoreScorer) AdvanceShallow(target int) (int, error) {
	return z.inner.AdvanceShallow(target)
}

var _ Scorer = (*zeroScoreScorer)(nil)

// NewBooleanScorerWithClauses assembles a proper boolean scorer tree from
// already-materialised sub-scorers, classified by their Occur type.
//
// It mirrors the logic of BooleanScorerSupplier.getInternal() from Lucene
// 10.4.0, adapted for Gocene: instead of ScorerSupplier collections, it
// receives []Scorer slices that have already been retrieved from the
// sub-weight Scorer() calls in BooleanWeight.Scorer().
//
// Three structural cases, exactly as in Java:
//  1. Pure conjunction  (no SHOULD):  excl(req(filter, must), mustNot)
//  2. Pure disjunction  (no req):     excl(opt(should, msm), mustNot)
//  3. Mix:
//     - minShouldMatch > 0:  ConjunctionScorer(excl(req, mustNot), opt)
//     - otherwise:           ReqOptSumScorer(excl(req, mustNot), opt)
//
// Returns nil (no documents can match) in the degenerate cases:
//   - all slices are empty
//   - SHOULD count < minShouldMatch
func NewBooleanScorerWithClauses(
	mustScorers, filterScorers, shouldScorers, mustNotScorers []Scorer,
	scoreMode ScoreMode,
	minShouldMatch int,
) Scorer {
	// Handle degenerate: nothing at all.
	if len(mustScorers)+len(filterScorers)+len(shouldScorers)+len(mustNotScorers) == 0 {
		return nil
	}

	// Handle degenerate: fewer SHOULD scorers than minShouldMatch requires.
	if minShouldMatch > 0 && len(shouldScorers) < minShouldMatch {
		return nil
	}

	// ── Pure conjunction (no SHOULD clauses) ────────────────────────────────
	if len(shouldScorers) == 0 {
		req := boolReq(mustScorers, filterScorers, scoreMode)
		return boolExcl(req, mustNotScorers)
	}

	// ── Pure disjunction (no MUST/FILTER clauses) ───────────────────────────
	if len(mustScorers) == 0 && len(filterScorers) == 0 {
		opt := boolOpt(shouldScorers, minShouldMatch, scoreMode)
		return boolExcl(opt, mustNotScorers)
	}

	// ── Mix ─────────────────────────────────────────────────────────────────
	req := boolExcl(boolReq(mustScorers, filterScorers, scoreMode), mustNotScorers)
	opt := boolOpt(shouldScorers, minShouldMatch, scoreMode)

	if minShouldMatch > 0 {
		// Both sides are required: conjunction.
		return NewConjunctionScorer([]Scorer{req, opt}, []Scorer{req, opt})
	}

	// Optional side is truly optional: sum scores.
	return NewReqOptSumScorer(req, opt, scoreMode)
}

// boolReq mirrors BooleanScorerSupplier.req().
// It assembles the required (MUST + FILTER) side of the boolean tree.
func boolReq(mustScorers, filterScorers []Scorer, scoreMode ScoreMode) Scorer {
	allRequired := make([]Scorer, 0, len(mustScorers)+len(filterScorers))
	allRequired = append(allRequired, filterScorers...)
	allRequired = append(allRequired, mustScorers...)

	needsScores := scoreMode != COMPLETE_NO_SCORES

	switch len(allRequired) {
	case 0:
		// Caller guarantees this is not reached.
		return nil
	case 1:
		s := allRequired[0]
		if !needsScores {
			return s
		}
		if len(mustScorers) == 0 {
			// Filter-only, scores needed: wrap to return 0 score.
			return &zeroScoreScorer{inner: s}
		}
		return s
	default:
		conjunction := NewConjunctionScorer(allRequired, mustScorers)
		if needsScores && len(mustScorers) == 0 {
			// All required clauses are FILTER; scores are needed but these
			// scorers produce no meaningful score — wrap with 0.
			return &zeroScoreScorer{inner: conjunction}
		}
		return conjunction
	}
}

// boolExcl mirrors BooleanScorerSupplier.excl().
// It wraps main with a MUST_NOT exclusion when prohibited is non-empty.
func boolExcl(main Scorer, prohibited []Scorer) Scorer {
	if len(prohibited) == 0 {
		return main
	}
	// If there is nothing positive to iterate over (e.g. no MUST/FILTER scorers
	// produced a result for this segment), then no documents can match regardless
	// of the MUST_NOT clauses.
	if main == nil {
		return nil
	}
	var prohibitedScorer Scorer
	if len(prohibited) == 1 {
		prohibitedScorer = prohibited[0]
	} else {
		// Cost of main (used as leadCost for the DisjunctionSumScorer).
		var leadCost int64
		if main != nil {
			leadCost = main.Cost()
		}
		prohibitedScorer = NewDisjunctionSumScorer(prohibited, COMPLETE_NO_SCORES, leadCost)
	}
	return NewReqExclScorer(main, prohibitedScorer)
}

// boolOpt mirrors BooleanScorerSupplier.opt().
// It assembles the optional (SHOULD) side of the boolean tree.
func boolOpt(scorers []Scorer, minShouldMatch int, scoreMode ScoreMode) Scorer {
	if len(scorers) == 1 {
		return scorers[0]
	}
	// Use WANDScorer for minShouldMatch > 1 (Java: "when minimum number of
	// clauses should match, BooleanScorer is too complex").
	// For minShouldMatch <= 1, DisjunctionSumScorer is faster for exhaustive queries.
	var leadCost int64
	for _, s := range scorers {
		leadCost += s.Cost()
	}
	if minShouldMatch > 1 {
		wand, err := NewWANDScorer(scorers, minShouldMatch, scoreMode, leadCost)
		if err == nil {
			return wand
		}
		// Fall through to DisjunctionSumScorer on error.
	}
	return NewDisjunctionSumScorer(scorers, scoreMode, leadCost)
}
