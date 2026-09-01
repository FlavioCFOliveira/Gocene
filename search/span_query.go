// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanQuery is the base interface for all span-based queries.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanQuery.
type SpanQuery interface {
	Query
	// GetField returns the name of the field matched by this query.
	GetField() string
	// CreateWeight creates a SpanWeight for this query.
	CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error)
}

// BaseSpanQuery provides common functionality for span queries.
type BaseSpanQuery struct {
	field string
}

// NewBaseSpanQuery creates a new BaseSpanQuery.
func NewBaseSpanQuery(field string) *BaseSpanQuery {
	return &BaseSpanQuery{
		field: field,
	}
}

// GetField returns the field name.
func (q *BaseSpanQuery) GetField() string {
	return q.field
}

// getField is an internal helper.
func (q *BaseSpanQuery) field() string {
	return q.field
}
