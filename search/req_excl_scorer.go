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
//   lucene/core/src/java/org/apache/lucene/search/ReqExclScorer.java

// advanceCostEstimate is an estimate of the cost to call Advance once.
// Mirrors ADVANCE_COST in Java.
const advanceCostEstimate = 10

// ReqExclScorer is a Scorer for queries with a required sub-scorer and an
// excluding (prohibited) sub-scorer. It yields documents that match the
// required scorer but do not match the excluded scorer.
//
// Mirrors org.apache.lucene.search.ReqExclScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java's Scorer.advanceShallow / setMinCompetitiveScore are not on Gocene's
//     Scorer interface; those delegation paths are omitted.
//   - getChildren() / ChildScorable delegation is omitted because Gocene's
//     Scorable.GetChildren returns []ChildScorable with Child Scorable, but
//     Scorer.Score returns float32 while Scorable.Score returns (float32,error),
//     making Scorer incompatible with Scorable — same limitation as DisjunctionScorer.
//   - TwoPhaseIterator() is exposed via the scorerTwoPhaseProvider optional interface
//     so callers that know about it can retrieve it.
//   - The two-phase-based DISI is cached on construction (tpDisi) so NextDoc/Advance
//     do not allocate on each call.
type ReqExclScorer struct {
	BaseScorer
	reqScorer Scorer
	// approximations (or the scorers themselves if they don't support two-phase)
	reqApproximation  DocIdSetIterator
	exclApproximation DocIdSetIterator
	// two-phase views, or nil
	reqTwoPhase  *TwoPhaseIterator
	exclTwoPhase *TwoPhaseIterator
	// cached two-phase DISI (mirrors Java's iterator() return value)
	tpDisi DocIdSetIterator
}

// NewReqExclScorer constructs a ReqExclScorer.
//
// Mirrors ReqExclScorer(Scorer, Scorer).
func NewReqExclScorer(reqScorer, exclScorer Scorer) *ReqExclScorer {
	s := &ReqExclScorer{reqScorer: reqScorer}

	if rp, ok := reqScorer.(scorerTwoPhaseProvider); ok {
		s.reqTwoPhase = rp.TwoPhaseIterator()
	}
	if s.reqTwoPhase == nil {
		s.reqApproximation = reqScorer
	} else {
		s.reqApproximation = s.reqTwoPhase.Approximation()
	}

	if ep, ok := exclScorer.(scorerTwoPhaseProvider); ok {
		s.exclTwoPhase = ep.TwoPhaseIterator()
	}
	if s.exclTwoPhase == nil {
		s.exclApproximation = exclScorer
	} else {
		s.exclApproximation = s.exclTwoPhase.Approximation()
	}

	// Pre-build the TwoPhaseIterator and cache its DISI so NextDoc/Advance
	// don't allocate on each call.
	s.tpDisi = NewTwoPhaseIteratorAsDocIdSetIterator(s.TwoPhaseIterator())

	return s
}

// matchesOrNil reports whether the TwoPhaseIterator matches the current doc,
// returning true when tpi is nil (meaning no two-phase check needed).
//
// Mirrors the static matchesOrNull in Java.
func matchesOrNil(tpi *TwoPhaseIterator) (bool, error) {
	if tpi == nil {
		return true, nil
	}
	return tpi.Matches()
}

// reqExclMatchCost computes the match cost for the TwoPhaseIterator.
func reqExclMatchCost(
	reqApprox DocIdSetIterator, reqTwoPhase *TwoPhaseIterator,
	exclApprox DocIdSetIterator, exclTwoPhase *TwoPhaseIterator,
) float32 {
	matchCost := float32(2) // 2 comparisons to advance exclApproximation
	if reqTwoPhase != nil {
		matchCost += reqTwoPhase.MatchCost()
	}

	exclMatchCost := float32(advanceCostEstimate)
	if exclTwoPhase != nil {
		exclMatchCost += exclTwoPhase.MatchCost()
	}

	var ratio float32
	if reqApprox.Cost() <= 0 {
		ratio = 1
	} else if exclApprox.Cost() <= 0 {
		ratio = 0
	} else {
		minCost := reqApprox.Cost()
		if exclApprox.Cost() < minCost {
			minCost = exclApprox.Cost()
		}
		ratio = float32(minCost) / float32(reqApprox.Cost())
	}
	matchCost += ratio * exclMatchCost
	return matchCost
}

