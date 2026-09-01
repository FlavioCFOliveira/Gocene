// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanOrQuery matches the union of its clauses.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanOrQuery.
type SpanOrQuery struct {
	BaseSpanQuery
	clauses []SpanQuery
}

// NewSpanOrQuery constructs a SpanOrQuery merging the provided clauses.
func NewSpanOrQuery(clauses []SpanQuery) *SpanOrQuery {
	field := ""
	if len(clauses) > 0 {
		field = clauses[0].GetField()
	}
	for _, q := range clauses {
		if q.GetField() != field {
			panic(fmt.Sprintf("Clauses must have same field: %s vs %s", field, q.GetField()))
		}
	}
	return &SpanOrQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		clauses:       clauses,
	}
}

// GetClauses returns the clauses whose spans are matched.
func (q *SpanOrQuery) GetClauses() []SpanQuery {
	return q.clauses
}

// CreateWeight creates a Weight for this query.
func (q *SpanOrQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	subWeights := make([]SpanWeight, 0, len(q.clauses))
	for _, q := range q.clauses {
		sw, err := q.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, sw)
	}
	return &SpanOrWeight{
		SpanWeight:  NewSpanWeight(q, nil),
		subWeights:  subWeights,
		searcher:    searcher,
		boost:       boost,
		needsScores: needsScores,
	}, nil
}

// SpanOrWeight is the Weight implementation for SpanOrQuery.
type SpanOrWeight struct {
	*SpanWeight
	subWeights  []SpanWeight
	searcher    *IndexSearcher
	boost       float32
	needsScores bool
}

func (w *SpanOrWeight) extractTermStates(contexts map[index.Term]*index.TermStates) {
	for _, sw := range w.subWeights {
		sw.ExtractTermStates(contexts)
	}
}

func (w *SpanOrWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	subSpans := make([]Spans, 0, len(w.subWeights))
	for _, sw := range w.subWeights {
		spans, err := sw.GetSpans(ctx, requiredPostings)
		if err != nil {
			return nil, err
		}
		if spans != nil {
			subSpans = append(subSpans, spans)
		}
	}

	if len(subSpans) == 0 {
		return nil, nil
	} else if len(subSpans) == 1 {
		return subSpans[0], nil
	}

	// For now, implement a simplified union of spans.
	// A full implementation would use a priority queue to merge sorted span streams.
	return NewNearSpansUnordered(0, subSpans), nil // Reuse NearSpansUnordered for union
}

func (w *SpanOrWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, sw := range w.subWeights {
		if !sw.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (w *SpanOrWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	spans, err := w.GetSpans(context, index.PostingsFlagPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	return NewSpanScorer(spans, w.boost), nil
}

func (w *SpanOrWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

func (q *SpanOrQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

func (q *SpanOrQuery) String(field string) string {
	res := "spanOr(["
	for i, clause := range q.clauses {
		res += clause.String(field)
		if i < len(q.clauses)-1 {
			res += ", "
		}
	}
	res += "])"
	return res
}

func (q *SpanOrQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanOrQuery); ok {
		if len(q.clauses) != len(o.clauses) {
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

func (q *SpanOrQuery) HashCode() int {
	h := 17
	for _, clause := range q.clauses {
		h = 31*h + clause.HashCode()
	}
	return h
}

var _ SpanQuery = (*SpanOrQuery)(nil)
