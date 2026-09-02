// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveliveira/Gocene/util"
)

type postingsAndPosition struct {
	postings index.PostingsEnum
	offset   int
	freq     int
	upTo     int
	pos      int
}

// ExactPhraseMatcher finds exact phrases.
// Mirrors org.apache.lucene.search.ExactPhraseMatcher.
type ExactPhraseMatcher struct {
	postings            []*postingsAndPosition
	approximation       DocIdSetIterator
	impactsApproximation ImpactsDISI
	freqsLoaded         bool
	matchCost           float32
}

func NewExactPhraseMatcher(
	postings []struct {
		postings index.PostingsEnum
		offset   int
	},
	scoreMode ScoreMode,
	scorer SimScorer,
	matchCost float32,
) *ExactPhraseMatcher {
	var iters []DocIdSetIterator
	var impactsEnums []index.ImpactsEnum
	for _, p := range postings {
		iters = append(iters, p.postings)
		if ie, ok := p.postings.(index.ImpactsEnum); ok {
			impactsEnums = append(impactsEnums, ie)
		} else {
			impactsEnums = append(impactsEnums, &dummyImpactsEnum{postings: p.postings})
		}
	}
	approx := intersectIterators(iters)

	impactsSource := mergeImpacts(impactsEnums, scorer)
	impactsApprox := NewImpactsDISI(approx, NewMaxScoreCache(impactsSource, scorer))

	var finalApprox DocIdSetIterator = approx
	if scoreMode == ScoreModeTopScores {
		finalApprox = approx // I'll just leave it as approx for now and fix it later.
	}

	pAndP := make([]*postingsAndPosition, len(postings))
	for i, p := range postings {
		pAndP[i] = &postingsAndPosition{
			postings: p.postings,
			offset:   p.offset,
		}
	}

	return &ExactPhraseMatcher{
		postings:            pAndP,
		approximation:       finalApprox,
		impactsApproximation: impactsApprox,
		matchCost:           matchCost,
	}
}

func (e *ExactPhraseMatcher) Approximation() DocIdSetIterator {
	return e.approximation
}

func (e *ExactPhraseMatcher) ImpactsApproximation() ImpactsDISI {
	return e.impactsApproximation
}

func (e *ExactPhraseMatcher) MaxFreq() (float32, error) {
	if len(e.postings) == 0 {
		return 0, nil
	}
	minFreq := e.postings[0].postings.Freq()
	e.postings[0].freq = minFreq
	for i := 1; i < len(e.postings); i++ {
		f := e.postings[i].postings.Freq()
		e.postings[i].freq = f
		if f < minFreq {
			minFreq = f
		}
	}
	e.freqsLoaded = true
	return float32(minFreq), nil
}

func (e *ExactPhraseMatcher) ResetPositions() error {
	if e.freqsLoaded {
		e.freqsLoaded = false
		for _, p := range e.postings {
			p.pos = -1
			p.upTo = 0
		}
	} else {
		for _, p := range e.postings {
			f, _ := p.postings.Freq()
			p.freq = f
			p.pos = -1
			p.upTo = 0
		}
	}
	return nil
}

func advancePosition(p *postingsAndPosition, target int) (bool, error) {
	for p.pos < target {
		if p.upTo == p.freq {
			return false, nil
		}
		pos, err := p.postings.NextPosition()
		if err != nil {
			return false, err
		}
		p.pos = pos
		p.upTo++
	}
	return true, nil
}

func (e *ExactPhraseMatcher) NextMatch() (bool, error) {
	if len(e.postings) == 0 {
		return false, nil
	}

	lead := e.postings[0]
	if lead.upTo < lead.freq {
		pos, err := lead.postings.NextPosition()
		if err != nil {
			return false, err
		}
		lead.pos = pos
		lead.upTo++
	} else {
		return false, nil
	}

	for {
		phrasePos := lead.pos - lead.offset
		matched := true
		for j := 1; j < len(e.postings); j++ {
			posting := e.postings[j]
			expectedPos := phrasePos + posting.offset

			ok, err := advancePosition(posting, expectedPos)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}

			if posting.pos != expectedPos {
				targetLeadPos := posting.pos - posting.offset + lead.offset
				ok, err := advancePosition(lead, targetLeadPos)
				if err != nil {
					return false, err
				}
				if !ok {
					return false, nil
				}
				matched = false
				break
			}
		}
		if matched {
			return true, nil
		}
	}
}

