// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MatchAllDocsQuery matches all documents in the index.
type MatchAllDocsQuery struct {
	*BaseQuery
}

// NewMatchAllDocsQuery creates a new MatchAllDocsQuery.
func NewMatchAllDocsQuery() *MatchAllDocsQuery {
	return &MatchAllDocsQuery{
		BaseQuery: &BaseQuery{},
	}
}

// Clone creates a copy of this query.
func (q *MatchAllDocsQuery) Clone() Query {
	return NewMatchAllDocsQuery()
}

// Equals checks if this query equals another.
func (q *MatchAllDocsQuery) Equals(other Query) bool {
	_, ok := other.(*MatchAllDocsQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *MatchAllDocsQuery) HashCode() int {
	return 0
}
