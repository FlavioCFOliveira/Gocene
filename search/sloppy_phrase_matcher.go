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

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/SloppyPhraseMatcher.java

import (
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SloppyPhraseMatcher finds all slop-valid position-combinations (matches)
// encountered while traversing/hopping the PhrasePositions.
//
// The sloppy frequency contribution of a match depends on the distance:
// highest freq for distance=0 (exact match), and freq gets lower as distance
// gets higher. Example: for query "a b"~2, a document "x a b a y" can be
// matched twice: once for "a b" (distance=0), and once for "b a" (distance=2).
//
// Possibly not all valid combinations are encountered, because for efficiency
// we always propagate the least PhrasePosition. This allows to base on
// PriorityQueue and move forward faster. As result, for example, document
// "a b c b a" would score differently for queries "a b c"~4 and "c b a"~4,
// although they really are equivalent. Similarly, for doc "a b c b a f g",
// query "c b"~2 would get same score as "g f"~2, although "c b"~2 could be
// matched twice. We may want to fix this in the future (currently not, for
// performance reasons).
//
// Mirrors org.apache.lucene.search.SloppyPhraseMatcher.
type SloppyPhraseMatcher struct {
	phrasePositions []*PhrasePositions

	slop             int
	numPostings      int
	pq               *PhraseQueue // for advancing min position
	captureLeadMatch bool

	approximation        DocIdSetIterator
	impactsApproximation *ImpactsDISI

	end int // current largest phrase position

	leadPosition  int
	leadOffset    int
	leadEndOffset int
	leadOrd       int

	// hasRpts flags that there are repetitions (as checked in first candidate doc).
	hasRpts bool
	// checkedRpts flags to only check for repetitions in first candidate doc.
	checkedRpts      bool
	hasMultiTermRpts bool
	// rptGroups holds, in each group, the PPs that repeat each other (i.e. same
	// term), sorted by (query) offset.
	rptGroups [][]*PhrasePositions
	// rptStack is a temporary stack for switching colliding repeating pps.
	rptStack []*PhrasePositions

	positioned  bool
	matchLength int
	freqsLoaded bool

	matchCost float32
}

// NewSloppyPhraseMatcher builds a SloppyPhraseMatcher over the supplied
// postings.
//
// Mirrors SloppyPhraseMatcher(PhraseQuery.PostingsAndFreq[], int, ScoreMode,
// SimScorer, float, boolean). As in Java, scoreMode is accepted but not
// consulted: sloppy phrase queries always use dummy impacts.
func NewSloppyPhraseMatcher(
	postings []*postingsAndFreq,
	slop int,
	scoreMode ScoreMode,
	scorer SimScorer,
	matchCost float32,
	captureLeadMatch bool,
) *SloppyPhraseMatcher {
	m := &SloppyPhraseMatcher{
		matchCost:        matchCost,
		slop:             slop,
		numPostings:      len(postings),
		captureLeadMatch: captureLeadMatch,
	}
	m.pq = NewPhraseQueue(len(postings))
	m.phrasePositions = make([]*PhrasePositions, len(postings))
	for i := 0; i < len(postings); i++ {
		m.phrasePositions[i] =
			NewPhrasePositions(postings[i].postings, postings[i].position, i, postings[i].terms)
	}

	iterators := make([]DocIdSetIterator, len(postings))
	for i, p := range postings {
		iterators[i] = p.postings
	}
	m.approximation = IntersectIterators(iterators)
	// What would be a good upper bound of the sloppy frequency? A sum of the
	// sub frequencies would be correct, but it is usually so much higher than
	// the actual sloppy frequency that it doesn't help skip irrelevant
	// documents. As a consequence for now, sloppy phrase queries use dummy
	// impacts:
	impactsSource := &sloppyPhraseImpactsSource{}
	m.impactsApproximation =
		NewImpactsDISI(m.approximation, NewMaxScoreCache(impactsSource, newSimImpactScorer(scorer)))
	return m
}

// sloppyPhraseImpactsSource is the dummy ImpactsSource used by
// SloppyPhraseMatcher. It reports a single impact level whose only impact is
// (freq=Integer.MAX_VALUE, norm=1) and which covers the whole postings list.
//
// Mirrors the anonymous ImpactsSource/Impacts pair created in the Java
// SloppyPhraseMatcher constructor. The search-side ImpactsSource contract
// flattens Impacts into the source, so getImpacts()/numLevels()/
// getDocIdUpTo()/getImpacts(level) are exposed directly, and AdvanceShallow
// returns what MaxScoreCache.advanceShallow would have returned in Java, i.e.
// impacts.getDocIdUpTo(0).
type sloppyPhraseImpactsSource struct {
	impactBuffer [1]Impact
}

// AdvanceShallow is a no-op and reports that the single impact level covers
// every remaining document.
func (s *sloppyPhraseImpactsSource) AdvanceShallow(_ int) (int, error) {
	return NO_MORE_DOCS, nil
}

// NumLevels mirrors the dummy Impacts.numLevels(), which is always 1.
func (s *sloppyPhraseImpactsSource) NumLevels() int { return 1 }

// GetDocIDUpTo mirrors the dummy Impacts.getDocIdUpTo(int), which always
// reports DocIdSetIterator.NO_MORE_DOCS.
func (s *sloppyPhraseImpactsSource) GetDocIDUpTo(_ int) int { return NO_MORE_DOCS }

// GetImpacts mirrors the dummy Impacts.getImpacts(int), whose buffer holds the
// single entry add(Integer.MAX_VALUE, 1).
func (s *sloppyPhraseImpactsSource) GetImpacts(_ int) []Impact {
	s.impactBuffer[0] = Impact{Freq: math.MaxInt32, Norm: 1}
	return s.impactBuffer[:]
}

// Compile-time assertion: the dummy source satisfies the ImpactsSource
// contract consumed by MaxScoreCache.
var _ ImpactsSource = (*sloppyPhraseImpactsSource)(nil)

// Approximation returns the approximation that only matches documents that
// have all terms.
//
// Mirrors SloppyPhraseMatcher.approximation().
func (s *SloppyPhraseMatcher) Approximation() DocIdSetIterator {
	return s.approximation
}

// ImpactsApproximation returns the approximation that is aware of impacts.
//
// Mirrors SloppyPhraseMatcher.impactsApproximation().
func (s *SloppyPhraseMatcher) ImpactsApproximation() *ImpactsDISI {
	return s.impactsApproximation
}

// MaxFreq returns an upper bound on the number of possible matches on this
// document.
//
// Mirrors SloppyPhraseMatcher.maxFreq(). Freqs are loaded eagerly so MaxFreq
// can be called before ResetPositions in TOP_SCORES mode: PhraseScorer uses
// this to short-circuit non-competitive documents before paying the cost of
// ResetPositions + initPhrasePositions.
func (s *SloppyPhraseMatcher) MaxFreq() (float32, error) {
	var maxFreq float32
	for _, phrasePosition := range s.phrasePositions {
		freq, err := phrasePosition.Postings.Freq()
		if err != nil {
			return 0, err
		}
		phrasePosition.Freq = freq
		maxFreq += float32(phrasePosition.Freq)
	}
	s.freqsLoaded = true
	return maxFreq, nil
}

// ResetPositions is called after the approximation has been advanced, to load
// positions for matching.
//
// Mirrors SloppyPhraseMatcher.resetPositions().
func (s *SloppyPhraseMatcher) ResetPositions() error {
	if s.freqsLoaded {
		// Freqs already loaded by MaxFreq.
		s.freqsLoaded = false
	} else {
		// Freqs not yet loaded. Load them now.
		for _, phrasePosition := range s.phrasePositions {
			freq, err := phrasePosition.Postings.Freq()
			if err != nil {
				return err
			}
			phrasePosition.Freq = freq
		}
	}
	positioned, err := s.initPhrasePositions()
	if err != nil {
		return err
	}
	s.positioned = positioned
	s.matchLength = math.MaxInt32
	s.leadPosition = math.MaxInt32
	return nil
}

// SloppyWeight returns the slop-adjusted weight of the current match.
//
// Mirrors SloppyPhraseMatcher.sloppyWeight().
func (s *SloppyPhraseMatcher) SloppyWeight() float32 {
	return 1.0 / (1.0 + float32(s.matchLength))
}

// NextMatch finds the next match on the current document, returning false if
// there are none.
//
// Mirrors SloppyPhraseMatcher.nextMatch().
func (s *SloppyPhraseMatcher) NextMatch() (bool, error) {
	if !s.positioned {
		return false, nil
	}
	pp := s.pq.Pop()
	// if the pq is not full, then positioned == false
	if err := s.captureLead(pp); err != nil {
		return false, err
	}
	s.matchLength = s.end - pp.Position
	next := s.pq.Top().Position
	for {
		advanced, err := s.advancePP(pp)
		if err != nil {
			return false, err
		}
		if !advanced {
			break
		}
		if s.hasRpts {
			ok, err := s.advanceRpts(pp)
			if err != nil {
				return false, err
			}
			if !ok {
				break // pps exhausted
			}
		}
		if pp.Position > next { // done minimizing current match-length
			s.pq.Add(pp)
			if s.matchLength <= s.slop {
				return true, nil
			}
			pp = s.pq.Pop()
			next = s.pq.Top().Position
			// if the pq is not full, then positioned == false
			s.matchLength = s.end - pp.Position
		} else {
			matchLength2 := s.end - pp.Position
			if matchLength2 < s.matchLength {
				s.matchLength = matchLength2
			}
		}
		if err := s.captureLead(pp); err != nil {
			return false, err
		}
	}
	s.positioned = false
	return s.matchLength <= s.slop, nil
}

// captureLead mirrors SloppyPhraseMatcher.captureLead(PhrasePositions).
func (s *SloppyPhraseMatcher) captureLead(pp *PhrasePositions) error {
	if !s.captureLeadMatch {
		return nil
	}
	s.leadOrd = pp.Ord
	s.leadPosition = pp.Position + pp.Offset
	startOffset, err := pp.Postings.StartOffset()
	if err != nil {
		return err
	}
	s.leadOffset = startOffset
	endOffset, err := pp.Postings.EndOffset()
	if err != nil {
		return err
	}
	s.leadEndOffset = endOffset
	return nil
}

// StartPosition returns the start position of the current match.
//
// When a match is detected, the top postings is advanced until it has moved
// beyond its successor, to ensure that the match is of minimal width. This
// means that we need to record the lead position before it is advanced.
// However, the priority queue doesn't guarantee that the top postings is in
// fact the earliest in the list, so we need to cycle through all terms to
// check. This is slow, but Matches is slow anyway.
//
// Mirrors SloppyPhraseMatcher.startPosition().
func (s *SloppyPhraseMatcher) StartPosition() int {
	leadPosition := s.leadPosition
	for _, pp := range s.phrasePositions {
		if v := pp.Position + pp.Offset; v < leadPosition {
			leadPosition = v
		}
	}
	return leadPosition
}

// EndPosition returns the end position of the current match.
//
// Mirrors SloppyPhraseMatcher.endPosition().
func (s *SloppyPhraseMatcher) EndPosition() int {
	endPosition := s.leadPosition
	for _, pp := range s.phrasePositions {
		if pp.Ord != s.leadOrd {
			if v := pp.Position + pp.Offset; v > endPosition {
				endPosition = v
			}
		}
	}
	return endPosition
}

// StartOffset returns the start offset of the current match.
//
// Mirrors SloppyPhraseMatcher.startOffset().
func (s *SloppyPhraseMatcher) StartOffset() (int, error) {
	leadOffset := s.leadOffset
	for _, pp := range s.phrasePositions {
		startOffset, err := pp.Postings.StartOffset()
		if err != nil {
			return 0, err
		}
		if startOffset < leadOffset {
			leadOffset = startOffset
		}
	}
	return leadOffset, nil
}

// EndOffset returns the end offset of the current match.
//
// Mirrors SloppyPhraseMatcher.endOffset().
func (s *SloppyPhraseMatcher) EndOffset() (int, error) {
	endOffset := s.leadEndOffset
	for _, pp := range s.phrasePositions {
		if pp.Ord != s.leadOrd {
			ppEndOffset, err := pp.Postings.EndOffset()
			if err != nil {
				return 0, err
			}
			if ppEndOffset > endOffset {
				endOffset = ppEndOffset
			}
		}
	}
	return endOffset, nil
}

// GetMatchCost returns an estimate of the average cost of finding all matches
// on a document.
//
// Mirrors PhraseMatcher.getMatchCost().
func (s *SloppyPhraseMatcher) GetMatchCost() float32 {
	return s.matchCost
}

// advancePP advances a PhrasePosition and updates 'end'; it returns false if
// exhausted.
//
// Mirrors SloppyPhraseMatcher.advancePP(PhrasePositions).
func (s *SloppyPhraseMatcher) advancePP(pp *PhrasePositions) (bool, error) {
	ok, err := pp.NextPosition()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if pp.Position > s.end {
		s.end = pp.Position
	}
	return true, nil
}

// advanceRpts resolves a repeater collision caused by pp having just been
// advanced, by advancing the lesser of the two colliding pps. Note that there
// can only be one collision, as by the initialization there were no collisions
// before pp was advanced.
//
// Mirrors SloppyPhraseMatcher.advanceRpts(PhrasePositions).
func (s *SloppyPhraseMatcher) advanceRpts(pp *PhrasePositions) (bool, error) {
	if pp.RptGroup < 0 {
		return true, nil // not a repeater
	}
	rg := s.rptGroups[pp.RptGroup]
	bits, err := util.NewFixedBitSet(len(rg)) // for re-queuing after collisions are resolved
	if err != nil {
		return false, err
	}
	k0 := pp.RptInd
	for {
		k := s.collide(pp)
		if k < 0 {
			break
		}
		pp = lesser(pp, rg[k]) // always advance the lesser of the (only) two colliding pps
		advanced, err := s.advancePP(pp)
		if err != nil {
			return false, err
		}
		if !advanced {
			return false, nil // exhausted
		}
		if k != k0 { // careful: mark only those currently in the queue
			bits = util.FixedBitSetEnsureCapacity(bits, k)
			bits.Set(k) // mark that pp2 need to be re-queued
		}
	}
	// collisions resolved, now re-queue
	// empty (partially) the queue until seeing all pps advanced for resolving collisions
	n := 0
	// TODO would be good if we can avoid calling Cardinality() in each iteration!
	numBits := bits.Length() // larges bit we set
	for bits.Cardinality() > 0 {
		pp2 := s.pq.Pop()
		s.rptStack[n] = pp2
		n++
		if pp2.RptGroup >= 0 &&
			pp2.RptInd < numBits && // this bit may not have been set
			bits.Get(pp2.RptInd) {
			bits.Clear(pp2.RptInd)
		}
	}
	// add back to queue
	for i := n - 1; i >= 0; i-- {
		s.pq.Add(s.rptStack[i])
	}
	return true, nil
}

// lesser compares two pps, but only by position and offset.
//
// Mirrors SloppyPhraseMatcher.lesser(PhrasePositions, PhrasePositions).
func lesser(pp, pp2 *PhrasePositions) *PhrasePositions {
	if pp.Position < pp2.Position || (pp.Position == pp2.Position && pp.Offset < pp2.Offset) {
		return pp
	}
	return pp2
}

// collide returns the index of a pp2 colliding with pp, or -1 if none.
//
// Mirrors SloppyPhraseMatcher.collide(PhrasePositions).
func (s *SloppyPhraseMatcher) collide(pp *PhrasePositions) int {
	ppTpPos := tpPos(pp)
	rg := s.rptGroups[pp.RptGroup]
	for _, pp2 := range rg {
		if pp2 != pp && tpPos(pp2) == ppTpPos {
			return pp2.RptInd
		}
	}
	return -1
}

// initPhrasePositions initializes PhrasePositions in place. A one time
// initialization for this scorer (on first doc matching all terms):
//
//   - Check if there are repetitions
//   - If there are, find groups of repetitions.
//
// Examples:
//
//  1. no repetitions: "ho my"~2
//  2. repetitions: "ho my my"~2
//  3. repetitions: "my ho my"~2
//
// It returns false if PPs are exhausted (and so current doc will not be a
// match).
//
// Mirrors SloppyPhraseMatcher.initPhrasePositions().
func (s *SloppyPhraseMatcher) initPhrasePositions() (bool, error) {
	s.end = math.MinInt32
	if !s.checkedRpts {
		return s.initFirstTime()
	}
	if !s.hasRpts {
		if err := s.initSimple(); err != nil {
			return false, err
		}
		return true, nil // PPs available
	}
	return s.initComplex()
}

// initSimple handles the no-repeats case: the simplest and most common. It is
// important to keep this piece of the code simple and efficient.
//
// Mirrors SloppyPhraseMatcher.initSimple().
func (s *SloppyPhraseMatcher) initSimple() error {
	s.pq.Clear()
	// position pps and build queue from list
	for _, pp := range s.phrasePositions {
		if err := pp.FirstPosition(); err != nil {
			return err
		}
		if pp.Position > s.end {
			s.end = pp.Position
		}
		s.pq.Add(pp)
	}
	return nil
}

// initComplex handles the with-repeats case: not so simple.
//
// Mirrors SloppyPhraseMatcher.initComplex().
func (s *SloppyPhraseMatcher) initComplex() (bool, error) {
	if err := s.placeFirstPositions(); err != nil {
		return false, err
	}
	ok, err := s.advanceRepeatGroups()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil // PPs exhausted
	}
	s.fillQueue()
	return true, nil // PPs available
}

