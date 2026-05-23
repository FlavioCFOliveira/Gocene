// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/ConjunctionIntervalsSource.java
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/MinimizingConjunctionIntervalsSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MatchCallback is a callback invoked when an interval iterator finds a match.
// Mirrors MinimizingConjunctionIntervalsSource.MatchCallback.
type MatchCallback func() error

// noOpMatchCallback is a MatchCallback that does nothing.
var noOpMatchCallback MatchCallback = func() error { return nil }

// cacheIteratorsCallback returns a MatchCallback that caches all CachingMatchesIterators.
func cacheIteratorsCallback(its []*CachingMatchesIterator) MatchCallback {
	return func() error {
		for _, it := range its {
			if err := it.Cache(); err != nil {
				return err
			}
		}
		return nil
	}
}

// conjunctionIntervalsSourceBase holds sub-sources and visit/list helpers.
// Used by both ConjunctionIntervalsSource and MinimizingConjunctionIntervalsSource.
type conjunctionIntervalsSourceBase struct {
	subSources []IntervalsSource
}

func (b *conjunctionIntervalsSourceBase) Visit(field string, visitor search.QueryVisitor) {
	parent := NewIntervalQuery(field, nil) // placeholder — not used by visitor
	_ = parent
	v := visitor.GetSubVisitor(search.MUST, nil)
	for _, src := range b.subSources {
		src.Visit(field, v)
	}
}

func (b *conjunctionIntervalsSourceBase) visitWithParent(field string, visitor search.QueryVisitor, parent search.Query) {
	v := visitor.GetSubVisitor(search.MUST, parent)
	for _, src := range b.subSources {
		src.Visit(field, v)
	}
}

func (b *conjunctionIntervalsSourceBase) buildIterators(field string, ctx *index.LeafReaderContext) ([]IntervalIterator, error) {
	out := make([]IntervalIterator, 0, len(b.subSources))
	for _, src := range b.subSources {
		it, err := src.Intervals(field, ctx)
		if err != nil {
			return nil, err
		}
		if it == nil {
			return nil, nil
		}
		out = append(out, it)
	}
	return out, nil
}

// ── ConjunctionIntervalsSource ─────────────────────────────────────────────

// ConjunctionIntervalsSource is the abstract base for conjunction interval sources
// that combine sub-sources with a simple combine function (no minimization).
//
// Mirrors org.apache.lucene.queries.intervals.ConjunctionIntervalsSource (abstract).
//
// Deviations from Java:
//   - combineFn replaces the abstract combine method.
type ConjunctionIntervalsSource struct {
	conjunctionIntervalsSourceBase
	combineFn func(iters []IntervalIterator) IntervalIterator
}

// NewConjunctionIntervalsSource creates a ConjunctionIntervalsSource.
func NewConjunctionIntervalsSource(subSources []IntervalsSource, combineFn func([]IntervalIterator) IntervalIterator) *ConjunctionIntervalsSource {
	return &ConjunctionIntervalsSource{
		conjunctionIntervalsSourceBase: conjunctionIntervalsSourceBase{subSources: subSources},
		combineFn: combineFn,
	}
}

// Intervals creates an IntervalIterator from all sub-sources.
func (s *ConjunctionIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	iters, err := s.buildIterators(field, ctx)
	if err != nil || iters == nil {
		return nil, err
	}
	return s.combineFn(iters), nil
}

// Matches creates an IntervalMatchesIterator for the given doc.
func (s *ConjunctionIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	subs := make([]IntervalMatchesIterator, 0, len(s.subSources))
	for _, src := range s.subSources {
		mi, err := src.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, nil
		}
		subs = append(subs, mi)
	}
	wraps := make([]IntervalIterator, len(subs))
	for i, mi := range subs {
		wraps[i] = WrapMatches(mi, doc)
	}
	it := s.combineFn(wraps)
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
	return NewConjunctionMatchesIterator(it, subs), nil
}

// ── MinimizingConjunctionIntervalsSource ──────────────────────────────────

// MinimizingConjunctionIntervalsSource is the abstract base for conjunction
// interval sources that minimize their intervals using a MatchCallback.
//
// Mirrors org.apache.lucene.queries.intervals.MinimizingConjunctionIntervalsSource (abstract).
//
// Deviations from Java:
//   - combineFn replaces the abstract combine method.
type MinimizingConjunctionIntervalsSource struct {
	conjunctionIntervalsSourceBase
	combineFn func(iters []IntervalIterator, onMatch MatchCallback) IntervalIterator
}

// NewMinimizingConjunctionIntervalsSource creates a MinimizingConjunctionIntervalsSource.
func NewMinimizingConjunctionIntervalsSource(
	subSources []IntervalsSource,
	combineFn func([]IntervalIterator, MatchCallback) IntervalIterator,
) *MinimizingConjunctionIntervalsSource {
	return &MinimizingConjunctionIntervalsSource{
		conjunctionIntervalsSourceBase: conjunctionIntervalsSourceBase{subSources: subSources},
		combineFn: combineFn,
	}
}

// Intervals creates an IntervalIterator from all sub-sources (no caching).
func (s *MinimizingConjunctionIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	iters, err := s.buildIterators(field, ctx)
	if err != nil || iters == nil {
		return nil, err
	}
	return s.combineFn(iters, noOpMatchCallback), nil
}

// Matches creates an IntervalMatchesIterator for the given doc (with caching).
func (s *MinimizingConjunctionIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	cachingSubs := make([]*CachingMatchesIterator, 0, len(s.subSources))
	for _, src := range s.subSources {
		mi, err := src.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, nil
		}
		cachingSubs = append(cachingSubs, NewCachingMatchesIterator(mi))
	}
	wraps := make([]IntervalIterator, len(cachingSubs))
	for i, cmi := range cachingSubs {
		wraps[i] = WrapMatches(cmi, doc)
	}
	onMatch := cacheIteratorsCallback(cachingSubs)
	it := s.combineFn(wraps, onMatch)
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
	// Wrap caching subs as IntervalMatchesIterator for the result.
	subs := make([]IntervalMatchesIterator, len(cachingSubs))
	for i, c := range cachingSubs {
		subs[i] = c
	}
	return NewConjunctionMatchesIterator(it, subs), nil
}
