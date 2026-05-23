// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/MinimumShouldMatchIntervalsSource.java

package intervals

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MinimumShouldMatchIntervalsSource returns intervals from a subset of sub-sources
// where at least minShouldMatch sub-sources have matching intervals.
//
// Mirrors org.apache.lucene.queries.intervals.MinimumShouldMatchIntervalsSource.
//
// Deviations from Java:
//   - MinimumShouldMatchIntervalIterator uses a Go min-heap instead of Lucene PriorityQueue.
type MinimumShouldMatchIntervalsSource struct {
	sources        []IntervalsSource
	minShouldMatch int
}

// NewMinimumShouldMatchIntervalsSource creates a MinimumShouldMatchIntervalsSource.
func NewMinimumShouldMatchIntervalsSource(sources []IntervalsSource, minShouldMatch int) *MinimumShouldMatchIntervalsSource {
	return &MinimumShouldMatchIntervalsSource{sources: sources, minShouldMatch: minShouldMatch}
}

// Intervals creates an IntervalIterator.
func (s *MinimumShouldMatchIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	var iters []IntervalIterator
	for _, src := range s.sources {
		it, err := src.Intervals(field, ctx)
		if err != nil {
			return nil, err
		}
		if it != nil {
			iters = append(iters, it)
		}
	}
	if len(iters) < s.minShouldMatch {
		return nil, nil
	}
	return newMinimumShouldMatchIntervalIterator(iters, s.minShouldMatch, noOpMatchCallback), nil
}

// Matches creates an IntervalMatchesIterator.
func (s *MinimumShouldMatchIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	type entry struct {
		wrap IntervalIterator
		cmi  *CachingMatchesIterator
	}
	var entries []entry
	for _, src := range s.sources {
		mi, err := src.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			cmi := NewCachingMatchesIterator(mi)
			wrap := WrapMatches(cmi, doc)
			entries = append(entries, entry{wrap: wrap, cmi: cmi})
		}
	}
	if len(entries) < s.minShouldMatch {
		return nil, nil
	}
	iters := make([]IntervalIterator, len(entries))
	cacheSubs := make([]*CachingMatchesIterator, len(entries))
	for i, e := range entries {
		iters[i] = e.wrap
		cacheSubs[i] = e.cmi
	}
	onMatch := cacheIteratorsCallback(cacheSubs)
	it := newMinimumShouldMatchIntervalIterator(iters, s.minShouldMatch, onMatch)
	advanced, err := it.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return nil, nil
	}
	next, err := it.NextInterval()
	if err != nil {
		return nil, err
	}
	if next == NoMoreIntervals {
		return nil, nil
	}
	// Build a matches iterator over the currently active iterators.
	return newMinimumMatchesIterator(it, iters, cacheSubs), nil
}

// Visit visits sub-sources.
func (s *MinimumShouldMatchIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	parent := NewIntervalQuery(field, s)
	v := visitor.GetSubVisitor(search.SHOULD, parent)
	for _, src := range s.sources {
		src.Visit(field, v)
	}
}

// MinExtent returns the sum of the smallest minShouldMatch sub-source extents.
func (s *MinimumShouldMatchIntervalsSource) MinExtent() int {
	extents := make([]int, len(s.sources))
	for i, src := range s.sources {
		extents[i] = src.MinExtent()
	}
	sort.Ints(extents)
	total := 0
	for i := 0; i < s.minShouldMatch && i < len(extents); i++ {
		total += extents[i]
	}
	return total
}

