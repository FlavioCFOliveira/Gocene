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
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not distribute the Software without (any Hera) permission.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SloppyPhraseMatcher finds all slop-valid position-combinations encountered
// while traversing the PhrasePositions.
type SloppyPhraseMatcher struct {
	phrasePositions []*PhrasePositions
	slop            int
	numPostings     int
	pq              PhraseQueue
	captureLead     bool
	approximation    DocIdSetIterator
	impactsApprox    ImpactsDISI
	end             int
	leadPosition    int
	leadOffset      int
	leadEndOffset    int
	leadOrd         int
	hasRpts         bool
	checkedRpts     bool
	hasMultiTermRpts bool
	rptGroups       [][]*PhrasePositions
	rptStack        []*PhrasePositions
	positioned      bool
	matchLength     int
	freqsLoaded     bool
}

func NewSloppyPhraseMatcher(
	postings []struct {
		postings index.PostingsEnum
		position index.PostingsEnum // This is actually a positions enum
		terms    []byte
		freq     int
	},
	slop int,
	scoreMode ScoreMode,
	simScorer SimScorer,
	matchCost float32,
	captureLead bool,
) *SloppyPhraseMatcher {
	numPostings := len(postings)
	pp := make([]*PhrasePositions, numPostings)
	var iters []DocIdSetIterator

	for i := 0; i < numPostings; i++ {
		pp[i] = NewPhrasePositions(postings[i].postings, 0, i, postings[i].terms)
		iters = append(iters, postings[i].postings)
	}

	approx := intersectIterators(iters)

	// For now, we use dummy impacts as in Lucene
	impactsSource := &dummyImpactsSource{simScorer: simScorer}
	impactsApprox := NewImpactsDISI(approx, NewMaxScoreCache(impactsSource, simScorer))

	return &SloppyPhraseMatcher{
		phrasePositions: pp,
		slop:            slop,
		numPostings:     numPostings,
		pq:              make(PhraseQueue, 0, numPostings),
		captureLead:     captureLead,
		approximation:   approx,
		impactsApprox:    impactsApprox,
	}
}

func intersectIterators(iters []DocIdSetIterator) DocIdSetIterator {
	if len(iters) == 0 {
		return nil
	}
	if len(iters) == 1 {
		return iters[0]
	}
	// Simplified intersection for now, usually a ConjunctionIterator
	return &conjunctionIterator{iters: iters}
}

type conjunctionIterator struct {
	iters []DocIdSetIterator
	doc   int
}

func (ci *conjunctionIterator) NextDoc() (int, error) {
	if ci.doc == -1 {
		ci.doc, _ = ci.iters[0].NextDoc()
	} else {
		ci.doc, _ = ci.iters[0].NextDoc()
	}
	for i := 1; i < len(ci.iters); i++ {
		for ci.doc != NO_MORE_DOCS && ci.doc != ci.iters[i].DocID() {
			ci.doc, _ = ci.iters[i].Advance(ci.doc)
		}
	}
	return ci.doc, nil
}

func (ci *conjunctionIterator) Advance(target int) (int, error) {
	ci.doc, _ = ci.iters[0].Advance(target)
	for i := 1; i < len(ci.iters); i++ {
		for ci.doc != NO_MORE_DOCS && ci.doc != ci.iters[i].DocID() {
			ci.doc, _ = ci.iters[i].Advance(ci.doc)
		}
	}
	return ci.doc, nil
}

func (ci *conjunctionIterator) DocID() int {
	return ci.doc
}

func (ci *conjunctionIterator) Cost() int64 {
	return 1
}

func (ci *conjunctionIterator) DocIDRunEnd() int {
	return -1
}

type dummyImpactsSource struct {
	simScorer SimScorer
}

func (s *dummyImpactsSource) getImpacts() (Impacts, error) {
	return &dummyImpacts{
		impactBuffer: &index.FreqAndNormBuffer{
			// Just one level with a very high freq
		},
	}, nil
}

func (s *dummyImpactsSource) advanceShallow(target int) error {
	return nil
}

type dummyImpacts struct {
	impactBuffer *index.FreqAndNormBuffer
}

func (d *dummyImpacts) numLevels() int { return 1 }
func (d *dummyImpacts) getImpacts(level int) *index.FreqAndNormBuffer { return d.impactBuffer }
func (d *dummyImpacts) getDocIdUpTo(level int) int { return NO_MORE_DOCS }

func (s *SloppyPhraseMatcher) Approximation() DocIdSetIterator {
	return s.approximation
}

func (s *SloppyPhraseMatcher) ImpactsApproximation() ImpactsDISI {
	return s.impactsApprox
}

func (s *SloppyPhraseMatcher) MaxFreq() (float32, error) {
	var maxFreq float32
	for _, pp := range s.phrasePositions {
		maxFreq += float32(pp.postings.Freq())
	}
	s.freqsLoaded = true
	return maxFreq, nil
}

