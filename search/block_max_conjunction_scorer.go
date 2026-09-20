// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/BlockMaxConjunctionScorer.java

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockMaxConjunctionScorer is a Scorer for conjunctions that checks
// maximum scores of each clause to potentially skip over blocks that cannot
// have competitive matches.
//
// Mirrors org.apache.lucene.search.BlockMaxConjunctionScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Block-max pruning (the advance-target loop that skips blocks when
//     maxScore < minScore) is omitted; the scorer degrades to a standard
//     conjunction iterator sorted by cost.
//   - ScorerUtil.likelyTermScorer / likelyImpactsEnum are unported; scorers
//     are used directly, so Java's scorables array coincides with scorers.
//   - twoPhaseIterator() and the block-max approximation() are not ported: they
//     need org.apache.lucene.search.FilterDocIdSetIterator and ScorerUtil,
//     neither of which exists in Gocene yet. iterator() therefore exposes the
//     plain conjunction iterator held in disi instead of
//     `twoPhases.length == 0 ? approximation() : TwoPhaseIterator
//     .asDocIdSetIterator(twoPhaseIterator())`.
type BlockMaxConjunctionScorer struct {
	BaseDocIdSetIterator
	BaseScorer
	scorers []Scorer
	disi    DocIdSetIterator
	// minScore mirrors the Java field of the same name, set by
	// setMinCompetitiveScore and read by the block-max approximation.
	minScore float32
}

// NewBlockMaxConjunctionScorer builds a scorer from a slice of required clauses.
// Scorers are sorted by ascending iterator cost so the cheapest is used as lead.
func NewBlockMaxConjunctionScorer(scorers []Scorer) *BlockMaxConjunctionScorer {
	cp := make([]Scorer, len(scorers))
	copy(cp, scorers)
	// Mirrors `Arrays.sort(this.scorers, Comparator.comparingLong(s -> s.iterator().cost()));`.
	sort.SliceStable(cp, func(i, j int) bool {
		return cp[i].Iterator().Cost() < cp[j].Iterator().Cost()
	})
	return &BlockMaxConjunctionScorer{
		scorers: cp,
		disi:    IntersectScorers(cp),
	}
}

// Iterator returns the conjunction iterator over the required clauses.
//
// Java's BlockMaxConjunctionScorer.iterator() returns the block-max
// approximation (or its two-phase view); see the divergence note on the type.
func (s *BlockMaxConjunctionScorer) Iterator() DocIdSetIterator { return s.disi }

// DocID mirrors BlockMaxConjunctionScorer.docID(), whose body is
// `return scorers[0].docID();`.
func (s *BlockMaxConjunctionScorer) DocID() int { return s.scorers[0].DocID() }

// NextDoc advances to the next matching document.
func (s *BlockMaxConjunctionScorer) NextDoc() (int, error) { return s.disi.NextDoc() }

// Advance advances to the first matching document >= target.
func (s *BlockMaxConjunctionScorer) Advance(target int) (int, error) {
	return s.disi.Advance(target)
}

// Cost returns an estimate of the number of documents this scorer will visit.
func (s *BlockMaxConjunctionScorer) Cost() int64 { return s.disi.Cost() }

// Score returns the sum of clause scores for the current document.
func (s *BlockMaxConjunctionScorer) Score() (float32, error) {
	var sum float64
	for _, sc := range s.scorers {
		sc0, err := sc.Score()
		if err != nil {
			return 0, err
		}
		sum += float64(sc0)
	}
	return float32(sum), nil
}

// GetMaxScore returns the sum of per-clause maximum scores up to upTo.
func (s *BlockMaxConjunctionScorer) GetMaxScore(upTo int) (float32, error) {
	var sum float64
	for _, sc := range s.scorers {
		m, err := sc.GetMaxScore(upTo)
		if err != nil {
			return 0, err
		}
		sum += float64(m)
	}
	return float32(sum), nil
}

// AdvanceShallow mirrors BlockMaxConjunctionScorer.advanceShallow(int):
//
//	// We use block boundaries of the lead scorer.
//	int result = scorers[0].advanceShallow(target);
//	// But we still need to shallow-advance other clauses, in order to have
//	// better score upper bounds
//	for (int i = 1; i < scorers.length; ++i) {
//	  scorers[i].advanceShallow(target);
//	}
//	return result;
func (s *BlockMaxConjunctionScorer) AdvanceShallow(target int) (int, error) {
	result, err := s.scorers[0].AdvanceShallow(target)
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(s.scorers); i++ {
		if _, err := s.scorers[i].AdvanceShallow(target); err != nil {
			return 0, err
		}
	}
	return result, nil
}

// SetMinCompetitiveScore mirrors
// BlockMaxConjunctionScorer.setMinCompetitiveScore(float), whose body is
// `minScore = score;`.
func (s *BlockMaxConjunctionScorer) SetMinCompetitiveScore(score float32) error {
	s.minScore = score
	return nil
}

// GetChildren mirrors BlockMaxConjunctionScorer.getChildren():
//
//	ArrayList<ChildScorable> children = new ArrayList<>();
//	for (Scorer scorer : scorers) {
//	  children.add(new ChildScorable(scorer, "MUST"));
//	}
//	return children;
func (s *BlockMaxConjunctionScorer) GetChildren() ([]ChildScorable, error) {
	children := make([]ChildScorable, 0, len(s.scorers))
	for _, scorer := range s.scorers {
		children = append(children, ChildScorable{Child: scorer, Relationship: "MUST"})
	}
	return children, nil
}

// Compile-time guarantee.
var _ Scorer = (*BlockMaxConjunctionScorer)(nil)

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which BlockMaxConjunctionScorer inherits unchanged.
func (s *BlockMaxConjunctionScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}
