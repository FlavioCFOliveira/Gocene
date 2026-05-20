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
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionScorer.java

// DisjunctionScorer is the base scorer for OR-like queries. Concrete
// subclasses implement score(topList *DisiWrapper) to combine the scores
// of all matching sub-scorers for a document.
//
// Mirrors org.apache.lucene.search.DisjunctionScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java's DisjunctionScorer extends Scorer directly and overrides
//     iterator()/twoPhaseIterator(). In Go, DisjunctionScorer embeds
//     BaseScorer and implements the Scorer interface (DocIdSetIterator +
//     Score + GetMaxScore).
//   - The TwoPhase inner class is inlined as disjunctionTwoPhase.
//     PriorityQueue for unverified two-phase matches uses a simple
//     slice min-heap of *DisiWrapper ordered by matchCost.
//   - Gocene's Scorer interface does not include AdvanceShallow;
//     GetMaxScore receives the upTo parameter but has no shallow-advance
//     mechanism yet — subclasses may override it when needed.
//   - getChildren() is not part of Gocene's Scorer interface; a separate
//     GetChildren() method is provided for callers that need it.
type DisjunctionScorer struct {
	BaseScorer
	approximation *DisjunctionDISIApproximation
	twoPhase      *disjunctionTwoPhase
	needsScores   bool
	numClauses    int
	scorers       []Scorer // kept for GetMaxScore / AdvanceShallow
}

// newDisjunctionScorer constructs a DisjunctionScorer from at least two
// sub-scorers. scoreMode is used to determine whether scoring is needed.
// leadCost bounds which iterators go into the priority queue.
func newDisjunctionScorer(
	subScorers []Scorer,
	scoreMode ScoreMode,
	leadCost int64,
) *DisjunctionScorer {
	if len(subScorers) < 2 {
		panic("DisjunctionScorer: at least 2 subScorers required")
	}
	needsScores := scoreMode != COMPLETE_NO_SCORES

	var totalApproxCost int64
	var sumMatchCost float32
	hasApproximation := false

	wrappers := make([]*DisiWrapper, len(subScorers))
	for i, s := range subScorers {
		w := NewDisiWrapper(s, false)
		wrappers[i] = w
		costWeight := w.cost
		if costWeight < 1 {
			costWeight = 1
		}
		totalApproxCost += costWeight
		if w.twoPhaseView != nil {
			hasApproximation = true
			sumMatchCost += w.matchCost * float32(costWeight)
		}
	}

	approx := NewDisjunctionDISIApproximation(wrappers, leadCost)

	var tp *disjunctionTwoPhase
	if hasApproximation {
		matchCost := sumMatchCost / float32(totalApproxCost)
		tp = newDisjunctionTwoPhase(approx, matchCost, needsScores, len(subScorers))
	}

	return &DisjunctionScorer{
		approximation: approx,
		twoPhase:      tp,
		needsScores:   needsScores,
		numClauses:    len(subScorers),
		scorers:       subScorers,
	}
}

// ─── DocIdSetIterator ────────────────────────────────────────────────────────

// DocID returns the current document ID.
func (s *DisjunctionScorer) DocID() int {
	return s.approximation.DocID()
}

// NextDoc advances to the next document.
func (s *DisjunctionScorer) NextDoc() (int, error) {
	if s.twoPhase != nil {
		return tpNextDoc(s.twoPhase)
	}
	return s.approximation.NextDoc()
}

// Advance moves to the first document ≥ target.
func (s *DisjunctionScorer) Advance(target int) (int, error) {
	if s.twoPhase != nil {
		return tpAdvance(s.twoPhase, target)
	}
	return s.approximation.Advance(target)
}

// Cost returns the estimated number of matching documents.
func (s *DisjunctionScorer) Cost() int64 {
	return s.approximation.Cost()
}

// DocIDRunEnd returns the end of the current run.
func (s *DisjunctionScorer) DocIDRunEnd() int {
	return s.approximation.DocIDRunEnd()
}

// ─── Scorer ──────────────────────────────────────────────────────────────────

// Score returns the score for the current document by delegating to
// scoreTopList which concrete subclasses must shadow.
// Errors from scoreTopList are silently dropped to satisfy the Scorer
// interface; use ScoreWithError when error propagation matters.
func (s *DisjunctionScorer) Score() float32 {
	v, _ := s.ScoreWithError()
	return v
}

// ScoreWithError is the error-returning variant used internally.
func (s *DisjunctionScorer) ScoreWithError() (float32, error) {
	topList, err := s.getSubMatches()
	if err != nil {
		return 0, err
	}
	return s.scoreTopList(topList)
}

// scoreTopList is the abstract method that concrete subclasses override
// by shadowing. The default returns 0.
func (s *DisjunctionScorer) scoreTopList(_ *DisiWrapper) (float32, error) {
	return 0, nil
}

// GetMaxScore returns an upper bound on the score for any document ≤ upTo.
// The default sums all sub-scorer max scores; subclasses may tighten this.
func (s *DisjunctionScorer) GetMaxScore(upTo int) float32 {
	var max float64
	for _, sc := range s.scorers {
		if sc.DocID() <= upTo {
			max += float64(sc.GetMaxScore(upTo))
		}
	}
	if max > float64(maxFloat32) {
		return maxFloat32
	}
	return float32(max)
}