func (s *SloppyPhraseMatcher) ResetPositions() error {
	if s.freqsLoaded {
		s.freqsLoaded = false
	}
	for _, pp := range s.phrasePositions {
		pp.firstPosition()
	}
	s.positioned = true
	s.end = -1
	for _, pp := range s.phrasePositions {
		if pp.position > s.end {
			s.end = pp.position
		}
	}
	s.pq.Clear()
	for _, pp := range s.phrasePositions {
		s.pq.Add(pp)
	}
	s.matchLength = math.MaxInt32
	s.leadPosition = math.MaxInt32
	return nil
}

func (s *SloppyPhraseMatcher) NextMatch() (bool, error) {
	if !s.positioned {
		return false, nil
	}

	pp := s.pq.PopMin()
	if s.captureLead {
		s.leadOrd = pp.ord
		s.leadPosition = pp.position + pp.offset
		s.leadOffset = pp.postings.StartOffset()
		s.leadEndOffset = pp.postings.EndOffset()
	}

	s.matchLength = s.end - pp.position
	next := s.pq.Top().position

	for {
		ok, err := pp.nextPosition()
		if err != nil || !ok {
			break
		}
		if pp.position > s.end {
			s.end = pp.position
		}
		if pp.position > next {
			s.pq.Add(pp)
			if s.matchLength <= s.slop {
				return true, nil
			}
			pp = s.pq.PopMin()
			next = s.pq.Top().position
		} else {
			ml := s.end - pp.position
			if ml < s.matchLength {
				s.matchLength = ml
			}
		}
	}

	s.positioned = false
	return s.matchLength <= s.slop, nil
}

func (s *SloppyPhraseMatcher) SloppyWeight() float32 {
	return 1.0 / (1.0 + float32(s.matchLength))
}

func (s *SloppyPhraseMatcher) StartPosition() int {
	leadPos := s.leadPosition
	for _, pp := range s.phrasePositions {
		if pos := pp.position + pp.offset; pos < leadPos {
			leadPos = pos
		}
	}
	return leadPos
}

func (s *SloppyPhraseMatcher) EndPosition() int {
	endPos := s.leadPosition
	for _, pp := range s.phrasePositions {
		if pp.ord != s.leadOrd {
			if pos := pp.position + pp.offset; pos > endPos {
				endPos = pos
			}
		}
	}
	return endPos
}

func (s *SloppyPhraseMatcher) StartOffset() (int, error) {
	leadOff := s.leadOffset
	for _, pp := range s.phrasePositions {
		if off := pp.postings.StartOffset(); off < leadOff {
			leadOff = off
		}
	}
	return leadOff, nil
}

func (s *SloppyPhraseMatcher) EndOffset() (int, error) {
	endOff := s.leadEndOffset
	for _, pp := range s.phrasePositions {
		if pp.ord != s.leadOrd {
			if off := pp.postings.EndOffset(); off > endOff {
				endOff = off
			}
		}
	}
	return endOff, nil
}

func (s *SloppyPhraseMatcher) GetMatchCost() float32 {
	return 1.0 // Default cost
}

func (s *SloppyPhraseMatcher) initFirstTime() {
	s.hasRpts = false
	for i := 0; i < s.numPostings; i++ {
		for j := i + 1; j < s.numPostings; j++ {
			if string(s.phrasePositions[i].terms) == string(s.phrasePositions[j].terms) {
				s.hasRpts = true
				break
			}
		}
		if s.hasRpts {
			break
		}
	}

	if s.hasRpts {
		s.gatherRptGroups()
	}
}

func (s *SloppyPhraseMatcher) gatherRptGroups() {
	s.rptGroups = make([][]*PhrasePositions, 0)
	for i := 0; i < s.numPostings; i++ {
		if s.isInRptGroup(i) {
			continue
		}
		group := []*PhrasePositions{s.phrasePositions[i]}
		for j := i + 1; j < s.numPostings; j++ {
			if string(s.phrasePositions[i].terms) == string(s.phrasePositions[j].terms) {
				group = append(group, s.phrasePositions[j])
			}
		}
		if len(group) > 1 {
			s.rptGroups = append(s.rptGroups, group)
		}
	}
}

func (s *SloppyPhraseMatcher) isInRptGroup(idx int) bool {
	for _, group := range s.rptGroups {
		for _, pp := range group {
			if pp == s.phrasePositions[idx] {
				return true
			}
		}
	}
	return false
}

func (s *SloppyPhraseMatcher) advanceRpts() bool {
	// Simplified logic for repeating terms:
	// In a full implementation, this would manage the rptStack and advance
	// repeated terms based on the current match's requirements.
	return true
}
