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
//   lucene/core/src/java/org/apache/lucene/search/WANDScorer.java

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FloatMantissaBits is the number of mantissa bits in a float32.
// Exported so tests can reference it directly.
// Mirrors WANDScorer.FLOAT_MANTISSA_BITS (Lucene 10.4.0).
const FloatMantissaBits = 24

// floatMantissaBits is the package-internal alias.
const floatMantissaBits = FloatMantissaBits

// maxScaledScore is the maximum value for a scaled max score (2^24 - 1).
const maxScaledScore = int64((1 << 24) - 1)

// WANDScoreScalingFactor returns the scaling factor for f such that
// f × 2^factor is in [2^23, 2^24).
//
// Special cases mirror the Java implementation:
//
//	scalingFactor(0)     = scalingFactor(MinValue) + 1
//	scalingFactor(+Inf)  = scalingFactor(MaxValue) - 1
//
// Panics if f < 0.
//
// Mirrors WANDScorer.scalingFactor(float) (Lucene 10.4.0).
func WANDScoreScalingFactor(f float32) int {
	if f < 0 {
		panic("WANDScoreScalingFactor: scores must be positive or zero")
	}
	if f == 0 {
		return WANDScoreScalingFactor(math.SmallestNonzeroFloat32) + 1
	}
	if math.IsInf(float64(f), 1) {
		return WANDScoreScalingFactor(math.MaxFloat32) - 1
	}
	// Use float64 for the exponent computation; doubles have more amplitude
	// than floats so the cast always produces a normal value.
	d := float64(f)
	exp := math.Ilogb(d) // floor(log2(d))
	return floatMantissaBits - 1 - exp
}

// WANDScaleMaxScore scales maxScore to an integer rounding up.
// Exported for testing.
// Mirrors WANDScorer.scaleMaxScore(float, int).
func WANDScaleMaxScore(maxScore float32, scalingFactor int) int64 {
	return wandScaleMaxScore(maxScore, scalingFactor)
}

// wandScaleMaxScore is the package-internal implementation.
func wandScaleMaxScore(maxScore float32, scalingFactor int) int64 {
	scaled := math.Ldexp(float64(maxScore), scalingFactor)
	if scaled > float64(maxScaledScore) {
		return maxScaledScore
	}
	return int64(math.Ceil(scaled))
}

// wandScaleMinScore scales minScore to an integer rounding down.
// Mirrors WANDScorer.scaleMinScore(float, int).
func wandScaleMinScore(minScore float32, scalingFactor int) int64 {
	scaled := math.Ldexp(float64(minScore), scalingFactor)
	return int64(math.Floor(scaled))
}

// WANDScorer implements the WAND (Weak AND) algorithm for dynamic pruning.
//
// Reference: "Efficient Query Evaluation using a Two-Level Retrieval Process"
// by Broder et al., enhanced with block-max techniques from Ding and Suel.
//
// For scoreMode == TOP_SCORES the scorer maintains a feedback loop with the
// collector to know the minimum competitive score at any time.
//
// Mirrors org.apache.lucene.search.WANDScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java exposes advanceShallow / setMinCompetitiveScore on Scorer; these are
//     not on Gocene's Scorer interface.  TOP_SCORES block-max pruning that relies
//     on advanceShallow (updateMaxScores / moveToNextBlock) is structurally present
//     but the advanceShallow calls are no-ops (returns NO_MORE_DOCS as a stub).
//   - allScorers is []Scorer instead of Scorer[] (Go slice).
//   - DisiPriorityQueue is the Gocene 1-indexed wrapper; the 0-indexed tail
//     heap is a plain []DisiWrapper with stand-alone heap helpers (disiLeftNode,
//     disiRightNode, disiParentNode) that mirror DisiPriorityQueueN.
//   - iterator() / twoPhaseIterator() are not on Gocene's Scorer interface;
//     they are exposed via the scorerTwoPhaseProvider optional interface.
//   - DisiWrapper.iterator is the raw DISI used for advance calls.
//   - MathUtil.sumUpperBound mirrors util.MathSumUpperBound.
//   - The constructor returns an error instead of throwing IOException.
type WANDScorer struct {
	BaseScorer

	scalingFactor       int
	minCompetitiveScore int64 // scaled

	allScorers []Scorer

	// lead is the linked list of wrappers currently on 'doc'.
	lead *DisiWrapper
	doc  int
	// leadScore is the cumulative score of scorers in 'lead' (TOP_SCORES only).
	leadScore float64

	// head is a min-heap of wrappers ahead of 'doc', ordered by doc ID.
	head *DisiPriorityQueue

	// tail is a max-heap of wrappers behind 'doc', ordered by scaledMaxScore desc.
	tail         []*DisiWrapper
	tailMaxScore int64 // sum of scaledMaxScore of tail entries
	tailSize     int

	cost int64
	upTo int // upper bound for which max scores are valid

	minShouldMatch int
	freq           int

	scoreMode ScoreMode
	leadCost  int64

	// tpDisi is the cached TwoPhaseIterator-as-DISI built from twoPhaseIterator().
	tpDisi DocIdSetIterator
}

