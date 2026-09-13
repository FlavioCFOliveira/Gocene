// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanFirstQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashSpanFirstQuery seeds SpanFirstQuery.hashCode() in place of Java's
// classHash(). SpanFirstQuery inherits SpanPositionRangeQuery.hashCode(), whose
// super.hashCode() seed is the hash of the concrete class.
const classHashSpanFirstQuery = 0x5370_4669 // "SpFi"

// SpanFirstQuery matches spans near the beginning of a field.
//
// This class is a simple extension of SpanPositionRangeQuery in that it assumes
// the start to be zero and only checks the end boundary.
//
// Mirrors org.apache.lucene.queries.spans.SpanFirstQuery.
type SpanFirstQuery struct {
	*SpanPositionRangeQuery
}

// NewSpanFirstQuery constructs a SpanFirstQuery matching spans in match whose
// end position is less than or equal to end.
//
// Mirrors SpanFirstQuery(SpanQuery, int).
func NewSpanFirstQuery(match SpanQuery, end int) (*SpanFirstQuery, error) {
	rangeQuery, err := NewSpanPositionRangeQuery(match, 0, end)
	if err != nil {
		return nil, err
	}
	q := &SpanFirstQuery{SpanPositionRangeQuery: rangeQuery}
	// Java's SpanFirstQuery overrides acceptPosition; the base holds the
	// abstract method as a function field, so the override is installed here.
	q.acceptPosition = q.acceptPositionFirst
	return q, nil
}

// acceptPositionFirst mirrors SpanFirstQuery.acceptPosition(Spans).
func (q *SpanFirstQuery) acceptPositionFirst(spans Spans) (AcceptStatus, error) {
	if spans.StartPosition() >= q.end {
		return AcceptNoMoreInCurrentDoc, nil
	} else if spans.EndPosition() <= q.end {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}

// ToString mirrors SpanFirstQuery.toString(String field).
func (q *SpanFirstQuery) ToString(field string) string {
	return fmt.Sprintf("spanFirst(%s, %d)", spanQueryToString(q.match, field), q.end)
}

// Rewrite mirrors SpanPositionCheckQuery.rewrite(IndexSearcher) for this
// subclass, whose clone is a SpanFirstQuery.
func (q *SpanFirstQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewritten, changed, err := q.rewriteMatch(searcher)
	if err != nil {
		return nil, err
	}
	if changed {
		return NewSpanFirstQuery(rewritten, q.end)
	}
	return q, nil
}

// Equals mirrors SpanPositionRangeQuery.equals(Object), whose sameClassAs()
// check makes a SpanFirstQuery unequal to a SpanPositionRangeQuery with the
// same match, start and end.
func (q *SpanFirstQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanFirstQuery)
	if !ok {
		return false
	}
	return q.equalsToRange(o.SpanPositionRangeQuery)
}

// HashCode mirrors SpanPositionRangeQuery.hashCode() seeded with this class's
// hash.
func (q *SpanFirstQuery) HashCode() int {
	return q.hashCodeWithClassHashRange(classHashSpanFirstQuery)
}

// CreateWeight mirrors SpanPositionCheckQuery.createWeight(IndexSearcher,
// ScoreMode, float), whose covariant return is a SpanWeight.
func (q *SpanFirstQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanPositionCheckQuery.createWeight.
func (q *SpanFirstQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	return q.createSpanWeightFor(q, searcher, scoreMode, boost)
}

var _ SpanQuery = (*SpanFirstQuery)(nil)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanFirstQuery) String() string { return q.ToString("") }
