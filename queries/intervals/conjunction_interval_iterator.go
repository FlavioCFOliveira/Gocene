// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/ConjunctionIntervalIterator.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ConjunctionIntervalIterator is the base for interval iterators that require
// all sub-iterators to match on the same document.
//
// Mirrors org.apache.lucene.queries.intervals.ConjunctionIntervalIterator (abstract).
//
// Deviations from Java:
//   - reset is a function field instead of an abstract method.
type ConjunctionIntervalIterator struct {
	Approximation search.DocIdSetIterator
	SubIterators  []IntervalIterator
	cost          float32
	resetFn       func() error
	start         int
	end           int
	gaps          int
}

// NewConjunctionIntervalIterator constructs a ConjunctionIntervalIterator.
// resetFn is called after each new document is advanced to.
func NewConjunctionIntervalIterator(subIterators []IntervalIterator, resetFn func() error) *ConjunctionIntervalIterator {
	approx := search.IntersectIterators(asDISI(subIterators))
	var costSum float32
	for _, it := range subIterators {
		costSum += it.MatchCost()
	}
	iters := make([]IntervalIterator, len(subIterators))
	copy(iters, subIterators)
	return &ConjunctionIntervalIterator{
		Approximation: approx,
		SubIterators:  iters,
		cost:          costSum,
		resetFn:       resetFn,
		start:         -1,
		end:           -1,
	}
}

func asDISI(iters []IntervalIterator) []search.DocIdSetIterator {
	out := make([]search.DocIdSetIterator, len(iters))
	for i, it := range iters {
		out[i] = it
	}
	return out
}

// DocID returns the current document ID.
func (c *ConjunctionIntervalIterator) DocID() int { return c.Approximation.DocID() }

// DocIDRunEnd returns a conservative upper bound.
func (c *ConjunctionIntervalIterator) DocIDRunEnd() int { return c.DocID() + 1 }

// Cost returns the estimated iteration cost.
func (c *ConjunctionIntervalIterator) Cost() int64 { return c.Approximation.Cost() }

// MatchCost returns the sum of sub-iterator match costs.
func (c *ConjunctionIntervalIterator) MatchCost() float32 { return c.cost }

// NextDoc advances to the next document.
func (c *ConjunctionIntervalIterator) NextDoc() (int, error) {
	doc, err := c.Approximation.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != search.NO_MORE_DOCS {
		if err := c.resetFn(); err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// Advance advances to at least the given target.
func (c *ConjunctionIntervalIterator) Advance(target int) (int, error) {
	doc, err := c.Approximation.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != search.NO_MORE_DOCS {
		if err := c.resetFn(); err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// Start returns the start of the current interval.
func (c *ConjunctionIntervalIterator) Start() int { return c.start }

// End returns the end of the current interval.
func (c *ConjunctionIntervalIterator) End() int { return c.end }

// Gaps returns the number of gaps in the current interval.
func (c *ConjunctionIntervalIterator) Gaps() int { return c.gaps }

// Width returns the width of the current interval.
func (c *ConjunctionIntervalIterator) Width() int {
	if c.end == NoMoreIntervals {
		return NoMoreIntervals
	}
	return c.end - c.start + 1
}