// NewWANDScorer constructs a WANDScorer.
//
// scorers must have at least minShouldMatch+1 elements.
// minShouldMatch must be ≥ 0 and < len(scorers).
//
// Mirrors WANDScorer(Collection<Scorer>, int, ScoreMode, long).
func NewWANDScorer(scorers []Scorer, minShouldMatch int, scoreMode ScoreMode, leadCost int64) (*WANDScorer, error) {
	if minShouldMatch >= len(scorers) {
		return nil, fmt.Errorf("WANDScorer: minShouldMatch (%d) must be < number of scorers (%d)", minShouldMatch, len(scorers))
	}

	ws := &WANDScorer{
		allScorers:     make([]Scorer, len(scorers)),
		minShouldMatch: minShouldMatch,
		doc:            -1,
		upTo:           -1,
		scoreMode:      scoreMode,
		leadCost:       leadCost,
		head:           NewDisiPriorityQueue(len(scorers)),
		tail:           make([]*DisiWrapper, len(scorers)),
	}
	copy(ws.allScorers, scorers)

	if scoreMode == TOP_SCORES {
		// Compute the global maximum score sum to derive the scaling factor.
		// advanceShallow is not on Gocene's Scorer; we skip it and call
		// GetMaxScore(NO_MORE_DOCS) directly.
		var maxScoreSumDouble float64
		for _, s := range scorers {
			ms := s.GetMaxScore(NO_MORE_DOCS)
			maxScoreSumDouble += float64(ms)
		}
		maxScoreSum := float32(util.MathSumUpperBound(maxScoreSumDouble, len(scorers)))
		ws.scalingFactor = WANDScoreScalingFactor(maxScoreSum)
	}

	// Add all scorers as unpositioned leads.
	for _, s := range scorers {
		ws.addUnpositionedLead(NewDisiWrapper(s, true))
	}

	// Compute iteration cost.
	costs := make([]int64, len(scorers))
	for i, s := range scorers {
		costs[i] = s.Cost()
	}
	ws.cost = CostWithMinShouldMatch(costs, len(scorers), minShouldMatch)

	// Build the two-phase iterator and cache its DISI.
	ws.tpDisi = NewTwoPhaseIteratorAsDocIdSetIterator(ws.buildTwoPhaseIterator())

	return ws, nil
}

// ─── static math helpers ────────────────────────────────────────────────────

// ─── lead management ────────────────────────────────────────────────────────

func (ws *WANDScorer) addLead(w *DisiWrapper) {
	w.next = ws.lead
	ws.lead = w
	ws.freq++
	if ws.scoreMode == TOP_SCORES {
		ws.leadScore += float64(w.scorable.Score())
	}
}

func (ws *WANDScorer) addUnpositionedLead(w *DisiWrapper) {
	// doc == -1, not yet positioned
	w.next = ws.lead
	ws.lead = w
	ws.freq++
}

// pushBackLeads moves all current 'lead' wrappers back to head or tail.
func (ws *WANDScorer) pushBackLeads(target int) error {
	for s := ws.lead; s != nil; s = s.next {
		evicted := ws.insertTailWithOverFlow(s)
		if evicted != nil {
			var err error
			evicted.doc, err = evicted.iterator.Advance(target)
			if err != nil {
				return err
			}
			ws.head.Add(evicted)
		}
	}
	ws.lead = nil
	return nil
}