// placeFirstPositions moves all PPs to their first position.
//
// Mirrors SloppyPhraseMatcher.placeFirstPositions().
func (s *SloppyPhraseMatcher) placeFirstPositions() error {
	for _, pp := range s.phrasePositions {
		if err := pp.FirstPosition(); err != nil {
			return err
		}
	}
	return nil
}

// fillQueue fills the queue; all pps are already placed.
//
// Mirrors SloppyPhraseMatcher.fillQueue().
func (s *SloppyPhraseMatcher) fillQueue() {
	s.pq.Clear()
	for _, pp := range s.phrasePositions { // iterate cyclic list: done once handled max
		if pp.Position > s.end {
			s.end = pp.Position
		}
		s.pq.Add(pp)
	}
}

// advanceRepeatGroups advances the repetition groups at initialization.
//
// At initialization (each doc), each repetition group is sorted by (query)
// offset. This provides the start condition: no collisions.
//
// Case 1: no multi-term repeats. It is sufficient to advance each pp in the
// group by one less than its group index. So lesser pp is not advanced, 2nd
// one advance once, 3rd one advanced twice, etc.
//
// Case 2: multi-term repeats.
//
// It returns false if PPs are exhausted.
//
// Mirrors SloppyPhraseMatcher.advanceRepeatGroups().
func (s *SloppyPhraseMatcher) advanceRepeatGroups() (bool, error) {
	for _, rg := range s.rptGroups {
		if s.hasMultiTermRpts {
			// more involved, some may not collide
			incr := 1
			for i := 0; i < len(rg); i += incr {
				incr = 1
				pp := rg[i]
				for {
					k := s.collide(pp)
					if k < 0 {
						break
					}
					pp2 := lesser(pp, rg[k])
					// at initialization always advance pp with higher offset
					advanced, err := s.advancePP(pp2)
					if err != nil {
						return false, err
					}
					if !advanced {
						return false, nil // exhausted
					}
					if pp2.RptInd < i { // should not happen?
						incr = 0
						break
					}
				}
			}
		} else {
			// simpler, we know exactly how much to advance
			for j := 1; j < len(rg); j++ {
				for k := 0; k < j; k++ {
					ok, err := rg[j].NextPosition()
					if err != nil {
						return false, err
					}
					if !ok {
						return false, nil // PPs exhausted
					}
				}
			}
		}
	}
	return true, nil // PPs available
}

