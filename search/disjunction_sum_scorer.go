package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DisjunctionSumScorer is a Scorer for OR like queries, counterpart of
// ConjunctionScorer.
//
// Mirrors org.apache.lucene.search.DisjunctionSumScorer (Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/search/DisjunctionSumScorer.java).
//
// Deviations from Java, forced by the target language:
//   - Java's DisjunctionSumScorer extends DisjunctionScorer; Go embeds
//     *DisjunctionScorer.
//   - Java's protected abstract score(DisiWrapper) is dispatched virtually
//     from DisjunctionScorer.score(); Go has no virtual dispatch, so Score()
//     is shadowed here and calls this type's own scoreTopList, exactly as
//     DisjunctionMaxScorer does.
type DisjunctionSumScorer struct {
	*DisjunctionScorer
	scorers []Scorer
}

// NewDisjunctionSumScorer constructs a DisjunctionSumScorer.
//
// Mirrors DisjunctionSumScorer(List<Scorer>, ScoreMode, long), whose body is
// `super(subScorers, scoreMode, leadCost); this.scorers = subScorers;`.
func NewDisjunctionSumScorer(subScorers []Scorer, scoreMode ScoreMode, leadCost int64) *DisjunctionSumScorer {
	return &DisjunctionSumScorer{
		DisjunctionScorer: newDisjunctionScorer(subScorers, scoreMode, leadCost),
		scorers:           subScorers,
	}
}

// Score returns the score of the current document.
//
// Mirrors DisjunctionScorer.score(), whose body is `return score(getSubMatches())`,
// with the virtual call resolved to this type's scoreTopList.
func (s *DisjunctionSumScorer) Score() (float32, error) {
	topList, err := s.DisjunctionScorer.getSubMatches()
	if err != nil {
		return 0, err
	}
	return s.scoreTopList(topList)
}

// scoreTopList sums the scores of every sub-scorer on the current document.
//
// Mirrors DisjunctionSumScorer.score(DisiWrapper topList):
//
//	double score = 0;
//	for (DisiWrapper w = topList; w != null; w = w.next) {
//	  score += w.scorable.score();
//	}
//	return (float) score;
func (s *DisjunctionSumScorer) scoreTopList(topList *DisiWrapper) (float32, error) {
	var score float64
	for w := topList; w != nil; w = w.next {
		sub, err := w.scorable.Score()
		if err != nil {
			return 0, err
		}
		score += float64(sub)
	}
	return float32(score), nil
}

// AdvanceShallow mirrors DisjunctionSumScorer.advanceShallow(int):
//
//	int min = DocIdSetIterator.NO_MORE_DOCS;
//	for (Scorer scorer : scorers) {
//	  if (scorer.docID() <= target) {
//	    min = Math.min(min, scorer.advanceShallow(target));
//	  }
//	}
//	return min;
func (s *DisjunctionSumScorer) AdvanceShallow(target int) (int, error) {
	min := NO_MORE_DOCS
	for _, scorer := range s.scorers {
		if scorer.DocID() <= target {
			shallow, err := scorer.AdvanceShallow(target)
			if err != nil {
				return 0, err
			}
			if shallow < min {
				min = shallow
			}
		}
	}
	return min, nil
}

// GetMaxScore mirrors DisjunctionSumScorer.getMaxScore(int):
//
//	double maxScore = 0;
//	for (Scorer scorer : scorers) {
//	  if (scorer.docID() <= upTo) {
//	    maxScore += scorer.getMaxScore(upTo);
//	  }
//	}
//	return (float) MathUtil.sumUpperBound(maxScore, scorers.size());
func (s *DisjunctionSumScorer) GetMaxScore(upTo int) (float32, error) {
	var maxScore float64
	for _, scorer := range s.scorers {
		if scorer.DocID() <= upTo {
			m, err := scorer.GetMaxScore(upTo)
			if err != nil {
				return 0, err
			}
			maxScore += float64(m)
		}
	}
	return float32(util.MathSumUpperBound(maxScore, len(s.scorers))), nil
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which DisjunctionSumScorer inherits unchanged.
func (s *DisjunctionSumScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ Scorer = (*DisjunctionSumScorer)(nil)
