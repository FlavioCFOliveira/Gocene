package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermScorer is a scorer for a single term.
type TermScorer struct {
	postings   index.PostingsEnum
	simScorer  SimScorer
	norms      index.NumericDocValues
	scoreMode  ScoreMode
	freq       int
	currentDoc int
}

func NewTermScorer(postings index.PostingsEnum, simScorer SimScorer, norms index.NumericDocValues, scoreMode ScoreMode) *TermScorer {
	return &TermScorer{
		postings:   postings,
		simScorer:  simScorer,
		norms:      norms,
		scoreMode:  scoreMode,
		currentDoc: -1,
	}
}

func (s *TermScorer) NextDoc() (int, error) {
	if s.postings == nil {
		return NO_MORE_DOCS, nil
	}
	doc, err := s.postings.Advance()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc == NO_MORE_DOCS {
		return NO_MORE_DOCS, nil
	}

	s.currentDoc = doc
	if s.scoreMode.NeedsScores() {
		s.freq = s.postings.Freq()
	} else {
		s.freq = 1
	}
	return doc, nil
}

func (s *TermScorer) Score() float32 {
	if s.simScorer == nil {
		return 0
	}
	var norm int64 = 1
	if s.norms != nil && s.currentDoc != -1 {
		if s.norms.AdvanceExact(s.currentDoc) {
			norm = s.norms.LongValue()
		}
	}
	return s.simScorer.Score(float32(s.freq), norm)
}

func (s *TermScorer) DocID() int {
	return s.currentDoc
}

func (s *TermScorer) Iterator() DocIdSetIterator {
	return s.postings
}

func (s *TermScorer) Advance(target int) (int, error) {
	if s.postings == nil {
		return NO_MORE_DOCS, nil
	}
	doc, err := s.postings.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	if doc != NO_MORE_DOCS {
		s.currentDoc = doc
		if s.scoreMode.NeedsScores() {
			s.freq = s.postings.Freq()
		} else {
			s.freq = 1
		}
	}
	return doc, nil
}

func (s *TermScorer) GetPositions() []int {
	if s.postings == nil || s.currentDoc == -1 {
		return nil
	}
	var positions []int
	for {
		pos, err := s.postings.NextPosition()
		if err != nil || pos == index.NO_MORE_POSITIONS {
			break
		}
		positions = append(positions, pos)
	}
	return positions
}

func (s *TermScorer) TwoPhaseIterator() TwoPhaseIterator {
	// For TermScorer, the approximation is the iterator itself.
	return &termTwoPhaseIterator{scorer: s}
}

type termTwoPhaseIterator struct {
	scorer *TermScorer
}

func (t *termTwoPhaseIterator) Approximation() DocIdSetIterator {
	return t.scorer.Iterator()
}

func (t *termTwoPhaseIterator) Matches() (bool, error) {
	// In a simple TermScorer, if it's on a doc, it matches.
	return t.scorer.currentDoc != -1, nil
}

func (t *termTwoPhaseIterator) MatchCost() float32 {
	return 1.0
}

func (t *termTwoPhaseIterator) DocIDRunEnd() (int, error) {
	return -1, nil
}

func (t *termTwoPhaseIterator) IntoBitSet(upTo int, bitSet util.BitSet, offset int) error {
	iter := t.scorer.Iterator()
	for {
		doc, err := iter.NextDoc()
		if err != nil || doc == NO_MORE_DOCS || doc >= upTo {
			break
		}
		bitSet.Set(doc + offset)
	}
	return nil
}
