// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/FilteringIntervalIterator.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FilteringIntervalIterator is the abstract base for two-source conjunction interval
// iterators that filter intervals from iterator A based on iterator B.
//
// Mirrors org.apache.lucene.queries.intervals.FilteringIntervalIterator (abstract).
//
// Deviations from Java:
//   - nextIntervalFn is a function field instead of an abstract method.
type FilteringIntervalIterator struct {
	A              IntervalIterator
	B              IntervalIterator
	Bpos           bool
	approximation  search.DocIdSetIterator
	cost           float32
	nextIntervalFn func() (int, error)
}

// NewFilteringIntervalIterator constructs a FilteringIntervalIterator.
func NewFilteringIntervalIterator(a, b IntervalIterator, nextFn func() (int, error)) (*FilteringIntervalIterator, error) {
	approx := search.IntersectIterators([]search.DocIdSetIterator{a, b})
	return &FilteringIntervalIterator{
		A:              a,
		B:              b,
		approximation:  approx,
		cost:           a.MatchCost() + b.MatchCost(),
		nextIntervalFn: nextFn,
	}, nil
}

// DocID returns the current document ID.
func (f *FilteringIntervalIterator) DocID() int { return f.approximation.DocID() }

// DocIDRunEnd returns a conservative upper bound.
func (f *FilteringIntervalIterator) DocIDRunEnd() int { return f.DocID() + 1 }

// Cost returns the estimated iteration cost.
func (f *FilteringIntervalIterator) Cost() int64 { return f.approximation.Cost() }

// MatchCost returns the combined match cost.
func (f *FilteringIntervalIterator) MatchCost() float32 { return f.cost }

// Start returns the start of the current interval.
func (f *FilteringIntervalIterator) Start() int {
	if !f.Bpos {
		return NoMoreIntervals
	}
	return f.A.Start()
}

// End returns the end of the current interval.
func (f *FilteringIntervalIterator) End() int {
	if !f.Bpos {
		return NoMoreIntervals
	}
	return f.A.End()
}

// Gaps returns the gaps from A.
func (f *FilteringIntervalIterator) Gaps() int { return f.A.Gaps() }

// Width returns the width from A.
func (f *FilteringIntervalIterator) Width() int { return f.A.Width() }

// NextDoc advances to the next document.
func (f *FilteringIntervalIterator) NextDoc() (int, error) {
	doc, err := f.approximation.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != search.NO_MORE_DOCS {
		if err := f.reset(); err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// Advance advances to at least the given target.
func (f *FilteringIntervalIterator) Advance(target int) (int, error) {
	doc, err := f.approximation.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != search.NO_MORE_DOCS {
		if err := f.reset(); err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// NextInterval delegates to nextIntervalFn.
func (f *FilteringIntervalIterator) NextInterval() (int, error) {
	return f.nextIntervalFn()
}

func (f *FilteringIntervalIterator) reset() error {
	next, err := f.B.NextInterval()
	if err != nil {
		return err
	}
	f.Bpos = next != NoMoreIntervals
	return nil
}

var _ search.DocIdSetIterator = (*FilteringIntervalIterator)(nil)
