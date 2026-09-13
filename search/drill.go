package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DrillDownQuery mirrors Lucene's org.apache.lucene.facet.DrillDownQuery.
type DrillDownQuery struct {
	query Query
}

func NewDrillDownQuery(query Query) *DrillDownQuery {
	return &DrillDownQuery{
		query: query,
	}
}

func (q *DrillDownQuery) ToString(field string) string {
	return queryToString(q.query, field)
}

func (q *DrillDownQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*DrillDownQuery)
	if !ok {
		return false
	}
	return q.query == o.query
}

// Rewrite mirrors the default body of Query.rewrite(IndexSearcher) in Apache
// Lucene 10.5.0, which returns this.
func (q *DrillDownQuery) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	return q, nil
}

func (q *DrillDownQuery) HashCode() int {
	return 0
}

func (q *DrillDownQuery) Visit(visitor QueryVisitor) {
	visitQuery(q.query, visitor)
}

func (q *DrillDownQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	weight, err := q.query.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &drillDownWeight{
		weight: weight,
	}, nil
}

type drillDownWeight struct {
	BaseWeight
	weight Weight
}

func (w *drillDownWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.weight.Explain(context, doc)
}

func (w *drillDownWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	return w.weight.ScorerSupplier(context)
}

func (w *drillDownWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

// DrillSidewaysQuery mirrors Lucene's org.apache.lucene.facet.DrillSidewaysQuery.
type DrillSidewaysQuery struct {
	query Query
}

func NewDrillSidewaysQuery(query Query) *DrillSidewaysQuery {
	return &DrillSidewaysQuery{
		query: query,
	}
}

func (q *DrillSidewaysQuery) ToString(field string) string {
	return queryToString(q.query, field)
}

func (q *DrillSidewaysQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*DrillSidewaysQuery)
	if !ok {
		return false
	}
	return q.query == o.query
}

// Rewrite mirrors the default body of Query.rewrite(IndexSearcher) in Apache
// Lucene 10.5.0, which returns this.
func (q *DrillSidewaysQuery) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	return q, nil
}

func (q *DrillSidewaysQuery) HashCode() int {
	return 0
}

func (q *DrillSidewaysQuery) Visit(visitor QueryVisitor) {
	visitQuery(q.query, visitor)
}

func (q *DrillSidewaysQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	weight, err := q.query.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &drillSidewaysWeight{
		weight: weight,
	}, nil
}

type drillSidewaysWeight struct {
	BaseWeight
	weight Weight
}

func (w *drillSidewaysWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.weight.Explain(context, doc)
}

func (w *drillSidewaysWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	supplier, err := w.weight.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return supplier, err
	}

	scorer, err := supplier.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}

	return NewDefaultScorerSupplier(&DrillSidewaysScorer{
		scorer: scorer,
	}), nil
}

func (w *drillSidewaysWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.weight.IsCacheable(ctx)
}

// DrillSidewaysScorer mirrors Lucene's org.apache.lucene.facet.DrillSidewaysScorer.
type DrillSidewaysScorer struct {
	BaseScorer
	scorer Scorer
}

func (s *DrillSidewaysScorer) Score() (float32, error) {
	return s.scorer.Score()
}

func (s *DrillSidewaysScorer) GetMaxScore(upTo int) (float32, error) {
	return s.scorer.GetMaxScore(upTo)
}

func (s *DrillSidewaysScorer) DocID() int {
	return s.scorer.DocID()
}

func (s *DrillSidewaysScorer) Iterator() DocIdSetIterator {
	return s.scorer.Iterator()
}

// TwoPhaseIterator returns the two-phase view of the wrapped Scorer.
//
// Apache Lucene 10.5.0 declares `public TwoPhaseIterator twoPhaseIterator()`
// on Scorer (Scorer.java:58), returning a nullable reference; the Go rendering
// of a nullable Java reference is the pointer type *TwoPhaseIterator, which is
// what the Scorer interface requires.
func (s *DrillSidewaysScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.scorer.TwoPhaseIterator()
}

func (s *DrillSidewaysScorer) AdvanceShallow(target int) (int, error) {
	return s.scorer.AdvanceShallow(target)
}

func (s *DrillSidewaysScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return s.scorer.NextDocsAndScores(upTo, liveDocs, buffer)
}
