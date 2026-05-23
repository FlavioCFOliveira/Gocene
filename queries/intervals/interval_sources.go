// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0 (intervals package):
// This file contains concrete IntervalsSource implementations for:
//   - NonOverlappingIntervalsSource
//   - NotContainingIntervalsSource
//   - NotContainedByIntervalsSource
//   - ContainingIntervalsSource
//   - ContainedByIntervalsSource
//   - OverlappingIntervalsSource
//   - OffsetIntervalsSource
//   - ExtendedIntervalsSource
//   - FixedFieldIntervalsSource  (in separate file)
//   - NoMatchIntervalsSource     (in separate file)
//
// Deviations from Java:
//   - All inner iterator classes are promoted to package-level named types.
//   - FilteringIntervalIterator nextIntervalFn delegates are implemented as closures.

package intervals

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ── NonOverlappingIntervalsSource ──────────────────────────────────────────

// NonOverlappingIntervalsSource returns intervals from minuend that do not
// overlap any interval from subtrahend.
//
// Mirrors org.apache.lucene.queries.intervals.NonOverlappingIntervalsSource.
type NonOverlappingIntervalsSource struct {
	*DifferenceIntervalsSource
}

// NewNonOverlappingIntervalsSource constructs a NonOverlappingIntervalsSource.
func NewNonOverlappingIntervalsSource(minuend, subtrahend IntervalsSource) *NonOverlappingIntervalsSource {
	s := &NonOverlappingIntervalsSource{}
	s.DifferenceIntervalsSource = NewDifferenceIntervalsSource(minuend, subtrahend, func(min, sub IntervalIterator) IntervalIterator {
		r := NewRelativeIterator(min, sub, nil)
		r.nextIntervalFn = func() (int, error) {
			if !r.Bpos {
				return r.A.NextInterval()
			}
			for {
				next, err := r.A.NextInterval()
				if err != nil || next == NoMoreIntervals {
					return next, err
				}
				for r.B.End() < r.A.Start() {
					bNext, err := r.B.NextInterval()
					if err != nil {
						return 0, err
					}
					if bNext == NoMoreIntervals {
						r.Bpos = false
						return r.A.Start(), nil
					}
				}
				if r.B.Start() > r.A.End() {
					return r.A.Start(), nil
				}
			}
		}
		return r
	})
	return s
}

