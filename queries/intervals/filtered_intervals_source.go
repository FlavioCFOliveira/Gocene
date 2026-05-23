// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/FilteredIntervalsSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FilteredIntervalsSource is an IntervalsSource that filters intervals from a sub-source
// by an accept predicate.
//
// Mirrors org.apache.lucene.queries.intervals.FilteredIntervalsSource.
type FilteredIntervalsSource struct {
	name           string
	in             IntervalsSource
	acceptFn       func(it IntervalIterator) bool
	maxWidthPullUp bool // true when this is a MaxWidth filter (pulls up disjunctions)
	maxWidthVal    int  // the width value when maxWidthPullUp is true
}

// MaxGapsIntervalsSource returns an IntervalsSource that filters intervals with more than
// maxGaps gaps.
func MaxGapsIntervalsSource(in IntervalsSource, maxGaps int) IntervalsSource {
	disjuncts := in.PullUpDisjunctions()
	sources := make([]IntervalsSource, len(disjuncts))
	for i, d := range disjuncts {
		sources[i] = newMaxGaps(d, maxGaps)
	}
	return NewDisjunctionIntervalsSource(sources, true)
}

func newMaxGaps(in IntervalsSource, maxGaps int) *FilteredIntervalsSource {
	name := "MAXGAPS/" + intToStr(maxGaps)
	return &FilteredIntervalsSource{
		name: name,
		in:   in,
		acceptFn: func(it IntervalIterator) bool {
			return it.Gaps() <= maxGaps
		},
	}
}

// MaxWidthIntervalsSource returns an IntervalsSource that filters intervals wider than maxWidth.
func MaxWidthIntervalsSource(in IntervalsSource, maxWidth int) IntervalsSource {
	return newMaxWidth(in, maxWidth)
}

func newMaxWidth(in IntervalsSource, maxWidth int) *FilteredIntervalsSource {
	name := "MAXWIDTH/" + intToStr(maxWidth)
	return &FilteredIntervalsSource{
		name: name,
		in:   in,
		acceptFn: func(it IntervalIterator) bool {
			return (it.End()-it.Start())+1 <= maxWidth
		},
		maxWidthPullUp: true,
		maxWidthVal:    maxWidth,
	}
}

// Intervals creates an IntervalIterator that yields only accepted intervals.
func (s *FilteredIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	i, err := s.in.Intervals(field, ctx)
	if err != nil || i == nil {
		return nil, err
	}
	accept := s.acceptFn
	return newFilteringIntervalIterator(i, accept), nil
}

// filteringIntervalIterator filters an IntervalIterator by an accept predicate.
type filteringIntervalIterator struct {
	inner    IntervalIterator
	acceptFn func(IntervalIterator) bool
}

func newFilteringIntervalIterator(inner IntervalIterator, accept func(IntervalIterator) bool) *filteringIntervalIterator {
	return &filteringIntervalIterator{inner: inner, acceptFn: accept}
}

func (f *filteringIntervalIterator) DocID() int        { return f.inner.DocID() }
func (f *filteringIntervalIterator) DocIDRunEnd() int   { return f.DocID() + 1 }
func (f *filteringIntervalIterator) Cost() int64       { return f.inner.Cost() }
func (f *filteringIntervalIterator) MatchCost() float32 { return f.inner.MatchCost() }
func (f *filteringIntervalIterator) Start() int        { return f.inner.Start() }
func (f *filteringIntervalIterator) End() int          { return f.inner.End() }
func (f *filteringIntervalIterator) Gaps() int         { return f.inner.Gaps() }
func (f *filteringIntervalIterator) Width() int        { return f.inner.Width() }

func (f *filteringIntervalIterator) NextDoc() (int, error) { return f.inner.NextDoc() }
func (f *filteringIntervalIterator) Advance(target int) (int, error) {
	return f.inner.Advance(target)
}

func (f *filteringIntervalIterator) NextInterval() (int, error) {
	for {
		next, err := f.inner.NextInterval()
		if err != nil || next == NoMoreIntervals {
			return NoMoreIntervals, err
		}
		if f.acceptFn(f.inner) {
			return next, nil
		}
	}
}

