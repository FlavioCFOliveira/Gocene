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
//   lucene/core/src/java/org/apache/lucene/search/DenseConjunctionBulkScorer.java

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// WindowSize is the size of each scoring window used by DenseConjunctionBulkScorer.
//
// Mirrors DenseConjunctionBulkScorer.WINDOW_SIZE (Lucene 10.4.0).
const WindowSize = 4096

// DensityThresholdInverse controls when bit-set intersection is used.
// Bit sets are used only when at least 1/DensityThresholdInverse of docs match.
//
// Mirrors DenseConjunctionBulkScorer.DENSITY_THRESHOLD_INVERSE (Lucene 10.4.0).
const DensityThresholdInverse = 64 / 2 // Long.SIZE / 2 = 32

// denseConjWrapperItem is the Go equivalent of Java's DisiWrapper record
// inside DenseConjunctionBulkScorer — it pairs an approximation DISI with
// an optional TwoPhaseIterator.
type denseConjWrapperItem struct {
	approximation DocIdSetIterator
	twoPhase      *TwoPhaseIterator
}

func newDenseConjWrapperFromDISI(it DocIdSetIterator) *denseConjWrapperItem {
	return &denseConjWrapperItem{approximation: it}
}

func newDenseConjWrapperFromTwoPhase(tp *TwoPhaseIterator) *denseConjWrapperItem {
	return &denseConjWrapperItem{approximation: tp.Approximation(), twoPhase: tp}
}

func (w *denseConjWrapperItem) docID() int { return w.approximation.DocID() }

func (w *denseConjWrapperItem) docIDRunEnd() int {
	if w.twoPhase == nil {
		return w.approximation.DocIDRunEnd()
	}
	return w.approximation.DocIDRunEnd()
}

// denseConjScorable is the per-window Scorable injected into LeafCollector.
// It holds the constant score and exposes SetMinCompetitiveScore so the
// collector can signal early termination.
//
// Mirrors DenseConjunctionBulkScorer.SimpleScorable (Lucene 10.4.0).
type denseConjScorable struct {
	BaseScorable
	score               float32
	minCompetitiveScore float32
}

func (s *denseConjScorable) Score() (float32, error) { return s.score, nil }
func (s *denseConjScorable) SetMinCompetitiveScore(v float32) error {
	if v > s.minCompetitiveScore {
		s.minCompetitiveScore = v
	}
	return nil
}

var _ Scorable = (*denseConjScorable)(nil)

// denseConjScorerAdapter wraps denseConjScorable to satisfy Scorer so it
// can be passed to LeafCollector.SetScorer.
type denseConjScorerAdapter struct {
	BaseScorer
	s *denseConjScorable
}

func (a *denseConjScorerAdapter) DocID() int                 { return -1 }
func (a *denseConjScorerAdapter) NextDoc() (int, error)      { return NO_MORE_DOCS, nil }
func (a *denseConjScorerAdapter) Advance(_ int) (int, error) { return NO_MORE_DOCS, nil }
func (a *denseConjScorerAdapter) Cost() int64                { return 0 }
func (a *denseConjScorerAdapter) DocIDRunEnd() int           { return NO_MORE_DOCS }
func (a *denseConjScorerAdapter) Score() float32             { return a.s.score }
func (a *denseConjScorerAdapter) GetMaxScore(_ int) float32  { return a.BaseScorer.GetMaxScore(0) }

// SetMinCompetitiveScore forwards the call to the underlying denseConjScorable
// so that the collector can signal early termination.
func (a *denseConjScorerAdapter) SetMinCompetitiveScore(v float32) error {
	return a.s.SetMinCompetitiveScore(v)
}

var _ Scorer = (*denseConjScorerAdapter)(nil)

// DenseConjunctionBulkScorer implements BulkScorer for conjunctions of
// dense clauses.  When clauses are dense enough, it intersects them using
// bit sets (WINDOW_SIZE-wide windows); otherwise it falls back to leap-frog
// iteration.
//
// Mirrors org.apache.lucene.search.DenseConjunctionBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Gocene's BulkScorer.Score takes (Collector, DocIdSetIterator) with no
//     min/max; the acceptDocs Bits parameter becomes a DISI filter.
//   - DocIdSetIterator.intoBitSet / Bits.applyMask are not on Gocene interfaces;
//     scoreWindowUsingBitSet degrades to leap-frog (scoreWindowUsingLeapFrog)
//     for all windows.
//   - LeafCollector.competitiveIterator / collectRange / collect(DocIdStream)
//     are not on Gocene's LeafCollector; early-termination and batch delivery
//     are omitted.
//   - Java's DenseConjunctionBulkScorer is package-private; Gocene exports it
//     for testability.
type DenseConjunctionBulkScorer struct {
	maxDoc    int
	iterators []*denseConjWrapperItem
	scorable  *denseConjScorable
	// windowMatches and clauseWindowMatches are scratch bit sets reused
	// across calls to scoreWindowUsingBitSet.  They are sized at WindowSize.
	windowMatches       *util.FixedBitSet
	clauseWindowMatches *util.FixedBitSet
}

