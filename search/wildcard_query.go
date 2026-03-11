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
