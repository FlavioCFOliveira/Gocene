// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// BoostQuery wraps another query with a boost factor.
type BoostQuery struct {
	*BaseQuery
	query Query
	boost float32
}

// NewBoostQuery creates a new BoostQuery.
func NewBoostQuery(query Query, boost float32) *BoostQuery {
	return &BoostQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
		boost:     boost,
	}
}

// Query returns the wrapped query.
func (q *BoostQuery) Query() Query {
	return q.query
}

// Boost returns the boost factor.
func (q *BoostQuery) Boost() float32 {
	return q.boost
}

// Clone creates a copy of this query.
func (q *BoostQuery) Clone() Query {
	if q.query == nil {
		return &BoostQuery{
			BaseQuery: &BaseQuery{},
			query:     nil,
			boost:     q.boost,
		}
	}
	return &BoostQuery{
		BaseQuery: &BaseQuery{},
		query:     q.query.Clone(),
		boost:     q.boost,
	}
}

// Equals checks if this query equals another.
func (q *BoostQuery) Equals(other Query) bool {
	if o, ok := other.(*BoostQuery); ok {
		if q.boost != o.boost {
			return false
		}
		if q.query == nil || o.query == nil {
			return q.query == nil && o.query == nil
		}
		return q.query.Equals(o.query)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *BoostQuery) HashCode() int {
	hash := 0
	if q.query != nil {
		hash = q.query.HashCode()
	}
	return hash*31 + int(q.boost*1000)
}

// Rewrite rewrites the query to a simpler form.
// Mirrors BoostQuery.rewrite(IndexSearcher) from Lucene 10.4.0.
func (q *BoostQuery) Rewrite(reader IndexReader) (Query, error) {
	rewritten, err := fullyRewrite(q.query, reader)
	if err != nil {
		return nil, err
	}

	if q.boost == 1.0 {
		return rewritten, nil
	}

	// Merge nested BoostQuery.
	if inner, ok := rewritten.(*BoostQuery); ok {
		return NewBoostQuery(inner.query, q.boost*inner.boost), nil
	}

	// Bubble up MatchNoDocsQuery.
	if isMatchNoDocsQuery(rewritten) {
		return rewritten, nil
	}

	// Boost==0 and inner is not already a CSQ: wrap in CSQ to suppress scoring.
	if q.boost == 0.0 {
		if _, ok := rewritten.(*ConstantScoreQuery); !ok {
			return NewBoostQuery(NewConstantScoreQuery(rewritten), 0), nil
		}
	}

	if rewritten != q.query {
		return NewBoostQuery(rewritten, q.boost), nil
	}

	return q, nil
}

// CreateWeight delegates to the inner query's Weight with the boost multiplied
// into the requested boost, so the inner scorer's scores (and max scores) are
// scaled by this query's boost factor.
//
// Faithful port of BoostQuery.createWeight(IndexSearcher, ScoreMode, float):
// query.createWeight(searcher, scoreMode, this.boost * boost).
func (q *BoostQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return q.query.CreateWeight(searcher, needsScores, q.boost*boost)
}
