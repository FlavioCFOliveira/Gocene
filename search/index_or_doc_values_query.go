// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// IndexOrDocValuesQuery wraps two queries that must match the same documents
// with the same scores. Lucene uses one for sequential iteration (index path)
// and one for random access (doc-values path), choosing dynamically based on
// the lead cost.
//
// Mirrors org.apache.lucene.search.IndexOrDocValuesQuery.
type IndexOrDocValuesQuery struct {
	BaseQuery
	indexQuery        Query
	randomAccessQuery Query
}

// NewIndexOrDocValuesQuery constructs a wrapper around two equivalent queries.
// Both queries must be non-nil and produce identical matches and scores; this
// is the caller's responsibility.
func NewIndexOrDocValuesQuery(indexQuery, randomAccessQuery Query) *IndexOrDocValuesQuery {
	if indexQuery == nil || randomAccessQuery == nil {
		panic("IndexOrDocValuesQuery: both indexQuery and randomAccessQuery are required")
	}
	return &IndexOrDocValuesQuery{indexQuery: indexQuery, randomAccessQuery: randomAccessQuery}
}

// GetIndexQuery returns the iteration-optimized query.
func (q *IndexOrDocValuesQuery) GetIndexQuery() Query { return q.indexQuery }

// GetRandomAccessQuery returns the random-access (doc-values) query.
func (q *IndexOrDocValuesQuery) GetRandomAccessQuery() Query { return q.randomAccessQuery }

// String returns a debug representation.
func (q *IndexOrDocValuesQuery) String() string {
	return fmt.Sprintf("IndexOrDocValuesQuery(indexQuery=%v, dvQuery=%v)", sprintQuery(q.indexQuery), sprintQuery(q.randomAccessQuery))
}

// Equals checks structural equality.
func (q *IndexOrDocValuesQuery) Equals(other Query) bool {
	o, ok := other.(*IndexOrDocValuesQuery)
	if !ok {
		return false
	}
	return q.indexQuery.Equals(o.indexQuery) && q.randomAccessQuery.Equals(o.randomAccessQuery)
}

// HashCode returns a stable hash.
func (q *IndexOrDocValuesQuery) HashCode() int {
	h := 17
	h = 31*h + q.indexQuery.HashCode()
	h = 31*h + q.randomAccessQuery.HashCode()
	return h
}

// Clone returns an independent copy.
func (q *IndexOrDocValuesQuery) Clone() Query {
	return &IndexOrDocValuesQuery{
		indexQuery:        q.indexQuery.Clone(),
		randomAccessQuery: q.randomAccessQuery.Clone(),
	}
}

// Rewrite rewrites both wrapped queries.
func (q *IndexOrDocValuesQuery) Rewrite(reader IndexReader) (Query, error) {
	rwIdx, err := q.indexQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	rwRand, err := q.randomAccessQuery.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rwIdx == q.indexQuery && rwRand == q.randomAccessQuery {
		return q, nil
	}
	return &IndexOrDocValuesQuery{indexQuery: rwIdx, randomAccessQuery: rwRand}, nil
}

// CreateWeight delegates to the indexQuery: in Lucene this returns a wrapper
// Weight that chooses between the two scorers based on lead cost. Here we
// take the indexQuery path by default and let downstream code optimise when
// random-access cost data becomes available.
func (q *IndexOrDocValuesQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.indexQuery.CreateWeight(searcher, needsScores, boost)
}