func (s *NonOverlappingIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*NonOverlappingIntervalsSource)
	return ok && s.Minuend.Equals(o.Minuend) && s.Subtrahend.Equals(o.Subtrahend)
}
func (s *NonOverlappingIntervalsSource) HashCode() int {
	return s.Minuend.HashCode()*31 ^ s.Subtrahend.HashCode()
}
func (s *NonOverlappingIntervalsSource) String() string {
	return "NON_OVERLAPPING(" + s.Minuend.String() + "," + s.Subtrahend.String() + ")"
}
func (s *NonOverlappingIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// ── NotContainingIntervalsSource ───────────────────────────────────────────

// NotContainingIntervalsSource returns minuend intervals that do not contain
// any interval from subtrahend.
//
// Mirrors org.apache.lucene.queries.intervals.NotContainingIntervalsSource.
type NotContainingIntervalsSource struct {
	*DifferenceIntervalsSource
}

// NewNotContainingIntervalsSource constructs a NotContainingIntervalsSource.
func NewNotContainingIntervalsSource(minuend, subtrahend IntervalsSource) *NotContainingIntervalsSource {
	s := &NotContainingIntervalsSource{}
	s.DifferenceIntervalsSource = NewDifferenceIntervalsSource(minuend, subtrahend, func(min, sub IntervalIterator) IntervalIterator {
		r := NewRelativeIterator(min, sub, nil)
		r.nextIntervalFn = func() (int, error) {
			if !r.Bpos {
				return r.A.NextInterval()
			}
			for {
				next, err := r.A.NextInterval()
				if err != nil || next == NoMoreIntervals {
					return next, err
				}
				for r.B.Start() < r.A.Start() && r.B.End() < r.A.End() {
					bNext, err := r.B.NextInterval()
					if err != nil {
						return 0, err
					}
					if bNext == NoMoreIntervals {
						r.Bpos = false
						return r.A.Start(), nil
					}
				}
				if r.B.Start() < r.A.Start() || r.B.End() > r.A.End() {
					return r.A.Start(), nil
				}
			}
		}
		return r
	})
	return s
}

func (s *NotContainingIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*NotContainingIntervalsSource)
	return ok && s.Minuend.Equals(o.Minuend) && s.Subtrahend.Equals(o.Subtrahend)
}
func (s *NotContainingIntervalsSource) HashCode() int {
	return s.Minuend.HashCode()*31 ^ s.Subtrahend.HashCode()
}
func (s *NotContainingIntervalsSource) String() string {
	return "NOT_CONTAINING(" + s.Minuend.String() + "," + s.Subtrahend.String() + ")"
}
func (s *NotContainingIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// ── NotContainedByIntervalsSource ──────────────────────────────────────────

// NotContainedByIntervalsSource returns minuend intervals that are not contained
// by any interval from subtrahend.
//
// Mirrors org.apache.lucene.queries.intervals.NotContainedByIntervalsSource.
type NotContainedByIntervalsSource struct {
	*DifferenceIntervalsSource
}

// NewNotContainedByIntervalsSource constructs a NotContainedByIntervalsSource.
func NewNotContainedByIntervalsSource(minuend, subtrahend IntervalsSource) *NotContainedByIntervalsSource {
	s := &NotContainedByIntervalsSource{}
	s.DifferenceIntervalsSource = NewDifferenceIntervalsSource(minuend, subtrahend, func(min, sub IntervalIterator) IntervalIterator {
		r := NewRelativeIterator(min, sub, nil)
		r.nextIntervalFn = func() (int, error) {
			if !r.Bpos {
				return r.A.NextInterval()
			}
			for {
				next, err := r.A.NextInterval()
				if err != nil || next == NoMoreIntervals {
					return next, err
				}
				for r.B.End() < r.A.End() {
					bNext, err := r.B.NextInterval()
					if err != nil {
						return 0, err
					}
					if bNext == NoMoreIntervals {
						return r.A.Start(), nil
					}
				}
				if r.A.Start() < r.B.Start() {
					return r.A.Start(), nil
				}
			}
		}
		return r
	})
	return s
}

func (s *NotContainedByIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*NotContainedByIntervalsSource)
	return ok && s.Minuend.Equals(o.Minuend) && s.Subtrahend.Equals(o.Subtrahend)
}
func (s *NotContainedByIntervalsSource) HashCode() int {
	return s.Minuend.HashCode()*31 ^ s.Subtrahend.HashCode()
}
func (s *NotContainedByIntervalsSource) String() string {
	return "NOT_CONTAINED_BY(" + s.Minuend.String() + "," + s.Subtrahend.String() + ")"
}
func (s *NotContainedByIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

// ── BaseConjunctionIntervalsSource ─────────────────────────────────────────

// baseConjunctionIntervalsSource is the base type for conjunction-based sources.
type baseConjunctionIntervalsSource struct {
	subSources []IntervalsSource
}

func (b *baseConjunctionIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	for _, s := range b.subSources {
		s.Visit(field, visitor)
	}
}

func (b *baseConjunctionIntervalsSource) intervalsList(field string, ctx *index.LeafReaderContext) ([]IntervalIterator, error) {
	out := make([]IntervalIterator, 0, len(b.subSources))
	for _, s := range b.subSources {
		it, err := s.Intervals(field, ctx)
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

func (b *baseConjunctionIntervalsSource) matchesList(field string, ctx *index.LeafReaderContext, doc int) ([]IntervalMatchesIterator, error) {
	out := make([]IntervalMatchesIterator, 0, len(b.subSources))
	for _, s := range b.subSources {
		mi, err := s.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, nil
		}
		out = append(out, mi)
	}
	return out, nil
}

// ── ContainingIntervalsSource ──────────────────────────────────────────────

// ContainingIntervalsSource returns intervals from big that contain at least
// one interval from small.
//
// Mirrors org.apache.lucene.queries.intervals.ContainingIntervalsSource.
type ContainingIntervalsSource struct {
	baseConjunctionIntervalsSource
	big   IntervalsSource
	small IntervalsSource
}

// NewContainingIntervalsSource constructs a ContainingIntervalsSource.
func NewContainingIntervalsSource(big, small IntervalsSource) *ContainingIntervalsSource {
	return &ContainingIntervalsSource{
		baseConjunctionIntervalsSource: baseConjunctionIntervalsSource{subSources: []IntervalsSource{big, small}},
		big:   big,
		small: small,
	}
}

func (s *ContainingIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	its, err := s.intervalsList(field, ctx)
	if err != nil || its == nil {
		return nil, err
	}
	a, b := its[0], its[1]
	fi, err := NewFilteringIntervalIterator(a, b, nil)
	if err != nil {
		return nil, err
	}
	fi.nextIntervalFn = func() (int, error) {
		if !fi.Bpos {
			return NoMoreIntervals, nil
		}
		for {
			next, err := fi.A.NextInterval()
			if err != nil || next == NoMoreIntervals {
				return next, err
			}
			for fi.B.Start() < fi.A.Start() && fi.B.End() < fi.A.End() {
				bNext, err := fi.B.NextInterval()
				if err != nil {
					return 0, err
				}
				if bNext == NoMoreIntervals {
					fi.Bpos = false
					return NoMoreIntervals, nil
				}
			}
			if fi.A.Start() <= fi.B.Start() && fi.A.End() >= fi.B.End() {
				return fi.A.Start(), nil
			}
		}
	}
	return fi, nil
}

func (s *ContainingIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mis, err := s.matchesList(field, ctx, doc)
	if err != nil || mis == nil {
		return nil, err
	}
	it, err := s.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	cmi := NewConjunctionMatchesIterator(it, mis)
	return AsMatches(it, cmi, doc)
}

func (s *ContainingIntervalsSource) MinExtent() int { return s.big.MinExtent() }
func (s *ContainingIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}
func (s *ContainingIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*ContainingIntervalsSource)
	return ok && s.big.Equals(o.big) && s.small.Equals(o.small)
}
func (s *ContainingIntervalsSource) HashCode() int {
	return s.big.HashCode()*31 ^ s.small.HashCode()
}
func (s *ContainingIntervalsSource) String() string {
	return "CONTAINING(" + s.big.String() + "," + s.small.String() + ")"
}

// ── ContainedByIntervalsSource ─────────────────────────────────────────────

// ContainedByIntervalsSource returns intervals from small that are contained
// by at least one interval from big.
//
// Mirrors org.apache.lucene.queries.intervals.ContainedByIntervalsSource.
type ContainedByIntervalsSource struct {
	baseConjunctionIntervalsSource
	small IntervalsSource
	big   IntervalsSource
}

// NewContainedByIntervalsSource constructs a ContainedByIntervalsSource.
func NewContainedByIntervalsSource(small, big IntervalsSource) *ContainedByIntervalsSource {
	return &ContainedByIntervalsSource{
		baseConjunctionIntervalsSource: baseConjunctionIntervalsSource{subSources: []IntervalsSource{small, big}},
		small: small,
		big:   big,
	}
}

func (s *ContainedByIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	its, err := s.intervalsList(field, ctx)
	if err != nil || its == nil {
		return nil, err
	}
	a, b := its[0], its[1]
	fi, err := NewFilteringIntervalIterator(a, b, nil)
	if err != nil {
		return nil, err
	}
	fi.nextIntervalFn = func() (int, error) {
		if !fi.Bpos {
			return NoMoreIntervals, nil
		}
		for {
			next, err := fi.A.NextInterval()
			if err != nil || next == NoMoreIntervals {
				return next, err
			}
			for fi.B.End() < fi.A.End() {
				bNext, err := fi.B.NextInterval()
				if err != nil {
					return 0, err
				}
				if bNext == NoMoreIntervals {
					fi.Bpos = false
					return NoMoreIntervals, nil
				}
			}
			if fi.B.Start() <= fi.A.Start() {
				return fi.A.Start(), nil
			}
		}
	}
	return fi, nil
}

func (s *ContainedByIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mis, err := s.matchesList(field, ctx, doc)
	if err != nil || mis == nil {
		return nil, err
	}
	it, err := s.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	// only the "small" sub is relevant for matches
	cmi := NewConjunctionMatchesIterator(it, mis[:1])
	return AsMatches(it, cmi, doc)
}

func (s *ContainedByIntervalsSource) MinExtent() int { return s.small.MinExtent() }
func (s *ContainedByIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}
func (s *ContainedByIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*ContainedByIntervalsSource)
	return ok && s.small.Equals(o.small) && s.big.Equals(o.big)
}
func (s *ContainedByIntervalsSource) HashCode() int {
	return s.small.HashCode()*31 ^ s.big.HashCode()
}
func (s *ContainedByIntervalsSource) String() string {
	return "CONTAINED_BY(" + s.small.String() + "," + s.big.String() + ")"
}

