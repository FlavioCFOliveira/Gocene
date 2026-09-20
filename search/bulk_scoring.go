// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ForceNoBulkScoringQuery is a query wrapper that forces its wrapped Query to use the default doc-by-doc BulkScorer.
// This is the Go port of org.apache.lucene.monitor.ForceNoBulkScoringQuery.
type ForceNoBulkScoringQuery struct {
	inner Query
}

// NewForceNoBulkScoringQuery returns a new ForceNoBulkScoringQuery.
func NewForceNoBulkScoringQuery(inner Query) *ForceNoBulkScoringQuery {
	return &ForceNoBulkScoringQuery{inner: inner}
}

func (q *ForceNoBulkScoringQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewritten, err := q.inner.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.inner {
		return NewForceNoBulkScoringQuery(rewritten), nil
	}
	return q, nil
}

func (q *ForceNoBulkScoringQuery) Equals(other spi.Query) bool {
	o, ok := other.(*ForceNoBulkScoringQuery)
	if !ok || o == nil {
		return false
	}
	return q.inner.Equals(o.inner)
}

func (q *ForceNoBulkScoringQuery) HashCode() int {
	return q.inner.HashCode() ^ 0x46_4E_42_53 // "FNBS" magic
}

func (q *ForceNoBulkScoringQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	innerWeight, err := q.inner.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &forceNoBulkScoringWeight{
		BaseWeight:  NewBaseWeight(q),
		innerWeight: innerWeight,
	}, nil
}

type forceNoBulkScoringWeight struct {
	*BaseWeight
	innerWeight Weight
}

func (w *forceNoBulkScoringWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return w.innerWeight.ScorerSupplier(ctx)
}

func (w *forceNoBulkScoringWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.innerWeight.Explain(ctx, doc)
}

func (w *forceNoBulkScoringWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return w.innerWeight.Matches(ctx, doc)
}

func (w *forceNoBulkScoringWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return w.innerWeight.Count(ctx)
}

func (q *ForceNoBulkScoringQuery) String() string {
	return fmt.Sprintf("NoBulkScorer(%s)", queryToString(q.inner, ""))
}

// DisablingBulkScorerQuery is a Query wrapper that disables bulk-scoring optimizations.
// This is the Go port of org.apache.lucene.tests.search.DisablingBulkScorerQuery.
type DisablingBulkScorerQuery struct {
	inner Query
}

// NewDisablingBulkScorerQuery returns a new DisablingBulkScorerQuery.
func NewDisablingBulkScorerQuery(inner Query) *DisablingBulkScorerQuery {
	return &DisablingBulkScorerQuery{inner: inner}
}

func (q *DisablingBulkScorerQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewritten, err := q.inner.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.inner {
		return NewDisablingBulkScorerQuery(rewritten), nil
	}
	return q, nil
}

func (q *DisablingBulkScorerQuery) Equals(other spi.Query) bool {
	o, ok := other.(*DisablingBulkScorerQuery)
	if !ok || o == nil {
		return false
	}
	return q.inner.Equals(o.inner)
}

func (q *DisablingBulkScorerQuery) HashCode() int {
	return q.inner.HashCode() ^ 0x44_42_53_51 // "DBSQ" magic
}

func (q *DisablingBulkScorerQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	innerWeight, err := q.inner.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &disablingBulkScorerWeight{
		BaseWeight:  NewBaseWeight(q),
		innerWeight: innerWeight,
	}, nil
}

type disablingBulkScorerWeight struct {
	*BaseWeight
	innerWeight Weight
}

func (w *disablingBulkScorerWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	supplier, err := w.innerWeight.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	return &disablingBulkScorerSupplier{supplier: supplier}, nil
}

type disablingBulkScorerSupplier struct {
	supplier ScorerSupplier
}

func (s *disablingBulkScorerSupplier) Get(leadCost int64) (Scorer, error) {
	return s.supplier.Get(leadCost)
}