// initFirstTime initializes with checking for repeats. Heavy work, but done
// only for the first candidate doc.
//
// If there are repetitions, check if multi-term postings (MTP) are involved.
//
// Without MTP, once PPs are placed in the first candidate doc, repeats (and
// groups) are visible. With MTP, a more complex check is needed, up-front, as
// there may be "hidden collisions". For example P1 has {A,B}, P1 has {B,C},
// and the first doc is: "A C B". At start, P1 would point to "A", p2 to "C",
// and it will not be identified that P1 and P2 are repetitions of each other.
//
// The more complex initialization has two parts: (1) identification of
// repetition groups; (2) advancing repeat groups at the start of the doc. For
// (1), a possible solution is to just create a single repetition group, made
// of all repeating pps. But this would slow down the check for collisions, as
// all pps would need to be checked. Instead, we compute "connected regions" on
// the bipartite graph of postings and terms.
//
// Mirrors SloppyPhraseMatcher.initFirstTime().
func (s *SloppyPhraseMatcher) initFirstTime() (bool, error) {
	s.checkedRpts = true
	if err := s.placeFirstPositions(); err != nil {
		return false, err
	}

	rptTerms := s.repeatingTerms()
	s.hasRpts = !rptTerms.isEmpty()

	if s.hasRpts {
		s.rptStack = make([]*PhrasePositions, s.numPostings) // needed with repetitions
		rgs, err := s.gatherRptGroups(rptTerms)
		if err != nil {
			return false, err
		}
		s.sortRptGroups(rgs)
		ok, err := s.advanceRepeatGroups()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil // PPs exhausted
		}
	}

	s.fillQueue()
	return true, nil // PPs available
}

