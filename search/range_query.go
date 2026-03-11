// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TermRangeQuery matches documents containing terms within a range.
type TermRangeQuery struct {
	*BaseQuery
	field        string
	lowerTerm    []byte
	upperTerm    []byte
	includeLower bool
	includeUpper bool
}

// NewTermRangeQuery creates a new TermRangeQuery.
func NewTermRangeQuery(field string, lowerTerm, upperTerm []byte, includeLower, includeUpper bool) *TermRangeQuery {
	return &TermRangeQuery{
		BaseQuery:    &BaseQuery{},
		field:        field,
		lowerTerm:    lowerTerm,
		upperTerm:    upperTerm,
		includeLower: includeLower,
		includeUpper: includeUpper,
	}
}

// Field returns the field name.
func (q *TermRangeQuery) Field() string {
	return q.field
}

// LowerTerm returns the lower bound term.
func (q *TermRangeQuery) LowerTerm() []byte {
	return q.lowerTerm
}

// UpperTerm returns the upper bound term.
func (q *TermRangeQuery) UpperTerm() []byte {
	return q.upperTerm
}

// IncludesLower returns true if the lower bound is inclusive.
func (q *TermRangeQuery) IncludesLower() bool {
	return q.includeLower
}

// IncludesUpper returns true if the upper bound is inclusive.
func (q *TermRangeQuery) IncludesUpper() bool {
	return q.includeUpper
}
