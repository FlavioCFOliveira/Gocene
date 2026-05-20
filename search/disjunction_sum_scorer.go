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
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionSumScorer.java

import "github.com/FlavioCFOliveira/Gocene/util"

// DisjunctionSumScorer is a Scorer for OR-like queries that sums the
// scores of all matching sub-scorers for each document. It is the
// counterpart of ConjunctionScorer.
//
// Mirrors org.apache.lucene.search.DisjunctionSumScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Extends DisjunctionScorer (Go composition) rather than inheriting
//     from it via Java class hierarchy.
//   - Score() satisfies Scorer.Score() float32 by calling ScoreWithError
//     and silently dropping errors; errors are accessible via
//     ScoreWithError().
//   - GetMaxScore uses util.MathSumUpperBound from the Gocene util
//     package (equivalent to org.apache.lucene.util.MathUtil.sumUpperBound).
type DisjunctionSumScorer struct {
	DisjunctionScorer
	scorers []Scorer
}

// NewDisjunctionSumScorer constructs a DisjunctionSumScorer for the
// given sub-scorers. At least two sub-scorers are required.
//
// Mirrors DisjunctionSumScorer(List<Scorer>, ScoreMode, long).
func NewDisjunctionSumScorer(subScorers []Scorer, scoreMode ScoreMode, leadCost int64) *DisjunctionSumScorer {
	base := newDisjunctionScorer(subScorers, scoreMode, leadCost)
	s := &DisjunctionSumScorer{
		DisjunctionScorer: *base,
		scorers:           subScorers,
	}
	return s
}

// scoreTopList sums the scores of all DisiWrappers in topList.
//
// Mirrors DisjunctionSumScorer.score(DisiWrapper).
func (s *DisjunctionSumScorer) scoreTopList(topList *DisiWrapper) (float32, error) {
	var sum float64
	for w := topList; w != nil; w = w.next {
		sum += float64(w.scorable.Score())
	}
	return float32(sum), nil
}

// Score returns the sum of scores of all matching sub-scorers.
func (s *DisjunctionSumScorer) Score() float32 {
	topList, err := s.DisjunctionScorer.getSubMatches()
	if err != nil {
		return 0
	}
	v, _ := s.scoreTopList(topList)
	return v
}

// GetMaxScore returns an upper bound on the score for documents up to
// upTo, computed as the sum-upper-bound of individual sub-scorer maxima.
//
// Mirrors DisjunctionSumScorer.getMaxScore(int).
func (s *DisjunctionSumScorer) GetMaxScore(upTo int) float32 {
	var maxScore float64
	for _, sc := range s.scorers {
		if sc.DocID() <= upTo {
			maxScore += float64(sc.GetMaxScore(upTo))
		}
	}
	return float32(util.MathSumUpperBound(maxScore, len(s.scorers)))
}

// Compile-time check: DisjunctionSumScorer satisfies Scorer.
var _ Scorer = (*DisjunctionSumScorer)(nil)
