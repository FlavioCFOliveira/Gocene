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

// Clone creates a copy of this query.
func (q *TermRangeQuery) Clone() Query {
	lowerCopy := make([]byte, len(q.lowerTerm))
	copy(lowerCopy, q.lowerTerm)
	upperCopy := make([]byte, len(q.upperTerm))
	copy(upperCopy, q.upperTerm)
	return &TermRangeQuery{
		BaseQuery:    &BaseQuery{},
		field:        q.field,
		lowerTerm:    lowerCopy,
		upperTerm:    upperCopy,
		includeLower: q.includeLower,
		includeUpper: q.includeUpper,
	}
}

// Equals checks if this query equals another.
func (q *TermRangeQuery) Equals(other Query) bool {
	if o, ok := other.(*TermRangeQuery); ok {
		if q.field != o.field || q.includeLower != o.includeLower || q.includeUpper != o.includeUpper {
			return false
		}
		if len(q.lowerTerm) != len(o.lowerTerm) || len(q.upperTerm) != len(o.upperTerm) {
			return false
		}
		for i := range q.lowerTerm {
			if q.lowerTerm[i] != o.lowerTerm[i] {
				return false
			}
		}
		for i := range q.upperTerm {
			if q.upperTerm[i] != o.upperTerm[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *TermRangeQuery) HashCode() int {
	hash := 0
	for _, b := range q.lowerTerm {
		hash = hash*31 + int(b)
	}
	for _, b := range q.upperTerm {
		hash = hash*31 + int(b)
	}
	if q.includeLower {
		hash = hash*31 + 1
	}
	if q.includeUpper {
		hash = hash*31 + 1
	}
	return hash
}
