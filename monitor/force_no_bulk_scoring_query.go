// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// ForceNoBulkScoringQuery is a query wrapper that forces its wrapped Query to
// use the default doc-by-doc BulkScorer.
//
// Port of org.apache.lucene.monitor.ForceNoBulkScoringQuery (Apache Lucene
// 10.5.0).
type ForceNoBulkScoringQuery struct {
	inner search.Query
}

// NewForceNoBulkScoringQuery creates a ForceNoBulkScoringQuery wrapping the
// given inner query.
func NewForceNoBulkScoringQuery(inner search.Query) *ForceNoBulkScoringQuery {
	return &ForceNoBulkScoringQuery{inner: inner}
}

// GetWrappedQuery returns the inner query.
func (q *ForceNoBulkScoringQuery) GetWrappedQuery() search.Query {
	return q.inner
}

// Rewrite rewrites the inner query and wraps the result.
func (q *ForceNoBulkScoringQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	rewritten, err := q.inner.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.inner {
		return NewForceNoBulkScoringQuery(rewritten), nil
	}
	// super.rewrite(indexSearcher) returns this.
	return q, nil
}

// Visit delegates to the inner query's Visit method.
//
// PORT NOTE: Gocene's search.Query interface does not declare Visit, so the
// call is made through the method set the concrete query carries, the same
// idiom already used by ConstantScoreWeight (search/constant_score_weight.go).
func (q *ForceNoBulkScoringQuery) Visit(visitor search.QueryVisitor) {
	if v, ok := q.inner.(interface{ Visit(search.QueryVisitor) }); ok {
		v.Visit(visitor)
	}
}

// Equals reports whether other is a ForceNoBulkScoringQuery wrapping an equal
// inner query.
func (q *ForceNoBulkScoringQuery) Equals(other spi.Query) bool {
	if q == other {
		return true
	}
	that, ok := other.(*ForceNoBulkScoringQuery)
	if !ok {
		return false
	}
	return q.inner.Equals(that.inner)
}

// HashCode renders Objects.hash(inner).
func (q *ForceNoBulkScoringQuery) HashCode() int {
	return 31 + q.inner.HashCode()
}

// CreateWeight delegates to the inner query's CreateWeight and wraps the
// result in a Weight that inherits the default doc-by-doc BulkScorer.
func (q *ForceNoBulkScoringQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	innerWeight, err := q.inner.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return &noBulkScoringWeight{
		inner:       innerWeight,
		parentQuery: q,
	}, nil
}

// ToString renders toString(String s).
func (q *ForceNoBulkScoringQuery) ToString(field string) string {
	return "NoBulkScorer(" + forceNoBulkScoringQueryText(q.inner, field) + ")"
}

// forceNoBulkScoringQueryText renders inner.toString(s).
//
// PORT NOTE: Gocene's search.Query interface does not declare ToString, so the
// call is made through the method set the concrete query carries, the same
// idiom already used by ConstantScoreWeight (search/constant_score_weight.go).
func forceNoBulkScoringQueryText(q search.Query, field string) string {
	if ts, ok := q.(interface{ ToString(string) string }); ok {
		return ts.ToString(field)
	}
	if ts, ok := q.(interface{ String(string) string }); ok {
		return ts.String(field)
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// noBulkScoringWeight renders the anonymous Weight created by
// ForceNoBulkScoringQuery.createWeight. It overrides exactly the four methods
// Java overrides (isCacheable, explain, scorerSupplier, matches) and reproduces
// Weight's own defaults for the rest, so that the wrapped query loses its
// optimised BulkScorer.
type noBulkScoringWeight struct {
	inner       search.Weight
	parentQuery search.Query
}

func (w *noBulkScoringWeight) GetQuery() search.Query {
	return w.parentQuery
}

func (w *noBulkScoringWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.inner.IsCacheable(ctx)
}

func (w *noBulkScoringWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return w.inner.Explain(ctx, doc)
}

func (w *noBulkScoringWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return w.inner.ScorerSupplier(ctx)
}

func (w *noBulkScoringWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	return w.inner.Matches(ctx, doc)
}

// Scorer reproduces Weight.scorer(LeafReaderContext), which this weight does
// not override.
func (w *noBulkScoringWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(math.MaxInt64)
}

// BulkScorer reproduces Weight.bulkScorer(LeafReaderContext), which this weight
// does not override: that default is the doc-by-doc scorer this class exists to
// impose.
func (w *noBulkScoringWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// Count reproduces Weight.count(LeafReaderContext), which this weight does not
// override.
func (w *noBulkScoringWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Ensure types implement the expected interfaces.
var _ search.Query = (*ForceNoBulkScoringQuery)(nil)
var _ search.Weight = (*noBulkScoringWeight)(nil)