// ── OverlappingIntervalsSource ─────────────────────────────────────────────

// OverlappingIntervalsSource returns intervals from source that overlap at
// least one interval from reference.
//
// Mirrors org.apache.lucene.queries.intervals.OverlappingIntervalsSource.
type OverlappingIntervalsSource struct {
	baseConjunctionIntervalsSource
	source    IntervalsSource
	reference IntervalsSource
}

// NewOverlappingIntervalsSource constructs an OverlappingIntervalsSource.
func NewOverlappingIntervalsSource(source, reference IntervalsSource) *OverlappingIntervalsSource {
	return &OverlappingIntervalsSource{
		baseConjunctionIntervalsSource: baseConjunctionIntervalsSource{subSources: []IntervalsSource{source, reference}},
		source:    source,
		reference: reference,
	}
}

func (s *OverlappingIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	its, err := s.intervalsList(field, ctx)
	if err != nil || its == nil {
		return nil, err
	}
	a, b := its[0], its[1]
	fi, err := NewFilteringIntervalIterator(a, b, nil)
	if err != nil {
		return nil, err
	}
	fi.nextIntervalFn = func() (int, error) {
		if !fi.Bpos {
			return NoMoreIntervals, nil
		}
		for {
			next, err := fi.A.NextInterval()
			if err != nil || next == NoMoreIntervals {
				fi.Bpos = false
				return next, err
			}
			for fi.B.End() < fi.A.Start() {
				bNext, err := fi.B.NextInterval()
				if err != nil {
					return 0, err
				}
				if bNext == NoMoreIntervals {
					fi.Bpos = false
					return NoMoreIntervals, nil
				}
			}
			if fi.B.Start() <= fi.A.End() {
				return fi.A.Start(), nil
			}
		}
	}
	return fi, nil
}

