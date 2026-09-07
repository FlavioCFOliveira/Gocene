// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

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

// TermStateExtractor is an interface for weights that can extract term states.
type TermStateExtractor interface {
	ExtractTermStates(terms map[*index.Term]*index.TermStates)
}

// BaseSpanQuery provides common functionality for span queries.
type BaseSpanQuery struct {
	BaseQuery
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

// ToString returns a user-readable version of this query.
func (q *BaseSpanQuery) ToString(field string) string {
	return ""
}

// String implements the fmt.Stringer interface.
func (q *BaseSpanQuery) String() string {
	return q.ToString("")
}

// GetTermStates builds a map of terms to TermStates, for use in constructing SpanWeights.
// This is the Go port of Lucene's SpanQuery.getTermStates(SpanWeight... weights).
func GetTermStates(weights ...TermStateExtractor) map[*index.Term]*index.TermStates {
	terms := make(map[*index.Term]*index.TermStates)
	for _, w := range weights {
		w.ExtractTermStates(terms)
	}
	return terms
}

// GetTermStatesCollection builds a map of terms to TermStates, for use in constructing SpanWeights.
// This is the Go port of Lucene's SpanQuery.getTermStates(Collection<SpanWeight> weights).
func GetTermStatesCollection(weights []TermStateExtractor) map[*index.Term]*index.TermStates {
	return GetTermStates(weights...)
}
