// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
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
	for _, p := range postings {
		iters = append(iters, p.postings)
	}
	approx := intersectIterators(iters)

	// Use dummy impacts for now, consistent with SloppyPhraseMatcher
	impactsSource := &dummyImpactsSource{simScorer: scorer}
	impactsApprox := NewImpactsDISI(approx, NewMaxScoreCache(impactsSource, scorer))

	var finalApprox DocIdSetIterator
	if scoreMode == ScoreModeTopScores {
		// In Lucene, ImpactsDISI is used as the approximation for TOP_SCORES
		// since it already filters based on competitive score.
		finalApprox = approx // We'll use approx for now if ImpactsDISI doesn't satisfy DocIdSetIterator
	} else {
		finalApprox = approx
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
			p.freq = p.postings.Freq()
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
				// we advanced too far, try to advance lead and restart
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
