// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DisjunctionMaxQuery is a query that generates the union of documents produced by its subqueries,
// and that scores each document with the maximum score for that document produced by any subquery,
// plus a tie breaking increment for any additional matching subqueries.
type DisjunctionMaxQuery struct {
	*BaseQuery
	disjuncts    []Query
	tieBreakerMultiplier float32
}

// NewDisjunctionMaxQuery creates a new DisjunctionMaxQuery.
func NewDisjunctionMaxQuery(disjuncts []Query) *DisjunctionMaxQuery {
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            disjuncts,
		tieBreakerMultiplier: 0.0,
	}
}

// NewDisjunctionMaxQueryWithTieBreaker creates a DisjunctionMaxQuery with a tie breaker multiplier.
// The tieBreakerMultiplier allows documents with multiple matching subqueries to be scored
// higher than documents with only a single matching subquery.
func NewDisjunctionMaxQueryWithTieBreaker(disjuncts []Query, tieBreakerMultiplier float32) *DisjunctionMaxQuery {
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            disjuncts,
		tieBreakerMultiplier: tieBreakerMultiplier,
	}
}

// Disjuncts returns the disjuncts (subqueries).
func (q *DisjunctionMaxQuery) Disjuncts() []Query {
	return q.disjuncts
}

// Add adds a subquery to this disjunction.
func (q *DisjunctionMaxQuery) Add(query Query) {
	q.disjuncts = append(q.disjuncts, query)
}

// TieBreakerMultiplier returns the tie breaker multiplier.
func (q *DisjunctionMaxQuery) TieBreakerMultiplier() float32 {
	return q.tieBreakerMultiplier
}

// SetTieBreakerMultiplier sets the tie breaker multiplier.
func (q *DisjunctionMaxQuery) SetTieBreakerMultiplier(tieBreakerMultiplier float32) {
	q.tieBreakerMultiplier = tieBreakerMultiplier
}

// Clone creates a copy of this query.
func (q *DisjunctionMaxQuery) Clone() Query {
	clonedDisjuncts := make([]Query, len(q.disjuncts))
	for i, disjunct := range q.disjuncts {
		if disjunct != nil {
			clonedDisjuncts[i] = disjunct.Clone()
		}
	}
	return &DisjunctionMaxQuery{
		BaseQuery:            &BaseQuery{},
		disjuncts:            clonedDisjuncts,
		tieBreakerMultiplier: q.tieBreakerMultiplier,
	}
}

// Equals checks if this query equals another.
func (q *DisjunctionMaxQuery) Equals(other Query) bool {
	if o, ok := other.(*DisjunctionMaxQuery); ok {
		if q.tieBreakerMultiplier != o.tieBreakerMultiplier || len(q.disjuncts) != len(o.disjuncts) {
			return false
		}
		for i, disjunct := range q.disjuncts {
			if disjunct == nil || o.disjuncts[i] == nil {
				if disjunct != nil || o.disjuncts[i] != nil {
					return false
				}
				continue
			}
			if !disjunct.Equals(o.disjuncts[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *DisjunctionMaxQuery) HashCode() int {
	hash := 0
	for _, disjunct := range q.disjuncts {
		if disjunct != nil {
			hash = hash*31 + disjunct.HashCode()
		}
	}
	return hash*31 + int(q.tieBreakerMultiplier*1000)
}
