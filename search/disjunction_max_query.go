// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"strings"
)

// DisjunctionMaxQuery is a query that generates the union of documents produced by its subqueries,
// and that scores each document with the maximum score for that document as produced by any subquery,
// plus a tie breaking increment for any additional matching subqueries.
// Mirrors org.apache.lucene.search.DisjunctionMaxQuery.
type DisjunctionMaxQuery struct {
	*BaseQuery
	disjuncts            []Query
	tieBreakerMultiplier float32
}

// NewDisjunctionMaxQuery creates a new DisjunctionMaxQuery.
// tieBreakerMultiplier must be in [0, 1].
func NewDisjunctionMaxQuery(disjuncts []Query, tieBreakerMultiplier float32) *DisjunctionMaxQuery {
	if tieBreakerMultiplier < 0 || tieBreakerMultiplier > 1 {
		panic("tieBreakerMultiplier must be in [0, 1]")
	}
	if disjuncts == nil {
		disjuncts = []Query{}
	}
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

// TieBreakerMultiplier returns the tie breaker value for multiple matches.
func (q *DisjunctionMaxQuery) TieBreakerMultiplier() float32 {
	return q.tieBreakerMultiplier
}

// ToString returns a user-readable version of this query.
// Mirrors DisjunctionMaxQuery.toString.
func (q *DisjunctionMaxQuery) ToString(field string) string {
	var sb strings.Builder
	sb.WriteString("(")
	for i, sub := range q.disjuncts {
		if bq, ok := sub.(*BooleanQuery); ok {
			sb.WriteString("(")
			sb.WriteString(bq.ToString(field))
			sb.WriteString(")")
		} else {
			sb.WriteString(queryToString(sub, field))
		}
		if i < len(q.disjuncts)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(")")
	if q.tieBreakerMultiplier != 0.0 {
		sb.WriteString(fmt.Sprintf("~%g", q.tieBreakerMultiplier))
	}
	return sb.String()
}

// Visit implements the Query visitor pattern.
// Mirrors DisjunctionMaxQuery.visit.
func (q *DisjunctionMaxQuery) Visit(visitor QueryVisitor) {
	v := visitor.GetSubVisitor(SHOULD, q)
	for _, sub := range q.disjuncts {
		sub.Visit(v)
	}
}

// Equals checks if this query equals another.
// Mirrors DisjunctionMaxQuery.equals.
func (q *DisjunctionMaxQuery) Equals(other spi.Query) bool {
	o, ok := other.(*DisjunctionMaxQuery)
	if !ok {
		return false
	}
	if q.tieBreakerMultiplier != o.tieBreakerMultiplier || len(q.disjuncts) != len(o.disjuncts) {
		return false
	}
	for i, disjunct := range q.disjuncts {
		if !disjunct.Equals(o.disjuncts[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
// Mirrors DisjunctionMaxQuery.hashCode.
func (q *DisjunctionMaxQuery) HashCode() int {
	h := 31 * 12345 // classHash approximation
	h = 31*h + int(q.tieBreakerMultiplier*1000)

	sliceHash := 0
	for _, d := range q.disjuncts {
		sliceHash = 31*sliceHash + d.HashCode()
	}
	h = 31*h + sliceHash

	return h
}

// Rewrite optimizes this query and its sub-queries.
// Mirrors DisjunctionMaxQuery.rewrite.
func (q *DisjunctionMaxQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.disjuncts) == 0 {
		return NewMatchNoDocsQuery("empty DisjunctionMaxQuery"), nil
	}
	if len(q.disjuncts) == 1 {
		return q.disjuncts[0], nil
	}
	if q.tieBreakerMultiplier == 1.0 {
		bq := NewBooleanQueryBuilder()
		for _, sub := range q.disjuncts {
			bq.Add(sub, SHOULD)
		}
		return bq.Build(), nil
	}

	actuallyRewritten := false
	rewrittenDisjuncts := make([]Query, 0, len(q.disjuncts))
	for _, sub := range q.disjuncts {
		rewrittenSub, err := sub.Rewrite(searcher)
		if err != nil {
			return nil, err
		}
		if rewrittenSub != sub || isMatchNoDocs(rewrittenSub) {
			actuallyRewritten = true
		}
		if !isMatchNoDocs(rewrittenSub) {
			rewrittenDisjuncts = append(rewrittenDisjuncts, rewrittenSub)
		}
	}

	if !actuallyRewritten {
		return q, nil
	}

	if len(rewrittenDisjuncts) == 0 {
		return NewMatchNoDocsQuery("empty DisjunctionMaxQuery"), nil
	}
	if len(rewrittenDisjuncts) == 1 {
		return rewrittenDisjuncts[0], nil
	}

	return NewDisjunctionMaxQuery(rewrittenDisjuncts, q.tieBreakerMultiplier), nil
}

// CreateWeight builds the Weight for this query.
func (q *DisjunctionMaxQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	mode := COMPLETE
	if !scoreMode.NeedsScores() {
		mode = COMPLETE_NO_SCORES
	}
	return q.CreateWeightScoreMode(searcher, mode, boost)
}

// CreateWeightScoreMode builds the DisjunctionMaxWeight, threading the full
// ScoreMode down to each disjunct's weight.
func (q *DisjunctionMaxQuery) CreateWeightScoreMode(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewDisjunctionMaxWeight(searcher, q, scoreMode, boost)
}