func (s *OverlappingIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mis, err := s.matchesList(field, ctx, doc)
	if err != nil || mis == nil {
		return nil, err
	}
	it, err := s.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	cmi := NewConjunctionMatchesIterator(it, mis)
	return AsMatches(it, cmi, doc)
}

func (s *OverlappingIntervalsSource) MinExtent() int { return s.source.MinExtent() }
func (s *OverlappingIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}
func (s *OverlappingIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*OverlappingIntervalsSource)
	return ok && s.source.Equals(o.source) && s.reference.Equals(o.reference)
}
func (s *OverlappingIntervalsSource) HashCode() int {
	return s.source.HashCode()*31 ^ s.reference.HashCode()
}
func (s *OverlappingIntervalsSource) String() string {
	return "OVERLAPPING(" + s.source.String() + "," + s.reference.String() + ")"
}

// ── OffsetIntervalsSource ──────────────────────────────────────────────────

// OffsetIntervalsSource tracks a reference source and produces pseudo-intervals
// that appear one position before or after each reference interval.
//
// Mirrors org.apache.lucene.queries.intervals.OffsetIntervalsSource.
type OffsetIntervalsSource struct {
	in     IntervalsSource
	before bool
}

// NewOffsetIntervalsSource constructs an OffsetIntervalsSource.
// If before is true, the produced interval precedes each reference interval;
// if false, it follows.
func NewOffsetIntervalsSource(in IntervalsSource, before bool) *OffsetIntervalsSource {
	return &OffsetIntervalsSource{in: in, before: before}
}

func (s *OffsetIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	it, err := s.in.Intervals(field, ctx)
	if err != nil || it == nil {
		return nil, err
	}
	return newOffsetIntervalIterator(it, s.before), nil
}

func (s *OffsetIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mi, err := s.in.Matches(field, ctx, doc)
	if err != nil || mi == nil {
		return nil, err
	}
	it := newOffsetIntervalIterator(WrapMatches(mi, doc), s.before)
	return AsMatches(it, mi, doc)
}