var _ search.DocIdSetIterator = (*filteringIntervalIterator)(nil)

// Matches creates an IntervalMatchesIterator filtered by the accept predicate.
func (s *FilteredIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mi, err := s.in.Matches(field, ctx, doc)
	if err != nil || mi == nil {
		return nil, err
	}
	accept := s.acceptFn
	return &filteredMatchesIterator{inner: mi, acceptFn: accept}, nil
}

// filteredMatchesIterator wraps an IntervalMatchesIterator, filtering by an accept predicate.
type filteredMatchesIterator struct {
	inner    IntervalMatchesIterator
	acceptFn func(IntervalIterator) bool
}

func (m *filteredMatchesIterator) Gaps() int  { return m.inner.Gaps() }
func (m *filteredMatchesIterator) Width() int { return m.inner.Width() }
func (m *filteredMatchesIterator) StartPosition() int { return m.inner.StartPosition() }
func (m *filteredMatchesIterator) EndPosition() int   { return m.inner.EndPosition() }
func (m *filteredMatchesIterator) StartOffset() (int, error) { return m.inner.StartOffset() }
func (m *filteredMatchesIterator) EndOffset() (int, error)   { return m.inner.EndOffset() }
func (m *filteredMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return m.inner.GetSubMatches()
}
func (m *filteredMatchesIterator) GetQuery() search.Query { return m.inner.GetQuery() }

func (m *filteredMatchesIterator) Next() (bool, error) {
	for {
		ok, err := m.inner.Next()
		if err != nil || !ok {
			return false, err
		}
		if m.acceptFn(matchesAsIntervalIterator{m.inner}) {
			return true, nil
		}
	}
}

// matchesAsIntervalIterator wraps an IntervalMatchesIterator as an IntervalIterator
// for the accept predicate.
type matchesAsIntervalIterator struct {
	mi IntervalMatchesIterator
}

func (w matchesAsIntervalIterator) DocID() int            { return 0 }
func (w matchesAsIntervalIterator) DocIDRunEnd() int      { return 1 }
func (w matchesAsIntervalIterator) NextDoc() (int, error) { return 0, nil }
func (w matchesAsIntervalIterator) Advance(_ int) (int, error) { return 0, nil }
func (w matchesAsIntervalIterator) Cost() int64           { return 0 }
func (w matchesAsIntervalIterator) MatchCost() float32    { return 0 }
func (w matchesAsIntervalIterator) Start() int            { return w.mi.StartPosition() }
func (w matchesAsIntervalIterator) End() int              { return w.mi.EndPosition() }
func (w matchesAsIntervalIterator) Gaps() int             { return w.mi.Gaps() }
func (w matchesAsIntervalIterator) Width() int            { return w.mi.Width() }
func (w matchesAsIntervalIterator) NextInterval() (int, error) { return NoMoreIntervals, nil }

// MinExtent delegates to the sub-source.
func (s *FilteredIntervalsSource) MinExtent() int { return s.in.MinExtent() }

// PullUpDisjunctions returns a singleton list by default.
// MaxWidth overrides this to pull up disjunctions (stored in maxWidthPullUp field).
func (s *FilteredIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	if s.maxWidthPullUp {
		return disjunctionsPullUpSingle(s.in, func(inner IntervalsSource) IntervalsSource {
			return newMaxWidth(inner, s.maxWidthVal)
		})
	}
	return []IntervalsSource{s}
}

// Visit delegates to the sub-source.
func (s *FilteredIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	s.in.Visit(field, visitor)
}

// Equals reports structural equality.
func (s *FilteredIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*FilteredIntervalsSource)
	if !ok {
		return false
	}
	return s.name == o.name && s.in.Equals(o.in)
}

// HashCode returns a hash code.
func (s *FilteredIntervalsSource) HashCode() int {
	return hashString(s.name)*31 + s.in.HashCode()
}

// String returns a human-readable representation.
func (s *FilteredIntervalsSource) String() string {
	return s.name + "(" + s.in.String() + ")"
}

// intToStr converts an int to string without fmt import.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 20)
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