// TwoPhaseIterator returns the TwoPhaseIterator for this scorer.
// Exposed via the scorerTwoPhaseProvider optional interface.
//
// Mirrors ReqExclScorer.twoPhaseIterator().
func (s *ReqExclScorer) TwoPhaseIterator() *TwoPhaseIterator {
	matchCost := reqExclMatchCost(s.reqApproximation, s.reqTwoPhase, s.exclApproximation, s.exclTwoPhase)

	approx := s.reqApproximation
	reqTwoPhase := s.reqTwoPhase
	exclApprox := s.exclApproximation
	exclTwoPhase := s.exclTwoPhase

	if reqTwoPhase == nil ||
		(exclTwoPhase != nil && reqTwoPhase.MatchCost() <= exclTwoPhase.MatchCost()) {
		// reqTwoPhase is cheaper (or absent) — check it first.
		return NewTwoPhaseIteratorWithMatchCost(approx, func() (bool, error) {
			doc := approx.DocID()
			exclDoc := exclApprox.DocID()
			if exclDoc < doc {
				var err error
				exclDoc, err = exclApprox.Advance(doc)
				if err != nil {
					return false, err
				}
			}
			if exclDoc != doc {
				return matchesOrNil(reqTwoPhase)
			}
			req, err := matchesOrNil(reqTwoPhase)
			if err != nil || !req {
				return false, err
			}
			excl, err := matchesOrNil(exclTwoPhase)
			if err != nil {
				return false, err
			}
			return !excl, nil
		}, matchCost)
	}

	// reqTwoPhase is more expensive — check exclusion first.
	return NewTwoPhaseIteratorWithMatchCost(approx, func() (bool, error) {
		doc := approx.DocID()
		exclDoc := exclApprox.DocID()
		if exclDoc < doc {
			var err error
			exclDoc, err = exclApprox.Advance(doc)
			if err != nil {
				return false, err
			}
		}
		if exclDoc != doc {
			return matchesOrNil(reqTwoPhase)
		}
		excl, err := matchesOrNil(exclTwoPhase)
		if err != nil {
			return false, err
		}
		if excl {
			return false, nil
		}
		return matchesOrNil(reqTwoPhase)
	}, matchCost)
}

// DocID returns the current document ID.
//
// Mirrors ReqExclScorer.docID() — Java tracks docID via reqApproximation;
// we delegate to tpDisi which drives reqApproximation internally.
func (s *ReqExclScorer) DocID() int {
	return s.tpDisi.DocID()
}

// NextDoc advances the scorer to the next document.
func (s *ReqExclScorer) NextDoc() (int, error) {
	return s.tpDisi.NextDoc()
}

// Advance advances the scorer to the first document ≥ target.
func (s *ReqExclScorer) Advance(target int) (int, error) {
	return s.tpDisi.Advance(target)
}

// Cost returns the cost of iteration.
func (s *ReqExclScorer) Cost() int64 {
	return s.tpDisi.Cost()
}

// DocIDRunEnd returns the end of the current run for block-based iteration.
func (s *ReqExclScorer) DocIDRunEnd() int {
	return s.tpDisi.DocIDRunEnd()
}

// Score returns the score of the current document from the required scorer.
//
// Mirrors ReqExclScorer.score().
func (s *ReqExclScorer) Score() float32 {
	return s.reqScorer.Score()
}

// GetMaxScore returns the maximum score up to the given document.
//
// Mirrors ReqExclScorer.getMaxScore(int).
func (s *ReqExclScorer) GetMaxScore(upTo int) float32 {
	return s.reqScorer.GetMaxScore(upTo)
}

var _ Scorer = (*ReqExclScorer)(nil)
var _ scorerTwoPhaseProvider = (*ReqExclScorer)(nil)
