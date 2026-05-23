// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/DifferenceIntervalsSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DifferenceIntervalsSource is the abstract base for interval sources that
// compute the difference between a minuend and a subtrahend interval source.
//
// Mirrors org.apache.lucene.queries.intervals.DifferenceIntervalsSource (abstract).
//
// Deviations from Java:
//   - combineFn replaces the abstract combine method.
type DifferenceIntervalsSource struct {
	Minuend    IntervalsSource
	Subtrahend IntervalsSource
	combineFn  func(minuend, subtrahend IntervalIterator) IntervalIterator
}

// NewDifferenceIntervalsSource constructs a DifferenceIntervalsSource.
func NewDifferenceIntervalsSource(
	minuend, subtrahend IntervalsSource,
	combineFn func(minuend, subtrahend IntervalIterator) IntervalIterator,
) *DifferenceIntervalsSource {
	return &DifferenceIntervalsSource{
		Minuend:    minuend,
		Subtrahend: subtrahend,
		combineFn:  combineFn,
	}
}

// Intervals creates an IntervalIterator for the given field and context.
func (s *DifferenceIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	minIt, err := s.Minuend.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	if minIt == nil {
		return nil, nil
	}
	subIt, err := s.Subtrahend.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	if subIt == nil {
		return minIt, nil
	}
	return s.combineFn(minIt, subIt), nil
}

// Matches returns an IntervalMatchesIterator for the given field, context and doc.
func (s *DifferenceIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	minIt, err := s.Minuend.Matches(field, ctx, doc)
	if err != nil {
		return nil, err
	}
	if minIt == nil {
		return nil, nil
	}
	subIt, err := s.Subtrahend.Matches(field, ctx, doc)
	if err != nil {
		return nil, err
	}
	if subIt == nil {
		return minIt, nil
	}
	difference := s.combineFn(WrapMatches(minIt, doc), WrapMatches(subIt, doc))
	return AsMatches(difference, minIt, doc)
}

// Visit visits the source tree.
func (s *DifferenceIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	s.Minuend.Visit(field, visitor)
	s.Subtrahend.Visit(field, visitor)
}

// MinExtent returns the minimum extent from the minuend.
func (s *DifferenceIntervalsSource) MinExtent() int { return s.Minuend.MinExtent() }

// PullUpDisjunctions returns a singleton list.
func (s *DifferenceIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports whether this source equals another DifferenceIntervalsSource.
func (s *DifferenceIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*DifferenceIntervalsSource)
	if !ok {
		return false
	}
	return s.Minuend.Equals(o.Minuend) && s.Subtrahend.Equals(o.Subtrahend)
}

// HashCode returns a hash code.
func (s *DifferenceIntervalsSource) HashCode() int {
	return s.Minuend.HashCode()*31 + s.Subtrahend.HashCode()
}

// String returns a string representation.
func (s *DifferenceIntervalsSource) String() string {
	return "DIFF(" + s.Minuend.String() + "," + s.Subtrahend.String() + ")"
}
