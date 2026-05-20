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

import (
	"slices"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// conjunctionDISI is a conjunction of DocIdSetIterators. It requires
// that all of its sub-iterators remain positioned on the same document
// at all times and iterates over the doc IDs present in every given
// iterator. Package-private, mirroring Java's package-private
// final class ConjunctionDISI.
//
// Ported from org.apache.lucene.search.ConjunctionDISI (Lucene 10.4.0).
type conjunctionDISI struct {
	lead1  DocIdSetIterator
	lead2  DocIdSetIterator
	others []DocIdSetIterator
}

// newConjunctionDISI builds a conjunctionDISI from a sorted list of at
// least two iterators.
func newConjunctionDISI(iterators []DocIdSetIterator) *conjunctionDISI {
	return &conjunctionDISI{
		lead1:  iterators[0],
		lead2:  iterators[1],
		others: iterators[2:],
	}
}

func (c *conjunctionDISI) DocID() int { return c.lead1.DocID() }

func (c *conjunctionDISI) NextDoc() (int, error) {
	doc, err := c.lead1.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return c.doNext(doc)
}

func (c *conjunctionDISI) Advance(target int) (int, error) {
	doc, err := c.lead1.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return c.doNext(doc)
}

func (c *conjunctionDISI) Cost() int64 { return c.lead1.Cost() }

func (c *conjunctionDISI) DocIDRunEnd() int { return c.lead1.DocID() + 1 }

// doNext advances all iterators until they agree on the same document.
func (c *conjunctionDISI) doNext(doc int) (int, error) {
	for {
		// Find agreement between the two cheapest iterators first.
		next2, err := c.lead2.Advance(doc)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if next2 != doc {
			doc, err = c.lead1.Advance(next2)
			if err != nil {
				return NO_MORE_DOCS, err
			}
			if doc != next2 {
				continue
			}
		}

		// Then find agreement with remaining iterators.
		advanced := false
		for _, other := range c.others {
			if other.DocID() < doc {
				next, err := other.Advance(doc)
				if err != nil {
					return NO_MORE_DOCS, err
				}
				if next > doc {
					// Iterator is ahead — advance lead1 to next and restart.
					doc, err = c.lead1.Advance(next)
					if err != nil {
						return NO_MORE_DOCS, err
					}
					advanced = true
					break
				}
			}
		}
		if advanced {
			continue
		}

		// All iterators agree on doc.
		return doc, nil
	}
}

// ─── bitSetConjunctionDISI ───────────────────────────────────────────────────

// bitSetConjunctionDISI is a conjunction between a lead DocIdSetIterator
// and one or more util.BitSetIterators. By checking bits directly it
// avoids the overhead of calling Advance on each BitSetIterator.
//
// Mirrors ConjunctionDISI.BitSetConjunctionDISI (private inner class).
type bitSetConjunctionDISI struct {
	lead        DocIdSetIterator
	bitSetIters []*util.BitSetIterator
	bitSets     []util.Bits
	minLength   int
}

func newBitSetConjunctionDISI(lead DocIdSetIterator, bsIters []*util.BitSetIterator) *bitSetConjunctionDISI {
	// Sort cheapest first so we exit early.
	slices.SortFunc(bsIters, func(a, b *util.BitSetIterator) int {
		return int(a.Cost() - b.Cost())
	})
	sets := make([]util.Bits, len(bsIters))
	minLen := int(^uint(0) >> 1) // math.MaxInt
	for i, it := range bsIters {
		bs := it.GetBitSet()
		sets[i] = bs
		if l := bs.Length(); l < minLen {
			minLen = l
		}
	}
	return &bitSetConjunctionDISI{
		lead:        lead,
		bitSetIters: bsIters,
		bitSets:     sets,
		minLength:   minLen,
	}
}

func (b *bitSetConjunctionDISI) DocID() int { return b.lead.DocID() }

func (b *bitSetConjunctionDISI) NextDoc() (int, error) {
	doc, err := b.lead.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return b.doNext(doc)
}

func (b *bitSetConjunctionDISI) Advance(target int) (int, error) {
	doc, err := b.lead.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	return b.doNext(doc)
}

func (b *bitSetConjunctionDISI) Cost() int64 { return b.lead.Cost() }

func (b *bitSetConjunctionDISI) DocIDRunEnd() int { return b.lead.DocID() + 1 }

func (b *bitSetConjunctionDISI) doNext(doc int) (int, error) {
	for {
		if doc >= b.minLength {
			if doc != NO_MORE_DOCS {
				_, err := b.lead.Advance(NO_MORE_DOCS)
				if err != nil {
					return NO_MORE_DOCS, err
				}
			}
			return NO_MORE_DOCS, nil
		}
		allMatch := true
		for _, bs := range b.bitSets {
			if !bs.Get(doc) {
				allMatch = false
				break
			}
		}
		if allMatch {
			for _, it := range b.bitSetIters {
				it.SetDocId(doc)
			}
			return doc, nil
		}
		var err error
		doc, err = b.lead.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
	}
}

// ─── internal helpers ────────────────────────────────────────────────────────

// scorerTwoPhaseProvider is an optional interface that Scorers may
// implement to expose their TwoPhaseIterator. This mirrors
// Scorer.twoPhaseIterator() in Java.
type scorerTwoPhaseProvider interface {
	TwoPhaseIterator() *TwoPhaseIterator
}

// addScorer decomposes a scorer into its DISI and TwoPhase parts and
// appends them to the accumulator slices.
//
// Mirrors ConjunctionDISI.addScorer (package-private static).
func addScorer(
	scorer Scorer,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	// Prefer explicit interface over type-switch heuristic.
	var twoPhase *TwoPhaseIterator
	if p, ok := scorer.(scorerTwoPhaseProvider); ok {
		twoPhase = p.TwoPhaseIterator()
	} else {
		twoPhase = AsTwoPhaseIterator(scorer)
	}
	if twoPhase != nil {
		addTwoPhaseIteratorToConjunction(twoPhase, allIterators, twoPhaseIterators)
	} else {
		addIteratorToConjunction(scorer, allIterators, twoPhaseIterators)
	}
}

// addIteratorToConjunction collapses nested ConjunctionDISIs and
// BitSetConjunctionDISIs into flat lists.
//
// Mirrors ConjunctionDISI.addIterator (package-private static).
func addIteratorToConjunction(
	disi DocIdSetIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	twoPhase := AsTwoPhaseIterator(disi)
	if twoPhase != nil {
		addTwoPhaseIteratorToConjunction(twoPhase, allIterators, twoPhaseIterators)
		return
	}
	switch c := disi.(type) {
	case *conjunctionDISI:
		// Collapse: take the already-split sub-iterators directly.
		*allIterators = append(*allIterators, c.lead1)
		*allIterators = append(*allIterators, c.lead2)
		*allIterators = append(*allIterators, c.others...)
	case *bitSetConjunctionDISI:
		*allIterators = append(*allIterators, c.lead)
		for _, it := range c.bitSetIters {
			*allIterators = append(*allIterators, it)
		}
	default:
		*allIterators = append(*allIterators, disi)
	}
}

// addTwoPhaseIteratorToConjunction appends the approximation DISI and
// the TwoPhaseIterator to the accumulator slices, collapsing nested
// ConjunctionTwoPhaseIterators.
//
// Mirrors ConjunctionDISI.addTwoPhaseIterator (package-private static).
func addTwoPhaseIteratorToConjunction(
	twoPhaseIter *TwoPhaseIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	addIteratorToConjunction(twoPhaseIter.Approximation(), allIterators, twoPhaseIterators)
	*twoPhaseIterators = append(*twoPhaseIterators, twoPhaseIter)
}

// createConjunction builds the final DocIdSetIterator from the
// accumulated DISI and TwoPhaseIterator lists. This is the central
// factory used by both IntersectScorers and IntersectIterators.
//
// Mirrors ConjunctionDISI.createConjunction (package-private static).
func createConjunction(
	allIterators []DocIdSetIterator,
	twoPhaseIterators []*TwoPhaseIterator,
) DocIdSetIterator {
	if len(allIterators) == 0 && len(twoPhaseIterators) == 0 {
		panic("search: createConjunction requires at least one iterator")
	}

	// Validate that all sub-iterators start on the same document.
	var curDoc int
	if len(allIterators) > 0 {
		curDoc = allIterators[0].DocID()
	} else {
		curDoc = twoPhaseIterators[0].Approximation().DocID()
	}
	var minCost int64 = 1<<62 - 1 // near max int64
	for _, it := range allIterators {
		if it.DocID() != curDoc {
			panic("search: sub-iterators of ConjunctionDISI are not on the same document")
		}
		if c := it.Cost(); c < minCost {
			minCost = c
		}
	}
	for _, it := range twoPhaseIterators {
		if it.Approximation().DocID() != curDoc {
			panic("search: sub-iterators of ConjunctionDISI are not on the same document")
		}
	}

	// Separate BitSetIterators from regular iterators.
	var bitSetIters []*util.BitSetIterator
	var iterators []DocIdSetIterator
	for _, it := range allIterators {
		if bsi, ok := it.(*util.BitSetIterator); ok && bsi.Cost() > minCost {
			// Only promote to bitSet path when not the cheapest iterator.
			bitSetIters = append(bitSetIters, bsi)
		} else {
			iterators = append(iterators, it)
		}
	}

	// Sort regular iterators by ascending cost so the cheapest leads.
	slices.SortFunc(iterators, func(a, b DocIdSetIterator) int {
		ca, cb := a.Cost(), b.Cost()
		if ca < cb {
			return -1
		}
		if ca > cb {
			return 1
		}
		return 0
	})

	var disi DocIdSetIterator
	if len(iterators) == 1 {
		disi = iterators[0]
	} else {
		disi = newConjunctionDISI(iterators)
	}

	if len(bitSetIters) > 0 {
		disi = newBitSetConjunctionDISI(disi, bitSetIters)
	}

	if len(twoPhaseIterators) > 0 {
		// Sort two-phase iterators by ascending matchCost.
		slices.SortFunc(twoPhaseIterators, func(a, b *TwoPhaseIterator) int {
			ca, cb := a.MatchCost(), b.MatchCost()
			if ca < cb {
				return -1
			}
			if ca > cb {
				return 1
			}
			return 0
		})
		matchFns := make([]func() (bool, error), len(twoPhaseIterators))
		var totalMatchCost float32
		for i, tpi := range twoPhaseIterators {
			tpi := tpi
			matchFns[i] = func() (bool, error) { return tpi.Matches() }
			totalMatchCost += tpi.MatchCost()
		}
		twoPhase := NewTwoPhaseIteratorWithMatchCost(disi, func() (bool, error) {
			for _, fn := range matchFns {
				ok, err := fn()
				if err != nil || !ok {
					return false, err
				}
			}
			return true, nil
		}, totalMatchCost)
		disi = NewTwoPhaseIteratorAsDocIdSetIterator(twoPhase)
	}

	return disi
}

// ─── ConjunctionUtils (public API) ───────────────────────────────────────────

// IntersectScorers creates a conjunction over the provided Scorers.
// The returned DocIdSetIterator may leverage two-phase iteration; use
// [AsTwoPhaseIterator] to retrieve the TwoPhaseIterator if available.
//
// Panics when len(scorers) < 2, mirroring Java's IllegalArgumentException.
//
// Mirrors ConjunctionUtils.intersectScorers.
func IntersectScorers(scorers []Scorer) DocIdSetIterator {
	if len(scorers) < 2 {
		panic("search: cannot make a ConjunctionDISI of fewer than 2 iterators")
	}
	var allIters []DocIdSetIterator
	var twoPhaseIters []*TwoPhaseIterator
	for _, s := range scorers {
		addScorer(s, &allIters, &twoPhaseIters)
	}
	return createConjunction(allIters, twoPhaseIters)
}

// IntersectIterators creates a conjunction over the provided
// DocIdSetIterators. The returned iterator may leverage two-phase
// iteration; use [AsTwoPhaseIterator] to retrieve the TwoPhaseIterator
// if available.
//
// Panics when len(iterators) < 2, mirroring Java's
// IllegalArgumentException.
//
// Mirrors ConjunctionUtils.intersectIterators.
func IntersectIterators(iterators []DocIdSetIterator) DocIdSetIterator {
	if len(iterators) < 2 {
		panic("search: cannot make a ConjunctionDISI of fewer than 2 iterators")
	}
	var allIters []DocIdSetIterator
	var twoPhaseIters []*TwoPhaseIterator
	for _, it := range iterators {
		addIteratorToConjunction(it, &allIters, &twoPhaseIters)
	}
	return createConjunction(allIters, twoPhaseIters)
}

// CreateConjunctionFromLists builds a conjunction from already-separated
// DISI and TwoPhaseIterator lists. Useful when the caller has already
// split scorers into approximations and confirmations.
//
// Mirrors ConjunctionUtils.createConjunction.
func CreateConjunctionFromLists(allIterators []DocIdSetIterator, twoPhaseIterators []*TwoPhaseIterator) DocIdSetIterator {
	return createConjunction(allIterators, twoPhaseIterators)
}

// AddTwoPhaseIteratorToConjunctionLists decomposes a TwoPhaseIterator
// and appends to the accumulator slices.
//
// Mirrors ConjunctionUtils.addTwoPhaseIterator.
func AddTwoPhaseIteratorToConjunctionLists(
	twoPhaseIter *TwoPhaseIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	addTwoPhaseIteratorToConjunction(twoPhaseIter, allIterators, twoPhaseIterators)
}

// AddIteratorToConjunctionLists decomposes a DocIdSetIterator and
// appends to the accumulator slices.
//
// Mirrors ConjunctionUtils.addIterator.
func AddIteratorToConjunctionLists(
	disi DocIdSetIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	addIteratorToConjunction(disi, allIterators, twoPhaseIterators)
}

// Compile-time checks.
var (
	_ DocIdSetIterator = (*conjunctionDISI)(nil)
	_ DocIdSetIterator = (*bitSetConjunctionDISI)(nil)
)
