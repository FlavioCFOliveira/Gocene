// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanPositionRangeQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashSpanPositionRangeQuery seeds SpanPositionRangeQuery.hashCode() in
// place of Java's classHash().
const classHashSpanPositionRangeQuery = 0x5370_5052 // "SpPR"

// SpanPositionRangeQuery checks to see if GetMatch() lies between a start and
// end position.
//
// See SpanFirstQuery for a derivation that is optimized for the case where the
// start position is 0.
//
// Mirrors org.apache.lucene.queries.spans.SpanPositionRangeQuery.
type SpanPositionRangeQuery struct {
	*SpanPositionCheckQuery
	start int
	end   int
}

// NewSpanPositionRangeQuery constructs a SpanPositionRangeQuery.
//
// Mirrors SpanPositionRangeQuery(SpanQuery, int, int).
func NewSpanPositionRangeQuery(match SpanQuery, start, end int) (*SpanPositionRangeQuery, error) {
	q := &SpanPositionRangeQuery{start: start, end: end}
	check, err := NewSpanPositionCheckQuery(match, q.acceptPositionRange)
	if err != nil {
		return nil, err
	}
	q.SpanPositionCheckQuery = check
	return q, nil
}

// acceptPositionRange mirrors SpanPositionRangeQuery.acceptPosition(Spans).
func (q *SpanPositionRangeQuery) acceptPositionRange(spans Spans) (AcceptStatus, error) {
	if spans.StartPosition() >= q.end {
		return AcceptNoMoreInCurrentDoc, nil
	}
	if spans.StartPosition() >= q.start && spans.EndPosition() <= q.end {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}

// GetStart returns the minimum position permitted in a match.
func (q *SpanPositionRangeQuery) GetStart() int { return q.start }

// GetEnd returns the maximum end position permitted in a match.
func (q *SpanPositionRangeQuery) GetEnd() int { return q.end }

// ToString mirrors SpanPositionRangeQuery.toString(String field).
func (q *SpanPositionRangeQuery) ToString(field string) string {
	return fmt.Sprintf("spanPosRange(%s, %d, %d)", spanQueryToString(q.match, field), q.start, q.end)
}

// Rewrite mirrors SpanPositionCheckQuery.rewrite(IndexSearcher) for this
// subclass, whose clone is a SpanPositionRangeQuery.
func (q *SpanPositionRangeQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewritten, changed, err := q.rewriteMatch(searcher)
	if err != nil {
		return nil, err
	}
	if changed {
		return NewSpanPositionRangeQuery(rewritten, q.start, q.end)
	}
	return q, nil
}

// equalsToRange mirrors SpanPositionRangeQuery.equals(Object) once the class
// check has succeeded: the inherited match comparison plus start and end.
func (q *SpanPositionRangeQuery) equalsToRange(other *SpanPositionRangeQuery) bool {
	if !q.equalsTo(other.SpanPositionCheckQuery) {
		return false
	}
	return q.end == other.end && q.start == other.start
}

// Equals mirrors SpanPositionRangeQuery.equals(Object).
func (q *SpanPositionRangeQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanPositionRangeQuery)
	if !ok {
		return false
	}
	return q.equalsToRange(o)
}

// hashCodeWithClassHashRange mirrors SpanPositionRangeQuery.hashCode(), whose
// super.hashCode() seed classHash() is the hash of the concrete subclass.
func (q *SpanPositionRangeQuery) hashCodeWithClassHashRange(classHash int) int {
	h := q.hashCodeWithClassHash(classHash) ^ q.end
	h = int(int32(h)*127) ^ q.start
	return h
}

// HashCode mirrors SpanPositionRangeQuery.hashCode().
func (q *SpanPositionRangeQuery) HashCode() int {
	return q.hashCodeWithClassHashRange(classHashSpanPositionRangeQuery)
}

// CreateWeight mirrors SpanPositionCheckQuery.createWeight(IndexSearcher,
// ScoreMode, float), whose covariant return is a SpanWeight.
func (q *SpanPositionRangeQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanPositionCheckQuery.createWeight.
func (q *SpanPositionRangeQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	return q.createSpanWeightFor(q, searcher, scoreMode, boost)
}

var _ SpanQuery = (*SpanPositionRangeQuery)(nil)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanPositionRangeQuery) String() string { return q.ToString("") }
