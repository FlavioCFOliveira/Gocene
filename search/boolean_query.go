package search

import (
	"fmt"
	"strings"
)

// BooleanQuery matches documents matching boolean combinations of other queries.
type BooleanQuery struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause
}

type BooleanQueryBuilder struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause
}

func NewBooleanQueryBuilder() *BooleanQueryBuilder {
	return &BooleanQueryBuilder{}
}

func (b *BooleanQueryBuilder) SetMinimumNumberShouldMatch(min int) *BooleanQueryBuilder {
	b.minimumNumberShouldMatch = min
	return b
}

func (b *BooleanQueryBuilder) Add(query Query, occur Occur) *BooleanQueryBuilder {
	b.clauses = append(b.clauses, NewBooleanClause(query, occur))
	return b
}

func (b *BooleanQueryBuilder) AddClause(clause *BooleanClause) *BooleanQueryBuilder {
	b.clauses = append(b.clauses, clause)
	return b
}

func (b *BooleanQueryBuilder) Build() *BooleanQuery {
	return &BooleanQuery{
		minimumNumberShouldMatch: b.minimumNumberShouldMatch,
		clauses:                  b.clauses,
	}
}

func (q *BooleanQuery) GetMinimumNumberShouldMatch() int {
	return q.minimumNumberShouldMatch
}

func (q *BooleanQuery) Clauses() []*BooleanClause {
	return q.clauses
}

func (q *BooleanQuery) Rewrite(reader IndexReader) (Query, error) {
	// Simplified rewrite for now: just return self.
	// In the future, we will implement the complex rewriting logic from Lucene.
	return q, nil
}

func (q *BooleanQuery) Clone() Query {
	return &BooleanQuery{
		minimumNumberShouldMatch: q.minimumNumberShouldMatch,
		clauses:                  append([]*BooleanClause(nil), q.clauses...),
	}
}

func (q *BooleanQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*BooleanQuery); ok {
		if q.minimumNumberShouldMatch != otherQuery.minimumNumberShouldMatch {
			return false
		}
		if len(q.clauses) != len(otherQuery.clauses) {
			return false
		}
		// Simple order-dependent comparison for now.
		for i := range q.clauses {
			if q.clauses[i].Query().Equals(otherQuery.clauses[i].Query()) ||
				q.clauses[i].Occur() != otherQuery.clauses[i].Occur() {
				return false
			}
		}
		return true
	}
	return false
}

func (q *BooleanQuery) HashCode() int {
	return 0 // placeholder
}

func (q *BooleanQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewBooleanWeight(q, searcher, scoreMode, boost), nil
}

func (q *BooleanQuery) ToString(field string) string {
	var sb strings.Builder
	if q.minimumNumberShouldMatch > 0 {
		sb.WriteString("(")
	}
	for i, c := range q.clauses {
		sb.WriteString(c.Occur().String())
		sb.WriteString(" ")
		sb.WriteString(c.Query().ToString(field))
		if i < len(q.clauses)-1 {
			sb.WriteString(" ")
		}
	}
	if q.minimumNumberShouldMatch > 0 {
		sb.WriteString(")")
	}
	if q.minimumNumberShouldMatch > 0 {
		sb.WriteString(fmt.Sprintf("~%d", q.minimumNumberShouldMatch))
	}
	return sb.String()
}
