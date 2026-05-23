// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/BlockIntervalsSource.java

package intervals

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockIntervalsSource returns intervals where sub-source intervals appear
// consecutively, with no gaps (phrase-like semantics).
//
// Mirrors org.apache.lucene.queries.intervals.BlockIntervalsSource.
type BlockIntervalsSource struct {
	*ConjunctionIntervalsSource
	subSrcs []IntervalsSource
}

// BuildBlockIntervalsSource builds a block interval source (phrase).
// Flattens nested BlockIntervalSources and pulls up disjunctions.
func BuildBlockIntervalsSource(subSources []IntervalsSource) IntervalsSource {
	if len(subSources) == 1 {
		return subSources[0]
	}
	// Pull up disjunctions first, then wrap each resulting list.
	pulled := disjunctionsPullUp(subSources, func(srcs []IntervalsSource) IntervalsSource {
		return newBlockIntervalsSource(srcs)
	})
	return NewDisjunctionIntervalsSource(pulled, true)
}

func flattenBlock(sources []IntervalsSource) []IntervalsSource {
	var out []IntervalsSource
	for _, s := range sources {
		if b, ok := s.(*BlockIntervalsSource); ok {
			out = append(out, flattenBlock(b.subSrcs)...)
		} else {
			out = append(out, s)
		}
	}
	return out
}

func newBlockIntervalsSource(subSrcs []IntervalsSource) *BlockIntervalsSource {
	flattened := flattenBlock(subSrcs)
	s := &BlockIntervalsSource{subSrcs: flattened}
	s.ConjunctionIntervalsSource = NewConjunctionIntervalsSource(
		flattened,
		func(iters []IntervalIterator) IntervalIterator {
			return newBlockIntervalIterator(iters)
		},
	)
	return s
}

// Intervals delegates to embedded source.
func (s *BlockIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	return s.ConjunctionIntervalsSource.Intervals(field, ctx)
}

// Matches delegates to embedded source.
func (s *BlockIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	return s.ConjunctionIntervalsSource.Matches(field, ctx, doc)
}

// Visit visits sub-sources.
func (s *BlockIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	parent := NewIntervalQuery(field, s)
	v := visitor.GetSubVisitor(search.MUST, parent)
	for _, src := range s.subSrcs {
		src.Visit(field, v)
	}
}

// MinExtent returns the sum of sub-source min extents.
func (s *BlockIntervalsSource) MinExtent() int {
	total := 0
	for _, src := range s.subSrcs {
		total += src.MinExtent()
	}
	return total
}

// PullUpDisjunctions returns singleton — disjunctions already handled in build.
func (s *BlockIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// Equals reports structural equality.
func (s *BlockIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*BlockIntervalsSource)
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
func (s *BlockIntervalsSource) HashCode() int {
	h := 17
	for _, src := range s.subSrcs {
		h = h*31 + src.HashCode()
	}
	return h
}

// String returns a human-readable representation.
func (s *BlockIntervalsSource) String() string {
	parts := make([]string, len(s.subSrcs))
	for i, src := range s.subSrcs {
		parts[i] = src.String()
	}
	return "BLOCK(" + strings.Join(parts, ",") + ")"
}

// ── blockIntervalIterator ──────────────────────────────────────────────────

// blockIntervalIterator iterates over phrase-like (zero-gap) intervals.
// Mirrors BlockIntervalsSource.BlockIntervalIterator.
type blockIntervalIterator struct {
	*ConjunctionIntervalIterator
	start int
	end   int
}

func newBlockIntervalIterator(iters []IntervalIterator) *blockIntervalIterator {
	b := &blockIntervalIterator{start: -1, end: -1}
	b.ConjunctionIntervalIterator = NewConjunctionIntervalIterator(iters, func() error {
		b.start = -1
		b.end = -1
		return nil
	})
	return b
}

func (b *blockIntervalIterator) Start() int { return b.start }
func (b *blockIntervalIterator) End() int   { return b.end }
func (b *blockIntervalIterator) Gaps() int  { return 0 }

func (b *blockIntervalIterator) NextInterval() (int, error) {
	n := len(b.SubIterators)
	if n == 0 {
		return NoMoreIntervals, nil
	}
	// Advance the first sub-iterator.
	next, err := b.SubIterators[0].NextInterval()
	if err != nil {
		return 0, err
	}
	if next == NoMoreIntervals {
		b.start = NoMoreIntervals
		b.end = NoMoreIntervals
		return NoMoreIntervals, nil
	}
	i := 1
	for i < n {
		cur := b.SubIterators[i]
		prev := b.SubIterators[i-1]
		// Advance cur until it starts after prev ends.
		for cur.Start() <= prev.End() {
			next, err = cur.NextInterval()
			if err != nil {
				return 0, err
			}
			if next == NoMoreIntervals {
				b.start = NoMoreIntervals
				b.end = NoMoreIntervals
				return NoMoreIntervals, nil
			}
		}
		// Check adjacency.
		if cur.Start() == prev.End()+1 {
			i++
		} else {
			// Not adjacent — restart from first iterator.
			next, err = b.SubIterators[0].NextInterval()
			if err != nil {
				return 0, err
			}
			if next == NoMoreIntervals {
				b.start = NoMoreIntervals
				b.end = NoMoreIntervals
				return NoMoreIntervals, nil
			}
			i = 1
		}
	}
	b.start = b.SubIterators[0].Start()
	b.end = b.SubIterators[n-1].End()
	return b.start, nil
}

var _ search.DocIdSetIterator = (*blockIntervalIterator)(nil)
