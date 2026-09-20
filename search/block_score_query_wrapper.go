package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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
	return queryToString(q.query, field)
}

func (q *BlockScoreQueryWrapper) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*BlockScoreQueryWrapper)
	if !ok {
		return false
	}
	return q.query == o.query && q.blockLength == o.blockLength
}

// Rewrite mirrors BlockScoreQueryWrapper.rewrite(IndexSearcher).
func (q *BlockScoreQueryWrapper) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	rewritten, err := q.query.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.query {
		return NewBlockScoreQueryWrapper(rewritten, q.blockLength), nil
	}
	// super.rewrite(indexSearcher) — Query.rewrite returns this.
	return q, nil
}

func (q *BlockScoreQueryWrapper) HashCode() int {
	return 0
}

func (q *BlockScoreQueryWrapper) Visit(visitor QueryVisitor) {
	q.query.Visit(visitor)
}

func (q *BlockScoreQueryWrapper) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
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
	BaseWeight
	weight      Weight
	blockLength int
}

func (w *blockScoreQueryWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.weight.Explain(context, doc)
}

func (w *blockScoreQueryWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	inScorer, err := w.weight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if inScorer == nil {
		return nil, nil
	}

	return NewDefaultScorerSupplier(&blockScoreScorer{
		scorer:      inScorer,
		blockLength: w.blockLength,
	}), nil
}

func (w *blockScoreQueryWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

type blockScoreScorer struct {
	BaseScorer
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

// TwoPhaseIterator returns the two-phase view of the wrapped Scorer.
//
// Apache Lucene 10.5.0 declares `public TwoPhaseIterator twoPhaseIterator()`
// on Scorer (Scorer.java:58), returning a nullable reference; the Go rendering
// of a nullable Java reference is the pointer type *TwoPhaseIterator, which is
// what the Scorer interface requires.
func (s *blockScoreScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

func (s *blockScoreScorer) AdvanceShallow(target int) (int, error) {
	return s.scorer.AdvanceShallow(target)
}

func (s *blockScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return s.scorer.NextDocsAndScores(upTo, liveDocs, buffer)
}