// sortRptGroups sorts each repetition group by (query) offset. Done only once
// (at first doc) and allows to initialize faster for each doc.
//
// Mirrors SloppyPhraseMatcher.sortRptGroups(ArrayList<ArrayList<PhrasePositions>>).
func (s *SloppyPhraseMatcher) sortRptGroups(rgs [][]*PhrasePositions) {
	s.rptGroups = make([][]*PhrasePositions, len(rgs))
	for i := 0; i < len(s.rptGroups); i++ {
		rg := make([]*PhrasePositions, len(rgs[i]))
		copy(rg, rgs[i])
		sort.SliceStable(rg, func(a, b int) bool { return rg[a].Offset < rg[b].Offset })
		s.rptGroups[i] = rg
		for j := 0; j < len(rg); j++ {
			rg[j].RptInd = j // we use this index for efficient re-queuing
		}
	}
}

// gatherRptGroups detects repetition groups. Done once, for the first doc.
//
// Mirrors SloppyPhraseMatcher.gatherRptGroups(LinkedHashMap<Term, Integer>).
func (s *SloppyPhraseMatcher) gatherRptGroups(rptTerms *termOrdMap) ([][]*PhrasePositions, error) {
	rpp := s.repeatingPPs(rptTerms)
	var res [][]*PhrasePositions
	if !s.hasMultiTermRpts {
		// simpler - no multi-terms - can base on positions in first doc
		for i := 0; i < len(rpp); i++ {
			pp := rpp[i]
			if pp.RptGroup >= 0 {
				continue // already marked as a repetition
			}
			tpPosI := tpPos(pp)
			for j := i + 1; j < len(rpp); j++ {
				pp2 := rpp[j]
				if pp2.RptGroup >= 0 || // already marked as a repetition
					pp2.Offset == pp.Offset || // not a repetition: two PPs are originally in same offset
					tpPos(pp2) != tpPosI { // not a repetition
					continue
				}
				// a repetition
				g := pp.RptGroup
				if g < 0 {
					g = len(res)
					pp.RptGroup = g
					rl := make([]*PhrasePositions, 0, 2)
					rl = append(rl, pp)
					res = append(res, rl)
				}
				pp2.RptGroup = g
				res[g] = append(res[g], pp2)
			}
		}
	} else {
		// more involved - has multi-terms
		bb, err := s.ppTermsBitSets(rpp, rptTerms)
		if err != nil {
			return nil, err
		}
		bb, err = unionTermGroups(bb)
		if err != nil {
			return nil, err
		}
		tg := termGroups(rptTerms, bb)
		distinct := make(map[int]struct{}, len(tg))
		for _, g := range tg {
			distinct[g] = struct{}{}
		}
		numDistinctGroupIds := len(distinct)
		// Java collects into HashSet<PhrasePositions> per group; a pp may be
		// reached through several of its repeating terms, so membership is
		// deduplicated before the group is materialised.
		tmp := make([]map[*PhrasePositions]struct{}, numDistinctGroupIds)
		order := make([][]*PhrasePositions, numDistinctGroupIds)
		for i := 0; i < numDistinctGroupIds; i++ {
			tmp[i] = make(map[*PhrasePositions]struct{})
		}
		for _, pp := range rpp {
			for _, t := range pp.Terms {
				if !rptTerms.containsKey(t) {
					continue
				}
				g := tg[termKeyOf(t)]
				if _, seen := tmp[g][pp]; !seen {
					tmp[g][pp] = struct{}{}
					order[g] = append(order[g], pp)
				}
				pp.RptGroup = g
			}
		}
		for _, hs := range order {
			res = append(res, hs)
		}
	}
	return res, nil
}

