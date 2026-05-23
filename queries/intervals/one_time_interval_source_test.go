// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/OneTimeIntervalSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// oneTimeIntervalSource is a mock IntervalsSource that returns a constant position
// for every document exactly once per document.
//
// Mirrors org.apache.lucene.queries.intervals.OneTimeIntervalSource.
type oneTimeIntervalSource struct{}

func (s *oneTimeIntervalSource) Intervals(_ string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	maxDoc := ctx.LeafReader().MaxDoc()
	return &oneTimeIntervalIterator{doc: -1, maxDoc: maxDoc}, nil
}

type oneTimeIntervalIterator struct {
	doc    int
	maxDoc int
	flag   bool
}

func (it *oneTimeIntervalIterator) DocID() int      { return it.doc }
func (it *oneTimeIntervalIterator) DocIDRunEnd() int { return it.doc + 1 }
func (it *oneTimeIntervalIterator) Start() int       { return 0 }
func (it *oneTimeIntervalIterator) End() int         { return 0 }
func (it *oneTimeIntervalIterator) Gaps() int        { return 0 }
func (it *oneTimeIntervalIterator) Width() int       { return 1 }
func (it *oneTimeIntervalIterator) MatchCost() float32 { return 0 }
func (it *oneTimeIntervalIterator) Cost() int64     { return 0 }

func (it *oneTimeIntervalIterator) NextDoc() (int, error) {
	it.doc++
	if it.doc >= it.maxDoc {
		it.doc = search.NO_MORE_DOCS
	}
	it.flag = true
	return it.doc, nil
}

func (it *oneTimeIntervalIterator) Advance(target int) (int, error) {
	it.doc = target
	if it.doc >= it.maxDoc {
		it.doc = search.NO_MORE_DOCS
	}
	it.flag = true
	return it.doc, nil
}

func (it *oneTimeIntervalIterator) NextInterval() (int, error) {
	if it.doc != search.NO_MORE_DOCS {
		if it.flag {
			it.flag = false
			return 0, nil
		}
		return NoMoreIntervals, nil
	}
	panic("nextInterval called with docID == NO_MORE_DOCS")
}

var _ search.DocIdSetIterator = (*oneTimeIntervalIterator)(nil)

func (s *oneTimeIntervalSource) Matches(_ string, _ *index.LeafReaderContext, _ int) (IntervalMatchesIterator, error) {
	return &oneTimeMatchesIterator{next: true}, nil
}

type oneTimeMatchesIterator struct {
	next bool
}

func (m *oneTimeMatchesIterator) Gaps() int  { return 0 }
func (m *oneTimeMatchesIterator) Width() int { return 1 }

func (m *oneTimeMatchesIterator) Next() (bool, error) {
	if m.next {
		m.next = false
		return true, nil
	}
	return false, nil
}

func (m *oneTimeMatchesIterator) StartPosition() int { return 0 }
func (m *oneTimeMatchesIterator) EndPosition() int   { return 0 }
func (m *oneTimeMatchesIterator) StartOffset() (int, error) { return 0, nil }
func (m *oneTimeMatchesIterator) EndOffset() (int, error)   { return 0, nil }
func (m *oneTimeMatchesIterator) GetSubMatches() (search.MatchesIterator, error) { return nil, nil }
func (m *oneTimeMatchesIterator) GetQuery() search.Query                          { return nil }

func (s *oneTimeIntervalSource) Visit(_ string, _ search.QueryVisitor) {}

func (s *oneTimeIntervalSource) MinExtent() int { return 0 }

func (s *oneTimeIntervalSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

func (s *oneTimeIntervalSource) Equals(_ IntervalsSource) bool  { return false }
func (s *oneTimeIntervalSource) HashCode() int                  { return 0 }
func (s *oneTimeIntervalSource) String() string                 { return "" }
