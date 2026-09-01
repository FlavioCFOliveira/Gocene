package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConstantScoreBulkScorer is a bulk scorer for constant-score iterators.
type ConstantScoreBulkScorer struct {
	scorer    Scorer
	iterator  DocIdSetIterator
	twoPhase  *TwoPhaseIterator
	windowMatches *util.FixedBitSet
}

func NewConstantScoreBulkScorer(score float32, scoreMode ScoreMode, iterator DocIdSetIterator) *ConstantScoreBulkScorer {
	return NewConstantScoreBulkScorerWithTwoPhase(score, scoreMode, iterator, nil)
}

func NewConstantScoreBulkScorerWithTwoPhase(score float32, scoreMode ScoreMode, iterator DocIdSetIterator, twoPhase *TwoPhaseIterator) *ConstantScoreBulkScorer {
	if scoreMode.NeedsScores() {
		panic("ScoreMode must not need scores for ConstantScoreBulkScorer")
	}
	
	var scorer Scorer
	if twoPhase == nil {
		scorer = NewConstantScoreScorer(score, scoreMode, iterator)
	} else {
		scorer = NewConstantScoreScorer(score, scoreMode, twoPhase)
	}
	
	wm, _ := util.NewFixedBitSet(WindowSize)
	
	return &ConstantScoreBulkScorer{
		scorer:        scorer,
		iterator:      iterator,
		twoPhase:      twoPhase,
		windowMatches: wm,
	}
}

func (bs *ConstantScoreBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	collector.SetScorer(bs.scorer)
	
	// In Gocene, we primarily use the linear iteration path.
	// We omit the competitiveIterator path as it's not yet fully wired in the collector.
	
	doc := bs.iterator.DocID()
	if doc < min {
		var err error
		doc, err = bs.iterator.Advance(min)
		if err != nil {
			return 0, err
		}
	}

	for doc < max && doc != NO_MORE_DOCS {
		windowBase := doc
		windowMax := max
		if windowBase+WindowSize < max {
			windowMax = windowBase + WindowSize
		}

		if bs.twoPhase == nil {
			bs.iterator.IntoBitSet(windowMax, bs.windowMatches, windowBase)
		} else {
			bs.twoPhase.IntoBitSet(windowMax, bs.windowMatches, windowBase)
		}

		if acceptDocs != nil {
			acceptDocs.ApplyMask(bs.windowMatches, windowBase)
		}

		collector.Collect(NewBitSetDocIdStream(bs.windowMatches, windowBase))
		bs.windowMatches.Clear()

		doc = bs.iterator.DocID()
	}

	return doc, nil
}

func (bs *ConstantScoreBulkScorer) Cost() int64 {
	return bs.iterator.Cost()
}

var _ BulkScorer = (*ConstantScoreBulkScorer)(nil)
