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

import "github.com/FlavioCFOliveira/Gocene/util"

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ReqOptSumScorer.java

// ReqOptSumScorer is a Scorer for queries with a required part and an
// optional part. The required scorer drives document iteration; the
// optional scorer is only consulted when scoring a document.
//
// Mirrors org.apache.lucene.search.ReqOptSumScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - The TOP_SCORES impact-based block-skipping path (advanceShallow /
//     FilterDocIdSetIterator subclass) is not implemented because
//     Gocene's Scorer interface does not yet expose advanceShallow.
//     When scoreMode == TOP_SCORES the scorer falls back to the same
//     behaviour as COMPLETE: the required scorer drives iteration and
//     the optional scorer contributes only to scoring.
//   - twoPhaseIterator() is exposed via the scorerTwoPhaseProvider
//     interface rather than as a method on the Scorer interface.
//   - setMinCompetitiveScore and the optIsRequired / minScore fields
//     are retained structurally but have no effect until advanceShallow
//     lands in a later sprint.
type ReqOptSumScorer struct {
	BaseScorer
	reqScorer     Scorer
	optScorer     Scorer
	reqApprox     DocIdSetIterator
	optApprox     DocIdSetIterator
	optTwoPhase   *TwoPhaseIterator
	twoPhase      *TwoPhaseIterator
	minScore      float32
	optIsRequired bool
}

// NewReqOptSumScorer constructs a ReqOptSumScorer.
// reqScorer must not be nil; optScorer must not be nil.
// scoreMode is retained for structural completeness but currently all
// score modes use the simple required-only iteration path.
func NewReqOptSumScorer(reqScorer, optScorer Scorer, _ ScoreMode) *ReqOptSumScorer {
	s := &ReqOptSumScorer{
		reqScorer: reqScorer,
		optScorer: optScorer,
	}

	// Extract optional two-phase views if available.
	var reqTwoPhase *TwoPhaseIterator
	if sp, ok := reqScorer.(scorerTwoPhaseProvider); ok {
		reqTwoPhase = sp.TwoPhaseIterator()
	}
	if sp, ok := optScorer.(scorerTwoPhaseProvider); ok {
		s.optTwoPhase = sp.TwoPhaseIterator()
	}

	if reqTwoPhase != nil {
		s.reqApprox = reqTwoPhase.Approximation()
	} else {
		// Java: reqApproximation = reqScorer.iterator()
		s.reqApprox = reqScorer.Iterator()
	}
	if s.optTwoPhase != nil {
		s.optApprox = s.optTwoPhase.Approximation()
	} else {
		// Java: optApproximation = optScorer.iterator()
		s.optApprox = optScorer.Iterator()
	}

	// Build combined TwoPhaseIterator when at least one side has one.
	if reqTwoPhase != nil || s.optTwoPhase != nil {
		var matchCost float32 = 1
		if reqTwoPhase != nil {
			matchCost += reqTwoPhase.MatchCost()
		}
		if s.optTwoPhase != nil {
			matchCost += s.optTwoPhase.MatchCost()
		}
		rotp := s
		s.twoPhase = NewTwoPhaseIteratorWithMatchCost(
			s.reqApprox,
			func() (bool, error) { return rotp.twoPhaseMatches(reqTwoPhase) },
			matchCost,
		)
	}
	return s
}

// twoPhaseMatches is the matches() function for the combined TwoPhaseIterator.
func (s *ReqOptSumScorer) twoPhaseMatches(reqTwoPhase *TwoPhaseIterator) (bool, error) {
	if reqTwoPhase != nil {
		ok, err := reqTwoPhase.Matches()
		if err != nil || !ok {
			return false, err
		}
	}
	if s.optTwoPhase != nil {
		if s.optIsRequired {
			// Ensure opt approximation is aligned.
			reqDoc := s.reqScorer.DocID()
			if s.optApprox.DocID() < reqDoc {
				if _, err := s.optApprox.Advance(reqDoc); err != nil {
					return false, err
				}
			}
			if s.optApprox.DocID() != reqDoc {
				return false, nil
			}
			ok, err := s.optTwoPhase.Matches()
			if err != nil {
				return false, err
			}
			if !ok {
				if _, err2 := s.optApprox.NextDoc(); err2 != nil {
					return false, err2
				}
				return false, nil
			}
		} else if s.optApprox.DocID() == s.reqScorer.DocID() {
			ok, err := s.optTwoPhase.Matches()
			if err != nil {
				return false, err
			}
			if !ok {
				if _, err2 := s.optApprox.NextDoc(); err2 != nil {
					return false, err2
				}
			}
		}
	}
	return true, nil
}

// ─── DocIdSetIterator ────────────────────────────────────────────────────────