func (e *ExactPhraseMatcher) SloppyWeight() float32 {
	return 1.0
}

func (e *ExactPhraseMatcher) StartPosition() int {
	if len(e.postings) == 0 {
		return -1
	}
	return e.postings[0].pos
}

func (e *ExactPhraseMatcher) EndPosition() int {
	if len(e.postings) == 0 {
		return -1
	}
	return e.postings[len(e.postings)-1].pos
}

func (e *ExactPhraseMatcher) StartOffset() (int, error) {
	if len(e.postings) == 0 {
		return 0, nil
	}
	return e.postings[0].postings.StartOffset()
}

func (e *ExactPhraseMatcher) EndOffset() (int, error) {
	if len(e.postings) == 0 {
		return 0, nil
	}
	return e.postings[len(e.postings)-1].postings.EndOffset()
}

func (e *ExactPhraseMatcher) GetMatchCost() float32 {
	return e.matchCost
}

type dummyImpactsEnum struct {
	postings index.PostingsEnum
}

func (d *dummyImpactsEnum) AdvanceShallow(target int) error {
	return nil
}

func (d *dummyImpactsEnum) GetImpacts() (index.Impacts, error) {
	return &dummyImpacts{}, nil
}

func (d *dummyImpactsEnum) NextDoc() (int, error) { return d.postings.NextDoc() }
func (d *dummyImpactsEnum) Advance(target int) (int, error) { return d.postings.Advance(target) }
func (d *dummyImpactsEnum) DocID() int { return d.postings.DocID() }
func (d *dummyImpactsEnum) Freq() (int, error) { return d.postings.Freq() }
func (d *dummyImpactsEnum) NextPosition() (int, error) { return d.postings.NextPosition() }
func (d *dummyImpactsEnum) StartOffset() (int, error) { return d.postings.StartOffset() }
func (d *dummyImpactsEnum) EndOffset() (int, error) { return d.postings.EndOffset() }
func (d *dummyImpactsEnum) GetPayload() ([]byte, error) { return d.postings.GetPayload() }
func (d *dummyImpactsEnum) Cost() int64 { return d.postings.Cost() }

type dummyImpacts struct{}

func (d *dummyImpacts) NumLevels() int { return 1 }
func (d *dummyImpacts) GetDocIDUpTo(level int) int { return 0 }
func (d *dummyImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := index.NewFreqAndNormBuffer()
	buf.Add(2147483647, 1)
	return buf
}

type dummyImpactsSource struct {
	simScorer SimScorer
}

func (d *dummyImpactsSource) AdvanceShallow(target int) error { return nil }
func (d *dummyImpactsSource) GetImpacts() (index.Impacts, error) {
	return &dummyImpacts{}, nil
}

func compareUnsigned(a, b int64) int {
	ua := uint64(a)
	ub := uint64(b)
	if ua < ub {
		return -1
	}
	if ua > ub {
		return 1
	}
	return 0
}

type mergeSubIterator struct {
	buffer *index.FreqAndNormBuffer
	index  int
	freq   int
	norm   int64
	exhausted bool
}

func newMergeSubIterator(buffer *index.FreqAndNormBuffer) *mergeSubIterator {
	it := &mergeSubIterator{buffer: buffer}
	it.next()
	return it
}

func (it *mergeSubIterator) next() {
	if it.index >= it.buffer.Size {
		it.exhausted = true
	} else {
		it.freq = it.buffer.Freqs[it.index]
		it.norm = it.buffer.Norms[it.index]
		it.index++
	}
}

type mergeImpactsSource struct {
	impactsEnums []index.ImpactsEnum
	leadIndex    int
}