// tpPos returns the actual position in doc of a PhrasePosition; it relies on
// position = tpPos - offset.
//
// Mirrors SloppyPhraseMatcher.tpPos(PhrasePositions).
func tpPos(pp *PhrasePositions) int {
	return pp.Position + pp.Offset
}

// termKey is the value-comparable identity of an index.Term: the field name
// and the term bytes. It stands in for Java's Term.equals/hashCode when a Term
// is used as a map key.
type termKey struct {
	field string
	text  string
}

// termKeyOf builds the map key for t.
func termKeyOf(t *index.Term) termKey {
	if t == nil {
		return termKey{}
	}
	if t.Bytes == nil {
		return termKey{field: t.Field}
	}
	return termKey{field: t.Field, text: string(t.Bytes.ValidBytes())}
}

// termOrdMap is an insertion-ordered map from Term to ordinal, standing in for
// Java's LinkedHashMap<Term, Integer>: iteration order and
// keySet().toArray(Term[]::new) both follow insertion order.
type termOrdMap struct {
	ord   map[termKey]int
	terms []*index.Term
}

func newTermOrdMap() *termOrdMap {
	return &termOrdMap{ord: make(map[termKey]int)}
}

// put records t with the supplied ordinal, preserving insertion order.
func (m *termOrdMap) put(t *index.Term, ord int) {
	k := termKeyOf(t)
	if _, ok := m.ord[k]; !ok {
		m.terms = append(m.terms, t)
	}
	m.ord[k] = ord
}

