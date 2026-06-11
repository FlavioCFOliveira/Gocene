// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ForceNoBulkScoringQuery wraps a Query to force doc-by-doc scoring by
// preventing the use of a BulkScorer. The created Weight delegates all
// methods to the inner weight but overrides BulkScorer to return nil,
// so IndexSearcher falls back to the Scorer-based path.
//
// This is the Go port of
// org.apache.lucene.monitor.ForceNoBulkScoringQuery from Apache Lucene 10.4.0.
//
// Deviation from Lucene: Gocene's Query.CreateWeight uses (needsScores bool)
// instead of (scoreMode ScoreMode). The inner query's CreateWeight is called
// with the same needsScores value.
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
func (q *ForceNoBulkScoringQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	rewritten, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten != q.inner {
		return NewForceNoBulkScoringQuery(rewritten), nil
	}
	return rewritten, nil
}

// Clone returns a copy of this query.
func (q *ForceNoBulkScoringQuery) Clone() search.Query {
	return NewForceNoBulkScoringQuery(q.inner.Clone())
}

// Equals returns true if the other query is a ForceNoBulkScoringQuery wrapping
// an equal inner query.
func (q *ForceNoBulkScoringQuery) Equals(other search.Query) bool {
	if other == nil {
		return false
	}
	otherQ, ok := other.(*ForceNoBulkScoringQuery)
	if !ok {
		return false
	}
	return q.inner.Equals(otherQ.inner)
}

// HashCode returns a hash code based on the inner query.
func (q *ForceNoBulkScoringQuery) HashCode() int {
	return q.inner.HashCode()*31 + 17
}

// CreateWeight delegates to the inner query's CreateWeight and wraps the
// result in a Weight that suppresses BulkScorer usage.
func (q *ForceNoBulkScoringQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	innerWeight, err := q.inner.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	return &noBulkScoringWeight{
		inner:       innerWeight,
		parentQuery: q,
	}, nil
}

// Visit delegates to the inner query's Visit method when available via type
// assertion; otherwise treats the inner query as a leaf.
//
// Deviation from Lucene: Gocene's Query interface does not include Visit,
// so we reach it via optional interface assertion.
func (q *ForceNoBulkScoringQuery) Visit(visitor search.QueryVisitor) {
	if v, ok := q.inner.(interface{ Visit(search.QueryVisitor) }); ok {
		v.Visit(visitor)
	} else {
		// If the inner query does not implement Visit, treat it as a leaf.
		visitor.VisitLeaf(q.inner)
	}
}

// String returns a string representation of this query.
func (q *ForceNoBulkScoringQuery) String(field string) string {
	return "NoBulkScorer(inner=" + fmt.Sprintf("%T", q.inner) + ")"
}

// noBulkScoringWeight is a Weight that wraps an inner weight but returns nil
// from BulkScorer, forcing IndexSearcher to use doc-by-doc scoring.
type noBulkScoringWeight struct {
	inner       search.Weight
	parentQuery search.Query
}

func (w *noBulkScoringWeight) GetQuery() search.Query {
	return w.parentQuery
}

func (w *noBulkScoringWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return w.inner.Explain(context, doc)
}

func (w *noBulkScoringWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return w.inner.ScorerSupplier(context)
}

func (w *noBulkScoringWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	return w.inner.Scorer(context)
}

func (w *noBulkScoringWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	// Return nil to suppress BulkScorer usage, forcing doc-by-doc scoring.
	return nil, nil
}

func (w *noBulkScoringWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.inner.IsCacheable(ctx)
}

func (w *noBulkScoringWeight) Count(context *index.LeafReaderContext) (int, error) {
	return w.inner.Count(context)
}

func (w *noBulkScoringWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	return w.inner.Matches(context, doc)
}

// Ensure types implement the expected interfaces.
var _ search.Query = (*ForceNoBulkScoringQuery)(nil)
var _ search.Weight = (*noBulkScoringWeight)(nil)
