// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"
)

// SpanOrQuery combines multiple span queries with OR logic.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanOrQuery.
type SpanOrQuery struct {
	BaseSpanQuery
	clauses []SpanQuery
}

// NewSpanOrQuery creates a new SpanOrQuery.
func NewSpanOrQuery(clauses ...SpanQuery) *SpanOrQuery {
	if len(clauses) == 0 {
		return nil
	}

	// All clauses must have the same field
	field := clauses[0].GetField()
	for _, clause := range clauses {
		if clause.GetField() != field {
			return nil
		}
	}

	return &SpanOrQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		clauses:       clauses,
	}
}

// Clauses returns the clauses.
func (q *SpanOrQuery) Clauses() []SpanQuery {
	return q.clauses
}

// AddClause adds a clause to this query.
func (q *SpanOrQuery) AddClause(clause SpanQuery) {
	if clause.GetField() == q.field {
		q.clauses = append(q.clauses, clause)
	}
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanOrQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	if len(q.clauses) == 1 {
		return q.clauses[0], nil
	}
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanOrQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanOrQuery) Clone() Query {
	clausesCopy := make([]SpanQuery, len(q.clauses))
	for i, clause := range q.clauses {
		clausesCopy[i] = clause.Clone().(SpanQuery)
	}
	return &SpanOrQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		clauses:       clausesCopy,
	}
}

// Equals checks if this query equals another.
func (q *SpanOrQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanOrQuery); ok {
		if q.field != o.field {
			return false
		}
		if len(q.clauses) != len(o.clauses) {
			return false
		}
		for i := range q.clauses {
			if !q.clauses[i].Equals(o.clauses[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanOrQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + len(q.clauses)
	return h
}

// String returns a string representation of the query.
func (q *SpanOrQuery) String(field string) string {
	var clauseStrs []string
	for _, clause := range q.clauses {
		clauseStrs = append(clauseStrs, clause.String(q.field))
	}

	if field == "" || field != q.field {
		return fmt.Sprintf("SpanOrQuery(field=%s, clauses=[%s])",
			q.field, strings.Join(clauseStrs, ", "))
	}
	return fmt.Sprintf("SpanOrQuery(clauses=[%s])", strings.Join(clauseStrs, ", "))
}

// Ensure SpanOrQuery implements SpanQuery
var _ SpanQuery = (*SpanOrQuery)(nil)
