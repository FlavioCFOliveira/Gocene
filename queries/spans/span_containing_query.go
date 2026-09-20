// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanContainingQuery.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// classHashSpanContainingQuery seeds SpanContainingQuery.hashCode() in place of
// Java's classHash(). See SpanContainQuery.hashCodeWithClassHash.
const classHashSpanContainingQuery = 0x5370_4374 // "SpCt"

// SpanContainingQuery keeps matches that contain another SpanScorer.
//
// Mirrors org.apache.lucene.queries.spans.SpanContainingQuery (final).
type SpanContainingQuery struct {
	*SpanContainQuery
}

// NewSpanContainingQuery constructs a SpanContainingQuery matching spans from
// big that contain at least one spans from little. This query has the boost of
// big. big and little must be in the same field.
//
// Mirrors SpanContainingQuery(SpanQuery, SpanQuery).
func NewSpanContainingQuery(big, little SpanQuery) (*SpanContainingQuery, error) {
	base, err := NewSpanContainQuery(big, little)
	if err != nil {
		return nil, err
	}
	return &SpanContainingQuery{SpanContainQuery: base}, nil
}

// ToString mirrors SpanContainingQuery.toString(String field).
func (q *SpanContainingQuery) ToString(field string) string {
	return q.toStringWithName(field, "SpanContaining")
}

// Rewrite mirrors SpanContainQuery.rewrite(IndexSearcher) for this subclass:
// Java clones the concrete query and replaces big and little.
func (q *SpanContainingQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewrittenBig, rewrittenLittle, changed, err := q.rewriteSubQueries(searcher)
	if err != nil {
		return nil, err
	}
	if changed {
		clone := &SpanContainingQuery{SpanContainQuery: &SpanContainQuery{Big: rewrittenBig, Little: rewrittenLittle}}
		return clone, nil
	}
	return q, nil
}

// Equals mirrors SpanContainQuery.equals(Object), whose sameClassAs() check
// makes a SpanContainingQuery unequal to a SpanWithinQuery with the same
// sub-queries.
func (q *SpanContainingQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanContainingQuery)
	if !ok {
		return false
	}
	return q.equalsTo(o.SpanContainQuery)
}

// HashCode mirrors SpanContainQuery.hashCode() seeded with this class's hash.
func (q *SpanContainingQuery) HashCode() int {
	return q.hashCodeWithClassHash(classHashSpanContainingQuery)
}

// CreateWeight mirrors SpanContainingQuery.createWeight(IndexSearcher,
// ScoreMode, float), whose covariant return is a SpanWeight.
func (q *SpanContainingQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanContainingQuery.createWeight.
func (q *SpanContainingQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	w, err := q.createSpanContainingWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return w.SpanWeight, nil
}

func (q *SpanContainingQuery) createSpanContainingWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanContainWeight, error) {
	bigWeight, err := q.Big.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	littleWeight, err := q.Little.CreateSpanWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return newSpanContainingWeight(q, bigWeight, littleWeight), nil
}

// newSpanContainingWeight mirrors the inner class
// SpanContainingQuery.SpanContainingWeight.
//
// Java passes `scoreMode.needsScores() ? getTermStates(bigWeight,
// littleWeight) : null` as the weight's termStates argument, which feeds
// SpanWeight.buildSimWeight and nothing else. This port carries a nil
// SimScorer here, exactly as SpanTermQuery and SpanNearQuery do, because
// buildSimWeight needs the Term behind each TermStates and the package's
// term-states map is keyed by "field:text" (see span_weight.go:57-59).
func newSpanContainingWeight(
	query *SpanContainingQuery,
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
			return newSpanContainingSpans(containerContained[0], containerContained[1])
		},
		func(ctx *index.LeafReaderContext) bool {
			return bigWeight.IsCacheable(ctx) && littleWeight.IsCacheable(ctx)
		},
	)
	return w
}

// spanContainingSpans is the anonymous ContainSpans subclass returned by
// SpanContainingQuery.SpanContainingWeight.getSpans: it returns spans from big
// that contain at least one spans from little, and the payload comes from the
// spans of big.
type spanContainingSpans struct {
	*ContainSpans
}

func newSpanContainingSpans(big, little Spans) (Spans, error) {
	s := &spanContainingSpans{}
	cs, err := NewContainSpans(big, little, big, func() (bool, error) {
		return s.twoPhaseCurrentDocMatches()
	})
	if err != nil {
		return nil, err
	}
	s.ContainSpans = cs
	return s, nil
}

func (s *spanContainingSpans) twoPhaseCurrentDocMatches() (bool, error) {
	s.OneExhaustedInCurrentDoc = false
	for {
		bigStart, err := s.BigSpans.NextStartPosition()
		if err != nil {
			return false, err
		}
		if bigStart == NoMorePositions {
			break
		}
		for s.LittleSpans.StartPosition() < s.BigSpans.StartPosition() {
			next, err := s.LittleSpans.NextStartPosition()
			if err != nil {
				return false, err
			}
			if next == NoMorePositions {
				s.OneExhaustedInCurrentDoc = true
				return false, nil
			}
		}
		if s.BigSpans.EndPosition() >= s.LittleSpans.EndPosition() {
			s.AtFirstInCurrentDoc = true
			return true, nil
		}
	}
	s.OneExhaustedInCurrentDoc = true
	return false, nil
}

// NextStartPosition mirrors the anonymous subclass's nextStartPosition().
func (s *spanContainingSpans) NextStartPosition() (int, error) {
	if s.AtFirstInCurrentDoc {
		s.AtFirstInCurrentDoc = false
		return s.BigSpans.StartPosition(), nil
	}
	for {
		bigStart, err := s.BigSpans.NextStartPosition()
		if err != nil {
			return 0, err
		}
		if bigStart == NoMorePositions {
			break
		}
		for s.LittleSpans.StartPosition() < s.BigSpans.StartPosition() {
			next, err := s.LittleSpans.NextStartPosition()
			if err != nil {
				return 0, err
			}
			if next == NoMorePositions {
				s.OneExhaustedInCurrentDoc = true
				return NoMorePositions, nil
			}
		}
		if s.BigSpans.EndPosition() >= s.LittleSpans.EndPosition() {
			return s.BigSpans.StartPosition(), nil
		}
	}
	s.OneExhaustedInCurrentDoc = true
	return NoMorePositions, nil
}

var (
	_ SpanQuery = (*SpanContainingQuery)(nil)
	_ Spans     = (*spanContainingSpans)(nil)
)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanContainingQuery) String() string { return q.ToString("") }
