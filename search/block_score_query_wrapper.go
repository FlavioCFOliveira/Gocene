package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BlockScoreQueryWrapper is a query wrapper that reduces the size of max-score blocks
// to more easily detect problems with the max-score logic.
// Mirrors Lucene's org.apache.lucene.tests.search.BlockScoreQueryWrapper.
type BlockScoreQueryWrapper struct {
	query       Query
	blockLength int
}

func NewBlockScoreQueryWrapper(query Query, blockLength int) *BlockScoreQueryWrapper {
	return &BlockScoreQueryWrapper{
		query:       query,
		blockLength: blockLength,
	}
}

func (q *BlockScoreQueryWrapper) ToString(field string) string {
	return q.query.ToString(field)
}

func (q *BlockScoreQueryWrapper) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*BlockScoreQueryWrapper)
	if !ok {
		return false
	}
	return q.query == o.query && q.blockLength == o.blockLength
}

func (q *BlockScoreQueryWrapper) HashCode() int {
	return 0
}

func (q *BlockScoreQueryWrapper) Visit(visitor QueryVisitor) {
	q.query.Visit(visitor)
}

func (q *BlockScoreQueryWrapper) CreateWeight(searcher IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	weight, err := q.query.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &blockScoreQueryWeight{
		weight:      weight,
		blockLength: q.blockLength,
	}, nil
}

type blockScoreQueryWeight struct {
	weight      Weight
	blockLength int
}

func (w *blockScoreQueryWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.weight.Explain(context, doc)
}

func (w *blockScoreQueryWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	supplier, err := w.weight.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return supplier, err
	}

	scorer, err := supplier.GetScorer()
	if err != nil {
		return nil, err
	}

	return NewDefaultScorerSupplier(&blockScoreScorer{
		scorer:      scorer,
		blockLength: w.blockLength,
	}), nil
}

func (w *blockScoreQueryWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

type blockScoreScorer struct {
	scorer      Scorer
	blockLength int
}

func (s *blockScoreScorer) Score() (float32, error) {
	return s.scorer.Score()
}

func (s *blockScoreScorer) GetMaxScore(upTo int) (float32, error) {
	// This is the core of BlockScoreQueryWrapper: it limits the max score range.
	// Lucene implementation: return scorer.getMaxScore(Math.min(upTo, docID() + blockLength));
	limit := s.scorer.DocID() + s.blockLength
	if upTo < limit {
		limit = upTo
	}
	return s.scorer.GetMaxScore(limit)
}

func (s *blockScoreScorer) DocID() int {
	return s.scorer.DocID()
}

func (s *blockScoreScorer) Iterator() DocIdSetIterator {
	return s.scorer.Iterator()
}

func (s *blockScoreScorer) TwoPhaseIterator() TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

func (s *blockScoreScorer) AdvanceShallow(target int) (int, error) {
	return s.scorer.AdvanceShallow(target)
}

func (s *blockScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return s.scorer.NextDocsAndScores(upTo, liveDocs, buffer)
}
