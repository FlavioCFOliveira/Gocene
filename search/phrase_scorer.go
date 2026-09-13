// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/PhraseScorer.java

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// phraseScorer is the Scorer produced by PhraseWeight.
//
// Mirrors org.apache.lucene.search.PhraseScorer, a package-private class.
type phraseScorer struct {
	BaseScorer
	approximation        DocIdSetIterator
	impactsApproximation *ImpactsDISI
	maxScoreCache        *MaxScoreCache
	matcher              PhraseMatcher
	scoreMode            ScoreMode
	simScorer            SimScorer
	norms                index.NumericDocValues
	matchCost            float32

	minCompetitiveScore float32
	freq                float32
}

// newPhraseScorer builds a phraseScorer over the supplied matcher.
//
// Mirrors PhraseScorer(PhraseMatcher, ScoreMode, SimScorer, NumericDocValues).
func newPhraseScorer(matcher PhraseMatcher, scoreMode ScoreMode, simScorer SimScorer, norms index.NumericDocValues) *phraseScorer {
	s := &phraseScorer{
		matcher:              matcher,
		scoreMode:            scoreMode,
		simScorer:            simScorer,
		norms:                norms,
		matchCost:            matcher.GetMatchCost(),
		approximation:        matcher.Approximation(),
		impactsApproximation: matcher.ImpactsApproximation(),
	}
	s.maxScoreCache = s.impactsApproximation.GetMaxScoreCache()
	return s
}

// phraseTwoPhaseVerifier is the anonymous TwoPhaseIterator body returned by
// PhraseScorer.twoPhaseIterator().
type phraseTwoPhaseVerifier struct {
	scorer *phraseScorer
}

// Matches mirrors the anonymous TwoPhaseIterator.matches().
func (v phraseTwoPhaseVerifier) Matches() (bool, error) {
	s := v.scorer
	if s.scoreMode == ScoreModeTopScores && s.minCompetitiveScore > 0 {
		maxFreq, err := s.matcher.MaxFreq()
		if err != nil {
			return false, err
		}
		var norm int64 = 1
		if s.norms != nil {
			ok, err := s.norms.AdvanceExact(s.DocID())
			if err != nil {
				return false, err
			}
			if ok {
				norm, err = s.norms.LongValue()
				if err != nil {
					return false, err
				}
			}
		}
		if s.simScorer.Score104(maxFreq, norm) < s.minCompetitiveScore {
			// The maximum score we could get is less than the min competitive score
			return false, nil
		}
	}
	if err := s.matcher.ResetPositions(); err != nil {
		return false, err
	}
	s.freq = 0
	return s.matcher.NextMatch()
}

// MatchCost mirrors the anonymous TwoPhaseIterator.matchCost().
func (v phraseTwoPhaseVerifier) MatchCost() float32 {
	return v.scorer.matchCost
}

// TwoPhaseIterator returns the two-phase view of this scorer.
//
// Mirrors PhraseScorer.twoPhaseIterator().
func (s *phraseScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return NewTwoPhaseIterator(s.approximation, phraseTwoPhaseVerifier{scorer: s})
}

// DocID returns the doc ID that is currently being scored.
//
// Mirrors PhraseScorer.docID().
func (s *phraseScorer) DocID() int {
	return s.approximation.DocID()
}

// Score returns the score of the current document.
//
// Mirrors PhraseScorer.score().
func (s *phraseScorer) Score() (float32, error) {
	if s.freq == 0 {
		s.freq = s.matcher.SloppyWeight()
		for {
			ok, err := s.matcher.NextMatch()
			if err != nil {
				return 0, err
			}
			if !ok {
				break
			}
			s.freq += s.matcher.SloppyWeight()
		}
	}
	var norm int64 = 1
	if s.norms != nil {
		ok, err := s.norms.AdvanceExact(s.DocID())
		if err != nil {
			return 0, err
		}
		if ok {
			norm, err = s.norms.LongValue()
			if err != nil {
				return 0, err
			}
		}
	}
	return s.simScorer.Score104(s.freq, norm), nil
}

// Iterator returns a DocIdSetIterator over matching documents.
//
// Mirrors PhraseScorer.iterator().
func (s *phraseScorer) Iterator() DocIdSetIterator {
	return AsDocIdSetIterator(s.TwoPhaseIterator())
}

// SetMinCompetitiveScore records the minimum competitive score and forwards it
// to the impacts approximation.
//
// Mirrors PhraseScorer.setMinCompetitiveScore(float).
func (s *phraseScorer) SetMinCompetitiveScore(minScore float32) error {
	s.minCompetitiveScore = minScore
	s.impactsApproximation.SetMinCompetitiveScore(minScore)
	return nil
}

// AdvanceShallow advances to the block of documents that contains target.
//
// Mirrors PhraseScorer.advanceShallow(int).
func (s *phraseScorer) AdvanceShallow(target int) (int, error) {
	return s.maxScoreCache.AdvanceShallow(target)
}

// GetMaxScore returns the maximum score up to and including upTo.
//
// Mirrors PhraseScorer.getMaxScore(int).
func (s *phraseScorer) GetMaxScore(upTo int) (float32, error) {
	return s.maxScoreCache.GetMaxScore(upTo)
}

// NextDocsAndScores carries the inherited default body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer), which
// PhraseScorer does not override.
func (s *phraseScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ Scorer = (*phraseScorer)(nil)
