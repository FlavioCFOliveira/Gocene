// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanNotQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// classHashSpanNotQuery seeds SpanNotQuery.hashCode() in place of Java's
// classHash().
const classHashSpanNotQuery = 0x5370_4e6f // "SpNo"

// SpanNotQuery removes matches which overlap with another SpanQuery or which
// are within x tokens before or y tokens after another SpanQuery.
//
// Mirrors org.apache.lucene.queries.spans.SpanNotQuery (final).
type SpanNotQuery struct {
	search.BaseQuery
	include SpanQuery
	exclude SpanQuery
	pre     int
	post    int
}

// NewSpanNotQuery constructs a SpanNotQuery matching spans from include which
// have no overlap with spans from exclude.
//
// Mirrors SpanNotQuery(SpanQuery, SpanQuery).
func NewSpanNotQuery(include, exclude SpanQuery) (*SpanNotQuery, error) {
	return NewSpanNotQueryWithPrePost(include, exclude, 0, 0)
}

// NewSpanNotQueryWithDist constructs a SpanNotQuery matching spans from include
// which have no overlap with spans from exclude within dist tokens of include.
// Inversely, a negative dist value may be used to specify a certain amount of
// allowable overlap.
//
// Mirrors SpanNotQuery(SpanQuery, SpanQuery, int).
func NewSpanNotQueryWithDist(include, exclude SpanQuery, dist int) (*SpanNotQuery, error) {
	return NewSpanNotQueryWithPrePost(include, exclude, dist, dist)
}

// NewSpanNotQueryWithPrePost constructs a SpanNotQuery matching spans from
// include which have no overlap with spans from exclude within pre tokens
// before or post tokens of include. Inversely, negative values for pre and/or
// post allow a certain amount of overlap to occur.
//
// Mirrors SpanNotQuery(SpanQuery, SpanQuery, int, int).
func NewSpanNotQueryWithPrePost(include, exclude SpanQuery, pre, post int) (*SpanNotQuery, error) {
	if include == nil || exclude == nil {
		return nil, fmt.Errorf("SpanNotQuery: include and exclude must not be nil")
	}
	if include.GetField() != "" && exclude.GetField() != "" && include.GetField() != exclude.GetField() {
		return nil, fmt.Errorf("Clauses must have same field.")
	}
	return &SpanNotQuery{include: include, exclude: exclude, pre: pre, post: post}, nil
}

// GetInclude returns the SpanQuery whose matches are filtered.
func (q *SpanNotQuery) GetInclude() SpanQuery { return q.include }

// GetExclude returns the SpanQuery whose matches must not overlap those
// returned.
func (q *SpanNotQuery) GetExclude() SpanQuery { return q.exclude }

// GetField returns the field of the include clause.
func (q *SpanNotQuery) GetField() string { return q.include.GetField() }

// ToString mirrors SpanNotQuery.toString(String field).
func (q *SpanNotQuery) ToString(field string) string {
	return fmt.Sprintf("spanNot(%s, %s, %d, %d)",
		spanQueryToString(q.include, field), spanQueryToString(q.exclude, field), q.pre, q.post)
}

// Rewrite mirrors SpanNotQuery.rewrite(IndexSearcher).
func (q *SpanNotQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewrittenIncludeQuery, err := q.include.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	rewrittenInclude, ok := rewrittenIncludeQuery.(SpanQuery)
	if !ok {
		return nil, fmt.Errorf("SpanNotQuery.Rewrite: include rewrote to non-SpanQuery %T", rewrittenIncludeQuery)
	}
	rewrittenExcludeQuery, err := q.exclude.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	rewrittenExclude, ok := rewrittenExcludeQuery.(SpanQuery)
	if !ok {
		return nil, fmt.Errorf("SpanNotQuery.Rewrite: exclude rewrote to non-SpanQuery %T", rewrittenExcludeQuery)
	}
	if rewrittenInclude != q.include || rewrittenExclude != q.exclude {
		return NewSpanNotQueryWithPrePost(rewrittenInclude, rewrittenExclude, q.pre, q.post)
	}
	return q, nil
}

// Visit mirrors SpanNotQuery.visit(QueryVisitor).
func (q *SpanNotQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.GetField()) {
		visitSpanQuery(q.include, visitor.GetSubVisitor(search.MUST, q))
		visitSpanQuery(q.exclude, visitor.GetSubVisitor(search.MUST_NOT, q))
	}
}

// Equals mirrors SpanNotQuery.equals(Object).
func (q *SpanNotQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanNotQuery)
	if !ok {
		return false
	}
	return q.include.Equals(o.include) &&
		q.exclude.Equals(o.exclude) &&
		q.pre == o.pre &&
		q.post == o.post
}

// HashCode mirrors SpanNotQuery.hashCode().
func (q *SpanNotQuery) HashCode() int {
	h := classHashSpanNotQuery
	h = rotateLeft32(h, 1)
	h ^= q.include.HashCode()
	h = rotateLeft32(h, 1)
	h ^= q.exclude.HashCode()
	h = rotateLeft32(h, 1)
	h ^= q.pre
	h = rotateLeft32(h, 1)
	h ^= q.post
	return h
}

