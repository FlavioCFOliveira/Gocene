package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReqExclScorer is a scorer that matches documents that match a required scorer
// but do NOT match any of the prohibited scorers.
type ReqExclScorer struct {
	required   Scorer
	prohibited Scorer
}

func NewReqExclScorer(required Scorer, prohibited Scorer) *ReqExclScorer {
	return &ReqExclScorer{
		required:   required,
		prohibited: prohibited,
	}
}

func (s *ReqExclScorer) NextDoc() (int, error) {
	doc, err := s.required.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}

	for doc != NO_MORE_DOCS {
		if s.prohibited == nil {
			return doc, nil
		}

		advanced, err := s.prohibited.Advance(doc)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if advanced != doc {
			return doc, nil
		}

		doc, err = s.required.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
	}

	return NO_MORE_DOCS, nil
}

func (s *ReqExclScorer) Score() float32 {
	return s.required.Score()
}

func (s *ReqExclScorer) DocID() int {
	return s.required.DocID()
}

func (s *ReqExclScorer) Iterator() DocIdSetIterator {
	return s.required.Iterator()
}

func (s *ReqExclScorer) Advance(target int) (int, error) {
	doc, err := s.required.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}

	for doc != NO_MORE_DOCS {
		if s.prohibited == nil {
			return doc, nil
		}

		advanced, err := s.prohibited.Advance(doc)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if advanced != doc {
			return doc, nil
		}

		doc, err = s.required.Advance(doc)
		if err != nil {
			return NO_MORE_DOCS, err
		}
	}

	return NO_MORE_DOCS, nil
}
