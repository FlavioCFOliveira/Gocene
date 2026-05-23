// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/ExtendedIntervalIterator.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExtendedIntervalIterator wraps an IntervalIterator and extends the bounds of
// its intervals by a fixed number of positions before and after.
//
// Mirrors org.apache.lucene.queries.intervals.ExtendedIntervalIterator (Lucene 10.4.0).
type ExtendedIntervalIterator struct {
	in         IntervalIterator
	before     int
	after      int
	positioned bool
}

// NewExtendedIntervalIterator creates an ExtendedIntervalIterator.
// before: positions to extend before the delegated interval.
// after: positions to extend beyond the delegated interval.
func NewExtendedIntervalIterator(in IntervalIterator, before, after int) *ExtendedIntervalIterator {
	return &ExtendedIntervalIterator{in: in, before: before, after: after}
}

// DocID returns the current document ID.
func (e *ExtendedIntervalIterator) DocID() int { return e.in.DocID() }

// DocIDRunEnd returns a conservative upper bound.
func (e *ExtendedIntervalIterator) DocIDRunEnd() int { return e.DocID() + 1 }

// Cost returns the estimated cost.
func (e *ExtendedIntervalIterator) Cost() int64 { return e.in.Cost() }

// MatchCost returns the match cost.
func (e *ExtendedIntervalIterator) MatchCost() float32 { return e.in.MatchCost() }

// Start returns the extended start position.
func (e *ExtendedIntervalIterator) Start() int {
	if !e.positioned {
		return -1
	}
	s := e.in.Start()
	if s == NoMoreIntervals {
		return NoMoreIntervals
	}
	if s-e.before < 0 {
		return 0
	}
	return s - e.before
}

// End returns the extended end position.
func (e *ExtendedIntervalIterator) End() int {
	if !e.positioned {
		return -1
	}
	end := e.in.End()
	if end == NoMoreIntervals {
		return NoMoreIntervals
	}
	end += e.after
	if end < 0 || end == NoMoreIntervals {
		return NoMoreIntervals - 1
	}
	return end
}

// Gaps returns the gaps from the inner iterator.
func (e *ExtendedIntervalIterator) Gaps() int { return e.in.Gaps() }

// Width returns end - start + 1.
func (e *ExtendedIntervalIterator) Width() int {
	end := e.End()
	if end == NoMoreIntervals {
		return NoMoreIntervals
	}
	return end - e.Start() + 1
}

// NextDoc advances to the next document.
func (e *ExtendedIntervalIterator) NextDoc() (int, error) {
	e.positioned = false
	return e.in.NextDoc()
}

// Advance advances to at least the given target.
func (e *ExtendedIntervalIterator) Advance(target int) (int, error) {
	e.positioned = false
	return e.in.Advance(target)
}

// NextInterval advances to the next interval.
func (e *ExtendedIntervalIterator) NextInterval() (int, error) {
	next, err := e.in.NextInterval()
	if err != nil {
		return 0, err
	}
	if next == NoMoreIntervals {
		e.positioned = false
		return NoMoreIntervals, nil
	}
	e.positioned = true
	// skip intervals where the extended start would be out of range
	for e.in.Start() < e.before {
		next, err = e.in.NextInterval()
		if err != nil {
			return 0, err
		}
		if next == NoMoreIntervals {
			e.positioned = false
			return NoMoreIntervals, nil
		}
	}
	return e.Start(), nil
}

var _ search.DocIdSetIterator = (*ExtendedIntervalIterator)(nil)
