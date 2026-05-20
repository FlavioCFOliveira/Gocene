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

import "sort"

// BlockMaxConjunctionScorer is a Scorer for conjunctions that checks
// maximum scores of each clause to potentially skip over blocks that cannot
// have competitive matches.
//
// Mirrors org.apache.lucene.search.BlockMaxConjunctionScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - advanceShallow / setMinCompetitiveScore / twoPhaseIterator are not on
//     Gocene's Scorer interface. Block-max pruning (the advance-target loop
//     that skips blocks when maxScore < minScore) is therefore omitted; the
//     scorer degrades to a standard conjunction iterator sorted by cost.
//   - ScorerUtil.likelyTermScorer / likelyImpactsEnum are unported; scorers
//     are used directly.
//   - GetChildren returns a slice of Scorer values (Gocene has no ChildScorable).
type BlockMaxConjunctionScorer struct {
	BaseDocIdSetIterator
	BaseScorer
	scorers []Scorer
	disi    DocIdSetIterator
}

// NewBlockMaxConjunctionScorer builds a scorer from a slice of required clauses.
// Scorers are sorted by ascending iterator cost so the cheapest is used as lead.
func NewBlockMaxConjunctionScorer(scorers []Scorer) *BlockMaxConjunctionScorer {
	cp := make([]Scorer, len(scorers))
	copy(cp, scorers)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Cost() < cp[j].Cost()
	})
	return &BlockMaxConjunctionScorer{
		scorers: cp,
		disi:    IntersectScorers(cp),
	}
}

// DocID returns the current document ID.
func (s *BlockMaxConjunctionScorer) DocID() int { return s.disi.DocID() }

// NextDoc advances to the next matching document.
func (s *BlockMaxConjunctionScorer) NextDoc() (int, error) { return s.disi.NextDoc() }

// Advance advances to the first matching document >= target.
func (s *BlockMaxConjunctionScorer) Advance(target int) (int, error) {
	return s.disi.Advance(target)
}

// Cost returns an estimate of the number of documents this scorer will visit.
func (s *BlockMaxConjunctionScorer) Cost() int64 { return s.disi.Cost() }

// Score returns the sum of clause scores for the current document.
func (s *BlockMaxConjunctionScorer) Score() float32 {
	var sum float64
	for _, sc := range s.scorers {
		sum += float64(sc.Score())
	}
	return float32(sum)
}

// GetMaxScore returns the sum of per-clause maximum scores up to upTo.
func (s *BlockMaxConjunctionScorer) GetMaxScore(upTo int) float32 {
	var sum float64
	for _, sc := range s.scorers {
		sum += float64(sc.GetMaxScore(upTo))
	}
	return float32(sum)
}

// GetChildren returns the constituent scorers (mirrors getChildren / ChildScorable).
func (s *BlockMaxConjunctionScorer) GetChildren() []Scorer {
	return s.scorers
}

// Compile-time guarantee.
var _ Scorer = (*BlockMaxConjunctionScorer)(nil)
