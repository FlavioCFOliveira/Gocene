// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// WildcardQuery matches documents containing terms matching a wildcard pattern.
// ? matches any single character
// * matches any character sequence (including empty)
type WildcardQuery struct {
	*BaseQuery
	term *index.Term
}

// NewWildcardQuery creates a new WildcardQuery.
func NewWildcardQuery(term *index.Term) *WildcardQuery {
	return &WildcardQuery{
		BaseQuery: &BaseQuery{},
		term:      term,
	}
}

// Term returns the wildcard term.
func (q *WildcardQuery) Term() *index.Term {
	return q.term
}

// GetField returns the field name.
func (q *WildcardQuery) GetField() string {
	if q.term != nil {
		return q.term.Field
	}
	return ""
}

// Pattern returns the wildcard pattern.
func (q *WildcardQuery) Pattern() []byte {
	if q.term != nil {
		return q.term.Bytes.Bytes
	}
	return nil
}

// Clone creates a copy of this query.
func (q *WildcardQuery) Clone() Query {
	if q.term == nil {
		return &WildcardQuery{
			BaseQuery: &BaseQuery{},
			term:      nil,
		}
	}
	return &WildcardQuery{
		BaseQuery: &BaseQuery{},
		term:      q.term.Clone(),
	}
}

// Equals checks if this query equals another.
func (q *WildcardQuery) Equals(other Query) bool {
	if o, ok := other.(*WildcardQuery); ok {
		if q.term == nil || o.term == nil {
			return q.term == nil && o.term == nil
		}
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *WildcardQuery) HashCode() int {
	if q.term != nil {
		return q.term.HashCode()
	}
	return 0
}

// Rewrite rewrites the query to a simpler form.
func (q *WildcardQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

func (q *WildcardQuery) String() string {
	if q.term == nil {
		return "WildcardQuery<nil>"
	}
	return "WildcardQuery(field=" + q.term.Field + ", pattern=" + q.term.Text() + ")"
}

// CreateWeight creates a Weight for this query.
func (q *WildcardQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewConstantScoreQuery(q).CreateWeight(searcher, needsScores, boost)
}

// NewWildcardQueryWithStrings creates a new WildcardQuery using strings.
func NewWildcardQueryWithStrings(field string, pattern string) *WildcardQuery {
	return NewWildcardQuery(index.NewTerm(field, pattern))
}
