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
