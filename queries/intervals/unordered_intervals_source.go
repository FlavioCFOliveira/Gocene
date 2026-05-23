// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/UnorderedIntervalsSource.java

package intervals

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// UnorderedIntervalsSource returns intervals that cover all sub-source intervals in
// any order, minimizing the total span.
//
// Mirrors org.apache.lucene.queries.intervals.UnorderedIntervalsSource.
//
// Deviations from Java:
//   - Uses an insertion-sorted slice instead of a PriorityQueue.
type UnorderedIntervalsSource struct {
	*MinimizingConjunctionIntervalsSource
	subSrcs []IntervalsSource
}

// BuildUnorderedIntervalsSource builds an unordered interval source.
func BuildUnorderedIntervalsSource(sources []IntervalsSource) IntervalsSource {
	if len(sources) == 1 {
		return sources[0]
	}
	rewritten := deduplicateUnordered(sources)
	if len(rewritten) == 1 {
		return rewritten[0]
	}
	return newUnorderedIntervalsSource(rewritten)
}

func deduplicateUnordered(sources []IntervalsSource) []IntervalsSource {
	// Count occurrences, preserving insertion order.
	type entry struct {
		src   IntervalsSource
		count int
	}
	var order []IntervalsSource
	counts := make(map[string]int)
	srcs := make(map[string]IntervalsSource)
	for _, src := range sources {
		k := src.String()
		if _, ok := counts[k]; !ok {
			order = append(order, src)
			srcs[k] = src
		}
		counts[k]++
	}
	var result []IntervalsSource
	for _, src := range order {
		k := src.String()
		cnt := counts[k]
		result = append(result, NewRepeatingIntervalsSource(srcs[k], cnt))
	}
	// Only set "UNORDERED" name when there is exactly one deduplicated group.
	if len(result) == 1 {
		if r, ok := result[0].(*RepeatingIntervalsSource); ok {
			r.SetName("UNORDERED")
		}
	}
	return result
}

func newUnorderedIntervalsSource(subSrcs []IntervalsSource) *UnorderedIntervalsSource {
	s := &UnorderedIntervalsSource{subSrcs: subSrcs}
	s.MinimizingConjunctionIntervalsSource = NewMinimizingConjunctionIntervalsSource(
		subSrcs,
		func(iters []IntervalIterator, onMatch MatchCallback) IntervalIterator {
			return newUnorderedIntervalIterator(iters, onMatch)
		},
	)
	return s
}

// Intervals delegates to embedded source.
func (s *UnorderedIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	return s.MinimizingConjunctionIntervalsSource.Intervals(field, ctx)
}

// Matches delegates to embedded source.
func (s *UnorderedIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	return s.MinimizingConjunctionIntervalsSource.Matches(field, ctx, doc)
}

// Visit visits sub-sources.
func (s *UnorderedIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	parent := NewIntervalQuery(field, s)
	v := visitor.GetSubVisitor(search.MUST, parent)
	for _, src := range s.subSrcs {
		src.Visit(field, v)
	}
}

// MinExtent returns the sum of sub-source min extents.
func (s *UnorderedIntervalsSource) MinExtent() int {
	total := 0
	for _, src := range s.subSrcs {
		total += src.MinExtent()
	}
	return total
}

// PullUpDisjunctions pulls up disjunctions.
func (s *UnorderedIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return disjunctionsPullUp(s.subSrcs, func(srcs []IntervalsSource) IntervalsSource {
		return newUnorderedIntervalsSource(srcs)
	})
}

// Equals reports structural equality.
func (s *UnorderedIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*UnorderedIntervalsSource)
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
func (s *UnorderedIntervalsSource) HashCode() int {
	h := 17
	for _, src := range s.subSrcs {
		h = h*31 + src.HashCode()
	}
	return h
}

// String returns a human-readable representation.
func (s *UnorderedIntervalsSource) String() string {
	parts := make([]string, len(s.subSrcs))
	for i, src := range s.subSrcs {
		parts[i] = src.String()
	}
	return "UNORDERED(" + strings.Join(parts, ",") + ")"
}

// ── UnorderedIntervalIterator ──────────────────────────────────────────────

