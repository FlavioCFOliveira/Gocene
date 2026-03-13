// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ConstantScoreQuery wraps another query and assigns a constant score to all matching documents.
// This is useful when you want to filter documents without affecting the ranking based on
// the original query's scoring.
type ConstantScoreQuery struct {
	*BaseQuery
	query Query
	score float32
}

// NewConstantScoreQuery creates a new ConstantScoreQuery.
// The default score is 1.0.
func NewConstantScoreQuery(query Query) *ConstantScoreQuery {
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
		score:     1.0,
	}
}

// NewConstantScoreQueryWithScore creates a ConstantScoreQuery with a custom score.
func NewConstantScoreQueryWithScore(query Query, score float32) *ConstantScoreQuery {
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     query,
		score:     score,
	}
}

// Query returns the wrapped query.
func (q *ConstantScoreQuery) Query() Query {
	return q.query
}

// Score returns the constant score.
func (q *ConstantScoreQuery) Score() float32 {
	return q.score
}

// SetScore sets the constant score.
func (q *ConstantScoreQuery) SetScore(score float32) {
	q.score = score
}

// Clone creates a copy of this query.
func (q *ConstantScoreQuery) Clone() Query {
	if q.query == nil {
		return &ConstantScoreQuery{
			BaseQuery: &BaseQuery{},
			query:     nil,
			score:     q.score,
		}
	}
	return &ConstantScoreQuery{
		BaseQuery: &BaseQuery{},
		query:     q.query.Clone(),
		score:     q.score,
	}
}

// Equals checks if this query equals another.
func (q *ConstantScoreQuery) Equals(other Query) bool {
	if o, ok := other.(*ConstantScoreQuery); ok {
		if q.score != o.score {
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
func (q *ConstantScoreQuery) HashCode() int {
	hash := 0
	if q.query != nil {
		hash = q.query.HashCode()
	}
	return hash*31 + int(q.score*1000)
}

// Rewrite rewrites the query to a simpler form.
func (q *ConstantScoreQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.query == nil {
		return q, nil
	}
	rewritten, err := q.query.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rewritten != q.query {
		return NewConstantScoreQueryWithScore(rewritten, q.score), nil
	}
	return q, nil
}