// getSubMatches returns the linked list of matching DisiWrappers for the
// current document.
func (s *DisjunctionScorer) getSubMatches() (*DisiWrapper, error) {
	if s.twoPhase == nil {
		return s.approximation.topList(), nil
	}
	return s.twoPhase.getSubMatches()
}

// TwoPhaseIterator returns the TwoPhaseIterator for this scorer, or nil
// when no sub-scorer uses two-phase iteration.
func (s *DisjunctionScorer) TwoPhaseIterator() *TwoPhaseIterator {
	if s.twoPhase == nil {
		return nil
	}
	return s.twoPhase.tpi
}

// maxFloat32 is the largest finite float32 value.
const maxFloat32 = float32(3.4028234663852886e+38)

// ─── disjunctionTwoPhase ─────────────────────────────────────────────────────

// disjunctionTwoPhase is the TwoPhase inner class of DisjunctionScorer.
// It verifies matches by collecting unverified DisiWrappers (sorted by
// ascending matchCost) and running their twoPhaseView in that order.
type disjunctionTwoPhase struct {
	tpi             *TwoPhaseIterator
	verifiedMatches *DisiWrapper
	unverified      []*DisiWrapper // min-heap by matchCost
	needsScores     bool
	approx          *DisjunctionDISIApproximation
}

func newDisjunctionTwoPhase(
	approx *DisjunctionDISIApproximation,
	matchCost float32,
	needsScores bool,
	numClauses int,
) *disjunctionTwoPhase {
	tp := &disjunctionTwoPhase{
		approx:      approx,
		needsScores: needsScores,
		unverified:  make([]*DisiWrapper, 0, numClauses),
	}
	tp.tpi = NewTwoPhaseIteratorWithMatchCost(approx, tp.matches, matchCost)
	return tp
}

// matches implements the TwoPhaseIterator matches function.
func (tp *disjunctionTwoPhase) matches() (bool, error) {
	tp.verifiedMatches = nil
	tp.unverified = tp.unverified[:0]

	for w := tp.approx.topList(); w != nil; {
		next := w.next
		if w.twoPhaseView == nil {
			// Implicitly verified.
			w.next = tp.verifiedMatches
			tp.verifiedMatches = w
			if !tp.needsScores {
				return true, nil
			}
		} else {
			tp.tpHeapPush(w)
		}
		w = next
	}

	if tp.verifiedMatches != nil {
		return true, nil
	}

	// Verify in ascending matchCost order.
	for len(tp.unverified) > 0 {
		w := tp.tpHeapPop()
		ok, err := w.twoPhaseView.Matches()
		if err != nil {
			return false, err
		}
		if ok {
			w.next = nil
			tp.verifiedMatches = w
			return true, nil
		}
	}
	return false, nil
}

// getSubMatches returns the verified match list after a matches() call.
func (tp *disjunctionTwoPhase) getSubMatches() (*DisiWrapper, error) {
	for len(tp.unverified) > 0 {
		w := tp.tpHeapPop()
		ok, err := w.twoPhaseView.Matches()
		if err != nil {
			return nil, err
		}
		if ok {
			w.next = tp.verifiedMatches
			tp.verifiedMatches = w
		}
	}
	return tp.verifiedMatches, nil
}

// ─── min-heap by matchCost for unverified wrappers ───────────────────────────

func (tp *disjunctionTwoPhase) tpHeapPush(w *DisiWrapper) {
	tp.unverified = append(tp.unverified, w)
	i := len(tp.unverified) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if tp.unverified[parent].matchCost <= tp.unverified[i].matchCost {
			break
		}
		tp.unverified[parent], tp.unverified[i] = tp.unverified[i], tp.unverified[parent]
		i = parent
	}
}

func (tp *disjunctionTwoPhase) tpHeapPop() *DisiWrapper {
	n := len(tp.unverified)
	top := tp.unverified[0]
	tp.unverified[0] = tp.unverified[n-1]
	tp.unverified = tp.unverified[:n-1]
	// sift down
	i := 0
	for {
		left := 2*i + 1
		if left >= len(tp.unverified) {
			break
		}
		smallest := i
		if tp.unverified[left].matchCost < tp.unverified[smallest].matchCost {
			smallest = left
		}
		if right := left + 1; right < len(tp.unverified) &&
			tp.unverified[right].matchCost < tp.unverified[smallest].matchCost {
			smallest = right
		}
		if smallest == i {
			break
		}
		tp.unverified[i], tp.unverified[smallest] = tp.unverified[smallest], tp.unverified[i]
		i = smallest
	}
	return top
}

// ─── tpNextDoc / tpAdvance helpers for two-phase scorers ────────────────────

func tpNextDoc(tp *disjunctionTwoPhase) (int, error) {
	approx := tp.approx
	doc, err := approx.NextDoc()
	for err == nil && doc != NO_MORE_DOCS {
		ok, merr := tp.matches()
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

func tpAdvance(tp *disjunctionTwoPhase, target int) (int, error) {
	approx := tp.approx
	doc, err := approx.Advance(target)
	for err == nil && doc != NO_MORE_DOCS {
		ok, merr := tp.matches()
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
