package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

type ConstantScoreWeight struct {
	BaseWeight
	query    *ConstantScoreQuery
	weight   Weight
	constant float32
}

func NewConstantScoreWeight(query *ConstantScoreQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) *ConstantScoreWeight {
	w, _ := query.query.CreateWeight(searcher, scoreMode, boost)
	return &ConstantScoreWeight{
		BaseWeight: BaseWeight{query: query},
		query:      query,
		weight:     w,
		constant:   query.score * boost,
	}
}

func (w *ConstantScoreWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	supplier, err := w.weight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return &constantScoreScorerSupplier{
		supplier: supplier,
		constant: w.constant,
	}, nil
}

type constantScoreScorerSupplier struct {
	supplier ScorerSupplier
	constant float32
}

func (s *constantScoreScorerSupplier) Get(weightIndex int) (Scorer, error) {
	scorer, err := s.supplier.Get(weightIndex)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return &constantScoreScorer{
		scorer:   scorer,
		constant: s.constant,
	}, nil
}

func (s *constantScoreScorerSupplier) GetMatchCost() float32 {
	return s.supplier.GetMatchCost()
}

func (s *constantScoreScorerSupplier) GetDocCount() int {
	return s.supplier.GetDocCount()
}

type constantScoreScorer struct {
	scorer   Scorer
	constant float32
}

func (s *constantScoreScorer) NextDoc() (int, error) {
	return s.scorer.NextDoc()
}

func (s *constantScoreScorer) Score() float32 {
	return s.constant
}

func (s *constantScoreScorer) DocID() int {
	return s.scorer.DocID()
}

func (s *constantScoreScorer) Iterator() DocIdSetIterator {
	return s.scorer.Iterator()
}

func (s *constantScoreScorer) Advance(target int) (int, error) {
	return s.scorer.Advance(target)
}

var _ Weight = (*ConstantScoreWeight)(nil)
