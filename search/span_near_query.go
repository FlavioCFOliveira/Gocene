// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"
)

// SpanNearQuery matches spans that are near each other within a specified slop distance.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanNearQuery.
type SpanNearQuery struct {
	BaseSpanQuery
	clauses []SpanQuery
	slop    int
	inOrder bool
}

// NewSpanNearQuery creates a new SpanNearQuery.
// clauses: the span queries to match near each other
// slop: the maximum distance between spans
// inOrder: whether the spans must appear in the same order as the clauses
func NewSpanNearQuery(clauses []SpanQuery, slop int, inOrder bool) *SpanNearQuery {
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

	return &SpanNearQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		clauses:       clauses,
		slop:          slop,
		inOrder:       inOrder,
	}
}

// Clauses returns the clauses.
func (q *SpanNearQuery) Clauses() []SpanQuery {
	return q.clauses
}

// Slop returns the slop value.
func (q *SpanNearQuery) Slop() int {
	return q.slop
}

// InOrder returns whether the query requires in-order matching.
func (q *SpanNearQuery) InOrder() bool {
	return q.inOrder
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanNearQuery) Rewrite(reader IndexReader) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	if len(q.clauses) == 1 {
		return q.clauses[0], nil
	}
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanNearQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanNearQuery) Clone() Query {
	clausesCopy := make([]SpanQuery, len(q.clauses))
	for i, clause := range q.clauses {
		clausesCopy[i] = clause.Clone().(SpanQuery)
	}
	return &SpanNearQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		clauses:       clausesCopy,
		slop:          q.slop,
		inOrder:       q.inOrder,
	}
}

// Equals checks if this query equals another.
func (q *SpanNearQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanNearQuery); ok {
		if q.field != o.field || q.slop != o.slop || q.inOrder != o.inOrder {
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
func (q *SpanNearQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + len(q.clauses)
	h = 31*h + q.slop
	if q.inOrder {
		h = 31*h + 1
	}
	return h
}

// String returns a string representation of the query.
func (q *SpanNearQuery) String(field string) string {
	var clauseStrs []string
	for _, clause := range q.clauses {
		clauseStrs = append(clauseStrs, clause.String(q.field))
	}

	if field == "" || field != q.field {
		return fmt.Sprintf("SpanNearQuery(field=%s, clauses=[%s], slop=%d, inOrder=%v)",
			q.field, strings.Join(clauseStrs, ", "), q.slop, q.inOrder)
	}
	return fmt.Sprintf("SpanNearQuery(clauses=[%s], slop=%d, inOrder=%v)",
		strings.Join(clauseStrs, ", "), q.slop, q.inOrder)
}

// Ensure SpanNearQuery implements SpanQuery
var _ SpanQuery = (*SpanNearQuery)(nil)
