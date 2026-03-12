// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// Occur specifies how a clause should occur in a BooleanQuery.
type Occur int

const (
	// MUST - the clause must match.
	MUST Occur = iota
	// SHOULD - the clause should match (at least one SHOULD must match).
	SHOULD
	// MUST_NOT - the clause must not match.
	MUST_NOT
	// FILTER - the clause must match (but doesn't affect scoring).
	FILTER
)

func (o Occur) String() string {
	switch o {
	case MUST:
		return "MUST"
	case SHOULD:
		return "SHOULD"
	case MUST_NOT:
		return "MUST_NOT"
	case FILTER:
		return "FILTER"
	default:
		return fmt.Sprintf("Occur(%d)", o)
	}
}

// BooleanClause represents a clause in a BooleanQuery.
type BooleanClause struct {
	Query Query
	Occur Occur
}

// NewBooleanClause creates a new BooleanClause.
func NewBooleanClause(query Query, occur Occur) *BooleanClause {
	return &BooleanClause{Query: query, Occur: occur}
}

// BooleanQuery matches documents matching boolean combinations of clauses.
type BooleanQuery struct {
	*BaseQuery
	clauses        []*BooleanClause
	minShouldMatch int
}

// NewBooleanQuery creates a new BooleanQuery.
func NewBooleanQuery() *BooleanQuery {
	return &BooleanQuery{
		BaseQuery:      &BaseQuery{},
		clauses:        make([]*BooleanClause, 0),
		minShouldMatch: 1,
	}
}

// Add adds a clause to this query.
func (q *BooleanQuery) Add(query Query, occur Occur) {
	q.clauses = append(q.clauses, NewBooleanClause(query, occur))
}

// Clauses returns the clauses in this query.
func (q *BooleanQuery) Clauses() []*BooleanClause {
	return q.clauses
}

// SetMinimumNumberShouldMatch sets the minimum number of SHOULD clauses that must match.
func (q *BooleanQuery) SetMinimumNumberShouldMatch(min int) {
	q.minShouldMatch = min
}

// MinimumNumberShouldMatch returns the minimum number of SHOULD clauses that must match.
func (q *BooleanQuery) MinimumNumberShouldMatch() int {
	return q.minShouldMatch
}

// NewBooleanQueryOrWithQueries creates a BooleanQuery with OR semantics.
func NewBooleanQueryOrWithQueries(queries ...Query) *BooleanQuery {
	bq := NewBooleanQuery()
	for _, q := range queries {
		bq.Add(q, SHOULD)
	}
	return bq
}

// NewBooleanQueryAndWithQueries creates a BooleanQuery with AND semantics.
func NewBooleanQueryAndWithQueries(queries ...Query) *BooleanQuery {
	bq := NewBooleanQuery()
	for _, q := range queries {
		bq.Add(q, MUST)
	}
	return bq
}

// NewBooleanQueryNotWithQuery creates a BooleanQuery with NOT semantics.
func NewBooleanQueryNotWithQuery(query Query) *BooleanQuery {
	bq := NewBooleanQuery()
	bq.Add(query, MUST_NOT)
	return bq
}