// advanceHead ensures all wrappers in 'head' are on or beyond target.
func (ws *WANDScorer) advanceHead(target int) (*DisiWrapper, error) {
	headTop := ws.head.Top()
	for headTop != nil && headTop.doc < target {
		evicted := ws.insertTailWithOverFlow(headTop)
		if evicted != nil {
			var err error
			evicted.doc, err = evicted.iterator.Advance(target)
			if err != nil {
				return nil, err
			}
			headTop = ws.head.UpdateTopWith(evicted)
		} else {
			ws.head.Pop()
			headTop = ws.head.Top()
		}
	}
	return headTop, nil
}

func (ws *WANDScorer) advanceTailOne(disi *DisiWrapper) error {
	var err error
	disi.doc, err = disi.iterator.Advance(ws.doc)
	if err != nil {
		return err
	}
	if disi.doc == ws.doc {
		ws.addLead(disi)
	} else {
		ws.head.Add(disi)
	}
	return nil
}

// advanceTail pops the highest-scoring tail entry and advances it to current doc.
func (ws *WANDScorer) advanceTail() error {
	top := ws.popTail()
	return ws.advanceTailOne(top)
}

// advanceAllTail advances all tail entries to current doc.
func (ws *WANDScorer) advanceAllTail() error {
	for i := ws.tailSize - 1; i >= 0; i-- {
		if err := ws.advanceTailOne(ws.tail[i]); err != nil {
			return err
		}
	}
	ws.tailSize = 0
	ws.tailMaxScore = 0
	return nil
}

// moveToNextCandidate sets 'doc' to the next candidate and populates 'lead'.
func (ws *WANDScorer) moveToNextCandidate() {
	ws.lead = ws.head.Pop()
	ws.lead.next = nil
	ws.freq = 1
	if ws.scoreMode == TOP_SCORES {
		ws.leadScore = float64(ws.lead.scorable.Score())
	}
	for ws.head.Size() > 0 && ws.head.Top().doc == ws.doc {
		ws.addLead(ws.head.Pop())
	}
}

// ─── TOP_SCORES block-max helpers ───────────────────────────────────────────
// advanceShallow is not on Gocene Scorer, so these are degraded stubs.

func (ws *WANDScorer) wandAdvanceShallow(_ Scorer, _ int) int {
	// advanceShallow not available on Gocene Scorer; return NO_MORE_DOCS.
	return NO_MORE_DOCS
}

func (ws *WANDScorer) updateMaxScores(target int) error {
	newUpTo := NO_MORE_DOCS
	for w := range ws.head.HeapAll() {
		if w.doc <= newUpTo && w.cost <= ws.leadCost {
			u := ws.wandAdvanceShallow(w.scorer, w.doc)
			if u < newUpTo {
				newUpTo = u
			}
		}
	}
	if newUpTo == NO_MORE_DOCS && ws.tailSize > 0 && ws.tail[0].cost <= ws.leadCost {
		newUpTo = ws.wandAdvanceShallow(ws.tail[0].scorer, target)
		headTop := ws.head.Top()
		if headTop != nil && headTop.doc > newUpTo {
			newUpTo = headTop.doc
		}
	}
	ws.upTo = newUpTo

	for w := range ws.head.HeapAll() {
		if w.doc <= ws.upTo {
			w.scaledMaxScore = wandScaleMaxScore(w.scorer.GetMaxScore(newUpTo), ws.scalingFactor)
		}
	}

	ws.tailMaxScore = 0
	for i := 0; i < ws.tailSize; i++ {
		w := ws.tail[i]
		ws.wandAdvanceShallow(w.scorer, target)
		w.scaledMaxScore = wandScaleMaxScore(w.scorer.GetMaxScore(ws.upTo), ws.scalingFactor)
		wandUpHeapMaxScore(ws.tail, i)
		ws.tailMaxScore += w.scaledMaxScore
	}

	// ensure tail alone cannot match competitive hits
	for ws.tailSize > 0 && ws.tailMaxScore >= ws.minCompetitiveScore {
		w := ws.popTail()
		var err error
		w.doc, err = w.iterator.Advance(target)
		if err != nil {
			return err
		}
		ws.head.Add(w)
	}
	return nil
}

