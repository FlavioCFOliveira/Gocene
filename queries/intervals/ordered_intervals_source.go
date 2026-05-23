// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/OrderedIntervalsSource.java

package intervals

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OrderedIntervalsSource returns intervals that appear in order across all sub-sources,
// with optional slop (gap) between them.
//
// Mirrors org.apache.lucene.queries.intervals.OrderedIntervalsSource.
//
// Deviations from Java:
//   - combineFn field replaces inheritance from MinimizingConjunctionIntervalsSource.
//   - RepeatingIntervalsSource deduplication is preserved.
type OrderedIntervalsSource struct {
	*MinimizingConjunctionIntervalsSource
	subSrcs []IntervalsSource
}

// BuildOrderedIntervalsSource builds an ordered interval source.
// Deduplicates consecutive equal sources by wrapping in RepeatingIntervalsSource.
func BuildOrderedIntervalsSource(sources []IntervalsSource) IntervalsSource {
	if len(sources) == 1 {
		return sources[0]
	}
	rewritten := deduplicateOrdered(sources)
	if len(rewritten) == 1 {
		return rewritten[0]
	}
	return newOrderedIntervalsSource(rewritten)
}

func deduplicateOrdered(sources []IntervalsSource) []IntervalsSource {
	var deduplicated []IntervalsSource
	var current []IntervalsSource
	for _, src := range sources {
		if len(current) == 0 || current[0].Equals(src) {
			current = append(current, src)
		} else {
			deduplicated = append(deduplicated, NewRepeatingIntervalsSource(current[0], len(current)))
			current = []IntervalsSource{src}
		}
	}
	if len(current) > 0 {
		deduplicated = append(deduplicated, NewRepeatingIntervalsSource(current[0], len(current)))
	}
	// Only set "ORDERED" name when there is exactly one deduplicated group (all sources equal).
	if len(deduplicated) == 1 {
		if r, ok := deduplicated[0].(*RepeatingIntervalsSource); ok {
			r.SetName("ORDERED")
		}
	}
	return deduplicated
}

func newOrderedIntervalsSource(subSrcs []IntervalsSource) *OrderedIntervalsSource {
	s := &OrderedIntervalsSource{subSrcs: subSrcs}
	s.MinimizingConjunctionIntervalsSource = NewMinimizingConjunctionIntervalsSource(
		subSrcs,
		func(iters []IntervalIterator, onMatch MatchCallback) IntervalIterator {
			return newOrderedIntervalIterator(iters, onMatch)
		},
	)
	return s
}

// Intervals overrides to use subSrcs (not the embedded subSources copy).
func (s *OrderedIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	return s.MinimizingConjunctionIntervalsSource.Intervals(field, ctx)
}

// Matches overrides to use subSrcs.
func (s *OrderedIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	return s.MinimizingConjunctionIntervalsSource.Matches(field, ctx, doc)
}

// Visit visits sub-sources.
func (s *OrderedIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	parent := NewIntervalQuery(field, s)
	v := visitor.GetSubVisitor(search.MUST, parent)
	for _, src := range s.subSrcs {
		src.Visit(field, v)
	}
}

// MinExtent returns the sum of sub-source min extents.
func (s *OrderedIntervalsSource) MinExtent() int {
	total := 0
	for _, src := range s.subSrcs {
		total += src.MinExtent()
	}
	return total
}

// PullUpDisjunctions pulls up disjunctions.
func (s *OrderedIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return disjunctionsPullUp(s.subSrcs, func(srcs []IntervalsSource) IntervalsSource {
		return newOrderedIntervalsSource(srcs)
	})
}

// Equals reports structural equality.
func (s *OrderedIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*OrderedIntervalsSource)
	if !ok || len(s.subSrcs) != len(o.subSrcs) {
		return false
	}
	for i, src := range s.subSrcs {
		if !src.Equals(o.subSrcs[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code.
func (s *OrderedIntervalsSource) HashCode() int {
	h := 17
	for _, src := range s.subSrcs {
		h = h*31 + src.HashCode()
	}
	return h
}

// String returns a human-readable representation.
func (s *OrderedIntervalsSource) String() string {
	parts := make([]string, len(s.subSrcs))
	for i, src := range s.subSrcs {
		parts[i] = src.String()
	}
	return "ORDERED(" + strings.Join(parts, ",") + ")"
}

// ── OrderedIntervalIterator ────────────────────────────────────────────────

// orderedIntervalIterator iterates over ordered intervals satisfying all sub-iterators
// in sequence. Mirrors OrderedIntervalsSource.OrderedIntervalIterator.
type orderedIntervalIterator struct {
	*ConjunctionIntervalIterator
	start   int
	end     int
	slop    int
	onMatch MatchCallback
}

func newOrderedIntervalIterator(iters []IntervalIterator, onMatch MatchCallback) *orderedIntervalIterator {
	it := &orderedIntervalIterator{onMatch: onMatch, start: -1, end: -1, slop: -1}
	it.ConjunctionIntervalIterator = NewConjunctionIntervalIterator(iters, func() error {
		return it.doReset()
	})
	return it
}

func (o *orderedIntervalIterator) Start() int { return o.start }
func (o *orderedIntervalIterator) End() int   { return o.end }
func (o *orderedIntervalIterator) Gaps() int  { return o.slop }

func (o *orderedIntervalIterator) NextInterval() (int, error) {
	o.start = NoMoreIntervals
	o.end = NoMoreIntervals
	o.slop = NoMoreIntervals
	n := len(o.SubIterators)
	if n == 0 {
		return NoMoreIntervals, nil
	}
	// Advance first iterator.
	next, err := o.SubIterators[0].NextInterval()
	if err != nil {
		return 0, err
	}
	if next == NoMoreIntervals {
		return NoMoreIntervals, nil
	}
	i := 1
	for {
		// Try to align all remaining iterators.
		for i < n {
			prevEnd := o.SubIterators[i-1].End()
			cur := o.SubIterators[i]
			// Advance cur past prevEnd.
			for cur.Start() <= prevEnd {
				next, err := cur.NextInterval()
				if err != nil {
					return 0, err
				}
				if next == NoMoreIntervals {
					return NoMoreIntervals, nil
				}
			}
			i++
		}
		// All aligned: record the match.
		o.start = o.SubIterators[0].Start()
		o.end = o.SubIterators[n-1].End()
		o.slop = o.end - o.start + 1
		for _, it := range o.SubIterators {
			o.slop -= it.Width()
		}
		if err := o.onMatch(); err != nil {
			return 0, err
		}
		// Try to minimize by advancing the first iterator.
		next, err = o.SubIterators[0].NextInterval()
		if err != nil {
			return 0, err
		}
		if next == NoMoreIntervals {
			return o.start, nil
		}
		i = 1
	}
}

func (o *orderedIntervalIterator) doReset() error {
	if len(o.SubIterators) == 0 {
		return nil
	}
	// Advance the first sub-iterator.
	_, err := o.SubIterators[0].NextInterval()
	o.start = -1
	o.end = -1
	o.slop = -1
	return err
}

var _ search.DocIdSetIterator = (*orderedIntervalIterator)(nil)
