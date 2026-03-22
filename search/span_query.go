// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// GC-1001: SpanQuery base class
// SpanQuery is the base class for all span-based queries.
// Span queries are used for positional/near matching of terms.

type SpanQuery interface {
	Query

	// GetField returns the field name for this span query
	GetField() string

	// ExtractTermContexts extracts TermContexts from all terms in the query
	ExtractTermContexts(context *index.TermContext) error
}

// BaseSpanQuery provides common functionality for span queries
type BaseSpanQuery struct {
	*BaseQuery
	field string
}

// NewBaseSpanQuery creates a new base span query
func NewBaseSpanQuery(field string) *BaseSpanQuery {
	return &BaseSpanQuery{
		BaseQuery: NewQuery(),
		field:     field,
	}
}

// GetField returns the field name for this span query
func (q *BaseSpanQuery) GetField() string {
	return q.field
}

// SetField sets the field name for this span query
func (q *BaseSpanQuery) SetField(field string) {
	q.field = field
}

// ExtractTermContexts extracts TermContexts from all terms in the query
// Default implementation - subclasses should override
func (q *BaseSpanQuery) ExtractTermContexts(context *index.TermContext) error {
	return nil
}

// SpanQueryBase is the abstract base class that all span queries must extend
type SpanQueryBase struct {
	*BaseSpanQuery
}

// NewSpanQueryBase creates a new span query base
func NewSpanQueryBase(field string) *SpanQueryBase {
	return &SpanQueryBase{
		BaseSpanQuery: NewBaseSpanQuery(field),
	}
}

// Rewrite rewrites the query to a primitive form
func (q *SpanQueryBase) Rewrite(reader index.IndexReader) (Query, error) {
	return q, nil
}

// SpanQueryUtils provides utility methods for span queries
var SpanQueryUtils = &spanQueryUtils{}

type spanQueryUtils struct{}

// CheckField verifies that all clauses have the same field
func (u *spanQueryUtils) CheckField(field string, queries []SpanQuery) string {
	if field == "" {
		// Determine field from first clause
		if len(queries) > 0 {
			return queries[0].GetField()
		}
		return ""
	}
	return field
}
