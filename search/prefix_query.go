// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// PrefixQuery matches documents containing terms with the given prefix.
type PrefixQuery struct {
	*BaseQuery
	prefix *index.Term
}

// NewPrefixQuery creates a new PrefixQuery.
func NewPrefixQuery(prefix *index.Term) *PrefixQuery {
	return &PrefixQuery{
		BaseQuery: &BaseQuery{},
		prefix:    prefix,
	}
}

// Prefix returns the prefix term.
func (q *PrefixQuery) Prefix() *index.Term {
	return q.prefix
}

// GetField returns the field name.
func (q *PrefixQuery) GetField() string {
	if q.prefix != nil {
		return q.prefix.Field
	}
	return ""
}

// Clone creates a copy of this query.
func (q *PrefixQuery) Clone() Query {
	return NewPrefixQuery(q.prefix.Clone())
}

// Equals checks if this query equals another.
func (q *PrefixQuery) Equals(other Query) bool {
	if o, ok := other.(*PrefixQuery); ok {
		return q.prefix.Equals(o.prefix)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *PrefixQuery) HashCode() int {
	return q.prefix.HashCode()
}