func (ws *WANDScorer) moveToNextBlock(target int) error {
	for ws.upTo < NO_MORE_DOCS {
		if ws.head.Size() == 0 {
			if target <= ws.upTo {
				target = ws.upTo + 1
			}
			if err := ws.updateMaxScores(target); err != nil {
				return err
			}
		} else if ws.head.Top().doc > ws.upTo {
			if err := ws.updateMaxScores(target); err != nil {
				return err
			}
			break
		} else {
			break
		}
	}
	return nil
}

// ─── tail heap (max-score ordered, 0-indexed) ────────────────────────────────

func (ws *WANDScorer) insertTailWithOverFlow(s *DisiWrapper) *DisiWrapper {
	if ws.tailMaxScore+s.scaledMaxScore < ws.minCompetitiveScore ||
		ws.tailSize+1 < ws.minShouldMatch {
		ws.addTail(s)
		ws.tailMaxScore += s.scaledMaxScore
		return nil
	}
	if ws.tailSize == 0 {
		return s
	}
	top := ws.tail[0]
	if !wandGreaterMaxScore(top, s) {
		return s
	}
	ws.tail[0] = s
	wandDownHeapMaxScore(ws.tail, ws.tailSize)
	ws.tailMaxScore = ws.tailMaxScore - top.scaledMaxScore + s.scaledMaxScore
	return top
}

func (ws *WANDScorer) addTail(s *DisiWrapper) {
	ws.tail[ws.tailSize] = s
	wandUpHeapMaxScore(ws.tail, ws.tailSize)
	ws.tailSize++
}

func (ws *WANDScorer) popTail() *DisiWrapper {
	result := ws.tail[0]
	ws.tailSize--
	ws.tail[0] = ws.tail[ws.tailSize]
	ws.tail[ws.tailSize] = nil
	wandDownHeapMaxScore(ws.tail, ws.tailSize)
	ws.tailMaxScore -= result.scaledMaxScore
	return result
}

// wandGreaterMaxScore compares two wrappers: higher scaledMaxScore wins; ties
// go to lower cost (so cheaper scorers advance further in the tail).
func wandGreaterMaxScore(w1, w2 *DisiWrapper) bool {
	if w1.scaledMaxScore != w2.scaledMaxScore {
		return w1.scaledMaxScore > w2.scaledMaxScore
	}
	return w1.cost < w2.cost
}

func wandUpHeapMaxScore(heap []*DisiWrapper, i int) {
	node := heap[i]
	j := disiParentNode(i)
	for j >= 0 && wandGreaterMaxScore(node, heap[j]) {
		heap[i] = heap[j]
		i = j
		j = disiParentNode(j)
	}
	heap[i] = node
}

func wandDownHeapMaxScore(heap []*DisiWrapper, size int) {
	i := 0
	node := heap[0]
	j := disiLeftNode(i)
	if j < size {
		k := disiRightNode(j)
		if k < size && wandGreaterMaxScore(heap[k], heap[j]) {
			j = k
		}
		if wandGreaterMaxScore(heap[j], node) {
			for {
				heap[i] = heap[j]
				i = j
				j = disiLeftNode(i)
				k := disiRightNode(j)
				if k < size && wandGreaterMaxScore(heap[k], heap[j]) {
					j = k
				}
				if j >= size || !wandGreaterMaxScore(heap[j], node) {
					break
				}
			}
			heap[i] = node
		}
	}
}

// ─── TwoPhaseIterator ────────────────────────────────────────────────────────

