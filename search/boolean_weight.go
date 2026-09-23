// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

const booleanRewriteTermCountThreshold = 1024

type weightedBooleanClause struct {
	clause *BooleanClause
	weight Weight
}

// BooleanWeight is the weight for a BooleanQuery, used to normalize, score and explain these queries.
type BooleanWeight struct {
	BaseWeight
	similarity      Similarity
	query           *BooleanQuery
	weightedClauses []weightedBooleanClause
	scoreMode       ScoreMode
}

// NewBooleanWeight constructs a BooleanWeight.
func NewBooleanWeight(query *BooleanQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (*BooleanWeight, error) {
	bw := &BooleanWeight{
		BaseWeight: BaseWeight{query: query},
		query:      query,
		scoreMode:  scoreMode,
		similarity: searcher.GetSimilarity(),
	}

	for _, c := range query.Clauses() {
		mode := scoreMode
		if !c.IsScoring() {
			mode = COMPLETE_NO_SCORES
		}
		w, err := c.Query().CreateWeight(searcher, mode, boost)
		if err != nil {
			return nil, fmt.Errorf("failed to create weight for boolean clause: %w", err)
		}
		bw.weightedClauses = append(bw.weightedClauses, weightedBooleanClause{
			clause: c,
			weight: w,
		})
	}

	return bw, nil
}

func (bw *BooleanWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	minShouldMatch := bw.query.GetMinimumNumberShouldMatch()
	subs := make([]Explanation, 0)
	failingOptionals := make([]Explanation, 0)
	fail := false
	matchCount := 0
	shouldMatchCount := 0

	for _, wc := range bw.weightedClauses {
		w := wc.weight
		c := wc.clause
		e, err := w.Explain(ctx, doc)
		if err != nil {
			return nil, err
		}
		if e.IsMatch() {
			if c.IsScoring() {
				subs = append(subs, e)
			} else if c.IsRequired() {
				subs = append(subs, MatchExplanationWithDetails(
					0,
					"match on required clause, product of:",
					MatchExplanation(0, fmt.Sprintf("%s clause", c.Occur().String())),
					e,
				))
			} else if c.IsProhibited() {
				subs = append(subs, NoMatchExplanationWithDetails(
					fmt.Sprintf("match on prohibited clause (%s)", queryToString(c.Query(), "")),
					e,
				))
				fail = true
			}
			if !c.IsProhibited() {
				matchCount++
			}
			if c.Occur() == SHOULD {
				shouldMatchCount++
			}
		} else if c.IsRequired() {
			subs = append(subs, NoMatchExplanationWithDetails(
				fmt.Sprintf("no match on required clause (%s)", queryToString(c.Query(), "")),
				e,
			))
			fail = true
		} else if c.Occur() == SHOULD {
			failingOptionals = append(failingOptionals, NoMatchExplanationWithDetails(
				fmt.Sprintf("no match on optional clause (%s)", queryToString(c.Query(), "")),
				e,
			))
		}
	}

	if fail {
		return NoMatchExplanationWithDetails("Failure to meet condition(s) of required/prohibited clause(s)", subs...), nil
	} else if matchCount == 0 {
		subs = append(subs, failingOptionals...)
		return NoMatchExplanationWithDetails("No matching clauses", subs...), nil
	} else if shouldMatchCount < minShouldMatch {
		subs = append(subs, failingOptionals...)
		return NoMatchExplanationWithDetails(
			fmt.Sprintf("Failure to match minimum number of optional clauses: %d, matched: %d", minShouldMatch, shouldMatchCount),
			subs...,
		), nil
	} else {
		scorer, err := bw.Scorer(ctx)
		if err != nil {
			return nil, err
		}
		if scorer == nil {
			return NoMatchExplanation("no scorer available"), nil
		}
		iter := scorer.Iterator()
		advanced, err := iter.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced != doc {
			return NoMatchExplanation("doc not matched by scorer"), nil
		}
		sc0, err := scorer.Score()
		if err != nil {
			return nil, err
		}
		return MatchExplanationWithDetails(sc0, "sum of:", subs...), nil
	}
}

func (bw *BooleanWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	minShouldMatch := bw.query.GetMinimumNumberShouldMatch()
	matches := make([]Matches, 0)
	shouldMatchCount := 0

	for _, wc := range bw.weightedClauses {
		w := wc.weight
		bc := wc.clause
		m, err := w.Matches(ctx, doc)
		if err != nil {
			return nil, err
		}
		if bc.IsProhibited() {
			if m != nil {
				return nil, nil
			}
		}
		if bc.IsRequired() {
			if m == nil {
				return nil, nil
			}
			matches = append(matches, m)
		}
		if bc.Occur() == SHOULD {
			if m != nil {
				matches = append(matches, m)
				shouldMatchCount++
			}
		}
	}

	if shouldMatchCount < minShouldMatch {
		return nil, nil
	}
	return MatchesUtils.FromSubMatches(matches), nil
}

func (bw *BooleanWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	numDocs := ctx.Reader().NumDocs()
	positiveCount := -1

	if bw.query.IsPureDisjunction() {
		return bw.optCount(ctx, SHOULD)
	}

	// Simplified check for required clauses
	hasRequired := false
	for _, c := range bw.query.Clauses() {
		if c.IsRequired() {
			hasRequired = true
			break
		}
	}

	if (hasRequired || bw.query.Clauses() != nil) && bw.query.GetMinimumNumberShouldMatch() == 0 {
		positiveCount, _ = bw.reqCount(ctx)
	}

	if positiveCount == 0 {
		return 0, nil
	}

	prohibitedCount, err := bw.optCount(ctx, MUST_NOT)
	if err != nil {
		return -1, err
	}

	if prohibitedCount == -1 {
		return -1, nil
	} else if prohibitedCount == 0 {
		return positiveCount, nil
	} else if prohibitedCount == numDocs {
		return 0, nil
	} else if positiveCount == numDocs {
		return numDocs - prohibitedCount, nil
	} else {
		return -1, nil
	}
}

func (bw *BooleanWeight) reqCount(ctx *index.LeafReaderContext) (int, error) {
	numDocs := ctx.Reader().NumDocs()
	reqCount := numDocs

	for _, wc := range bw.weightedClauses {
		if !wc.clause.IsRequired() {
			continue
		}
		count, err := wc.weight.Count(ctx)
		if err != nil {
			return -1, err
		}
		if count == -1 || count == 0 {
			return count, nil
		} else if count == numDocs {
			// ignore
		} else if reqCount == numDocs {
			reqCount = count
		} else {
			return -1, nil
		}
	}
	return reqCount, nil
}

func (bw *BooleanWeight) optCount(ctx *index.LeafReaderContext, occur Occur) (int, error) {
	numDocs := ctx.Reader().NumDocs()
	optCount := 0
	unknownCount := false

	for _, wc := range bw.weightedClauses {
		if wc.clause.Occur() != occur {
			continue
		}
		count, err := wc.weight.Count(ctx)
		if err != nil {
			return -1, err
		}
		if count == -1 {
			unknownCount = true
			continue
		} else if count == numDocs {
			return count, nil
		} else if count == 0 {
			// ignore
		} else if optCount == 0 {
			optCount = count
		} else {
			unknownCount = true
		}
	}

	if unknownCount {
		return -1, nil
	}
	return optCount, nil
}

func (bw *BooleanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	if len(bw.query.Clauses()) > booleanRewriteTermCountThreshold {
		return false
	}
	for _, wc := range bw.weightedClauses {
		// Note: Weight interface doesn't have IsCacheable.
		// In Lucene, Weight is a class with this method.
		// We need to check if the concrete weight implements it.
		if cacheable, ok := wc.weight.(interface {
			IsCacheable(*index.LeafReaderContext) bool
		}); ok {
			if !cacheable.IsCacheable(ctx) {
				return false
			}
		}
	}
	return true
}

func (bw *BooleanWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	minShouldMatch := bw.query.GetMinimumNumberShouldMatch()

	scorers := make(map[Occur][]ScorerSupplier)
	scorers[MUST] = []ScorerSupplier{}
	scorers[SHOULD] = []ScorerSupplier{}
	scorers[MUST_NOT] = []ScorerSupplier{}
	scorers[FILTER] = []ScorerSupplier{}

	for _, wc := range bw.weightedClauses {
		w := wc.weight
		c := wc.clause
		subScorer, err := w.ScorerSupplier(ctx)
		if err != nil {
			return nil, err
		}
		if subScorer == nil {
			if c.IsRequired() {
				return nil, nil
			}
		} else {
			scorers[c.Occur()] = append(scorers[c.Occur()], subScorer)
		}
	}

	// scorer simplifications:
	if len(scorers[SHOULD]) == minShouldMatch {
		scorers[MUST] = append(scorers[MUST], scorers[SHOULD]...)
		scorers[SHOULD] = []ScorerSupplier{}
		minShouldMatch = 0
	}

	if len(scorers[FILTER]) == 0 &&
		len(scorers[MUST]) == 0 &&
		len(scorers[SHOULD]) == 0 {
		return nil, nil
	} else if len(scorers[SHOULD]) < minShouldMatch {
		return nil, nil
	}

	if bw.scoreMode.NeedsScores() == false &&
		minShouldMatch == 0 &&
		(len(scorers[MUST])+len(scorers[FILTER]) > 0) {
		scorers[SHOULD] = []ScorerSupplier{}
	}

	return NewBooleanScorerSupplier(bw, scorers, bw.scoreMode, minShouldMatch, ctx.Reader().MaxDoc()), nil
}

var _ Weight = (*BooleanWeight)(nil)

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (bw *BooleanWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	scorerSupplier, err := bw.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (bw *BooleanWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorerSupplier, err := bw.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
