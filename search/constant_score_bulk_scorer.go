// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConstantScoreBulkScorer is a bulk scorer for constant-score iterators that batches
// doc IDs via DocIdSetIterator.IntoBitSet.
//
// Mirrors org.apache.lucene.search.ConstantScoreBulkScorer (Lucene 10.5.0).
type ConstantScoreBulkScorer struct {
	scorer        Scorer
	iterator      DocIdSetIterator
	twoPhase      *TwoPhaseIterator
	windowMatches *util.FixedBitSet
}

// NewConstantScoreBulkScorer builds a ConstantScoreBulkScorer over the given iterator.
//
// Mirrors ConstantScoreBulkScorer(float, ScoreMode, DocIdSetIterator).
func NewConstantScoreBulkScorer(score float32, scoreMode ScoreMode, iterator DocIdSetIterator) (*ConstantScoreBulkScorer, error) {
	return newConstantScoreBulkScorer(score, scoreMode, iterator, nil)
}

// NewConstantScoreBulkScorerFromTwoPhase builds a ConstantScoreBulkScorer that confirms
// the matches of a TwoPhaseIterator.
//
// Mirrors ConstantScoreBulkScorer(float, ScoreMode, TwoPhaseIterator).
func NewConstantScoreBulkScorerFromTwoPhase(score float32, scoreMode ScoreMode, twoPhase *TwoPhaseIterator) (*ConstantScoreBulkScorer, error) {
	return newConstantScoreBulkScorer(score, scoreMode, twoPhase.Approximation(), twoPhase)
}

func newConstantScoreBulkScorer(score float32, scoreMode ScoreMode, iterator DocIdSetIterator, twoPhase *TwoPhaseIterator) (*ConstantScoreBulkScorer, error) {
	if scoreMode.NeedsScores() {
		return nil, fmt.Errorf("ScoreMode must not need scores: %v", scoreMode)
	}

	// Mirrors: if (twoPhase == null && TwoPhaseIterator.unwrap(iterator) != null)
	if twoPhase == nil && Unwrap(iterator) != nil {
		return nil, fmt.Errorf("Iterator must not wrap a TwoPhaseIterator")
	}

	scorer := NewConstantScoreScorer(score, scoreMode, iterator)

	windowMatches, err := util.NewFixedBitSet(WindowSize)
	if err != nil {
		return nil, err
	}

	return &ConstantScoreBulkScorer{
		scorer:        scorer,
		iterator:      iterator,
		twoPhase:      twoPhase,
		windowMatches: windowMatches,
	}, nil
}

// Score collects matching documents in [min, max) and returns the first document the
// iterator advanced to at or beyond max.
//
// Faithful port of ConstantScoreBulkScorer.score(LeafCollector, Bits, int, int).
func (bs *ConstantScoreBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if err := collector.SetScorer(bs.scorer); err != nil {
		return 0, err
	}

	competitiveIterator, err := collector.CompetitiveIterator()
	if err != nil {
		return 0, err
	}

	if competitiveIterator != nil {
		return bs.scoreIterator(collector, acceptDocs, competitiveIterator, min, max)
	}

	return bs.scoreIteratorIntoBitSet(collector, acceptDocs, min, max)
}

// Cost returns the iteration cost of the underlying iterator.
func (bs *ConstantScoreBulkScorer) Cost() int64 {
	return bs.iterator.Cost()
}

func (bs *ConstantScoreBulkScorer) scoreIterator(collector LeafCollector, acceptDocs util.Bits, competitiveIterator DocIdSetIterator, min, max int) (int, error) {
	effMin := min
	compDoc := competitiveIterator.DocID()
	if compDoc > effMin {
		if compDoc < max {
			effMin = compDoc
		} else {
			effMin = max
		}
	}

	doc := bs.iterator.DocID()
	if doc < effMin {
		if doc == effMin-1 {
			var err error
			doc, err = bs.iterator.NextDoc()
			if err != nil {
				return 0, err
			}
		} else {
			var err error
			doc, err = bs.iterator.Advance(effMin)
			if err != nil {
				return 0, err
			}
		}
	}

	for doc < max {
		compDoc = competitiveIterator.DocID()
		if compDoc < doc {
			var err error
			compNext, err := competitiveIterator.Advance(doc)
			if err != nil {
				return 0, err
			}
			if compNext != doc {
				var err2 error
				doc, err2 = bs.iterator.Advance(compNext)
				if err2 != nil {
					return 0, err2
				}
				continue
			}
		}

		// Confirmation phase
		confirmed := true
		if bs.twoPhase != nil {
			var err error
			confirmed, err = bs.twoPhase.Matches()
			if err != nil {
				return 0, err
			}
		}

		if confirmed && (acceptDocs == nil || acceptDocs.Get(doc)) {
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
		}

		var err error
		doc, err = bs.iterator.NextDoc()
		if err != nil {
			return 0, err
		}
	}

	return doc, nil
}

func (bs *ConstantScoreBulkScorer) scoreIteratorIntoBitSet(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	doc := bs.iterator.DocID()
	if doc < min {
		if doc == min-1 {
			var err error
			doc, err = bs.iterator.NextDoc()
			if err != nil {
				return 0, err
			}
		} else {
			var err error
			doc, err = bs.iterator.Advance(min)
			if err != nil {
				return 0, err
			}
		}
	}

	for doc < max {
		windowBase := doc
		windowMax := windowBase + WindowSize
		if windowMax > max {
			windowMax = max
		}

		if bs.twoPhase == nil {
			if err := bs.iterator.IntoBitSet(windowMax, bs.windowMatches, windowBase); err != nil {
				return 0, err
			}
		} else {
			if err := bs.twoPhase.IntoBitSet(windowMax, bs.windowMatches, windowBase); err != nil {
				return 0, err
			}
		}

		if acceptDocs != nil {
			util.ApplyMask(acceptDocs, bs.windowMatches, windowBase)
		}

		if err := collector.CollectStream(NewBitSetDocIdStream(bs.windowMatches, windowBase)); err != nil {
			return 0, err
		}
		bs.windowMatches.ClearAll()

		doc = bs.iterator.DocID()
	}

	return doc, nil
}

var _ BulkScorer = (*ConstantScoreBulkScorer)(nil)
