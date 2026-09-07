// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// phraseScorer is the Go port of org.apache.lucene.search.PhraseScorer.
type phraseScorer struct {
	approximation       DocIdSetIterator
	impactsApproximation ImpactsDISI
	matcher             PhraseMatcher
	scoreMode           ScoreMode
	simScorer           SimScorer
	norms               index.NumericDocValues
	minCompetitiveScore float32
	freq                float32
	matchCost           float32
}

// NewPhraseScorer creates a new PhraseScorer for exact phrases.
func NewPhraseScorer(weight Weight, postings []index.PostingsEnum, positions []int, simScorer SimScorer, norms index.NumericDocValues) Scorer {
	mw, ok := weight.(*MultiPhraseWeight)
	if !ok {
		return nil
	}

	// Convert to format expected by NewExactPhraseMatcher
	matcherPostings := make([]struct {
		postings index.PostingsEnum
		offset   int
	}, len(postings))

	for i := 0; i < len(postings); i++ {
		matcherPostings[i].postings = postings[i]
		matcherPostings[i].offset = positions[i]
	}

	// Using 1.0 as default matchCost as in Lucene's default behavior for phrase matchers
	matcher := NewExactPhraseMatcher(matcherPostings, mw.needsScores, simScorer, 1.0)

	return &phraseScorer{
		approximation:       matcher.Approximation(),
		impactsApproximation: matcher.ImpactsApproximation(),
		matcher:             matcher,
		scoreMode:           mw.needsScores, // Using needsScores as a placeholder for ScoreMode if not available in MultiPhraseWeight
		simScorer:           simScorer,
		norms:               norms,
		matchCost:           matcher.GetMatchCost(),
	}
}

// NewSloppyPhraseScorer creates a new PhraseScorer for sloppy phrases.
func NewSloppyPhraseScorer(weight Weight, postings []index.PostingsEnum, positions []int, simScorer SimScorer, slop int, norms index.NumericDocValues) Scorer {
	mw, ok := weight.(*MultiPhraseWeight)
	if !ok {
		return nil
	}

	// Convert to format expected by NewSloppyPhraseMatcher
	matcherPostings := make([]struct {
		postings index.PostingsEnum
		position index.PostingsEnum
		terms    []byte
		freq     int
	}, len(postings))

	for i := 0; i < len(postings); i++ {
		// Use the first term of the array as the term representative for the matcher
		var termBytes []byte
		if len(mw.query.termArrays[i]) > 0 {
			termBytes = mw.query.termArrays[i][0].Bytes()
		}

		matcherPostings[i] = struct {
			postings index.PostingsEnum
			position index.PostingsEnum
			terms    []byte
			freq     int
		}{
			postings: postings[i],
			position: postings[i],
			terms:    termBytes,
			freq:     0,
		}
	}

	matcher := NewSloppyPhraseMatcher(
		matcherPostings,
		slop,
		mw.needsScores,
		simScorer,
		1.0,
		true,
	)

	return &phraseScorer{
		approximation:       matcher.Approximation(),
		impactsApproximation: matcher.ImpactsApproximation(),
		matcher:             matcher,
		scoreMode:           mw.needsScores,
		simScorer:           simScorer,
		norms:               norms,
		matchCost:           matcher.GetMatchCost(),
	}
}

func (s *phraseScorer) DocID() int {
	return s.approximation.DocID()
}

func (s *phraseScorer) Score() float32 {
	if s.freq == 0 {
		s.freq = s.matcher.SloppyWeight()
		for {
			ok, err := s.matcher.NextMatch()
			if err != nil || !ok {
				break
			}
			s.freq += s.matcher.SloppyWeight()
		}
	}
	var norm int64 = 1
	if s.norms != nil {
		if ok, err := s.norms.AdvanceExact(s.DocID()); err == nil && ok {
			norm = s.norms.LongValue()
		}
	}
	return s.simScorer.Score(s.freq, norm)
}

func (s *phraseScorer) Iterator() util.DocIdSetIterator {
	return s.TwoPhaseIterator().AsDocIdSetIterator()
}

func (s *phraseScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return &TwoPhaseIterator{
		approximation: s.approximation,
		matches: func() (bool, error) {
			if s.scoreMode == ScoreModeTopScores && s.minCompetitiveScore > 0 {
				maxFreq, err := s.matcher.MaxFreq()
				if err != nil {
					return false, err
				}
				var norm int64 = 1
				if s.norms != nil {
					if ok, err := s.norms.AdvanceExact(s.approximation.DocID()); err == nil && ok {
						norm = s.norms.LongValue()
					}
				}
				if s.simScorer.Score(maxFreq, norm) < s.minCompetitiveScore {
					return false, nil
				}
			}
			if err := s.matcher.ResetPositions(); err != nil {
				return false, err
			}
			s.freq = 0
			return s.matcher.NextMatch()
		},
		matchCost: func() float32 {
			return s.matchCost
		},
	}
}

func (s *phraseScorer) AdvanceShallow(target int) (int, error) {
	return s.impactsApproximation.AdvanceShallow(target)
}

func (s *phraseScorer) GetMaxScore(upTo int) (float32, error) {
	return s.impactsApproximation.GetMaxScore(upTo)
}

func (s *phraseScorer) SetMinCompetitiveScore(minScore float32) {
	s.minCompetitiveScore = minScore
	s.impactsApproximation.SetMinCompetitiveScore(minScore)
}

// BulkScorer implementation: delegates to DefaultBulkScorer as per Lucene.
func (s *phraseScorer) BulkScorer() BulkScorer {
	return NewDefaultBulkScorer(s)
}

var _ Scorer = (*phraseScorer)(nil)
