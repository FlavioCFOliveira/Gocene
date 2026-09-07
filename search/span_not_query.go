// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanNotQuery removes matches which overlap with another SpanQuery or which are within x tokens before or y
// tokens after another SpanQuery.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanNotQuery.
type SpanNotQuery struct {
	BaseSpanQuery
	include SpanQuery
	exclude SpanQuery
	pre     int
	post    int
}

// NewSpanNotQuery constructs a SpanNotQuery matching spans from include which have no overlap with
// spans from exclude.
func NewSpanNotQuery(include, exclude SpanQuery) *SpanNotQuery {
	return NewSpanNotQueryWithPrePost(include, exclude, 0, 0)
}

// NewSpanNotQueryWithDist constructs a SpanNotQuery matching spans from include which have no overlap with
// spans from exclude within dist tokens of include.
// Inversely, a negative dist value may be used to specify a certain amount of allowable overlap.
func NewSpanNotQueryWithDist(include, exclude SpanQuery, dist int) *SpanNotQuery {
	return NewSpanNotQueryWithPrePost(include, exclude, dist, dist)
}

// NewSpanNotQueryWithPrePost constructs a SpanNotQuery matching spans from include which have no overlap with
// spans from exclude within pre tokens before or post tokens of include. Inversely, negative values for pre
// and/or post allow a certain amount of overlap to occur.
func NewSpanNotQueryWithPrePost(include, exclude SpanQuery, pre, post int) *SpanNotQuery {
	if include == nil || exclude == nil {
		panic("include and exclude must not be nil")
	}

	incField := include.GetField()
	excField := exclude.GetField()
	if incField != "" && excField != "" && incField != excField {
		panic("Clauses must have same field")
	}

	field := incField
	if field == "" {
		field = excField
	}

	return &SpanNotQuery{
		BaseSpanQuery: *NewBaseSpanQuery(field),
		include:       include,
		exclude:       exclude,
		pre:           pre,
		post:          post,
	}
}

// Include returns the SpanQuery whose matches are filtered.
func (q *SpanNotQuery) Include() SpanQuery {
	return q.include
}

// Exclude returns the SpanQuery whose matches must not overlap those returned.
func (q *SpanNotQuery) Exclude() SpanQuery {
	return q.exclude
}

// GetField returns the field name.
func (q *SpanNotQuery) GetField() string {
	return q.include.GetField()
}

// ToString returns a user-readable version of this query.
func (q *SpanNotQuery) ToString(field string) string {
	return fmt.Sprintf("spanNot(%s, %s, %d, %d)",
		q.include.ToString(field),
		q.exclude.ToString(field),
		q.pre,
		q.post)
}

// CreateWeight creates a Weight for this query.
func (q *SpanNotQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	includeWeight, err := q.include.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}

	excludeWeight, err := q.exclude.CreateWeight(searcher, false, boost) // COMPLETE_NO_SCORES
	if err != nil {
		return nil, err
	}

	var terms map[*index.Term]*index.TermStates
	if needsScores {
		if extractor, ok := includeWeight.(TermStateExtractor); ok {
			terms = make(map[*index.Term]*index.TermStates)
			extractor.ExtractTermStates(terms)
		}
	}

	return newSpanNotWeight(searcher, terms, includeWeight.(*SpanWeight), excludeWeight.(*SpanWeight), boost, q), nil
}

// Rewrite rewrites the query to a simpler form.
func (q *SpanNotQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	rewrittenInclude, err := q.include.Rewrite(searcher)
	if err != nil {
		return nil, err
	}

	rewrittenExclude, err := q.exclude.Rewrite(searcher)
	if err != nil {
		return nil, err
	}

	if rewrittenInclude != q.include || rewrittenExclude != q.exclude {
		return NewSpanNotQueryWithPrePost(rewrittenInclude.(SpanQuery), rewrittenExclude.(SpanQuery), q.pre, q.post), nil
	}

	return q.RewriteBase(), nil
}