// CreateWeight mirrors SpanNotQuery.createWeight(IndexSearcher, ScoreMode,
// float), whose covariant return is a SpanWeight.
func (q *SpanNotQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanNotQuery.createWeight.
//
// Java passes `scoreMode.needsScores() ? getTermStates(includeWeight) : null`
// as the weight's termStates argument, which feeds SpanWeight.buildSimWeight
// and nothing else. This port carries a nil SimScorer here, exactly as
// SpanTermQuery and SpanNearQuery do, because buildSimWeight needs the Term
// behind each TermStates and the package's term-states map is keyed by
// "field:text" (see span_weight.go:57-59).
func (q *SpanNotQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	includeWeight, err := q.include.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	excludeWeight, err := q.exclude.CreateSpanWeight(searcher, search.ScoreModeCompleteNoScores, boost)
	if err != nil {
		return nil, err
	}
	return newSpanNotWeight(q, includeWeight, excludeWeight), nil
}

// newSpanNotWeight mirrors the inner class SpanNotQuery.SpanNotWeight.
func newSpanNotWeight(query *SpanNotQuery, includeWeight, excludeWeight *SpanWeight) *SpanWeight {
	return NewSpanWeight(query, SpanWeightConfig{
		Field:     query.GetField(),
		SimScorer: nil,
		GetSpans: func(ctx *index.LeafReaderContext, requiredPostings Postings) (Spans, error) {
			return spanNotGetSpans(query, includeWeight, excludeWeight, ctx, requiredPostings)
		},
		ExtractStates: func(contexts map[string]*index.TermStates) {
			includeWeight.ExtractTermStates(contexts)
		},
		IsCacheable: func(ctx *index.LeafReaderContext) bool {
			return includeWeight.IsCacheable(ctx) && excludeWeight.IsCacheable(ctx)
		},
	})
}

// spanNotGetSpans mirrors SpanNotWeight.getSpans(LeafReaderContext, Postings).
func spanNotGetSpans(
	query *SpanNotQuery,
	includeWeight, excludeWeight *SpanWeight,
	ctx *index.LeafReaderContext,
	requiredPostings Postings,
) (Spans, error) {
	includeSpans, err := includeWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if includeSpans == nil {
		return nil, nil
	}

	excludeSpans, err := excludeWeight.GetSpans(ctx, requiredPostings)
	if err != nil {
		return nil, err
	}
	if excludeSpans == nil {
		return includeSpans, nil
	}

	excludeTwoPhase := excludeSpans.AsTwoPhaseIterator()
	var excludeApproximation util.DocIdSetIterator
	if excludeTwoPhase != nil {
		excludeApproximation = excludeTwoPhase.Approximation()
	}

	return NewFilterSpans(includeSpans, &spanNotFilter{
		pre:                  query.pre,
		post:                 query.post,
		excludeSpans:         excludeSpans,
		excludeTwoPhase:      excludeTwoPhase,
		excludeApproximation: excludeApproximation,
		lastApproxDoc:        -1,
	}), nil
}

// spanNotFilter is the anonymous FilterSpans subclass returned by
// SpanNotWeight.getSpans; its Accept carries that subclass's accept(Spans).
type spanNotFilter struct {
	pre                  int
	post                 int
	excludeSpans         Spans
	excludeTwoPhase      *search.TwoPhaseIterator
	excludeApproximation util.DocIdSetIterator

	// lastApproxDoc is the last document checked with Matches() for the
	// exclusion, and failed, when using approximations, so it is not called
	// again and all inclusions pass through.
	lastApproxDoc    int
	lastApproxResult bool
}

// Accept mirrors the anonymous subclass's accept(Spans candidate).
func (f *spanNotFilter) Accept(candidate Spans) (AcceptStatus, error) {
	doc := candidate.DocID()
	if doc > f.excludeSpans.DocID() {
		// catch up 'exclude' to the current doc
		if f.excludeTwoPhase != nil {
			advanced, err := f.excludeApproximation.Advance(doc)
			if err != nil {
				return AcceptNo, err
			}
			if advanced == doc {
				f.lastApproxDoc = doc
				f.lastApproxResult, err = f.excludeTwoPhase.Matches()
				if err != nil {
					return AcceptNo, err
				}
			}
		} else {
			if _, err := f.excludeSpans.Advance(doc); err != nil {
				return AcceptNo, err
			}
		}
	} else if f.excludeTwoPhase != nil && doc == f.excludeSpans.DocID() && doc != f.lastApproxDoc {
		// excludeSpans already sitting on our candidate doc, but matches not called yet.
		f.lastApproxDoc = doc
		matches, err := f.excludeTwoPhase.Matches()
		if err != nil {
			return AcceptNo, err
		}
		f.lastApproxResult = matches
	}

	if doc != f.excludeSpans.DocID() || (doc == f.lastApproxDoc && !f.lastApproxResult) {
		return AcceptYes, nil
	}

	if f.excludeSpans.StartPosition() == -1 { // init exclude start position if needed
		if _, err := f.excludeSpans.NextStartPosition(); err != nil {
			return AcceptNo, err
		}
	}

	for f.excludeSpans.EndPosition() <= candidate.StartPosition()-f.pre {
		// exclude end position is before a possible exclusion
		next, err := f.excludeSpans.NextStartPosition()
		if err != nil {
			return AcceptNo, err
		}
		if next == NoMorePositions {
			return AcceptYes, nil // no more exclude at current doc.
		}
	}

	// exclude end position far enough in current doc, check start position:
	if f.excludeSpans.StartPosition()-f.post >= candidate.EndPosition() {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}

var (
	_ SpanQuery   = (*SpanNotQuery)(nil)
	_ SpansFilter = (*spanNotFilter)(nil)
)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanNotQuery) String() string { return q.ToString("") }
