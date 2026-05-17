// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// RescoreTopNQuery wraps a Query so the top N hits are rescored using a
// Rescorer before being returned. CreateWeight defers to the inner Query;
// rescoring happens in IndexSearcher.search after the initial hits are
// gathered.
//
// Mirrors org.apache.lucene.search.RescoreTopNQuery.
type RescoreTopNQuery struct {
	BaseQuery
	inner    Query
	rescorer Rescorer
	topN     int
}

// NewRescoreTopNQuery constructs a RescoreTopNQuery. topN must be > 0 and
// inner and rescorer must be non-nil.
func NewRescoreTopNQuery(inner Query, rescorer Rescorer, topN int) *RescoreTopNQuery {
	if inner == nil {
		panic("RescoreTopNQuery: inner query is required")
	}
	if rescorer == nil {
		panic("RescoreTopNQuery: rescorer is required")
	}
	if topN <= 0 {
		panic("RescoreTopNQuery: topN must be > 0")
	}
	return &RescoreTopNQuery{inner: inner, rescorer: rescorer, topN: topN}
}

// Inner returns the wrapped query.
func (q *RescoreTopNQuery) Inner() Query { return q.inner }

// Rescorer returns the rescorer used after initial scoring.
func (q *RescoreTopNQuery) Rescorer() Rescorer { return q.rescorer }

// TopN returns the rescore window size.
func (q *RescoreTopNQuery) TopN() int { return q.topN }

// String returns a debug representation.
func (q *RescoreTopNQuery) String() string {
	return fmt.Sprintf("RescoreTopNQuery(inner=%v, topN=%d)", sprintQuery(q.inner), q.topN)
}

// Equals checks structural equality (ignoring the rescorer's internal state).
func (q *RescoreTopNQuery) Equals(other Query) bool {
	o, ok := other.(*RescoreTopNQuery)
	if !ok {
		return false
	}
	return q.topN == o.topN && q.inner.Equals(o.inner)
}

// HashCode returns a stable hash.
func (q *RescoreTopNQuery) HashCode() int {
	h := 17
	h = 31*h + q.inner.HashCode()
	h = 31*h + q.topN
	return h
}

// Clone returns an independent copy that shares the same rescorer.
func (q *RescoreTopNQuery) Clone() Query {
	return &RescoreTopNQuery{inner: q.inner.Clone(), rescorer: q.rescorer, topN: q.topN}
}

// Rewrite rewrites the inner query.
func (q *RescoreTopNQuery) Rewrite(reader IndexReader) (Query, error) {
	rw, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rw == q.inner {
		return q, nil
	}
	return &RescoreTopNQuery{inner: rw, rescorer: q.rescorer, topN: q.topN}, nil
}

// CreateWeight delegates to the inner query; rescoring is applied by the
// caller (IndexSearcher) after top-N hits have been collected.
func (q *RescoreTopNQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.inner.CreateWeight(searcher, needsScores, boost)
}
