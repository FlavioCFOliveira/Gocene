// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// SpanWeight is the base class for SpanQuery weights.
// SpanWeights are used for scoring and matching span queries.
//
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanWeight.
type SpanWeight struct {
	// Query is the SpanQuery this weight is for
	Query *SpanQuery

	// Similarity is the similarity used for scoring
	Similarity Similarity
}

// NewSpanWeight creates a new SpanWeight for the given query.
func NewSpanWeight(query *SpanQuery, similarity Similarity) *SpanWeight {
	return &SpanWeight{
		Query:      query,
		Similarity: similarity,
	}
}

// GetQuery returns the SpanQuery this weight is for.
func (sw *SpanWeight) GetQuery() *SpanQuery {
	return sw.Query
}

// GetValue returns the weight value (used for scoring).
func (sw *SpanWeight) GetValue() float32 {
	return 1.0
}

// IsCacheable returns true if this weight can be cached.
func (sw *SpanWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// SpanQuery represents a span query.
// This is a placeholder for the full SpanQuery implementation.
type SpanQuery struct {
	// Field is the field being queried
	Field string

	// Term is the term being searched
	Term string
}

// NewSpanQuery creates a new SpanQuery.
func NewSpanQuery(field, term string) *SpanQuery {
	return &SpanQuery{
		Field: field,
		Term:  term,
	}
}

// GetField returns the field for this query.
func (sq *SpanQuery) GetField() string {
	return sq.Field
}

// GetTerm returns the term for this query.
func (sq *SpanQuery) GetTerm() string {
	return sq.Term
}
