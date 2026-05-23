// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/ConjunctionMatchesIterator.java

package intervals

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ConjunctionMatchesIterator implements IntervalMatchesIterator for conjunction
// interval sources.
//
// Mirrors org.apache.lucene.queries.intervals.ConjunctionMatchesIterator.
type ConjunctionMatchesIterator struct {
	iterator IntervalIterator
	subs     []IntervalMatchesIterator
	cached   bool
}

// NewConjunctionMatchesIterator constructs a ConjunctionMatchesIterator.
func NewConjunctionMatchesIterator(iterator IntervalIterator, subs []IntervalMatchesIterator) *ConjunctionMatchesIterator {
	return &ConjunctionMatchesIterator{iterator: iterator, subs: subs, cached: true}
}

// Next advances to the next match.
func (c *ConjunctionMatchesIterator) Next() (bool, error) {
	if c.cached {
		c.cached = false
		return true, nil
	}
	next, err := c.iterator.NextInterval()
	if err != nil {
		return false, err
	}
	return next != NoMoreIntervals, nil
}

// StartPosition returns the start position of the current match.
func (c *ConjunctionMatchesIterator) StartPosition() int { return c.iterator.Start() }

// EndPosition returns the end position of the current match.
func (c *ConjunctionMatchesIterator) EndPosition() int { return c.iterator.End() }

// StartOffset returns the minimum start offset across sub-matches.
func (c *ConjunctionMatchesIterator) StartOffset() (int, error) {
	start := math.MaxInt32
	for _, s := range c.subs {
		v, err := s.StartOffset()
		if err != nil {
			return 0, err
		}
		if v == -1 {
			return -1, nil
		}
		if v < start {
			start = v
		}
	}
	if start == math.MaxInt32 {
		return -1, nil
	}
	return start, nil
}

// EndOffset returns the maximum end offset across sub-matches.
func (c *ConjunctionMatchesIterator) EndOffset() (int, error) {
	end := -1
	for _, s := range c.subs {
		v, err := s.EndOffset()
		if err != nil {
			return 0, err
		}
		if v == -1 {
			return -1, nil
		}
		if v > end {
			end = v
		}
	}
	return end, nil
}

// GetSubMatches returns a disjunction over sub-match iterators.
func (c *ConjunctionMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	subs := make([]search.MatchesIterator, 0, len(c.subs))
	for _, mi := range c.subs {
		sub, err := mi.GetSubMatches()
		if err != nil {
			return nil, err
		}
		if sub == nil {
			sub = &singletonMatchesIterator{in: mi}
		}
		subs = append(subs, sub)
	}
	return newDisjunctionMatchesIterator(subs), nil
}

// GetQuery panics: not supported on ConjunctionMatchesIterator.
func (c *ConjunctionMatchesIterator) GetQuery() search.Query {
	panic("ConjunctionMatchesIterator.GetQuery() is not supported")
}

// Gaps returns the gaps in the current interval.
func (c *ConjunctionMatchesIterator) Gaps() int { return c.iterator.Gaps() }

// Width returns the width of the current interval.
func (c *ConjunctionMatchesIterator) Width() int { return c.iterator.Width() }

// singletonMatchesIterator returns one match (the current position) and then exhausts.
type singletonMatchesIterator struct {
	in       search.MatchesIterator
	exhausted bool
}

func (s *singletonMatchesIterator) Next() (bool, error) {
	if s.exhausted {
		return false, nil
	}
	s.exhausted = true
	return true, nil
}
func (s *singletonMatchesIterator) StartPosition() int                         { return s.in.StartPosition() }
func (s *singletonMatchesIterator) EndPosition() int                           { return s.in.EndPosition() }
func (s *singletonMatchesIterator) StartOffset() (int, error)                  { return s.in.StartOffset() }
func (s *singletonMatchesIterator) EndOffset() (int, error)                    { return s.in.EndOffset() }
func (s *singletonMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return s.in.GetSubMatches()
}
func (s *singletonMatchesIterator) GetQuery() search.Query { return s.in.GetQuery() }

// disjunctionMatchesIterator iterates over a list of MatchesIterators sequentially.
type disjunctionMatchesIterator struct {
	subs []search.MatchesIterator
	idx  int
}

func newDisjunctionMatchesIterator(subs []search.MatchesIterator) *disjunctionMatchesIterator {
	return &disjunctionMatchesIterator{subs: subs, idx: 0}
}

func (d *disjunctionMatchesIterator) Next() (bool, error) {
	for d.idx < len(d.subs) {
		ok, err := d.subs[d.idx].Next()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		d.idx++
	}
	return false, nil
}
func (d *disjunctionMatchesIterator) StartPosition() int {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].StartPosition()
	}
	return -1
}
func (d *disjunctionMatchesIterator) EndPosition() int {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].EndPosition()
	}
	return -1
}
func (d *disjunctionMatchesIterator) StartOffset() (int, error) {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].StartOffset()
	}
	return -1, nil
}
func (d *disjunctionMatchesIterator) EndOffset() (int, error) {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].EndOffset()
	}
	return -1, nil
}
func (d *disjunctionMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].GetSubMatches()
	}
	return nil, nil
}
func (d *disjunctionMatchesIterator) GetQuery() search.Query {
	if d.idx < len(d.subs) {
		return d.subs[d.idx].GetQuery()
	}
	return nil
}
