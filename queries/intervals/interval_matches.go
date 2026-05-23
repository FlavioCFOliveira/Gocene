// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalMatches.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalMatches provides utility functions for creating IntervalMatchesIterators
// and wrapping IntervalMatchesIterators as IntervalIterators.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalMatches (Lucene 10.4.0).

type wrapMatchesState int

const (
	wrapStateUnpositioned wrapMatchesState = iota
	wrapStateIterating
	wrapStateNoMoreIntervals
	wrapStateExhausted
)

// AsMatches creates an IntervalMatchesIterator from an IntervalIterator and
// its associated IntervalMatchesIterator, advanced to the given document.
func AsMatches(iterator IntervalIterator, source IntervalMatchesIterator, doc int) (IntervalMatchesIterator, error) {
	if source == nil {
		return nil, nil
	}
	advDoc, err := iterator.Advance(doc)
	if err != nil {
		return nil, err
	}
	if advDoc != doc {
		return nil, nil
	}
	next, err := iterator.NextInterval()
	if err != nil {
		return nil, err
	}
	if next == NoMoreIntervals {
		return nil, nil
	}
	return &asMatchesIterator{iterator: iterator, source: source, cached: true}, nil
}

type asMatchesIterator struct {
	iterator IntervalIterator
	source   IntervalMatchesIterator
	cached   bool
}

func (a *asMatchesIterator) Next() (bool, error) {
	if a.cached {
		a.cached = false
		return true, nil
	}
	next, err := a.iterator.NextInterval()
	if err != nil {
		return false, err
	}
	return next != NoMoreIntervals, nil
}
func (a *asMatchesIterator) StartPosition() int                       { return a.iterator.Start() }
func (a *asMatchesIterator) EndPosition() int                         { return a.iterator.End() }
func (a *asMatchesIterator) StartOffset() (int, error)                { return a.source.StartOffset() }
func (a *asMatchesIterator) EndOffset() (int, error)                  { return a.source.EndOffset() }
func (a *asMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return a.source.GetSubMatches()
}
func (a *asMatchesIterator) GetQuery() search.Query { return a.source.GetQuery() }
func (a *asMatchesIterator) Gaps() int              { return a.iterator.Gaps() }
func (a *asMatchesIterator) Width() int             { return a.iterator.Width() }

// WrapMatches wraps an IntervalMatchesIterator as an IntervalIterator positioned
// at the given document.
func WrapMatches(mi IntervalMatchesIterator, doc int) IntervalIterator {
	return &wrappedMatchesIterator{mi: mi, doc: doc, state: wrapStateUnpositioned}
}

type wrappedMatchesIterator struct {
	mi    IntervalMatchesIterator
	doc   int
	state wrapMatchesState
}

func (w *wrappedMatchesIterator) Start() int {
	if w.state == wrapStateNoMoreIntervals {
		return NoMoreIntervals
	}
	return w.mi.StartPosition()
}

func (w *wrappedMatchesIterator) End() int {
	if w.state == wrapStateNoMoreIntervals {
		return NoMoreIntervals
	}
	return w.mi.EndPosition()
}

func (w *wrappedMatchesIterator) Gaps() int  { return w.mi.Gaps() }
func (w *wrappedMatchesIterator) Width() int { return w.mi.Width() }

func (w *wrappedMatchesIterator) NextInterval() (int, error) {
	ok, err := w.mi.Next()
	if err != nil {
		return 0, err
	}
	if !ok {
		w.state = wrapStateNoMoreIntervals
		return NoMoreIntervals, nil
	}
	w.state = wrapStateIterating
	return w.mi.StartPosition(), nil
}

func (w *wrappedMatchesIterator) MatchCost() float32 { return 1 }

func (w *wrappedMatchesIterator) DocID() int {
	switch w.state {
	case wrapStateUnpositioned:
		return -1
	case wrapStateIterating, wrapStateNoMoreIntervals:
		return w.doc
	default:
		return search.NO_MORE_DOCS
	}
}

func (w *wrappedMatchesIterator) NextDoc() (int, error) {
	switch w.state {
	case wrapStateUnpositioned:
		w.state = wrapStateIterating
		return w.doc, nil
	default:
		w.state = wrapStateExhausted
		return search.NO_MORE_DOCS, nil
	}
}

func (w *wrappedMatchesIterator) Advance(target int) (int, error) {
	if target == w.doc {
		w.state = wrapStateIterating
		return w.doc, nil
	}
	w.state = wrapStateExhausted
	return search.NO_MORE_DOCS, nil
}

func (w *wrappedMatchesIterator) Cost() int64     { return 1 }
func (w *wrappedMatchesIterator) DocIDRunEnd() int { return w.DocID() + 1 }