// DocID returns the current document ID.
func (s *ReqOptSumScorer) DocID() int { return s.reqScorer.DocID() }

// NextDoc advances to the next document.
func (s *ReqOptSumScorer) NextDoc() (int, error) {
	if s.twoPhase != nil {
		return tpNextDocReqOpt(s.twoPhase, s.reqApprox)
	}
	return s.reqApprox.NextDoc()
}

// Advance moves to the first document ≥ target.
func (s *ReqOptSumScorer) Advance(target int) (int, error) {
	if s.twoPhase != nil {
		return tpAdvanceReqOpt(s.twoPhase, s.reqApprox, target)
	}
	return s.reqApprox.Advance(target)
}

// Cost returns the estimated number of matching documents (req cost).
func (s *ReqOptSumScorer) Cost() int64 { return s.reqApprox.Cost() }

// DocIDRunEnd returns the end of the current run.
func (s *ReqOptSumScorer) DocIDRunEnd() (int, error) {
	return s.reqApprox.DocIDRunEnd()
}

// ─── Scorer ──────────────────────────────────────────────────────────────────

// Score returns the sum of the required and optional scores.
// If the optional scorer has not yet reached the current document,
// it is advanced lazily.
//
// Mirrors ReqOptSumScorer.score().
func (s *ReqOptSumScorer) Score() (float32, error) {
	curDoc := s.reqScorer.DocID()
	score, err := s.reqScorer.Score()
	if err != nil {
		return 0, err
	}

	optDoc := s.optApprox.DocID()
	if optDoc < curDoc {
		optDoc, err = s.optApprox.Advance(curDoc)
		if err != nil {
			return score, err
		}
		if s.optTwoPhase != nil && optDoc == curDoc {
			ok, _ := s.optTwoPhase.Matches()
			if !ok {
				_, _ = s.optApprox.NextDoc()
				optDoc = NO_MORE_DOCS
			}
		}
	}
	if optDoc == curDoc {
		optScore, err := s.optScorer.Score()
		if err != nil {
			return 0, err
		}
		score += optScore
	}
	return score, nil
}

// GetMaxScore returns an upper bound on the score for documents up to upTo.
//
// Mirrors ReqOptSumScorer.getMaxScore(int).
func (s *ReqOptSumScorer) GetMaxScore(upTo int) (float32, error) {
	max, err := s.reqScorer.GetMaxScore(upTo)
	if err != nil {
		return 0, err
	}
	if s.optScorer.DocID() <= upTo {
		optMax, err := s.optScorer.GetMaxScore(upTo)
		if err != nil {
			return 0, err
		}
		max += optMax
	}
	return max, nil
}

// TwoPhaseIterator returns the combined TwoPhaseIterator, or nil when
// neither sub-scorer uses two-phase iteration.
func (s *ReqOptSumScorer) TwoPhaseIterator() *TwoPhaseIterator { return s.twoPhase }

// ─── helpers ─────────────────────────────────────────────────────────────────

// tpNextDocReqOpt advances via the TwoPhaseIterator over reqApprox.
func tpNextDocReqOpt(tp *TwoPhaseIterator, approx DocIdSetIterator) (int, error) {
	doc, err := approx.NextDoc()
	for err == nil && doc != NO_MORE_DOCS {
		ok, merr := tp.Matches()
		if merr != nil {
			return NO_MORE_DOCS, merr
		}
		if ok {
			return doc, nil
		}
		doc, err = approx.NextDoc()
	}
	return NO_MORE_DOCS, err
}

// tpAdvanceReqOpt advances via the TwoPhaseIterator over reqApprox to target.
func tpAdvanceReqOpt(tp *TwoPhaseIterator, approx DocIdSetIterator, target int) (int, error) {
	doc, err := approx.Advance(target)
	for err == nil && doc != NO_MORE_DOCS {
		ok, merr := tp.Matches()
		if merr != nil {
			return NO_MORE_DOCS, merr
		}
		if ok {
			return doc, nil
		}
		doc, err = approx.NextDoc()
	}
	return NO_MORE_DOCS, err
}

// Compile-time check: ReqOptSumScorer satisfies Scorer.
var _ Scorer = (*ReqOptSumScorer)(nil)

// Iterator mirrors ReqOptSumScorer.iterator() of Apache Lucene 10.5.0: the
// required approximation when there is no two-phase iterator, otherwise the
// two-phase iterator viewed as a DocIdSetIterator.
func (s *ReqOptSumScorer) Iterator() DocIdSetIterator {
	if s.twoPhase == nil {
		return s.reqApprox
	}
	return AsDocIdSetIterator(s.twoPhase)
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0.
func (r *ReqOptSumScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(r, upTo, liveDocs, buffer)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *ReqOptSumScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
