// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

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
		minShouldMatch: 0,
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

// Rewrite rewrites the query to a simpler form.
func (q *BooleanQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchAllDocsQuery(), nil
	}

	// If there's only one clause, and it's MUST or FILTER, we can potentially simplify it
	if len(q.clauses) == 1 {
		clause := q.clauses[0]
		if clause.Occur == MUST || clause.Occur == FILTER {
			rewritten, err := clause.Query.Rewrite(reader)
			if err != nil {
				return nil, err
			}
			if clause.Occur == FILTER {
				return NewConstantScoreQueryWithScore(rewritten, 0.0), nil
			}
			return rewritten, nil
		}
	}

	// Default: return itself with rewritten clauses
	newBQ := &BooleanQuery{
		BaseQuery:      q.BaseQuery,
		clauses:        make([]*BooleanClause, len(q.clauses)),
		minShouldMatch: q.minShouldMatch,
	}
	for i, clause := range q.clauses {
		rewritten, err := clause.Query.Rewrite(reader)
		if err != nil {
			return nil, err
		}
		newBQ.clauses[i] = &BooleanClause{
			Query: rewritten,
			Occur: clause.Occur,
		}
	}
	return newBQ, nil
}

// Clone creates a copy of this query.
func (q *BooleanQuery) Clone() Query {
	clonedClauses := make([]*BooleanClause, len(q.clauses))
	for i, clause := range q.clauses {
		clonedClauses[i] = &BooleanClause{
			Query: clause.Query.Clone(),
			Occur: clause.Occur,
		}
	}
	return &BooleanQuery{
		BaseQuery:      q.BaseQuery,
		clauses:        clonedClauses,
		minShouldMatch: q.minShouldMatch,
	}
}

// Equals checks if this query equals another.
func (q *BooleanQuery) Equals(other Query) bool {
	o, ok := other.(*BooleanQuery)
	if !ok {
		return false
	}
	if q.minShouldMatch != o.minShouldMatch || len(q.clauses) != len(o.clauses) {
		return false
	}
	for i, clause := range q.clauses {
		if clause.Occur != o.clauses[i].Occur || !clause.Query.Equals(o.clauses[i].Query) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *BooleanQuery) HashCode() int {
	hash := q.minShouldMatch
	for _, clause := range q.clauses {
		hash = hash*31 + int(clause.Occur)
		hash = hash*31 + clause.Query.HashCode()
	}
	return hash
}

func (q *BooleanQuery) String() string {
	buffer := ""
	if q.minShouldMatch > 0 {
		buffer += fmt.Sprintf("minShouldMatch=%d ", q.minShouldMatch)
	}
	for i, clause := range q.clauses {
		if i > 0 {
			buffer += " "
		}
		switch clause.Occur {
		case MUST:
			buffer += "+"
		case MUST_NOT:
			buffer += "-"
		case FILTER:
			buffer += "#"
		}
		buffer += fmt.Sprintf("%v", clause.Query)
	}
	return buffer
}

// CreateWeight creates a Weight for this query.
func (q *BooleanQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewBooleanWeight(q, searcher, needsScores)
}

// Ensure BooleanQuery implements Query
var _ Query = (*BooleanQuery)(nil)
