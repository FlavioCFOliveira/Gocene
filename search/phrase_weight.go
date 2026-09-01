package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

type PhraseWeight struct {
	BaseWeight
	query      *PhraseQuery
	searcher   *IndexSearcher
	scoreMode  ScoreMode
	similarity Similarity
}

func NewPhraseWeight(query *PhraseQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) *PhraseWeight {
	return &PhraseWeight{
		BaseWeight: BaseWeight{query: query},
		query:      query,
		searcher:   searcher,
		scoreMode:  scoreMode,
		similarity: searcher.GetSimilarity(),
	}
}

func (w *PhraseWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	var suppliers []ScorerSupplier
	for _, term := range w.query.terms {
		_ = term
	}
	
	return &phraseScorerSupplier{
		weight:  w,
		context: context,
	}, nil
}

type phraseScorerSupplier struct {
	weight  *PhraseWeight
	context *index.LeafReaderContext
}

func (s *phraseScorerSupplier) Get(weightIndex int) (Scorer, error) {
	var scorers []Scorer
	var postingsData []struct {
		postings index.PostingsEnum
		position index.PostingsEnum
		terms    []byte
		freq     int
	}

	for _, term := range s.weight.query.terms {
		te := s.context.Reader().Terms(s.weight.query.field).Iterator()
		te.SeekExact(term.Bytes(), nil)

		scorer := NewTermScorer(te.Postings(nil, index.PostingsEnumFreqs), nil, nil, s.weight.scoreMode)
		scorers = append(scorers, scorer)

		postingsData = append(postingsData, struct {
			postings index.PostingsEnum
			position index.PostingsEnum
			terms    []byte
			freq     int
		}{
			postings: te.Postings(nil, index.PostingsEnumFreqs),
			position: te.Postings(nil, index.PostingsEnumFreqs),
			terms:    term.Bytes(),
			freq:     0,
		})
	}

	matcher := NewSloppyPhraseMatcher(
		postingsData,
		s.weight.query.slop,
		s.weight.scoreMode,
		nil,
		1.0,
		true,
	)

	return &phraseScorer{
		conjunction: NewConjunctionScorer(scorers, scorers),
		matcher:     matcher,
		query:       s.weight.query,
		scoreMode:   s.weight.scoreMode,
	}, nil
}

func (s *phraseScorerSupplier) GetMatchCost() float32 {
	return 1.0
}

func (s *phraseScorerSupplier) GetDocCount() int {
	return -1
}

type phraseScorer struct {
	conjunction *ConjunctionScorer
	matcher     PhraseMatcher
	query       *PhraseQuery
	scoreMode   ScoreMode
}

func (s *phraseScorer) NextDoc() (int, error) {
	for {
		doc, err := s.conjunction.NextDoc()
		if err != nil || doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, err
		}
		s.matcher.ResetPositions()
		if s.matcher.NextMatch() {
			return doc, nil
		}
	}
}

func (s *phraseScorer) verifyPositions(doc int) bool {
	return true
}

func (s *phraseScorer) Score() float32 {
	return s.conjunction.Score()
}

func (s *phraseScorer) DocID() int {
	return s.conjunction.DocID()
}

func (s *phraseScorer) Iterator() DocIdSetIterator {
	return s.conjunction.Iterator()
}

func (s *phraseScorer) Advance(target int) (int, error) {
	for {
		doc, err := s.conjunction.Advance(target)
		if err != nil || doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, err
		}
		s.matcher.ResetPositions()
		if s.matcher.NextMatch() {
			return doc, nil
		}
		target = doc + 1
	}
}

var _ Weight = (*PhraseWeight)(nil)