func (ws *WANDScorer) buildTwoPhaseIterator() *TwoPhaseIterator {
	approx := &wandApproximation{ws: ws}

	return NewTwoPhaseIteratorWithMatchCost(approx, func() (bool, error) {
		// lead is nil here (approximation.Advance sets doc but not lead)
		ws.moveToNextCandidate()

		var scaledLeadScore int64
		if ws.scoreMode == TOP_SCORES {
			scaledLeadScore = wandScaleMaxScore(
				float32(util.MathSumUpperBound(ws.leadScore, floatMantissaBits)),
				ws.scalingFactor,
			)
		}

		for scaledLeadScore < ws.minCompetitiveScore || ws.freq < ws.minShouldMatch {
			if scaledLeadScore+ws.tailMaxScore < ws.minCompetitiveScore ||
				ws.freq+ws.tailSize < ws.minShouldMatch {
				return false, nil
			}
			prevLead := ws.lead
			if err := ws.advanceTail(); err != nil {
				return false, err
			}
			if ws.scoreMode == TOP_SCORES && ws.lead != prevLead {
				scaledLeadScore = wandScaleMaxScore(
					float32(util.MathSumUpperBound(ws.leadScore, floatMantissaBits)),
					ws.scalingFactor,
				)
			}
		}
		return true, nil
	}, float32(len(ws.tail)))
}

// wandApproximation implements the DocIdSetIterator approximation for WANDScorer.
type wandApproximation struct {
	ws *WANDScorer
}

func (a *wandApproximation) DocID() int { return a.ws.doc }

func (a *wandApproximation) NextDoc() (int, error) {
	return a.Advance(a.ws.doc + 1)
}

func (a *wandApproximation) Advance(target int) (int, error) {
	ws := a.ws

	if err := ws.pushBackLeads(target); err != nil {
		return NO_MORE_DOCS, err
	}

	headTop, err := ws.advanceHead(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}

	if ws.scoreMode == TOP_SCORES && (headTop == nil || headTop.doc > ws.upTo) {
		if err := ws.moveToNextBlock(target); err != nil {
			return NO_MORE_DOCS, err
		}
		headTop = ws.head.Top()
	}

	if headTop == nil {
		ws.doc = NO_MORE_DOCS
	} else {
		ws.doc = headTop.doc
	}
	return ws.doc, nil
}

func (a *wandApproximation) Cost() int64 { return a.ws.cost }

func (a *wandApproximation) DocIDRunEnd() int { return a.ws.doc + 1 }

// ─── Scorer / DocIdSetIterator surface ──────────────────────────────────────

// TwoPhaseIterator exposes the two-phase iterator for optional callers.
// Implements scorerTwoPhaseProvider.
func (ws *WANDScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return ws.buildTwoPhaseIterator()
}

// DocID returns the current document ID.
func (ws *WANDScorer) DocID() int { return ws.tpDisi.DocID() }

// NextDoc advances to the next document.
func (ws *WANDScorer) NextDoc() (int, error) { return ws.tpDisi.NextDoc() }

// Advance advances to the first document ≥ target.
func (ws *WANDScorer) Advance(target int) (int, error) { return ws.tpDisi.Advance(target) }

// Cost returns the iteration cost.
func (ws *WANDScorer) Cost() int64 { return ws.cost }

// DocIDRunEnd returns the end of the current consecutive doc-ID run.
func (ws *WANDScorer) DocIDRunEnd() int { return ws.tpDisi.DocIDRunEnd() }

// Score returns the score for the current document.
//
// Mirrors WANDScorer.score().
func (ws *WANDScorer) Score() float32 {
	if err := ws.advanceAllTail(); err != nil {
		return 0
	}
	leadScore := ws.leadScore
	if ws.scoreMode != TOP_SCORES {
		for s := ws.lead; s != nil; s = s.next {
			leadScore += float64(s.scorable.Score())
		}
	}
	return float32(leadScore)
}

// GetMaxScore returns the maximum possible score up to upTo.
//
// Mirrors WANDScorer.getMaxScore(int).
func (ws *WANDScorer) GetMaxScore(upTo int) float32 {
	var maxScoreSum float64
	for _, s := range ws.allScorers {
		if s.DocID() <= upTo {
			maxScoreSum += float64(s.GetMaxScore(upTo))
		}
	}
	return float32(util.MathSumUpperBound(maxScoreSum, len(ws.allScorers)))
}

var _ Scorer = (*WANDScorer)(nil)
var _ scorerTwoPhaseProvider = (*WANDScorer)(nil)
