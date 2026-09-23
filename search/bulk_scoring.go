// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
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

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (w *disablingBulkScorerWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// Scorer renders the final Weight.scorer(LeafReaderContext): the scorer of
// scorerSupplier(context).get(Long.MAX_VALUE), or nil when no document
// matches. It is restated because the embedded BaseWeight.Scorer would call
// BaseWeight.ScorerSupplier, not this type's override.
func (w *forceNoBulkScoringWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		return nil, err
	}
	return scorerSupplier.Get(math.MaxInt64)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *forceNoBulkScoringWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
