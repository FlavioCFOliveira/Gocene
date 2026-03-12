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