// unorderedIntervalIterator iterates over unordered intervals minimizing total span.
// Mirrors UnorderedIntervalsSource.UnorderedIntervalIterator.
type unorderedIntervalIterator struct {
	*ConjunctionIntervalIterator
	// heap stores iterators sorted by start ascending, end descending (min-heap by start).
	heap     []IntervalIterator
	allIters []IntervalIterator
	start    int
	end      int
	queueEnd int
	slop     int
	onMatch  MatchCallback
}

func newUnorderedIntervalIterator(iters []IntervalIterator, onMatch MatchCallback) *unorderedIntervalIterator {
	allIters := make([]IntervalIterator, len(iters))
	copy(allIters, iters)
	it := &unorderedIntervalIterator{
		allIters: allIters,
		onMatch:  onMatch,
		start:    -1,
		end:      -1,
		queueEnd: -1,
	}
	it.ConjunctionIntervalIterator = NewConjunctionIntervalIterator(iters, func() error {
		return it.doReset()
	})
	return it
}

func (u *unorderedIntervalIterator) Start() int { return u.start }
func (u *unorderedIntervalIterator) End() int   { return u.end }
func (u *unorderedIntervalIterator) Gaps() int  { return u.slop }

// unorderedHeapLess returns true if a should come before b in the min-heap.
func unorderedHeapLess(a, b IntervalIterator) bool {
	return a.Start() < b.Start() || (a.Start() == b.Start() && a.End() >= b.End())
}

func (u *unorderedIntervalIterator) heapPush(it IntervalIterator) {
	u.heap = append(u.heap, it)
	i := len(u.heap) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if unorderedHeapLess(u.heap[i], u.heap[parent]) {
			u.heap[i], u.heap[parent] = u.heap[parent], u.heap[i]
			i = parent
		} else {
			break
		}
	}
}

func (u *unorderedIntervalIterator) heapPop() IntervalIterator {
	top := u.heap[0]
	last := len(u.heap) - 1
	u.heap[0] = u.heap[last]
	u.heap = u.heap[:last]
	u.heapDown(0)
	return top
}

func (u *unorderedIntervalIterator) heapDown(i int) {
	n := len(u.heap)
	for {
		left := 2*i + 1
		if left >= n {
			break
		}
		smallest := left
		right := left + 1
		if right < n && unorderedHeapLess(u.heap[right], u.heap[left]) {
			smallest = right
		}
		if unorderedHeapLess(u.heap[smallest], u.heap[i]) {
			u.heap[i], u.heap[smallest] = u.heap[smallest], u.heap[i]
			i = smallest
		} else {
			break
		}
	}
}

func (u *unorderedIntervalIterator) updateRightExtreme(it IntervalIterator) {
	if it.End() > u.queueEnd {
		u.queueEnd = it.End()
	}
}

func (u *unorderedIntervalIterator) NextInterval() (int, error) {
	n := len(u.allIters)
	// Advance the top if it matches the current start (to find next candidate).
	for len(u.heap) == n && u.heap[0].Start() == u.start {
		it := u.heapPop()
		next, err := it.NextInterval()
		if err != nil {
			return 0, err
		}
		if next != NoMoreIntervals {
			u.heapPush(it)
			u.updateRightExtreme(it)
		}
	}
	if len(u.heap) < n {
		u.start = NoMoreIntervals
		u.end = NoMoreIntervals
		return NoMoreIntervals, nil
	}
	// Minimize.
	for {
		u.start = u.heap[0].Start()
		u.end = u.queueEnd
		u.slop = u.end - u.start + 1
		for _, it := range u.allIters {
			u.slop -= it.Width()
		}
		if err := u.onMatch(); err != nil {
			return 0, err
		}
		if u.heap[0].End() == u.end {
			return u.start, nil
		}
		it := u.heapPop()
		next, err := it.NextInterval()
		if err != nil {
			return 0, err
		}
		if next != NoMoreIntervals {
			u.heapPush(it)
			u.updateRightExtreme(it)
		}
		if len(u.heap) < n || u.end != u.queueEnd {
			return u.start, nil
		}
	}
}

func (u *unorderedIntervalIterator) doReset() error {
	u.queueEnd = -1
	u.start = -1
	u.end = -1
	u.heap = u.heap[:0]
	for _, it := range u.allIters {
		next, err := it.NextInterval()
		if err != nil {
			return err
		}
		if next == NoMoreIntervals {
			break
		}
		u.heapPush(it)
		u.updateRightExtreme(it)
	}
	return nil
}

var _ search.DocIdSetIterator = (*unorderedIntervalIterator)(nil)
