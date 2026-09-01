package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConjunctionScorer is a scorer that matches documents that match all of its required clauses.
type ConjunctionScorer struct {
	scorers     []Scorer
	scoringOnly []Scorer
}

func NewConjunctionScorer(allScorers []Scorer, scoringScorers []Scorer) *ConjunctionScorer {
	return &ConjunctionScorer{
		scorers:     allScorers,
		scoringOnly: scoringScorers,
	}
}

func (s *ConjunctionScorer) NextDoc() (int, error) {
	if len(s.scorers) == 0 {
		return NO_MORE_DOCS, nil
	}

	// Start with the first scorer
	doc, err := s.scorers[0].NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}

	for doc != NO_MORE_DOCS {
		matched := true
		for i := 1; i < len(s.scorers); i++ {
			advanced, err := s.scorers[i].Advance(doc)
			if err != nil {
				return NO_MORE_DOCS, err
			}
			if advanced != doc {
				doc = advanced
				// Reset first scorer to this new doc
				doc, err = s.scorers[0].Advance(doc)
				if err != nil {
					return NO_MORE_DOCS, err
				}
				matched = false
				break
			}
		}
		if matched {
			return doc, nil
		}
		if doc == NO_MORE_DOCS {
			break
		}
	}

	return NO_MORE_DOCS, nil
}

func (s *ConjunctionScorer) Score() float32 {
	var total float32
	for _, sc := range s.scoringOnly {
		total += sc.Score()
	}
	return total
}

func (s *ConjunctionScorer) DocID() int {
	if len(s.scorers) == 0 {
		return -1
	}
	return s.scorers[0].DocID()
}

func (s *ConjunctionScorer) Iterator() DocIdSetIterator {
	if len(s.scorers) == 0 {
		return nil
	}
	// Lucene's ConjunctionScorer returns a specialized iterator.
	// For now, we return the first one as a placeholder.
	return s.scorers[0].Iterator()
}

func (s *ConjunctionScorer) Advance(target int) (int, error) {
	if len(s.scorers) == 0 {
		return NO_MORE_DOCS, nil
	}

	doc, err := s.scorers[0].Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}

	for doc != NO_MORE_DOCS {
		matched := true
		for i := 1; i < len(s.scorers); i++ {
			advanced, err := s.scorers[i].Advance(doc)
			if err != nil {
				return NO_MORE_DOCS, err
			}
			if advanced != doc {
				doc = advanced
				doc, err = s.scorers[0].Advance(doc)
				if err != nil {
					return NO_MORE_DOCS, err
				}
				matched = false
				break
			}
		}
		if matched {
			return doc, nil
		}
		if doc == NO_MORE_DOCS {
			break
		}
	}

	return NO_MORE_DOCS, nil
}
