// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// DisjunctionMaxWeight is the Weight for a DisjunctionMaxQuery. It builds the
// child weights through the searcher (so the ScoreMode flows down the query
// tree), combines their per-leaf ScorerSuppliers into a single supplier that
// constructs a DisjunctionMaxScorer, and explains a document as the max of the
// matching sub-explanations plus the tie-breaker times the remaining ones.
//
// It is a faithful port of DisjunctionMaxQuery.DisjunctionMaxWeight
// (Apache Lucene 10.4.0). The bulk-scorer specialisation Lucene applies for the
// tieBreaker==0 / TOP_SCORES case is intentionally omitted: Gocene's stock
// collectors do not request TOP_SCORES (see the ScoreMode notes), so the
// default DefaultBulkScorer over the DisjunctionMaxScorer is used, which is
// behaviourally equivalent for COMPLETE scoring.
type DisjunctionMaxWeight struct {
	*BaseWeight

	query                *DisjunctionMaxQuery
	weights              []Weight
	scoreMode            ScoreMode
	tieBreakerMultiplier float32
}

// NewDisjunctionMaxWeight constructs the Weight for query, recursively creating
// the sub-weight for each disjunct via the searcher.
func NewDisjunctionMaxWeight(searcher *IndexSearcher, query *DisjunctionMaxQuery, scoreMode ScoreMode, boost float32) (*DisjunctionMaxWeight, error) {
	weights := make([]Weight, 0, len(query.disjuncts))
	for _, disjunct := range query.disjuncts {
		w, err := searcher.CreateWeight(disjunct, scoreMode, boost)
		if err != nil {
			return nil, err
		}
		weights = append(weights, w)
	}
	return &DisjunctionMaxWeight{
		BaseWeight:           NewBaseWeight(query),
		query:                query,
		weights:              weights,
		scoreMode:            scoreMode,
		tieBreakerMultiplier: query.tieBreakerMultiplier,
	}, nil
}

// Matches consolidates the per-sub-weight Matches for doc into one Matches,
// mirroring DisjunctionMaxWeight.matches.
func (w *DisjunctionMaxWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	mis := make([]Matches, 0, len(w.weights))
	for _, weight := range w.weights {
		if weight == nil {
			continue
		}
		mi, err := weight.Matches(context, doc)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			mis = append(mis, mi)
		}
	}
	return FromSubMatches(mis), nil
}

// ScorerSupplier combines the non-nil sub-suppliers for the leaf. With zero it
// returns nil (no matches), with one it returns that supplier directly, and
// with more it returns a supplier that builds a DisjunctionMaxScorer.
func (w *DisjunctionMaxWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	suppliers := make([]ScorerSupplier, 0, len(w.weights))
	for _, weight := range w.weights {
		if weight == nil {
			continue
		}
		ss, err := weight.ScorerSupplier(context)
		if err != nil {
			return nil, err
		}
		if ss != nil {
			suppliers = append(suppliers, ss)
		}
	}
	switch len(suppliers) {
	case 0:
		return nil, nil
	case 1:
		return suppliers[0], nil
	default:
		return &disjunctionMaxScorerSupplier{
			suppliers:            suppliers,
			scoreMode:            w.scoreMode,
			tieBreakerMultiplier: w.tieBreakerMultiplier,
			cost:                 -1,
		}, nil
	}
}

// Scorer builds the leaf scorer via ScorerSupplier with an unconstrained
// leadCost, matching the inherited Weight.scorer default.
func (w *DisjunctionMaxWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if ss == nil {
		return nil, nil
	}
	return ss.Get(0)
}

// BulkScorer wraps the scorer in the default DefaultBulkScorer.
func (w *DisjunctionMaxWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true only if every sub-weight is cacheable for the leaf,
// mirroring DisjunctionMaxWeight.isCacheable (the term-count threshold guard is
// inert here because Gocene's QueryCache does not yet special-case dismax size).
func (w *DisjunctionMaxWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, weight := range w.weights {
		if weight == nil || !weight.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

// Explain produces the dismax explanation for doc: the max of the matching
// sub-explanation values plus tieBreaker times the sum of the rest, with the
// matching sub-explanations as details. A faithful port of
// DisjunctionMaxWeight.explain.
func (w *DisjunctionMaxWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	match := false
	var max, otherSum float64
	subsOnMatch := make([]Explanation, 0, len(w.weights))
	subsOnNoMatch := make([]Explanation, 0, len(w.weights))
	for _, weight := range w.weights {
		if weight == nil {
			continue
		}
		e, err := weight.Explain(context, doc)
		if err != nil {
			return nil, err
		}
		if e == nil {
			continue
		}
		if e.IsMatch() {
			match = true
			subsOnMatch = append(subsOnMatch, e)
			score := float64(e.GetValue())
			if score >= max {
				otherSum += max
				max = score
			} else {
				otherSum += score
			}
		} else if !match {
			subsOnNoMatch = append(subsOnNoMatch, e)
		}
	}
	if match {
		score := float32(max + otherSum*float64(w.tieBreakerMultiplier))
		desc := "max of:"
		if w.tieBreakerMultiplier != 0 {
			desc = "max plus " + formatFloat(w.tieBreakerMultiplier) + " times others of:"
		}
		return MatchExplanationWithDetails(score, desc, subsOnMatch...), nil
	}
	return NoMatchExplanationWithDetails("No matching clause", subsOnNoMatch...), nil
}

// disjunctionMaxScorerSupplier combines several sub-suppliers into a single
// supplier whose Get builds a DisjunctionMaxScorer over the sub-scorers.
type disjunctionMaxScorerSupplier struct {
	suppliers            []ScorerSupplier
	scoreMode            ScoreMode
	tieBreakerMultiplier float32
	cost                 int64
}

// Get builds each sub-scorer at the requested leadCost and wraps them in a
// DisjunctionMaxScorer.
func (s *disjunctionMaxScorerSupplier) Get(leadCost int64) (Scorer, error) {
	scorers := make([]Scorer, 0, len(s.suppliers))
	for _, ss := range s.suppliers {
		scorer, err := ss.Get(leadCost)
		if err != nil {
			return nil, err
		}
		scorers = append(scorers, scorer)
	}
	return NewDisjunctionMaxScorer(s.tieBreakerMultiplier, scorers, s.scoreMode, leadCost)
}

// Cost sums (and caches) the sub-supplier costs.
func (s *disjunctionMaxScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		var total int64
		for _, ss := range s.suppliers {
			total += ss.Cost()
		}
		s.cost = total
	}
	return s.cost
}

// SetTopLevelScoringClause propagates the top-level marker to sub-suppliers
// only when there is no tie-breaker, so they may prune via
// setMinCompetitiveScore. Mirrors the inner supplier's override.
func (s *disjunctionMaxScorerSupplier) SetTopLevelScoringClause() {
	if s.tieBreakerMultiplier == 0 {
		for _, ss := range s.suppliers {
			ss.SetTopLevelScoringClause()
		}
	}
}

// Ensure the weight satisfies the Weight contract.
var _ Weight = (*DisjunctionMaxWeight)(nil)
