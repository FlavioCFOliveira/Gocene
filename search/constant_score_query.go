// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConstantScoreQuery is a query that wraps another query and simply returns a constant score
// equal to 1 for every document that matches the query. It therefore simply strips of all
// scores and always returns 1.
//
// This is the Go port of org.apache.lucene.search.ConstantScoreQuery (Lucene 10.5.0).
type ConstantScoreQuery struct {
	*BaseQuery
	query Query
}

// NewConstantScoreQuery strips off scores from the passed in Query.
// The hits will get a constant score of 1.
func NewConstantScoreQuery(query Query) *ConstantScoreQuery {
	if query == nil {
		panic("Query must not be null")
	}
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
	}
}

// GetQuery returns the encapsulated query.
func (q *ConstantScoreQuery) GetQuery() Query {
	return q.query
}

// Rewrite rewrites the query to a simpler form.
func (q *ConstantScoreQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewritten, err := q.query.Rewrite(searcher)
	if err != nil {
		return nil, err
	}

	// Do some extra simplifications that are legal since scores are not needed on the wrapped query.
	if bq, ok := rewritten.(*BoostQuery); ok {
		rewritten = bq.Query()
	} else if csq, ok := rewritten.(*ConstantScoreQuery); ok {
		rewritten = csq.GetQuery()
	} else if bq, ok := rewritten.(*BooleanQuery); ok {
		rewritten = bq.RewriteNoScoring()
	}

	if _, ok := rewritten.(*MatchNoDocsQuery); ok {
		// bubble up MatchNoDocsQuery
		return rewritten, nil
	}

	if rewritten != q.query {
		return NewConstantScoreQuery(rewritten), nil
	}

	if _, ok := rewritten.(*ConstantScoreQuery); ok {
		return rewritten, nil
	}

	if bq, ok := rewritten.(*BoostQuery); ok {
		return NewConstantScoreQuery(bq.Query()), nil
	}

	return q.BaseQuery.Rewrite(searcher)
}

// Visit walks the query tree.
func (q *ConstantScoreQuery) Visit(visitor QueryVisitor) {
	// In Lucene: query.visit(visitor.getSubVisitor(BooleanClause.Occur.FILTER, this));
	q.query.Visit(visitor.GetSubVisitor(MUST, q))
}

// CreateWeight creates a Weight for this query.
func (q *ConstantScoreQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return q.CreateWeightScoreMode(searcher, scoreMode, boost)
}

func scoreModeFromNeedsScores(needsScores bool) ScoreMode {
	if needsScores {
		return COMPLETE
	}
	return COMPLETE_NO_SCORES
}

// CreateWeightScoreMode builds a Weight for the given full ScoreMode.
func (q *ConstantScoreQuery) CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	// If the score mode is exhaustive then pass COMPLETE_NO_SCORES, otherwise pass TOP_DOCS to make
	// sure to not disable any of the dynamic pruning optimizations for queries sorted by field or
	// top scores.
	var innerScoreMode ScoreMode
	if scoreMode.IsExhaustive() {
		innerScoreMode = COMPLETE_NO_SCORES
	} else {
		innerScoreMode = TOP_DOCS
	}

	innerWeight, err := searcher.CreateWeight(q.query, innerScoreMode, 1.0)
	if err != nil {
		return nil, err
	}

	if scoreMode.NeedsScores() {
		// Java builds `new ConstantScoreWeight(this, boost) { ... }`, whose
		// anonymous body overrides scorerSupplier, matches, isCacheable and
		// count over innerWeight. In Go the weight is allocated first so the
		// scorerSupplier override can be handed to the base class as a method
		// value bound to it, exactly as the anonymous class closes over its
		// enclosing instance.
		w := &constantScoreQueryWeight{
			scoreMode:   scoreMode,
			innerWeight: innerWeight,
		}
		w.ConstantScoreWeight = *NewConstantScoreWeight(q, boost, w.scorerSupplier, nil)
		return w, nil
	}

	return innerWeight, nil
}

type constantScoreQueryWeight struct {
	ConstantScoreWeight
	scoreMode   ScoreMode
	innerWeight Weight
}

// scorerSupplier mirrors the anonymous ConstantScoreWeight's
// scorerSupplier(LeafReaderContext) override in ConstantScoreQuery.java.
func (w *constantScoreQueryWeight) scorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	innerSupplier, err := w.innerWeight.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if innerSupplier == nil {
		return nil, nil
	}
	return &constantScoreQueryScorerSupplier{
		innerSupplier: innerSupplier,
		weight:         w,
	}, nil
}

