package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DisjunctionSumScorer is a scorer that matches documents that match any of its clauses,
// summing their scores.
type DisjunctionSumScorer struct {
	scorers   []Scorer
	scoreMode ScoreMode
	leadCost  int64
}

func NewDisjunctionSumScorer(scorers []Scorer, scoreMode ScoreMode, leadCost int64) *DisjunctionSumScorer {
	return &DisjunctionSumScorer{
		scorers:   scorers,
		scoreMode: scoreMode,
		leadCost:  leadCost,
	}
}

func (s *DisjunctionSumScorer) NextDoc() (int, error) {
	if len(s.scorers) == 0 {
		return NO_MORE_DOCS, nil
	}

	minDoc := NO_MORE_DOCS
	for _, sc := range s.scorers {
		doc, err := sc.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc != NO_MORE_DOCS && (minDoc == NO_MORE_DOCS || doc < minDoc) {
			minDoc = doc
		}
	}

	if minDoc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}

	// Advance all other scorers to this minDoc to ensure correct scoring
	for _, sc := range s.scorers {
		advanced, err := sc.Advance(minDoc)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if advanced != minDoc {
			// This should not happen if we use NextDoc properly, but for safety:
			// If it doesn't match, we just don't count its score.
		}
	}

	return minDoc, nil
}

func (s *DisjunctionSumScorer) Score() float32 {
	var total float32
	for _, sc := range s.scorers {
		if sc.DocID() == s.currentDoc() { // Need currentDoc
			total += sc.Score()
		}
	}
	return total
}

func (s *DisjunctionSumScorer) currentDoc() int {
	if len(s.scorers) == 0 {
		return -1
	}
	return s.scorers[0].DocID()
}

func (s *DisjunctionSumScorer) DocID() int {
	return s.currentDoc()
}

func (s *DisjunctionSumScorer) Iterator() DocIdSetIterator {
	// Lucene uses a specialized iterator for disjunctions.
	// For now, we return nil or a simple wrap.
	return nil
}

func (s *DisjunctionSumScorer) Advance(target int) (int, error) {
	if len(s.scorers) == 0 {
		return NO_MORE_DOCS, nil
	}

	minDoc := NO_MORE_DOCS
	for _, sc := range s.scorers {
		doc, err := sc.Advance(target)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if doc != NO_MORE_DOCS && (minDoc == NO_MORE_DOCS || doc < minDoc) {
			minDoc = doc
		}
	}

	if minDoc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}

	for _, sc := range s.scorers {
		sc.Advance(minDoc)
	}

	return minDoc, nil
}
