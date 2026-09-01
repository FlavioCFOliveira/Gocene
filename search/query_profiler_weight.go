package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// QueryProfilerWeight is a weight that profiles the query execution.
// Mirrors Lucene's org.apache.lucene.sandbox.search.QueryProfilerWeight.
type QueryProfilerWeight struct {
	weight Weight
}

func NewQueryProfilerWeight(weight Weight) *QueryProfilerWeight {
	return &QueryProfilerWeight{
		weight: weight,
	}
}

func (w *QueryProfilerWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.weight.Explain(context, doc)
}

func (w *QueryProfilerWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	supplier, err := w.weight.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return supplier, err
	}

	scorer, err := supplier.GetScorer()
	if err != nil {
		return nil, err
	}

	return NewDefaultScorerSupplier(&queryProfilerScorer{
		scorer: scorer,
	}), nil
}

func (w *QueryProfilerWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

type queryProfilerScorer struct {
	scorer Scorer
}

func (s *queryProfilerScorer) Score() (float32, error) {
	return s.scorer.Score()
}

func (s *queryProfilerScorer) GetMaxScore(upTo int) (float32, error) {
	return s.scorer.GetMaxScore(upTo)
}

func (s *queryProfilerScorer) DocID() int {
	return s.scorer.DocID()
}

func (s *queryProfilerScorer) Iterator() DocIdSetIterator {
	return s.scorer.Iterator()
}

func (s *queryProfilerScorer) TwoPhaseIterator() TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

func (s *queryProfilerScorer) AdvanceShallow(target int) (int, error) {
	return s.scorer.AdvanceShallow(target)
}

func (s *queryProfilerScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return s.scorer.NextDocsAndScores(upTo, liveDocs, buffer)
}