func (s *OffsetIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	s.in.Visit(field, visitor)
}
func (s *OffsetIntervalsSource) MinExtent() int { return 1 }
func (s *OffsetIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}
func (s *OffsetIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*OffsetIntervalsSource)
	return ok && s.in.Equals(o.in) && s.before == o.before
}
func (s *OffsetIntervalsSource) HashCode() int {
	h := s.in.HashCode()
	if s.before {
		h ^= 1
	}
	return h
}
func (s *OffsetIntervalsSource) String() string {
	dir := "AFTER"
	if s.before {
		dir = "BEFORE"
	}
	return "OFFSET_" + dir + "(" + s.in.String() + ")"
}

type offsetIntervalIterator struct {
	in     IntervalIterator
	before bool
}

func newOffsetIntervalIterator(in IntervalIterator, before bool) *offsetIntervalIterator {
	return &offsetIntervalIterator{in: in, before: before}
}

func (o *offsetIntervalIterator) DocID() int             { return o.in.DocID() }
func (o *offsetIntervalIterator) DocIDRunEnd() int        { return o.DocID() + 1 }
func (o *offsetIntervalIterator) Cost() int64            { return o.in.Cost() }
func (o *offsetIntervalIterator) MatchCost() float32     { return o.in.MatchCost() }
func (o *offsetIntervalIterator) Gaps() int              { return 0 }
func (o *offsetIntervalIterator) Width() int             { return 1 }
func (o *offsetIntervalIterator) NextDoc() (int, error)  { return o.in.NextDoc() }
func (o *offsetIntervalIterator) Advance(target int) (int, error) { return o.in.Advance(target) }
func (o *offsetIntervalIterator) NextInterval() (int, error) {
	next, err := o.in.NextInterval()
	if err != nil || next == NoMoreIntervals {
		return next, err
	}
	return o.Start(), nil
}
func (o *offsetIntervalIterator) Start() int {
	s := o.in.Start()
	if s == -1 {
		return -1
	}
	if s == NoMoreIntervals {
		return NoMoreIntervals
	}
	if o.before {
		if s == 0 {
			return 0
		}
		return s - 1
	}
	return o.in.End() + 1
}
func (o *offsetIntervalIterator) End() int {
	e := o.in.End()
	if e == -1 {
		return -1
	}
	if e == NoMoreIntervals {
		return NoMoreIntervals
	}
	if o.before {
		if o.in.Start() == 0 {
			return 0
		}
		return o.in.Start() - 1
	}
	return e + 1
}

var _ search.DocIdSetIterator = (*offsetIntervalIterator)(nil)

// ── ExtendedIntervalsSource ────────────────────────────────────────────────

// ExtendedIntervalsSource wraps another IntervalsSource and extends the bounds
// of its intervals by a fixed number of positions.
//
// Mirrors org.apache.lucene.queries.intervals.ExtendedIntervalsSource.
type ExtendedIntervalsSource struct {
	source IntervalsSource
	before int
	after  int
}

// NewExtendedIntervalsSource constructs an ExtendedIntervalsSource.
func NewExtendedIntervalsSource(source IntervalsSource, before, after int) *ExtendedIntervalsSource {
	return &ExtendedIntervalsSource{source: source, before: before, after: after}
}

func (s *ExtendedIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	it, err := s.source.Intervals(field, ctx)
	if err != nil || it == nil {
		return nil, err
	}
	return NewExtendedIntervalIterator(it, s.before, s.after), nil
}

func (s *ExtendedIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mi, err := s.source.Matches(field, ctx, doc)
	if err != nil || mi == nil {
		return nil, err
	}
	// Build a no-offset wrapper around the matches for inner positions
	innerWrap := &passthruMatchesIterator{inner: mi}
	it := NewExtendedIntervalIterator(WrapMatches(innerWrap, doc), s.before, s.after)
	return AsMatches(it, mi, doc)
}

func (s *ExtendedIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	s.source.Visit(field, visitor)
}
func (s *ExtendedIntervalsSource) MinExtent() int { return 1 }
func (s *ExtendedIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	inner := s.source.PullUpDisjunctions()
	out := make([]IntervalsSource, len(inner))
	for i, src := range inner {
		out[i] = NewExtendedIntervalsSource(src, s.before, s.after)
	}
	return out
}
func (s *ExtendedIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*ExtendedIntervalsSource)
	return ok && s.source.Equals(o.source) && s.before == o.before && s.after == o.after
}
func (s *ExtendedIntervalsSource) HashCode() int {
	return s.source.HashCode()*31 + s.before*7 + s.after
}
func (s *ExtendedIntervalsSource) String() string {
	return fmt.Sprintf("EXTEND(%s,%d,%d)", s.source.String(), s.before, s.after)
}