// get returns the ordinal of t and whether it is present.
func (m *termOrdMap) get(t *index.Term) (int, bool) {
	ord, ok := m.ord[termKeyOf(t)]
	return ord, ok
}

// containsKey reports whether t is present.
func (m *termOrdMap) containsKey(t *index.Term) bool {
	_, ok := m.ord[termKeyOf(t)]
	return ok
}

// size returns the number of distinct terms recorded.
func (m *termOrdMap) size() int { return len(m.ord) }

// isEmpty reports whether no term is recorded.
func (m *termOrdMap) isEmpty() bool { return len(m.ord) == 0 }

// repeatingTerms finds repeating terms and assigns them ordinal values.
//
// Mirrors SloppyPhraseMatcher.repeatingTerms().
func (s *SloppyPhraseMatcher) repeatingTerms() *termOrdMap {
	tord := newTermOrdMap()
	tcnt := make(map[termKey]int)
	for _, pp := range s.phrasePositions {
		for _, t := range pp.Terms {
			k := termKeyOf(t)
			cnt := tcnt[k] + 1
			tcnt[k] = cnt
			if cnt == 2 {
				tord.put(t, tord.size())
			}
		}
	}
	return tord
}

// repeatingPPs finds repeating pps, and for each, if it has multi-terms,
// updates s.hasMultiTermRpts.
//
// Mirrors SloppyPhraseMatcher.repeatingPPs(HashMap<Term, Integer>).
func (s *SloppyPhraseMatcher) repeatingPPs(rptTerms *termOrdMap) []*PhrasePositions {
	var rp []*PhrasePositions
	for _, pp := range s.phrasePositions {
		for _, t := range pp.Terms {
			if rptTerms.containsKey(t) {
				rp = append(rp, pp)
				s.hasMultiTermRpts = s.hasMultiTermRpts || len(pp.Terms) > 1
				break
			}
		}
	}
	return rp
}

