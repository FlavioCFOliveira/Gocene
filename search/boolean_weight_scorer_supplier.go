// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/BooleanScorerSupplier.java
//   (the WANDScorer routing for top-level SHOULD disjunctions under TOP_SCORES)

import "github.com/FlavioCFOliveira/Gocene/index"

// booleanWeightScorerSupplier is the ScorerSupplier returned by
// BooleanWeight.ScorerSupplier. It defers scorer construction so that
// SetTopLevelScoringClause can change the produced scorer: a top-level,
// score-needing, SHOULD-only disjunction (minShouldMatch <= 1) is routed to a
// WANDScorer, which supports block-max SetMinCompetitiveScore early
// termination, mirroring BooleanScorerSupplier.get/getInternal in Lucene
// 10.4.0. Every other shape falls back to the eager BooleanScorer built by
// BooleanWeight.Scorer.
//
// Deviation from Java: Lucene's BooleanScorerSupplier holds per-occur
// ScorerSupplier maps and computes a cost-based plan. Gocene's BooleanWeight
// builds sub-scorers eagerly via Weight.Scorer, so this supplier wraps that
// path and only adds the WANDScorer routing required for TOP_SCORES disjunction
// early termination; the non-WAND path is byte-for-byte the previous behaviour.
type booleanWeightScorerSupplier struct {
	weight   *BooleanWeight
	context  *index.LeafReaderContext
	topLevel bool
}

// Get returns a Scorer for the given leadCost. When this supplier is the
// top-level scoring clause and the query is a score-needing SHOULD-only
// disjunction, a WANDScorer is produced; otherwise the standard BooleanScorer
// is returned.
func (s *booleanWeightScorerSupplier) Get(leadCost int64) (Scorer, error) {
	if s.topLevel {
		if wand, ok, err := s.weight.wandScorer(s.context, leadCost); err != nil {
			return nil, err
		} else if ok {
			return wand, nil
		}
	}
	return s.weight.Scorer(s.context)
}

// Cost returns an estimate of the number of matching documents. It reuses the
// query cost computed by the standard scorer path.
func (s *booleanWeightScorerSupplier) Cost() int64 {
	scorer, err := s.weight.Scorer(s.context)
	if err != nil || scorer == nil {
		return 0
	}
	return scorer.Cost()
}

// SetTopLevelScoringClause marks this supplier as the top-level scoring clause,
// enabling WANDScorer routing on the next Get. Mirrors
// BooleanScorerSupplier.setTopLevelScoringClause().
func (s *booleanWeightScorerSupplier) SetTopLevelScoringClause() {
	s.topLevel = true
}

var _ ScorerSupplier = (*booleanWeightScorerSupplier)(nil)
