// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SrndBooleanQuery is the surround node for boolean expressions.
// It mirrors org.apache.lucene.queryparser.surround.query.SrndBooleanQuery.
type SrndBooleanQuery struct {
	SrndQueryBase
	queries []SrndQuery
	occur   search.Occur
}

// NewSrndBooleanQuery builds a boolean surround query node.
func NewSrndBooleanQuery(queries []SrndQuery, occur search.Occur) *SrndBooleanQuery {
	return &SrndBooleanQuery{
		queries: queries,
		occur:   occur,
	}
}

// MakeLuceneQueryField produces a BooleanQuery from the children.
func (q *SrndBooleanQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	if len(q.queries) <= 1 {
		return nil, fmt.Errorf("surround: too few subqueries for SrndBooleanQuery: %d", len(q.queries))
	}

	bq := search.NewBooleanQuery()
	for _, sq := range q.queries {
		lq, err := sq.MakeLuceneQueryField(field, factory)
		if err != nil {
			return nil, err
		}
		bq.Add(lq, q.occur)
	}

	return q.WrapWithBoost(bq), nil
}

// String returns the string representation of the boolean query.
func (q *SrndBooleanQuery) String() string {
	var sb strings.Builder
	sb.WriteByte('(')
	for i, sq := range q.queries {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(sq.String())
	}
	sb.WriteByte(')')
	q.WeightToString(&sb)
	return sb.String()
}

// AddQueriesToBoolean is a legacy utility that appends queries to a BooleanQuery.
func AddQueriesToBoolean(bq *search.BooleanQuery, queries []search.Query, occur search.Occur) {
	for _, q := range queries {
		bq.Add(q, occur)
	}
}

// MakeBooleanQuery is a legacy utility that assembles a BooleanQuery from search.Queries.
func MakeBooleanQuery(queries []search.Query, occur search.Occur) search.Query {
	if len(queries) <= 1 {
		panic("surround: too few subqueries for MakeBooleanQuery: " + itoa(len(queries)))
	}
	bq := search.NewBooleanQuery()
	AddQueriesToBoolean(bq, queries, occur)
	return bq
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