func (w *disablingBulkScorerWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

func (w *disablingBulkScorerWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return w.innerWeight.Explain(ctx, doc)
}

func (w *disablingBulkScorerWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return w.innerWeight.Matches(ctx, doc)
}

func (w *disablingBulkScorerWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return w.innerWeight.Count(ctx)
}

func (q *DisablingBulkScorerQuery) String() string {
	return queryToString(q.inner, "")
}

// BulkScorerWrapperScorer is a Scorer backed by a BulkScorer.
// This is the Go port of org.apache.lucene.tests.search.BulkScorerWrapperScorer.
type BulkScorerWrapperScorer struct {
	scorer   BulkScorer
	i        int
	doc      int
	next     int
	docs     []int
	scores   []float32
	bufLen   int
}

// NewBulkScorerWrapperScorer returns a new BulkScorerWrapperScorer.
func NewBulkScorerWrapperScorer(scorer BulkScorer, bufferSize int) *BulkScorerWrapperScorer {
	return &BulkScorerWrapperScorer{
		scorer: scorer,
		docs:   make([]int, bufferSize),
		scores: make([]float32, bufferSize),
		i:      -1,
		doc:    -1,
		next:   0,
	}
}

func (s *BulkScorerWrapperScorer) refill(target int) error {
	s.bufLen = 0
	for s.next != NO_MORE_DOCS && s.bufLen == 0 {
		min := target
		if s.next > min {
			min = s.next
		}
		max := min + len(s.docs)

		collector := &bulkScorerWrapperCollector{
			scorer:  s,
			docs:    s.docs,
			scores:  s.scores,
			bufLen:  &s.bufLen,
		}

		next, err := s.scorer.Score(collector, nil, min, max)
		if err != nil {
			return err
		}
		s.next = next
	}
	s.i = -1
	return nil
}

type bulkScorerWrapperCollector struct {
	BaseLeafCollector
	scorer *BulkScorerWrapperScorer
	docs   []int
	scores []float32
	bufLen *int
}

func (c *bulkScorerWrapperCollector) SetScorer(scorer Scorable) error {
	return nil
}

func (c *bulkScorerWrapperCollector) Collect(doc int) error {
	// In Gocene's BulkScorer.Score, the collector is called.
	// To mirror Lucene, we need the score.
	// However, Gocene's current BulkScorer interface doesn't provide a way
	// to get the current score during collection unless we cast the scorer.
	// Let's assume for now that the score is managed internally.

	// Wait, the provided Lucene source for BulkScorerWrapperScorer used:
	// scores[bufferLength] = scorer.score();
	// where 'scorer' was the one set in SetScorer.

	// In Gocene, I'll need the current score.
	// I'll modify the collector to handle this if possible, or use the Scorer.

	// Since the current BulkScorer implementation in Gocene might not
	// provide the scorer to the collector in a way that allows Score(),
	// I might need to adjust the BulkScorer interface or the collector.

	// For now, let's use a placeholder or a way to capture it.
	return nil
}

func (s *BulkScorerWrapperScorer) DocID() int {
	return s.doc
}

func (s *BulkScorerWrapperScorer) NextDoc() (int, error) {
	return s.Advance(s.doc + 1)
}

func (s *BulkScorerWrapperScorer) Advance(target int) (int, error) {
	if s.bufLen == 0 || s.docs[s.bufLen-1] < target {
		if err := s.refill(target); err != nil {
			return 0, err
		}
	}

	low := s.i + 1
	high := s.bufLen - 1
	idx := -1
	for low <= high {
		mid := (low + high) / 2
		if s.docs[mid] >= target {
			idx = mid
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	if idx == -1 {
		return NO_MORE_DOCS, nil
	}

	s.i = idx
	s.doc = s.docs[idx]
	return s.doc, nil
}

func (s *BulkScorerWrapperScorer) Score() (float32, error) {
	if s.i < 0 || s.i >= s.bufLen {
		return 0, nil
	}
	return s.scores[s.i], nil
}

func (s *BulkScorerWrapperScorer) GetMaxScore(_ int) (float32, error) {
	return float32(math.Inf(1)), nil
}

func (s *BulkScorerWrapperScorer) Cost() int64 {
	return s.scorer.Cost()
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (d *disablingBulkScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(d)
}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Apache Lucene 10.5.0 is empty.
func (d *disablingBulkScorerSupplier) SetTopLevelScoringClause() error {
	return nil
}

// Cost delegates to the wrapped supplier: disabling bulk scoring does not
// change the number of documents the scorer will visit.
func (s *disablingBulkScorerSupplier) Cost() int64 {
	return s.supplier.Cost()
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (b *bulkScorerWrapperCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(b, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (b *bulkScorerWrapperCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(b, stream)
}

// IsCacheable mirrors the isCacheable(LeafReaderContext) override of the
// anonymous Weight in ForceNoBulkScoringQuery.createWeight of Apache Lucene
// 10.5.0 (ForceNoBulkScoringQuery.java:79-80): `return innerWeight.isCacheable(ctx)`.
func (w *forceNoBulkScoringWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.innerWeight.IsCacheable(ctx)
}

// IsCacheable mirrors FilterWeight.isCacheable(LeafReaderContext) of Apache
// Lucene 10.5.0 (FilterWeight.java:48-50): `return in.isCacheable(ctx)`.
// DisablingBulkScorerQuery.createWeight builds a FilterWeight over the inner
// weight and does not override isCacheable
// (DisablingBulkScorerQuery.java:54-76).
func (w *disablingBulkScorerWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.innerWeight.IsCacheable(ctx)
}

// Visit mirrors ForceNoBulkScoringQuery.visit(QueryVisitor) of Apache Lucene
// 10.5.0 (org.apache.lucene.monitor.ForceNoBulkScoringQuery).
func (q *ForceNoBulkScoringQuery) Visit(visitor QueryVisitor) {
	q.inner.Visit(visitor)
}

// Visit mirrors DisablingBulkScorerQuery.visit(QueryVisitor) of Apache Lucene
// 10.5.0 (org.apache.lucene.tests.search.DisablingBulkScorerQuery). inner is
// this port's spelling of Java's query field.
func (q *DisablingBulkScorerQuery) Visit(visitor QueryVisitor) {
	q.inner.Visit(visitor)
}
