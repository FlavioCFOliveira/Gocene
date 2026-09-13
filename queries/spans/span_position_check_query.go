// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanPositionCheckQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SpanPositionCheckQuery is the base class for filtering a SpanQuery based on
// the position of a match.
//
// Mirrors org.apache.lucene.queries.spans.SpanPositionCheckQuery (abstract).
//
// Deviations from Java:
//   - Java's abstract acceptPosition(Spans) is a function field here, set by
//     the concrete subclass, which is the pattern the package already uses for
//     ConjunctionSpans.twoPhaseCurrentDocMatches.
//   - Java's rewrite() clones the concrete subclass through Object.clone();
//     Go cannot, so rewriteMatch reports the rewritten match and whether it
//     changed, and each subclass builds its own clone.
//   - Java's equals()/hashCode() pair uses sameClassAs()/classHash(); each
//     concrete subclass therefore declares Equals with the type assertion and
//     passes its own class discriminator to hashCodeWithClassHash.
type SpanPositionCheckQuery struct {
	search.BaseQuery
	match SpanQuery

	// acceptPosition carries the abstract
	// SpanPositionCheckQuery.acceptPosition(Spans): implementing classes
	// return whether the current position is a match for the match SpanQuery.
	// It is only called if the underlying last Spans.NextStartPosition() for
	// the match indicated a valid start position.
	acceptPosition func(spans Spans) (AcceptStatus, error)
}

// NewSpanPositionCheckQuery constructs a SpanPositionCheckQuery over match.
//
// Mirrors SpanPositionCheckQuery(SpanQuery); acceptPosition supplies the
// abstract method the concrete subclass implements.
func NewSpanPositionCheckQuery(match SpanQuery, acceptPosition func(Spans) (AcceptStatus, error)) (*SpanPositionCheckQuery, error) {
	if match == nil {
		return nil, fmt.Errorf("SpanPositionCheckQuery: match must not be nil")
	}
	return &SpanPositionCheckQuery{match: match, acceptPosition: acceptPosition}, nil
}

// GetMatch returns the SpanQuery whose matches are filtered.
func (q *SpanPositionCheckQuery) GetMatch() SpanQuery { return q.match }

// GetField returns the field of the match clause.
func (q *SpanPositionCheckQuery) GetField() string { return q.match.GetField() }

// Visit mirrors SpanPositionCheckQuery.visit(QueryVisitor).
func (q *SpanPositionCheckQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.GetField()) {
		visitSpanQuery(q.match, visitor.GetSubVisitor(search.MUST, q))
	}
}

// rewriteMatch carries the body of SpanPositionCheckQuery.rewrite(IndexSearcher)
// up to the point where Java clones the concrete subclass: it returns the
// rewritten match and whether it differs from the current one.
func (q *SpanPositionCheckQuery) rewriteMatch(searcher *search.IndexSearcher) (SpanQuery, bool, error) {
	rewrittenQuery, err := q.match.Rewrite(searcher)
	if err != nil {
		return nil, false, err
	}
	rewritten, ok := rewrittenQuery.(SpanQuery)
	if !ok {
		return nil, false, fmt.Errorf("SpanPositionCheckQuery.Rewrite: match rewrote to non-SpanQuery %T", rewrittenQuery)
	}
	return rewritten, rewritten != q.match, nil
}

// equalsTo mirrors the match comparison of SpanPositionCheckQuery.equals.
func (q *SpanPositionCheckQuery) equalsTo(other *SpanPositionCheckQuery) bool {
	return q.match.Equals(other.match)
}

// hashCodeWithClassHash mirrors SpanPositionCheckQuery.hashCode(), whose seed
// classHash() is the hash of the concrete subclass.
func (q *SpanPositionCheckQuery) hashCodeWithClassHash(classHash int) int {
	return classHash ^ q.match.HashCode()
}

// createSpanWeightFor mirrors
// SpanPositionCheckQuery.createWeight(IndexSearcher, ScoreMode, float). query
// is the enclosing concrete query, which Java reaches as
// SpanPositionCheckQuery.this.
//
// Java passes `scoreMode.needsScores() ? getTermStates(matchWeight) : null` as
// the weight's termStates argument, which feeds SpanWeight.buildSimWeight and
// nothing else. This port carries a nil SimScorer here, exactly as
// SpanTermQuery and SpanNearQuery do, because buildSimWeight needs the Term
// behind each TermStates and the package's term-states map is keyed by
// "field:text" (see span_weight.go:57-59).
func (q *SpanPositionCheckQuery) createSpanWeightFor(
	query search.Query,
	searcher *search.IndexSearcher,
	scoreMode search.ScoreMode,
	boost float32,
) (*SpanWeight, error) {
	matchWeight, err := q.match.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return newSpanPositionCheckWeight(query, q, matchWeight), nil
}

// newSpanPositionCheckWeight mirrors the inner class
// SpanPositionCheckQuery.SpanPositionCheckWeight.
func newSpanPositionCheckWeight(query search.Query, check *SpanPositionCheckQuery, matchWeight *SpanWeight) *SpanWeight {
	return NewSpanWeight(query, SpanWeightConfig{
		Field:     check.GetField(),
		SimScorer: nil,
		GetSpans: func(ctx *index.LeafReaderContext, requiredPostings Postings) (Spans, error) {
			matchSpans, err := matchWeight.GetSpans(ctx, requiredPostings)
			if err != nil {
				return nil, err
			}
			if matchSpans == nil {
				return nil, nil
			}
			return NewFilterSpans(matchSpans, &spanPositionCheckFilter{check: check}), nil
		},
		ExtractStates: func(contexts map[string]*index.TermStates) {
			matchWeight.ExtractTermStates(contexts)
		},
		IsCacheable: func(ctx *index.LeafReaderContext) bool {
			return matchWeight.IsCacheable(ctx)
		},
	})
}

// spanPositionCheckFilter is the anonymous FilterSpans subclass returned by
// SpanPositionCheckWeight.getSpans; its accept delegates to acceptPosition.
type spanPositionCheckFilter struct {
	check *SpanPositionCheckQuery
}

// Accept mirrors the anonymous subclass's accept(Spans candidate).
func (f *spanPositionCheckFilter) Accept(candidate Spans) (AcceptStatus, error) {
	return f.check.acceptPosition(candidate)
}

var _ SpansFilter = (*spanPositionCheckFilter)(nil)
