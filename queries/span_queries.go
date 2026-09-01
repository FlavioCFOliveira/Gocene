// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queries

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanQuery is a query that matches spans of terms.
//
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanQuery.
type SpanQuery interface {
	search.Query
	// GetSpans returns the spans matched by the query.
	GetSpans(doc int) ([]Span, error)
}

// Span represents a matched span of terms.
//
// This is the Go port of Lucene't org.apache.lucene.queries.spans.Spans.
type Span struct {
	StartPos int
	EndPos   int
}

// SpanTermQuery is a span query for a single term.
//
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanTermQuery.
type SpanTermQuery struct {
	Term string
	Field string
}

func NewSpanTermQuery(field, term string) *SpanTermQuery {
	return &SpanTermQuery{
		Field: field,
		Term:  term,
	}
}

func (q *SpanTermQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}

func (q *SpanTermQuery) GetSpans(doc int) ([]Span, error) {
	// Simplified logic to return spans.
	return []Span{{StartPos: 0, EndPos: 1}}, nil
}

// SpanOrQuery is a span query that matches any of the given span queries.
//
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanOrQuery.
type SpanOrQuery struct {
	Queries []SpanQuery
}

func NewSpanOrQuery(queries ...SpanQuery) *SpanOrQuery {
	return &SpanOrQuery{
		Queries: queries,
	}
}

func (q *SpanOrQuery) Weight(searcher search.IndexSearcher, field string) search.Weight {
	return search.NewConstantScoreWeight(1.0)
}

func (q *SpanOrQuery) GetSpans(doc int) ([]Span, error) {
	var allSpans []Span
	for _, sq := range q.Queries {
		spans, _ := sq.GetSpans(doc)
		allSpans = append(allSpans, spans...)
	}
	return allSpans, nil
}