// PullUpDisjunctions returns a singleton list.
func (s *MinimumShouldMatchIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports structural equality.
func (s *MinimumShouldMatchIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*MinimumShouldMatchIntervalsSource)
	if !ok || s.minShouldMatch != o.minShouldMatch || len(s.sources) != len(o.sources) {
		return false
	}
	for i, src := range s.sources {
		if !src.Equals(o.sources[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code.
func (s *MinimumShouldMatchIntervalsSource) HashCode() int {
	h := s.minShouldMatch * 31
	for _, src := range s.sources {
		h = h*31 + src.HashCode()
	}
	return h
}

// String returns a human-readable representation.
func (s *MinimumShouldMatchIntervalsSource) String() string {
	parts := make([]string, len(s.sources))
	for i, src := range s.sources {
		parts[i] = src.String()
	}
	return fmt.Sprintf("AtLeast(%s~%d)", strings.Join(parts, ","), s.minShouldMatch)
}

// ── minimumShouldMatchIntervalIterator ────────────────────────────────────

// minimumShouldMatchIntervalIterator iterates using two priority queues:
// - a proximity queue holding the top-minShouldMatch interval iterators
// - a background queue holding the remaining iterators
//
// Mirrors MinimumShouldMatchIntervalsSource.MinimumShouldMatchIntervalIterator.
type minimumShouldMatchIntervalIterator struct {
	approximation  search.DocIdSetIterator
	disiQueue      *DisiPriorityQueue
	proximityQueue []IntervalIterator // min-heap by start
	backgroundQueue []IntervalIterator // min-heap by end
	minShouldMatch int
	matchCostVal   float32
	onMatch        MatchCallback
	start          int
	end            int
	queueEnd       int
	slop           int
}

func newMinimumShouldMatchIntervalIterator(subs []IntervalIterator, minShouldMatch int, onMatch MatchCallback) *minimumShouldMatchIntervalIterator {
	dq := NewDisiPriorityQueue(len(subs))
	var mc float32
	for _, it := range subs {
		dq.Add(NewDisiWrapper(it))
		mc += it.MatchCost()
	}
	return &minimumShouldMatchIntervalIterator{
		approximation:  NewDisjunctionDISIApproximation(dq),
		disiQueue:      dq,
		minShouldMatch: minShouldMatch,
		matchCostVal:   mc,
		onMatch:        onMatch,
		start:          -1,
		end:            -1,
		queueEnd:       -1,
	}
}

func (it *minimumShouldMatchIntervalIterator) DocID() int        { return it.approximation.DocID() }
func (it *minimumShouldMatchIntervalIterator) DocIDRunEnd() int   { return it.DocID() + 1 }
func (it *minimumShouldMatchIntervalIterator) Cost() int64       { return it.approximation.Cost() }
func (it *minimumShouldMatchIntervalIterator) MatchCost() float32 { return it.matchCostVal }
func (it *minimumShouldMatchIntervalIterator) Start() int        { return it.start }
func (it *minimumShouldMatchIntervalIterator) End() int          { return it.end }
func (it *minimumShouldMatchIntervalIterator) Gaps() int         { return it.slop }
func (it *minimumShouldMatchIntervalIterator) Width() int {
	if it.end == NoMoreIntervals {
		return NoMoreIntervals
	}
	return it.end - it.start + 1
}

func (it *minimumShouldMatchIntervalIterator) NextDoc() (int, error) {
	doc, err := it.approximation.NextDoc()
	if err != nil {
		return 0, err
	}
	if err := it.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (it *minimumShouldMatchIntervalIterator) Advance(target int) (int, error) {
	doc, err := it.approximation.Advance(target)
	if err != nil {
		return 0, err
	}
	if err := it.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (it *minimumShouldMatchIntervalIterator) reset() error {
	it.proximityQueue = it.proximityQueue[:0]
	it.backgroundQueue = it.backgroundQueue[:0]
	// Populate background queue from disi top list.
	for dw := it.disiQueue.TopList(); dw != nil; dw = dw.Next {
		if dw.Intervals == nil {
			continue
		}
		next, err := dw.Intervals.NextInterval()
		if err != nil {
			return err
		}
		if next != NoMoreIntervals {
			it.bgPush(dw.Intervals)
		}
	}
	it.queueEnd = -1
	for i := 0; i < it.minShouldMatch; i++ {
		if len(it.backgroundQueue) == 0 {
			break
		}
		pop := it.bgPop()
		it.pqPush(pop)
		it.updateRightExtreme(pop)
	}
	it.start = -1
	it.end = -1
	return nil
}

func (it *minimumShouldMatchIntervalIterator) updateRightExtreme(iter IntervalIterator) {
	if iter.End() > it.queueEnd {
		it.queueEnd = iter.End()
	}
}

// proximity queue: min-heap ordered by (start asc, end desc).
func (it *minimumShouldMatchIntervalIterator) pqLess(a, b IntervalIterator) bool {
	return a.Start() < b.Start() || (a.Start() == b.Start() && a.End() >= b.End())
}
func (it *minimumShouldMatchIntervalIterator) pqPush(x IntervalIterator) {
	it.proximityQueue = append(it.proximityQueue, x)
	i := len(it.proximityQueue) - 1
	for i > 0 {
		p := (i - 1) / 2
		if it.pqLess(it.proximityQueue[i], it.proximityQueue[p]) {
			it.proximityQueue[i], it.proximityQueue[p] = it.proximityQueue[p], it.proximityQueue[i]
			i = p
		} else {
			break
		}
	}
}
func (it *minimumShouldMatchIntervalIterator) pqPop() IntervalIterator {
	top := it.proximityQueue[0]
	n := len(it.proximityQueue) - 1
	it.proximityQueue[0] = it.proximityQueue[n]
	it.proximityQueue = it.proximityQueue[:n]
	for i := 0; ; {
		left := 2*i + 1
		if left >= len(it.proximityQueue) {
			break
		}
		s := left
		if right := left + 1; right < len(it.proximityQueue) && it.pqLess(it.proximityQueue[right], it.proximityQueue[left]) {
			s = right
		}
		if it.pqLess(it.proximityQueue[s], it.proximityQueue[i]) {
			it.proximityQueue[i], it.proximityQueue[s] = it.proximityQueue[s], it.proximityQueue[i]
			i = s
		} else {
			break
		}
	}
	return top
}

// background queue: min-heap ordered by (end asc, start desc).
func (it *minimumShouldMatchIntervalIterator) bgLess(a, b IntervalIterator) bool {
	return a.End() < b.End() || (a.End() == b.End() && a.Start() >= b.Start())
}
func (it *minimumShouldMatchIntervalIterator) bgPush(x IntervalIterator) {
	it.backgroundQueue = append(it.backgroundQueue, x)
	i := len(it.backgroundQueue) - 1
	for i > 0 {
		p := (i - 1) / 2
		if it.bgLess(it.backgroundQueue[i], it.backgroundQueue[p]) {
			it.backgroundQueue[i], it.backgroundQueue[p] = it.backgroundQueue[p], it.backgroundQueue[i]
			i = p
		} else {
			break
		}
	}
}
func (it *minimumShouldMatchIntervalIterator) bgPop() IntervalIterator {
	if len(it.backgroundQueue) == 0 {
		return nil
	}
	top := it.backgroundQueue[0]
	n := len(it.backgroundQueue) - 1
	it.backgroundQueue[0] = it.backgroundQueue[n]
	it.backgroundQueue = it.backgroundQueue[:n]
	for i := 0; ; {
		left := 2*i + 1
		if left >= len(it.backgroundQueue) {
			break
		}
		s := left
		if right := left + 1; right < len(it.backgroundQueue) && it.bgLess(it.backgroundQueue[right], it.backgroundQueue[left]) {
			s = right
		}
		if it.bgLess(it.backgroundQueue[s], it.backgroundQueue[i]) {
			it.backgroundQueue[i], it.backgroundQueue[s] = it.backgroundQueue[s], it.backgroundQueue[i]
			i = s
		} else {
			break
		}
	}
	return top
}

func (it *minimumShouldMatchIntervalIterator) NextInterval() (int, error) {
	// Advance top of proximity queue if it matches current start.
	for len(it.proximityQueue) == it.minShouldMatch && it.proximityQueue[0].Start() == it.start {
		popped := it.pqPop()
		next, err := popped.NextInterval()
		if err != nil {
			return 0, err
		}
		if next != NoMoreIntervals {
			it.bgPush(popped)
			next2 := it.bgPop()
			if next2 != nil {
				it.pqPush(next2)
				it.updateRightExtreme(next2)
			}
		}
	}
	if len(it.proximityQueue) < it.minShouldMatch {
		it.start = NoMoreIntervals
		it.end = NoMoreIntervals
		return NoMoreIntervals, nil
	}
	// Minimize.
	for {
		if err := it.onMatch(); err != nil {
			return 0, err
		}
		it.start = it.proximityQueue[0].Start()
		it.end = it.queueEnd
		it.slop = it.end - it.start + 1
		for _, x := range it.proximityQueue {
			it.slop -= x.Width()
		}
		if it.proximityQueue[0].End() == it.end {
			return it.start, nil
		}
		popped := it.pqPop()
		next, err := popped.NextInterval()
		if err != nil {
			return 0, err
		}
		if next != NoMoreIntervals {
			it.bgPush(popped)
		}
		next2 := it.bgPop()
		if next2 != nil {
			it.pqPush(next2)
			it.updateRightExtreme(next2)
		}
		if len(it.proximityQueue) < it.minShouldMatch || it.end != it.queueEnd {
			return it.start, nil
		}
	}
}

var _ search.DocIdSetIterator = (*minimumShouldMatchIntervalIterator)(nil)

// ── minimumMatchesIterator ─────────────────────────────────────────────────

// minimumMatchesIterator wraps a minimumShouldMatchIntervalIterator and provides
// sub-match information from the proximity queue's CachingMatchesIterators.
type minimumMatchesIterator struct {
	iterator    *minimumShouldMatchIntervalIterator
	allWraps    []IntervalIterator
	cacheSubs   []*CachingMatchesIterator
	cached      bool
}

func newMinimumMatchesIterator(it *minimumShouldMatchIntervalIterator, wraps []IntervalIterator, cacheSubs []*CachingMatchesIterator) *minimumMatchesIterator {
	return &minimumMatchesIterator{iterator: it, allWraps: wraps, cacheSubs: cacheSubs, cached: true}
}

func (m *minimumMatchesIterator) Next() (bool, error) {
	if m.cached {
		m.cached = false
		return true, nil
	}
	next, err := m.iterator.NextInterval()
	if err != nil {
		return false, err
	}
	return next != NoMoreIntervals, nil
}

func (m *minimumMatchesIterator) StartPosition() int { return m.iterator.Start() }
func (m *minimumMatchesIterator) EndPosition() int   { return m.iterator.End() }

func (m *minimumMatchesIterator) StartOffset() (int, error) {
	min := int(^uint(0) >> 1)
	for _, cmi := range m.activeCacheSubs() {
		so, err := cmi.StartOffset()
		if err != nil {
			return 0, err
		}
		if so < min {
			min = so
		}
	}
	if min == int(^uint(0)>>1) {
		return -1, nil
	}
	return min, nil
}

func (m *minimumMatchesIterator) EndOffset() (int, error) {
	max := 0
	for _, cmi := range m.activeCacheSubs() {
		eo, err := cmi.EndOffset()
		if err != nil {
			return 0, err
		}
		if eo > max {
			max = eo
		}
	}
	return max, nil
}

func (m *minimumMatchesIterator) Gaps() int  { return m.iterator.Gaps() }
func (m *minimumMatchesIterator) Width() int { return m.iterator.Width() }

func (m *minimumMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	var mis []search.MatchesIterator
	for _, cmi := range m.activeCacheSubs() {
		subs, err := cmi.GetSubMatches()
		if err != nil {
			return nil, err
		}
		if subs != nil {
			mis = append(mis, subs)
		} else {
			mis = append(mis, cmi)
		}
	}
	return search.DisjunctionMatchesIterator(mis), nil
}

func (m *minimumMatchesIterator) GetQuery() search.Query { return nil }

// activeCacheSubs returns the CachingMatchesIterators for the current active iterators.
func (m *minimumMatchesIterator) activeCacheSubs() []*CachingMatchesIterator {
	var active []*CachingMatchesIterator
	for i, wrap := range m.allWraps {
		if wrap.End() <= m.iterator.End() {
			active = append(active, m.cacheSubs[i])
		}
	}
	return active
}