type passthruMatchesIterator struct {
	inner IntervalMatchesIterator
}

func (p *passthruMatchesIterator) Next() (bool, error) { return p.inner.Next() }
func (p *passthruMatchesIterator) StartPosition() int  { return p.inner.StartPosition() }
func (p *passthruMatchesIterator) EndPosition() int    { return p.inner.EndPosition() }
func (p *passthruMatchesIterator) StartOffset() (int, error) { return -1, nil }
func (p *passthruMatchesIterator) EndOffset() (int, error)   { return -1, nil }
func (p *passthruMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return p.inner.GetSubMatches()
}
func (p *passthruMatchesIterator) GetQuery() search.Query { return p.inner.GetQuery() }
func (p *passthruMatchesIterator) Gaps() int              { return p.inner.Gaps() }
func (p *passthruMatchesIterator) Width() int             { return p.inner.Width() }

// ── RepeatingIntervalsSource ───────────────────────────────────────────────

// RepeatingIntervalsSource generates an iterator spanning repeating instances
// of a sub-iterator, useful for repeated terms within an unordered interval.
//
// Mirrors org.apache.lucene.queries.intervals.RepeatingIntervalsSource.
type RepeatingIntervalsSource struct {
	in         IntervalsSource
	childCount int
	name       string
}

// NewRepeatingIntervalsSource creates a RepeatingIntervalsSource.
// If childCount == 1, returns in directly.
func NewRepeatingIntervalsSource(in IntervalsSource, childCount int) IntervalsSource {
	if childCount == 1 {
		return in
	}
	return &RepeatingIntervalsSource{in: in, childCount: childCount}
}

// SetName sets an optional display name for debugging.
func (s *RepeatingIntervalsSource) SetName(name string) { s.name = name }

func (s *RepeatingIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	it, err := s.in.Intervals(field, ctx)
	if err != nil || it == nil {
		return nil, err
	}
	return &duplicateIntervalIterator{in: it, count: s.childCount}, nil
}

func (s *RepeatingIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	subs := make([]IntervalMatchesIterator, s.childCount)
	for i := 0; i < s.childCount; i++ {
		mi, err := s.in.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi == nil {
			return nil, nil
		}
		subs[i] = mi
	}
	it, err := s.Intervals(field, ctx)
	if err != nil {
		return nil, err
	}
	cmi := NewConjunctionMatchesIterator(it, subs)
	return AsMatches(it, cmi, doc)
}

func (s *RepeatingIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	s.in.Visit(field, visitor)
}
func (s *RepeatingIntervalsSource) MinExtent() int { return s.in.MinExtent() * s.childCount }
func (s *RepeatingIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}
func (s *RepeatingIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*RepeatingIntervalsSource)
	return ok && s.in.Equals(o.in) && s.childCount == o.childCount
}
func (s *RepeatingIntervalsSource) HashCode() int {
	return s.in.HashCode()*31 + s.childCount
}
func (s *RepeatingIntervalsSource) String() string {
	str := s.in.String()
	var buf strings.Builder
	buf.WriteString(str)
	for i := 1; i < s.childCount; i++ {
		buf.WriteByte(',')
		buf.WriteString(str)
	}
	if s.name != "" {
		return s.name + "(" + buf.String() + ")"
	}
	return buf.String()
}

// duplicateIntervalIterator is an iterator returning the same sub-iterator childCount times.
type duplicateIntervalIterator struct {
	in    IntervalIterator
	count int
	start int
	end   int
}

func (d *duplicateIntervalIterator) DocID() int             { return d.in.DocID() }
func (d *duplicateIntervalIterator) DocIDRunEnd() int        { return d.DocID() + 1 }
func (d *duplicateIntervalIterator) Cost() int64            { return d.in.Cost() }
func (d *duplicateIntervalIterator) MatchCost() float32     { return d.in.MatchCost() * float32(d.count) }
func (d *duplicateIntervalIterator) Start() int             { return d.start }
func (d *duplicateIntervalIterator) End() int               { return d.end }
func (d *duplicateIntervalIterator) Gaps() int              { return d.count - 1 }
func (d *duplicateIntervalIterator) Width() int             { return d.end - d.start + 1 }
func (d *duplicateIntervalIterator) NextDoc() (int, error)  { return d.in.NextDoc() }
func (d *duplicateIntervalIterator) Advance(t int) (int, error) { return d.in.Advance(t) }
func (d *duplicateIntervalIterator) NextInterval() (int, error) {
	next, err := d.in.NextInterval()
	if err != nil || next == NoMoreIntervals {
		return next, err
	}
	d.start = d.in.Start()
	d.end = d.in.End()
	return d.start, nil
}

