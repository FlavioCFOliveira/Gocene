// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/RelativeIterator.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// RelativeIterator is the abstract base for interval iterators that operate
// relative to another iterator (used by NonOverlapping, NotContaining, etc.).
//
// Mirrors org.apache.lucene.queries.intervals.RelativeIterator (abstract).
//
// Deviations from Java:
//   - nextIntervalFn is a function field instead of an abstract method.
type RelativeIterator struct {
	A             IntervalIterator
	B             IntervalIterator
	Bpos          bool
	nextIntervalFn func() (int, error)
}

// NewRelativeIterator constructs a RelativeIterator.
// nextFn implements the abstract nextInterval logic.
func NewRelativeIterator(a, b IntervalIterator, nextFn func() (int, error)) *RelativeIterator {
	return &RelativeIterator{A: a, B: b, nextIntervalFn: nextFn}
}

// DocID returns the current document ID.
func (r *RelativeIterator) DocID() int { return r.A.DocID() }

// DocIDRunEnd returns a conservative upper bound.
func (r *RelativeIterator) DocIDRunEnd() int { return r.DocID() + 1 }

// Cost returns the estimated cost.
func (r *RelativeIterator) Cost() int64 { return r.A.Cost() }

// MatchCost returns the combined match cost.
func (r *RelativeIterator) MatchCost() float32 { return r.A.MatchCost() + r.B.MatchCost() }

// Start returns the start of the current interval.
func (r *RelativeIterator) Start() int { return r.A.Start() }

// End returns the end of the current interval.
func (r *RelativeIterator) End() int { return r.A.End() }

// Gaps returns the gaps in the current interval.
func (r *RelativeIterator) Gaps() int { return r.A.Gaps() }

// Width returns the width of the current interval.
func (r *RelativeIterator) Width() int { return r.A.Width() }

// NextDoc advances to the next document.
func (r *RelativeIterator) NextDoc() (int, error) {
	doc, err := r.A.NextDoc()
	if err != nil {
		return 0, err
	}
	if err := r.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

// Advance advances to at least the given target.
func (r *RelativeIterator) Advance(target int) (int, error) {
	doc, err := r.A.Advance(target)
	if err != nil {
		return 0, err
	}
	if err := r.reset(); err != nil {
		return 0, err
	}
	return doc, nil
}

// NextInterval delegates to the nextIntervalFn.
func (r *RelativeIterator) NextInterval() (int, error) { return r.nextIntervalFn() }

func (r *RelativeIterator) reset() error {
	doc := r.A.DocID()
	if r.B.DocID() == doc {
		r.Bpos = true
		return nil
	}
	if r.B.DocID() < doc {
		advDoc, err := r.B.Advance(doc)
		if err != nil {
			return err
		}
		r.Bpos = advDoc == doc
		return nil
	}
	r.Bpos = false
	return nil
}

// AdvanceBToStart moves B past A's current start position.
func (r *RelativeIterator) AdvanceBToStart() error {
	for r.B.Start() < r.A.Start() {
		next, err := r.B.NextInterval()
		if err != nil {
			return err
		}
		if next == NoMoreIntervals {
			return nil
		}
	}
	return nil
}

// Ensure RelativeIterator satisfies search.DocIdSetIterator at compile time.
var _ search.DocIdSetIterator = (*RelativeIterator)(nil)