// ppTermsBitSets builds bit-sets: for each repeating pp, for each of its
// repeating terms, the term ordinal value is set.
//
// Mirrors SloppyPhraseMatcher.ppTermsBitSets(PhrasePositions[], HashMap<Term, Integer>).
func (s *SloppyPhraseMatcher) ppTermsBitSets(rpp []*PhrasePositions, tord *termOrdMap) ([]*util.FixedBitSet, error) {
	bb := make([]*util.FixedBitSet, 0, len(rpp))
	for _, pp := range rpp {
		b, err := util.NewFixedBitSet(tord.size())
		if err != nil {
			return nil, err
		}
		for _, t := range pp.Terms {
			if ord, ok := tord.get(t); ok {
				b.Set(ord)
			}
		}
		bb = append(bb, b)
	}
	return bb, nil
}

// unionTermGroups unions (term group) bit-sets until they are disjoint
// (O(n^^2)), and each group has different terms.
//
// Mirrors SloppyPhraseMatcher.unionTermGroups(ArrayList<FixedBitSet>).
func unionTermGroups(bb []*util.FixedBitSet) ([]*util.FixedBitSet, error) {
	var incr int
	for i := 0; i < len(bb)-1; i += incr {
		incr = 1
		j := i + 1
		for j < len(bb) {
			if fixedBitSetIntersects(bb[i], bb[j]) {
				if err := bb[i].Or(bb[j]); err != nil {
					return nil, err
				}
				bb = append(bb[:j], bb[j+1:]...)
				incr = 0
			} else {
				j++
			}
		}
	}
	return bb, nil
}

