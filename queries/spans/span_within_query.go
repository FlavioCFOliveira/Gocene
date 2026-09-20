// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanWithinQuery.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashSpanWithinQuery seeds SpanWithinQuery.hashCode() in place of Java's
// classHash(). See SpanContainQuery.hashCodeWithClassHash.
const classHashSpanWithinQuery = 0x5370_5769 // "SpWi"

// SpanWithinQuery keeps matches that are contained within another Spans.
//
// Mirrors org.apache.lucene.queries.spans.SpanWithinQuery (final).
type SpanWithinQuery struct {
	*SpanContainQuery
}

// NewSpanWithinQuery constructs a SpanWithinQuery matching spans from little
// that are inside of big. This query has the boost of little. big and little
// must be in the same field.
//
// Mirrors SpanWithinQuery(SpanQuery, SpanQuery).
func NewSpanWithinQuery(big, little SpanQuery) (*SpanWithinQuery, error) {
	base, err := NewSpanContainQuery(big, little)
	if err != nil {
		return nil, err
	}
	return &SpanWithinQuery{SpanContainQuery: base}, nil
}

// ToString mirrors SpanWithinQuery.toString(String field).
func (q *SpanWithinQuery) ToString(field string) string {
	return q.toStringWithName(field, "SpanWithin")
}

// Rewrite mirrors SpanContainQuery.rewrite(IndexSearcher) for this subclass.
func (q *SpanWithinQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewrittenBig, rewrittenLittle, changed, err := q.rewriteSubQueries(searcher)
	if err != nil {
		return nil, err
	}
	if changed {
		clone := &SpanWithinQuery{SpanContainQuery: &SpanContainQuery{Big: rewrittenBig, Little: rewrittenLittle}}
		return clone, nil
	}
	return q, nil
}

// Equals mirrors SpanContainQuery.equals(Object), whose sameClassAs() check
// makes a SpanWithinQuery unequal to a SpanContainingQuery with the same
// sub-queries.
func (q *SpanWithinQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanWithinQuery)
	if !ok {
		return false
	}
	return q.equalsTo(o.SpanContainQuery)
}

// HashCode mirrors SpanContainQuery.hashCode() seeded with this class's hash.
func (q *SpanWithinQuery) HashCode() int {
	return q.hashCodeWithClassHash(classHashSpanWithinQuery)
}

// CreateWeight mirrors SpanWithinQuery.createWeight(IndexSearcher, ScoreMode,
// float), whose covariant return is a SpanWeight.
func (q *SpanWithinQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanWithinQuery.createWeight.
func (q *SpanWithinQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	bigWeight, err := q.Big.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	littleWeight, err := q.Little.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return newSpanWithinWeight(q, bigWeight, littleWeight).SpanWeight, nil
}

// newSpanWithinWeight mirrors the inner class SpanWithinQuery.SpanWithinWeight.
//
// Java passes `scoreMode.needsScores() ? getTermStates(bigWeight,
// littleWeight) : null` as the weight's termStates argument, which feeds
// SpanWeight.buildSimWeight and nothing else. This port carries a nil
// SimScorer here, exactly as SpanTermQuery and SpanNearQuery do, because
// buildSimWeight needs the Term behind each TermStates and the package's
// term-states map is keyed by "field:text" (see span_weight.go:57-59).
func newSpanWithinWeight(
	query *SpanWithinQuery,
	bigWeight, littleWeight *SpanWeight,
) *SpanContainWeight {
	var w *SpanContainWeight
	w = NewSpanContainWeight(
		query,
		query.GetField(),
		nil,
		bigWeight,
		littleWeight,
		func(ctx *index.LeafReaderContext, requiredPostings Postings) (Spans, error) {
			containerContained, err := w.PrepareConjunction(ctx, requiredPostings)
			if err != nil {
				return nil, err
			}
			if containerContained == nil {
				return nil, nil
			}
			return newSpanWithinSpans(containerContained[0], containerContained[1])
		},
		func(ctx *index.LeafReaderContext) bool {
			return littleWeight.IsCacheable(ctx) && bigWeight.IsCacheable(ctx)
		},
	)
	return w
}

// spanWithinSpans is the anonymous ContainSpans subclass returned by
// SpanWithinQuery.SpanWithinWeight.getSpans: it returns spans from little that
// are contained in a spans from big, and the payload comes from the spans of
// little.
type spanWithinSpans struct {
	*ContainSpans
}

func newSpanWithinSpans(big, little Spans) (Spans, error) {
	s := &spanWithinSpans{}
	cs, err := NewContainSpans(big, little, little, func() (bool, error) {
		return s.twoPhaseCurrentDocMatches()
	})
	if err != nil {
		return nil, err
	}
	s.ContainSpans = cs
	return s, nil
}

func (s *spanWithinSpans) twoPhaseCurrentDocMatches() (bool, error) {
	s.OneExhaustedInCurrentDoc = false
	for {
		littleStart, err := s.LittleSpans.NextStartPosition()
		if err != nil {
			return false, err
		}
		if littleStart == NoMorePositions {
			break
		}
		for s.BigSpans.EndPosition() < s.LittleSpans.EndPosition() {
			next, err := s.BigSpans.NextStartPosition()
			if err != nil {
				return false, err
			}
			if next == NoMorePositions {
				s.OneExhaustedInCurrentDoc = true
				return false, nil
			}
		}
		if s.BigSpans.StartPosition() <= s.LittleSpans.StartPosition() {
			s.AtFirstInCurrentDoc = true
			return true, nil
		}
	}
	s.OneExhaustedInCurrentDoc = true
	return false, nil
}

// NextStartPosition mirrors the anonymous subclass's nextStartPosition().
func (s *spanWithinSpans) NextStartPosition() (int, error) {
	if s.AtFirstInCurrentDoc {
		s.AtFirstInCurrentDoc = false
		return s.LittleSpans.StartPosition(), nil
	}
	for {
		littleStart, err := s.LittleSpans.NextStartPosition()
		if err != nil {
			return 0, err
		}
		if littleStart == NoMorePositions {
			break
		}
		for s.BigSpans.EndPosition() < s.LittleSpans.EndPosition() {
			next, err := s.BigSpans.NextStartPosition()
			if err != nil {
				return 0, err
			}
			if next == NoMorePositions {
				s.OneExhaustedInCurrentDoc = true
				return NoMorePositions, nil
			}
		}
		if s.BigSpans.StartPosition() <= s.LittleSpans.StartPosition() {
			return s.LittleSpans.StartPosition(), nil
		}
	}
	s.OneExhaustedInCurrentDoc = true
	return NoMorePositions, nil
}

var (
	_ SpanQuery = (*SpanWithinQuery)(nil)
	_ Spans     = (*spanWithinSpans)(nil)
)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanWithinQuery) String() string { return q.ToString("") }
