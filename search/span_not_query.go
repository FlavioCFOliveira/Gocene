// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// SpanNotQuery matches spans that match the include query but not the exclude query.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanNotQuery.
type SpanNotQuery struct {
	BaseSpanQuery
	include SpanQuery
	exclude SpanQuery
}

// NewSpanNotQuery creates a new SpanNotQuery.
func NewSpanNotQuery(include, exclude SpanQuery) *SpanNotQuery {
	if include.GetField() != exclude.GetField() {
		return nil
	}

	return &SpanNotQuery{
		BaseSpanQuery: *NewBaseSpanQuery(include.GetField()),
		include:       include,
		exclude:       exclude,
	}
}

// Include returns the include query.
func (q *SpanNotQuery) Include() SpanQuery {
	return q.include
}

// Exclude returns the exclude query.
func (q *SpanNotQuery) Exclude() SpanQuery {
	return q.exclude
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanNotQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *SpanNotQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewSpanWeight(q, nil), nil
}

// Clone creates a copy of this query.
func (q *SpanNotQuery) Clone() Query {
	return &SpanNotQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		include:       q.include.Clone().(SpanQuery),
		exclude:       q.exclude.Clone().(SpanQuery),
	}
}

// Equals checks if this query equals another.
func (q *SpanNotQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanNotQuery); ok {
		return q.field == o.field &&
			q.include.Equals(o.include) &&
			q.exclude.Equals(o.exclude)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanNotQuery) HashCode() int {
	h := 17
	h = 31*h + len(q.field)
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.include.HashCode()
	h = 31*h + q.exclude.HashCode()
	return h
}

// String returns a string representation of the query.
func (q *SpanNotQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanNotQuery(field=%s, include=%s, exclude=%s)",
			q.field, q.include.String(q.field), q.exclude.String(q.field))
	}
	return fmt.Sprintf("SpanNotQuery(include=%s, exclude=%s)",
		q.include.String(q.field), q.exclude.String(q.field))
}

// Ensure SpanNotQuery implements SpanQuery
var _ SpanQuery = (*SpanNotQuery)(nil)
