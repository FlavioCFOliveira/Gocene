// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanNearQuery matches spans which are near one another. One can specify slop, the maximum number of
// intervening unmatched positions, as well as whether matches are required to be in-order.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanNearQuery.
type SpanNearQuery struct {
	BaseSpanQuery
	clauses []SpanQuery
	slop    int
	inOrder bool
}

// NewSpanNearQuery constructs a SpanNearQuery.
func NewSpanNearQuery(clauses []SpanQuery, slop int, inOrder bool) *SpanNearQuery {
	field := ""
	if len(clauses) > 0 {
		field = clauses[0].GetField()
	}
	for _, q := range clauses {
		if q.GetField() != field {
			// In a real implementation, this should return an error.
			// For now, we panic to match Lucene's IllegalArgumentException.
			panic(fmt.Sprintf("Clauses must have same field: %s vs %s", field, q.GetField()))
		}
	}
	return &SpanNearQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		clauses:       clauses,
		slop:          slop,
		inOrder:       inOrder,
	}
}

// GetClauses returns the clauses whose spans are matched.
func (q *SpanNearQuery) GetClauses() []SpanQuery {
	return q.clauses
}

// GetSlop returns the maximum number of intervening unmatched positions permitted.
func (q *SpanNearQuery) GetSlop() int {
	return q.slop
}

// IsInOrder returns true if order is important.
func (q *SpanNearQuery) IsInOrder() bool {
	return q.inOrder
}

// CreateWeight creates a Weight for this query.
func (q *SpanNearQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	subWeights := make([]SpanWeight, 0, len(q.clauses))
	for _, q := range q.clauses {
		sw, err := q.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, sw)
	}
	return &SpanNearWeight{
		SpanWeight:  NewSpanWeight(q, nil), // similarity handled by subweights/scorer
		subWeights:   subWeights,
		searcher:     searcher,
		boost:        boost,
		needsScores:  needsScores,
	}, nil
}

// SpanNearWeight is the Weight implementation for SpanNearQuery.
type SpanNearWeight struct {
	*SpanWeight
	subWeights  []SpanWeight
	searcher    *IndexSearcher
	boost       float32
	needsScores bool
}

func (w *SpanNearWeight) extractTermStates(contexts map[index.Term]*index.TermStates) {
	for _, sw := range w.subWeights {
		sw.ExtractTermStates(contexts)
	}
}

func (w *SpanNearWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	leafReader := ctx.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	terms := leafReader.Terms(w.SpanQuery.GetField())
	if terms == nil {
		return nil, nil
	}

	subSpans := make([]Spans, 0, len(w.subWeights))
	for _, sw := range w.subWeights {
		subSpan, err := sw.GetSpans(ctx, requiredPostings)
		if err != nil {
			return nil, err
		}
		if subSpan != nil {
			subSpans = append(subSpans, subSpan)
		} else {
			return nil, nil // all required
		}
	}

	if !w.inOrder {
		return NewNearSpansUnordered(w.slop, subSpans), nil
	}
	return NewNearSpansOrdered(w.slop, subSpans), nil
}

func (w *SpanNearWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, sw := range w.subWeights {
		if !sw.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (w *SpanNearWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	spans, err := w.GetSpans(context, index.PostingsFlagPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	// For simplicity, use a constant score for now.
	// Real SpanNearQuery scoring is more complex.
	return NewSpanScorer(spans, w.boost), nil
}

func (w *SpanNearWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

func (q *SpanNearQuery) Rewrite(reader IndexReader) (Query, error) {
	// Simplified rewrite: just return self.
	return q, nil
}

func (q *SpanNearQuery) String(field string) string {
	res := "spanNear(["
	for i, clause := range q.clauses {
		res += clause.String(field)
		if i < len(q.clauses)-1 {
			res += ", "
		}
	}
	res += fmt.Sprintf("], %d, %v)", q.slop, q.inOrder)
	return res
}

func (q *SpanNearQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanNearQuery); ok {
		if q.inOrder != o.inOrder || q.slop != o.slop || len(q.clauses) != len(o.clauses) {
			return false
		}
		for i := range q.clauses {
			if !q.clauses[i].Equals(o.clauses[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func (q *SpanNearQuery) HashCode() int {
	h := 17
	h = 31*h + q.slop
	if q.inOrder {
		h = 31*h + 1
	}
	for _, clause := range q.clauses {
		h = 31*h + clause.HashCode()
	}
	return h
}

var _ SpanQuery = (*SpanNearQuery)(nil)
