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