// NewDenseConjunctionBulkScorer builds a scorer from separated iterator and
// two-phase lists.
//
// Mirrors DenseConjunctionBulkScorer(List<DocIdSetIterator>, List<TwoPhaseIterator>, int, float).
func NewDenseConjunctionBulkScorer(
	iterators []DocIdSetIterator,
	twoPhases []*TwoPhaseIterator,
	maxDoc int,
	constantScore float32,
) (*DenseConjunctionBulkScorer, error) {
	if len(iterators) == 0 && len(twoPhases) == 0 {
		return nil, fmt.Errorf("DenseConjunctionBulkScorer: expected one or more iterators, got 0")
	}

	wm, err := util.NewFixedBitSet(WindowSize)
	if err != nil {
		return nil, err
	}
	cwm, err := util.NewFixedBitSet(WindowSize)
	if err != nil {
		return nil, err
	}

	items := make([]*denseConjWrapperItem, 0, len(iterators)+len(twoPhases))
	for _, it := range iterators {
		items = append(items, newDenseConjWrapperFromDISI(it))
	}
	for _, tp := range twoPhases {
		items = append(items, newDenseConjWrapperFromTwoPhase(tp))
	}
	// Sort by ascending approximation cost (cheapest leads).
	sort.Slice(items, func(i, j int) bool {
		return items[i].approximation.Cost() < items[j].approximation.Cost()
	})

	s := &denseConjScorable{}
	s.score = constantScore

	return &DenseConjunctionBulkScorer{
		maxDoc:              maxDoc,
		iterators:           items,
		scorable:            s,
		windowMatches:       wm,
		clauseWindowMatches: cwm,
	}, nil
}

// NewDenseConjunctionBulkScorerFromScorers creates a DenseConjunctionBulkScorer
// from a list of Scorers, separating two-phase iterators automatically.
//
// Mirrors DenseConjunctionBulkScorer.of(List<Scorer>, int, float).
func NewDenseConjunctionBulkScorerFromScorers(
	scorers []Scorer,
	maxDoc int,
	constantScore float32,
) (*DenseConjunctionBulkScorer, error) {
	var iters []DocIdSetIterator
	var twoPhases []*TwoPhaseIterator
	for _, sc := range scorers {
		if sp, ok := sc.(scorerTwoPhaseProvider); ok {
			if tp := sp.TwoPhaseIterator(); tp != nil {
				twoPhases = append(twoPhases, tp)
				continue
			}
		}
		iters = append(iters, sc)
	}
	return NewDenseConjunctionBulkScorer(iters, twoPhases, maxDoc, constantScore)
}

// Score scores documents in [min, max) that match every clause, delivering
// each accepted match to the collector, and returns the first document the
// lead iterator advanced to on or beyond the effective window end (max capped
// to maxDoc).
//
// Mirrors DenseConjunctionBulkScorer.score(LeafCollector, Bits, int, int),
// degraded to leap-frog iteration (intoBitSet / applyMask / collectRange are
// not available on Gocene interfaces). acceptDocs is a util.Bits filter (nil
// accepts all).
func (bs *DenseConjunctionBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if max > bs.maxDoc {
		max = bs.maxDoc
	}

	scorerAdapter := &denseConjScorerAdapter{s: bs.scorable}
	if err := collector.SetScorer(scorerAdapter); err != nil {
		return 0, err
	}

	iterators := bs.iterators
	if len(iterators) == 0 {
		return NO_MORE_DOCS, nil
	}

	// Position the lead iterator at min.
	lead := iterators[0]
	doc := lead.approximation.DocID()
	if doc < min {
		var err error
		doc, err = lead.approximation.Advance(min)
		if err != nil {
			return 0, err
		}
	}

	// Align the remaining iterators with the lead.
	for i := 1; i < len(iterators); i++ {
		d, advErr := iterators[i].approximation.Advance(doc)
		if advErr != nil {
			return 0, advErr
		}
		if d > doc {
			doc = d
			d2, advErr := lead.approximation.Advance(doc)
			if advErr != nil {
				return 0, advErr
			}
			doc = d2
			i = 0 // restart scan
		}
	}

	var err error
	// Main leap-frog loop.
	for doc < max && doc != NO_MORE_DOCS {
		// Early termination: if minCompetitiveScore was raised above the constant
		// score, stop.
		if bs.scorable.minCompetitiveScore > bs.scorable.score {
			return NO_MORE_DOCS, nil
		}

		// Verify all iterators are at the same doc; advance lagging ones.
		allMatch := true
		for _, w := range iterators {
			d := w.docID()
			if d < doc {
				d, err = w.approximation.Advance(doc)
				if err != nil {
					return 0, err
				}
			}
			if d > doc {
				doc = d
				allMatch = false
				doc, err = lead.approximation.Advance(doc)
				if err != nil {
					return 0, err
				}
				break
			}
		}
		if !allMatch {
			continue
		}

		// Two-phase confirmation.
		confirmed := true
		for _, w := range iterators {
			if w.twoPhase != nil {
				ok2, err2 := w.twoPhase.Matches()
				if err2 != nil {
					return 0, err2
				}
				if !ok2 {
					confirmed = false
					break
				}
			}
		}

		if confirmed && (acceptDocs == nil || acceptDocs.Get(doc)) {
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
		}

		// Advance lead to next doc.
		doc, err = lead.approximation.NextDoc()
		if err != nil {
			return 0, err
		}
	}

	return doc, nil
}

// Cost returns the estimated number of matching documents (cost of the
// cheapest clause).
func (bs *DenseConjunctionBulkScorer) Cost() int64 {
	return bs.iterators[0].approximation.Cost()
}

var _ BulkScorer = (*DenseConjunctionBulkScorer)(nil)