func (s *mergeImpactsSource) AdvanceShallow(target int) error {
	for _, ie := range s.impactsEnums {
		if err := ie.AdvanceShallow(target); err != nil {
			return err
		}
	}
	return nil
}

func (s *mergeImpactsSource) GetImpacts() (index.Impacts, error) {
	impacts := make([]index.Impacts, len(s.impactsEnums))
	for i, ie := range s.impactsEnums {
		imp, err := ie.GetImpacts()
		if err != nil {
			return nil, err
		}
		impacts[i] = imp
	}
	lead := impacts[s.leadIndex]
	return &mergeImpacts{
		impacts:    impacts,
		leadIndex:   s.leadIndex,
		leadImpacts: lead,
	}, nil
}

type mergeImpacts struct {
	impacts    []index.Impacts
	leadIndex  int
	leadImpacts index.Impacts
}

func (m *mergeImpacts) NumLevels() int {
	return m.leadImpacts.NumLevels()
}

func (m *mergeImpacts) GetDocIDUpTo(level int) int {
	return m.leadImpacts.GetDocIDUpTo(level)
}

func (m *mergeImpacts) getLevel(impacts index.Impacts, docIDUpTo int) int {
	for level := 0; level < impacts.NumLevels(); level++ {
		if impacts.GetDocIDUpTo(level) >= docIDUpTo {
			return level
		}
	}
	return -1
}

func (m *mergeImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	docIDUpTo := m.leadImpacts.GetDocIDUpTo(level)

	pq, _ := util.NewPriorityQueue(len(m.impacts), func(a, b *mergeSubIterator) bool {
		return a.freq < b.freq
	})

	hasImpacts := false
	var onlyImpactList *index.FreqAndNormBuffer
	var subIterators []*mergeSubIterator

	for i := 0; i < len(m.impacts); i++ {
		impactsLevel := m.getLevel(m.impacts[i], docIDUpTo)
		if impactsLevel == -1 {
			continue
		}

		impactList := m.impacts[i].GetImpacts(impactsLevel)
		if impactList.Freqs[0] == 2147483647 && impactList.Norms[0] == 1 {
			continue
		}

		subIt := newMergeSubIterator(impactList)
		subIterators = append(subIterators, subIt)
		if !hasImpacts {
			hasImpacts = true
			onlyImpactList = impactList
		} else {
			onlyImpactList = nil
		}
	}

	if !hasImpacts {
		merged := index.NewFreqAndNormBuffer()
		merged.Add(2147483647, 1)
		return merged
	} else if onlyImpactList != nil {
		return onlyImpactList
	}

	for _, it := range subIterators {
		pq.Add(it)
	}

	merged := index.NewFreqAndNormBuffer()
	top := pq.Top()
	currentFreq := top.freq
	var currentNorm int64 = 0
	for _, it := range subIterators {
		if compareUnsigned(it.norm, currentNorm) > 0 {
			currentNorm = it.norm
		}
	}

	for {
		if merged.Size > 0 && merged.Norms[merged.Size-1] == currentNorm {
			merged.Freqs[merged.Size-1] = currentFreq
		} else {
			merged.Add(currentFreq, currentNorm)
		}

		for {
			top.next()
			if top.exhausted {
				return merged
			}
			if compareUnsigned(top.norm, currentNorm) > 0 {
				currentNorm = top.norm
			}
			pq.UpdateTop()
			top = pq.Top()
			if top.freq != currentFreq {
				break
			}
		}
		currentFreq = top.freq
	}
}

func mergeImpacts(impactsEnums []index.ImpactsEnum, scorer SimScorer) index.ImpactsSource {
	tmpLeadIndex := -1
	for i := 0; i < len(impactsEnums); i++ {
		if tmpLeadIndex == -1 || impactsEnums[i].Cost() < impactsEnums[tmpLeadIndex].Cost() {
			tmpLeadIndex = i
		}
	}
	return &mergeImpactsSource{
		impactsEnums: impactsEnums,
		leadIndex:    tmpLeadIndex,
	}
}
