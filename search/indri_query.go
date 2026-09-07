// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"
)

// IndriQuery is a basic abstract query that all IndriQueries can extend to implement ToString, Equals,
// GetClauses, and iterator.
type IndriQuery struct {
	BaseQuery
	clauses []*BooleanClause
}

// NewIndriQuery constructs an IndriQuery.
func NewIndriQuery(clauses []*BooleanClause) *IndriQuery {
	return &IndriQuery{
		clauses: clauses,
	}
}

// ToString returns a user-readable version of this query.
func (q *IndriQuery) ToString(field string) string {
	var sb strings.Builder

	for i, c := range q.clauses {
		sb.WriteString(c.Occur().String())

		subQuery := c.Query()
		if bq, ok := subQuery.(*BooleanQuery); ok {
			sb.WriteString("(")
			sb.WriteString(bq.ToString(field))
			sb.WriteString(")")
		} else {
			// We use a type assertion here because ToString is not in the Query interface
			// but is implemented by most query types in Gocene.
			if ts, ok := subQuery.(interface{ ToString(string) string }); ok {
				sb.WriteString(ts.ToString(field))
			} else {
				sb.WriteString(fmt.Sprintf("%v", subQuery))
			}
		}

		if i != len(q.clauses)-1 {
			sb.WriteString(" ")
		}
	}

	return sb.String()
}

// Equals checks if this query equals another.
func (q *IndriQuery) Equals(other Query) bool {
	otherQuery, ok := other.(*IndriQuery)
	if !ok {
		return false
	}
	return q.equalsTo(otherQuery)
}

func (q *IndriQuery) equalsTo(other *IndriQuery) bool {
	if len(q.clauses) != len(other.clauses) {
		return false
	}
	for i := range q.clauses {
		if q.clauses[i].Query() != other.clauses[i].Query() || q.clauses[i].Occur() != other.clauses[i].Occur() {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *IndriQuery) HashCode() int {
	h := 1
	for _, c := range q.clauses {
		// Mirroring the effect of Objects.hash(clauses) by combining
		// the hash codes of the clauses' components.
		h = 31*h + c.Query().HashCode()
		h = 31*h + int(c.Occur())
	}
	if h == 0 {
		h = 1
	}
	return h
}

// Visit implements the Query visitor pattern.
func (q *IndriQuery) Visit(visitor QueryVisitor) {
	visitor.VisitLeaf(q)
}

// GetClauses returns the clauses of this query.
func (q *IndriQuery) GetClauses() []*BooleanClause {
	return q.clauses
}