var _ search.DocIdSetIterator = (*duplicateIntervalIterator)(nil)

// ── DisjunctionIntervalsSource ─────────────────────────────────────────────

// DisjunctionIntervalsSource returns intervals from any of the provided sub-sources.
//
// Mirrors org.apache.lucene.queries.intervals.DisjunctionIntervalsSource.
type DisjunctionIntervalsSource struct {
	subSources        []IntervalsSource
	pullUpDisjunctions bool
}

// NewDisjunctionIntervalsSource constructs a DisjunctionIntervalsSource.
func NewDisjunctionIntervalsSource(subSources []IntervalsSource, pullUpDisjunctions bool) IntervalsSource {
	simplified := simplifyDisjunctions(subSources)
	if len(simplified) == 1 {
		return simplified[0]
	}
	return &DisjunctionIntervalsSource{subSources: simplified, pullUpDisjunctions: pullUpDisjunctions}
}

func simplifyDisjunctions(sources []IntervalsSource) []IntervalsSource {
	seen := make(map[string]bool)
	var out []IntervalsSource
	for _, s := range sources {
		key := s.String()
		if !seen[key] {
			seen[key] = true
			out = append(out, s)
		}
	}
	return out
}

func (s *DisjunctionIntervalsSource) Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	iters := make([]IntervalIterator, 0, len(s.subSources))
	for _, src := range s.subSources {
		it, err := src.Intervals(field, ctx)
		if err != nil {
			return nil, err
		}
		if it != nil {
			iters = append(iters, it)
		}
	}
	if len(iters) == 0 {
		return nil, nil
	}
	if len(iters) == 1 {
		return iters[0], nil
	}
	return newDisjunctionIntervalIterator(iters), nil
}

func (s *DisjunctionIntervalsSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	mis := make([]IntervalMatchesIterator, 0, len(s.subSources))
	for _, src := range s.subSources {
		mi, err := src.Matches(field, ctx, doc)
		if err != nil {
			return nil, err
		}
		if mi != nil {
			mis = append(mis, mi)
		}
	}
	if len(mis) == 0 {
		return nil, nil
	}
	if len(mis) == 1 {
		return mis[0], nil
	}
	// return a merged matches iterator
	return &mergedMatchesIterator{subs: mis}, nil
}

func (s *DisjunctionIntervalsSource) Visit(field string, visitor search.QueryVisitor) {
	for _, src := range s.subSources {
		src.Visit(field, visitor)
	}
}
func (s *DisjunctionIntervalsSource) MinExtent() int {
	min := NoMoreIntervals
	for _, src := range s.subSources {
		if e := src.MinExtent(); e < min {
			min = e
		}
	}
	if min == NoMoreIntervals {
		return 0
	}
	return min
}
func (s *DisjunctionIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	if s.pullUpDisjunctions {
		return s.subSources
	}
	return []IntervalsSource{s}
}
func (s *DisjunctionIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*DisjunctionIntervalsSource)
	if !ok || len(s.subSources) != len(o.subSources) {
		return false
	}
	// Java uses a HashSet; equality is order-insensitive.
	set := make(map[string]bool, len(o.subSources))
	for _, src := range o.subSources {
		set[src.String()] = true
	}
	for _, src := range s.subSources {
		if !set[src.String()] {
			return false
		}
	}
	return true
}
func (s *DisjunctionIntervalsSource) HashCode() int {
	h := 17
	for _, src := range s.subSources {
		h = h*31 + src.HashCode()
	}
	return h
}
func (s *DisjunctionIntervalsSource) String() string {
	parts := make([]string, len(s.subSources))
	for i, src := range s.subSources {
		parts[i] = src.String()
	}
	return "OR(" + strings.Join(parts, ",") + ")"
}

// disjunctionIntervalIterator merges multiple interval iterators via a priority queue.
type disjunctionIntervalIterator struct {
	pq    *DisiPriorityQueue
	subs  []IntervalIterator
	start int
	end   int
}

