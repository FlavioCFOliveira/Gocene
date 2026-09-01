// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanWithinQuery matches spans from the 'little' query that are contained within
// a span from the 'big' query.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanWithinQuery.
type SpanWithinQuery struct {
	*SpanContainQuery
}

// NewSpanWithinQuery constructs a SpanWithinQuery.
func NewSpanWithinQuery(big, little SpanQuery) *SpanWithinQuery {
	return &SpanWithinQuery{
		SpanContainQuery: NewSpanContainQuery(big, little),
	}
}

// CreateWeight creates a Weight for this query.
func (q *SpanWithinQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	bigWeight, err := q.big.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	littleWeight, err := q.little.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	return &SpanWithinWeight{
		SpanContainWeight: &SpanContainWeight{
			SpanWeight:    NewSpanWeight(q, nil),
			bigWeight:     bigWeight,
			littleWeight: littleWeight,
		},
		searcher:    searcher,
		boost:       boost,
		needsScores: needsScores,
	}, nil
}

// SpanWithinWeight is the Weight implementation for SpanWithinQuery.
type SpanWithinWeight struct {
	*SpanContainWeight
	searcher    *IndexSearcher
	boost       float32
	needsScores bool
}

func (w *SpanWithinWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	bigSpans, err := w.bigWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if bigSpans == nil {
		return nil, nil
	}
	littleSpans, err := w.littleWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if littleSpans == nil {
		return nil, nil
	}

	return NewContainSpans(bigSpans, littleSpans, littleSpans, true), nil
}

func (w *SpanWithinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.bigWeight.IsCacheable(ctx) && w.littleWeight.IsCacheable(ctx)
}

func (w *SpanWithinWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	spans, err := w.GetSpans(context, index.PostingsFlagPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	return NewSpanScorer(spans, w.boost), nil
}

func (w *SpanWithinWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

func (q *SpanWithinQuery) String(field string) string {
	return q.SpanContainQuery.String(field, "SpanWithin")
}

var _ SpanQuery = (*SpanWithinQuery)(nil)