func (w *constantScoreQueryWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return w.innerWeight.Matches(ctx, doc)
}

func (w *constantScoreQueryWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.innerWeight.IsCacheable(ctx)
}

func (w *constantScoreQueryWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return w.innerWeight.Count(ctx)
}

type constantScoreQueryScorerSupplier struct {
	innerSupplier ScorerSupplier
	weight        *constantScoreQueryWeight
}

func (s *constantScoreQueryScorerSupplier) Get(leadCost int64) (Scorer, error) {
	innerScorer, err := s.innerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	if innerScorer == nil {
		return nil, nil
	}

	twoPhase := innerScorer.TwoPhaseIterator()
	if twoPhase == nil {
		return NewConstantScoreScorer(s.weight.Score(), s.weight.scoreMode, innerScorer.Iterator()), nil
	}
	return NewConstantScoreScorerFromTwoPhase(s.weight.Score(), s.weight.scoreMode, twoPhase), nil
}

func (s *constantScoreQueryScorerSupplier) BulkScorer() (BulkScorer, error) {
	if !s.weight.scoreMode.IsExhaustive() {
		return nil, nil // Use default BulkScorer from Weight if not exhaustive
	}
	innerBulkScorer, err := s.innerSupplier.BulkScorer()
	if err != nil {
		return nil, err
	}
	if innerBulkScorer == nil {
		return nil, nil
	}
	return &constantBulkScorer{
		bulkScorer: innerBulkScorer,
		weight:     s.weight,
		theScore:   s.weight.Score(),
	}, nil
}

func (s *constantScoreQueryScorerSupplier) Cost() int64 {
	return s.innerSupplier.Cost()
}

func (s *constantScoreQueryScorerSupplier) SetTopLevelScoringClause() error {
	return s.innerSupplier.SetTopLevelScoringClause()
}

type constantBulkScorer struct {
	bulkScorer BulkScorer
	weight     *constantScoreQueryWeight
	theScore   float32
}

func (bs *constantBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	return bs.bulkScorer.Score(bs.wrapCollector(collector), acceptDocs, min, max)
}

func (bs *constantBulkScorer) wrapCollector(collector LeafCollector) LeafCollector {
	return &filterLeafCollector{
		collector: collector,
		wrapScorer: func(scorer Scorable) error {
			return collector.SetScorer(&filterScorable{
				scorable: scorer,
				score:    bs.theScore,
			})
		},
	}
}

func (bs *constantBulkScorer) Cost() int64 {
	return bs.bulkScorer.Cost()
}

type filterLeafCollector struct {
	collector  LeafCollector
	wrapScorer func(Scorable) error
}

func (f *filterLeafCollector) SetScorer(scorer Scorable) error {
	return f.wrapScorer(scorer)
}

func (f *filterLeafCollector) Collect(doc int) error {
	return f.collector.Collect(doc)
}

func (f *filterLeafCollector) CollectRange(min, max int) error {
	return f.collector.CollectRange(min, max)
}

func (f *filterLeafCollector) CollectStream(stream DocIdStream) error {
	return f.collector.CollectStream(stream)
}

func (f *filterLeafCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return f.collector.CompetitiveIterator()
}

func (f *filterLeafCollector) Finish() error {
	return f.collector.Finish()
}

type filterScorable struct {
	scorable Scorable
	score    float32
}

func (f *filterScorable) Score() (float32, error) {
	return f.score, nil
}

func (f *filterScorable) SmoothingScore(docID int) (float32, error) {
	return f.scorable.SmoothingScore(docID)
}

func (f *filterScorable) SetMinCompetitiveScore(minScore float32) error {
	return f.scorable.SetMinCompetitiveScore(minScore)
}

func (f *filterScorable) GetChildren() ([]ChildScorable, error) {
	return f.scorable.GetChildren()
}

func (q *ConstantScoreQuery) ToString(field string) string {
	return fmt.Sprintf("ConstantScore(%s)", queryToString(q.query, field))
}

func (q *ConstantScoreQuery) Equals(other spi.Query) bool {
	if o, ok := other.(*ConstantScoreQuery); ok {
		return q.query.Equals(o.query)
	}
	return false
}

func (q *ConstantScoreQuery) HashCode() int {
	return 31*0 + q.query.HashCode() // Using classHash() = 0 as a simplification
}