// fixedBitSetIntersects reports whether a and b have any bit in common.
//
// Stands in for org.apache.lucene.util.FixedBitSet.intersects(FixedBitSet),
// which util.FixedBitSet does not yet expose.
func fixedBitSetIntersects(a, b *util.FixedBitSet) bool {
	aw := a.GetBits()
	bw := b.GetBits()
	n := len(aw)
	if len(bw) < n {
		n = len(bw)
	}
	for i := 0; i < n; i++ {
		if aw[i]&bw[i] != 0 {
			return true
		}
	}
	return false
}

// termGroups maps each term to the single group that contains it.
//
// Mirrors SloppyPhraseMatcher.termGroups(LinkedHashMap<Term, Integer>,
// ArrayList<FixedBitSet>). Note that util.FixedBitSet.NextSetBit reports
// exhaustion as -1, where Java's FixedBitSet reports
// DocIdSetIterator.NO_MORE_DOCS; the loop condition is expressed against the
// Go sentinel.
func termGroups(tord *termOrdMap, bb []*util.FixedBitSet) map[termKey]int {
	tg := make(map[termKey]int)
	t := tord.terms
	for i := 0; i < len(bb); i++ { // i is the group no.
		bits := bb[i]
		for ord := bits.NextSetBit(0); ord != -1; {
			tg[termKeyOf(t[ord])] = i
			if ord+1 >= bits.Length() {
				break
			}
			ord = bits.NextSetBit(ord + 1)
		}
	}
	return tg
}
