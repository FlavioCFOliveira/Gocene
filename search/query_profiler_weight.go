package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
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
	subQueryScorerSupplier, err := w.weight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if subQueryScorerSupplier == nil {
		return nil, nil
	}
	return &queryProfilerScorerSupplier{in: subQueryScorerSupplier}, nil
}

// queryProfilerScorerSupplier is the anonymous ScorerSupplier returned by
// QueryProfilerWeight.scorerSupplier.
type queryProfilerScorerSupplier struct {
	in ScorerSupplier
}

// Get mirrors the anonymous ScorerSupplier.get(long).
func (s *queryProfilerScorerSupplier) Get(leadCost int64) (Scorer, error) {
	scorer, err := s.in.Get(leadCost)
	if err != nil {
		return nil, err
	}
	return &queryProfilerScorer{scorer: scorer}, nil
}

// BulkScorer mirrors the anonymous ScorerSupplier.bulkScorer(), which
// deliberately falls back to the default bulk scorer: BulkScorers do everything
// at once, which would make it impossible to see where time is spent.
func (s *queryProfilerScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(s)
}

// Cost mirrors the anonymous ScorerSupplier.cost().
func (s *queryProfilerScorerSupplier) Cost() int64 {
	return s.in.Cost()
}

// SetTopLevelScoringClause mirrors the anonymous
// ScorerSupplier.setTopLevelScoringClause().
func (s *queryProfilerScorerSupplier) SetTopLevelScoringClause() error {
	return s.in.SetTopLevelScoringClause()
}

func (w *QueryProfilerWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

type queryProfilerScorer struct {
	BaseScorer
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

// TwoPhaseIterator returns the two-phase view of the wrapped Scorer.
//
// Apache Lucene 10.5.0 declares `public TwoPhaseIterator twoPhaseIterator()`
// on Scorer (Scorer.java:58), returning a nullable reference; the Go rendering
// of a nullable Java reference is the pointer type *TwoPhaseIterator, which is
// what the Scorer interface requires.
func (s *queryProfilerScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

func (s *queryProfilerScorer) AdvanceShallow(target int) (int, error) {
	return s.scorer.AdvanceShallow(target)
}

func (s *queryProfilerScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return s.scorer.NextDocsAndScores(upTo, liveDocs, buffer)
}