func newDisjunctionIntervalIterator(iters []IntervalIterator) *disjunctionIntervalIterator {
	pq := NewDisiPriorityQueue(len(iters))
	for _, it := range iters {
		pq.Add(NewDisiWrapper(it))
	}
	return &disjunctionIntervalIterator{pq: pq, subs: iters, start: -1, end: -1}
}

func (d *disjunctionIntervalIterator) DocID() int { return d.pq.Top().Doc }
func (d *disjunctionIntervalIterator) DocIDRunEnd() int { return d.DocID() + 1 }
func (d *disjunctionIntervalIterator) Cost() int64 {
	var cost int64
	for _, w := range d.pq.All() {
		cost += w.Cost
	}
	return cost
}
func (d *disjunctionIntervalIterator) MatchCost() float32 {
	var cost float32
	for _, it := range d.subs {
		cost += it.MatchCost()
	}
	return cost
}
func (d *disjunctionIntervalIterator) Start() int { return d.start }
func (d *disjunctionIntervalIterator) End() int   { return d.end }
func (d *disjunctionIntervalIterator) Gaps() int  { return 0 }
func (d *disjunctionIntervalIterator) Width() int {
	if d.end == NoMoreIntervals {
		return NoMoreIntervals
	}
	return d.end - d.start + 1
}

func (d *disjunctionIntervalIterator) NextDoc() (int, error) {
	top := d.pq.Top()
	doc := top.Doc
	for {
		nextDoc, err := top.Approximation.NextDoc()
		if err != nil {
			return 0, err
		}
		top.Doc = nextDoc
		top = d.pq.UpdateTop()
		if top.Doc != doc {
			break
		}
	}
	d.start = -1
	d.end = -1
	return top.Doc, nil
}

func (d *disjunctionIntervalIterator) Advance(target int) (int, error) {
	top := d.pq.Top()
	for {
		nextDoc, err := top.Approximation.Advance(target)
		if err != nil {
			return 0, err
		}
		top.Doc = nextDoc
		top = d.pq.UpdateTop()
		if top.Doc >= target {
			break
		}
	}
	d.start = -1
	d.end = -1
	return top.Doc, nil
}

func (d *disjunctionIntervalIterator) NextInterval() (int, error) {
	// Advance all iterators at current doc to find minimum next interval
	best := NoMoreIntervals
	var bestIter IntervalIterator
	for _, it := range d.subs {
		if it.DocID() != d.DocID() {
			continue
		}
		next, err := it.NextInterval()
		if err != nil {
			return 0, err
		}
		if next < best {
			best = next
			bestIter = it
		}
	}
	if bestIter == nil || best == NoMoreIntervals {
		d.start = NoMoreIntervals
		d.end = NoMoreIntervals
		return NoMoreIntervals, nil
	}
	d.start = bestIter.Start()
	d.end = bestIter.End()
	return d.start, nil
}

var _ search.DocIdSetIterator = (*disjunctionIntervalIterator)(nil)

// mergedMatchesIterator chains multiple IntervalMatchesIterators.
type mergedMatchesIterator struct {
	subs []IntervalMatchesIterator
	idx  int
}

func (m *mergedMatchesIterator) Next() (bool, error) {
	for m.idx < len(m.subs) {
		ok, err := m.subs[m.idx].Next()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		m.idx++
	}
	return false, nil
}
func (m *mergedMatchesIterator) StartPosition() int {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].StartPosition()
	}
	return -1
}
func (m *mergedMatchesIterator) EndPosition() int {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].EndPosition()
	}
	return -1
}
func (m *mergedMatchesIterator) StartOffset() (int, error) {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].StartOffset()
	}
	return -1, nil
}
func (m *mergedMatchesIterator) EndOffset() (int, error) {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].EndOffset()
	}
	return -1, nil
}
func (m *mergedMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].GetSubMatches()
	}
	return nil, nil
}
func (m *mergedMatchesIterator) GetQuery() search.Query {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].GetQuery()
	}
	return nil
}
func (m *mergedMatchesIterator) Gaps() int {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].Gaps()
	}
	return 0
}
func (m *mergedMatchesIterator) Width() int {
	if m.idx < len(m.subs) {
		return m.subs[m.idx].Width()
	}
	return 0
}