// Visit visits the components of this query.
func (q *SpanNotQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.GetField()) {
		visitor.GetSubVisitor(MUST, q).Visit(q.include)
		visitor.GetSubVisitor(MUST_NOT, q).Visit(q.exclude)
	}
}

// Equals checks if this query equals another.
func (q *SpanNotQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*SpanNotQuery)
	if !ok {
		return false
	}
	return q.include.Equals(o.include) &&
		q.exclude.Equals(o.exclude) &&
		q.pre == o.pre &&
		q.post == o.post
}

// HashCode returns a hash code for this query.
func (q *SpanNotQuery) HashCode() int {
	h := 17 // class hash
	h = (h << 5) - h + q.include.HashCode()
	h = (h << 5) - h + q.exclude.HashCode()
	h = (h << 5) - h + q.pre
	h = (h << 5) - h + q.post
	return h
}

// Clone creates a copy of this query.
func (q *SpanNotQuery) Clone() Query {
	return &SpanNotQuery{
		BaseSpanQuery: *NewBaseSpanQuery(q.field),
		include:       q.include.Clone().(SpanQuery),
		exclude:       q.exclude.Clone().(SpanQuery),
		pre:           q.pre,
		post:          q.post,
	}
}

type spanNotWeight struct {
	*SpanWeight
	includeWeight *SpanWeight
	excludeWeight *SpanWeight
}

func newSpanNotWeight(searcher *IndexSearcher, terms map[*index.Term]*index.TermStates, includeWeight, excludeWeight *SpanWeight, boost float32, query *SpanNotQuery) *spanNotWeight {
	return &spanNotWeight{
		SpanWeight:    NewSpanWeight(query, similarityFromSearcher(searcher)),
		includeWeight: includeWeight,
		excludeWeight: excludeWeight,
	}
}

func (sw *spanNotWeight) ExtractTermStates(contexts map[*index.Term]*index.TermStates) {
	sw.includeWeight.ExtractTermStates(contexts)
}

func (sw *spanNotWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	includeSpans, err := sw.includeWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if includeSpans == nil {
		return nil, nil
	}

	excludeSpans, err := sw.excludeWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if excludeSpans == nil {
		return includeSpans, nil
	}

	excludeTwoPhase := excludeSpans.AsTwoPhaseIterator()
	var excludeApproximation DocIdSetIterator
	if excludeTwoPhase != nil {
		excludeApproximation = excludeTwoPhase.Approximation()
	}

	// state for the filter
	lastApproxDoc := -1
	lastApproxResult := false

	query := sw.SpanQuery.(*SpanNotQuery)

	accept := func(candidate Spans) AcceptStatus {
		doc := candidate.DocID()

		if doc > excludeSpans.DocID() {
			if excludeTwoPhase != nil {
				if excludeApproximation.Advance(doc) == doc {
					lastApproxDoc = doc
					res, _ := excludeTwoPhase.Matches()
					lastApproxResult = res
				}
			} else {
				excludeSpans.Advance(doc)
			}
		} else if excludeTwoPhase != nil && doc == excludeSpans.DocID() && doc != lastApproxDoc {
			lastApproxDoc = doc
			res, _ := excludeTwoPhase.Matches()
			lastApproxResult = res
		}

		if doc != excludeSpans.DocID() || (doc == lastApproxDoc && lastApproxResult == false) {
			return AcceptStatusYES
		}

		if excludeSpans.StartPosition() == -1 {
			excludeSpans.NextStartPosition()
		}

		for excludeSpans.EndPosition() <= candidate.StartPosition()-query.pre {
			if excludeSpans.NextStartPosition() == -1 {
				return AcceptStatusYES
			}
		}

		if excludeSpans.StartPosition()-query.post >= candidate.EndPosition() {
			return AcceptStatusYES
		}
		return AcceptStatusNO
	}

	return NewFilterSpans(includeSpans, accept), nil
}

func (sw *spanNotWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return sw.includeWeight.IsCacheable(ctx) && sw.excludeWeight.IsCacheable(ctx)
}
