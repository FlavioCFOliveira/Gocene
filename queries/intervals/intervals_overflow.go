// Package intervals provides the Go port of org.apache.lucene.queries.intervals.
//
// This package mirrors the Apache Lucene 10.4.0 interval query framework.
// The types defined here correspond to the Java classes in
// org.apache.lucene.queries.intervals.
package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// NoMoreIntervals is the sentinel value returned by IntervalIterator.NextInterval
// when there are no more matching intervals on the current document.
const NoMoreIntervals = int(^uint(0) >> 1) // math.MaxInt == Integer.MAX_VALUE

// IntervalIterator is a DocIdSetIterator that also allows iteration over matching
// intervals in a document.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalIterator (abstract class).
type IntervalIterator interface {
	search.DocIdSetIterator

	// Start returns the start of the current interval, or -1 before NextInterval
	// is called, or NoMoreIntervals when exhausted.
	Start() int

	// End returns the end of the current interval, or -1 before NextInterval
	// is called, or NoMoreIntervals when exhausted.
	End() int

	// Gaps returns the number of gaps within the current interval.
	Gaps() int

	// Width returns the width of the current interval (end - start + 1).
	Width() int

	// NextInterval advances to the next interval.
	// Returns NoMoreIntervals when there are no more intervals.
	NextInterval() (int, error)

	// MatchCost returns an estimate of the per-document cost of calling NextInterval.
	MatchCost() float32
}

// IntervalMatchesIterator extends search.MatchesIterator to expose gaps and width.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalMatchesIterator.
type IntervalMatchesIterator interface {
	search.MatchesIterator

	// Gaps returns the number of top-level gaps inside the current match.
	Gaps() int

	// Width returns the width of the current match.
	Width() int
}

// IntervalsSource is the abstract source of intervals for a given field and segment.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalsSource (abstract class).
type IntervalsSource interface {
	// Intervals creates an IntervalIterator for the given field and leaf context.
	// Returns nil if no intervals for this field exist in this segment.
	Intervals(field string, ctx *index.LeafReaderContext) (IntervalIterator, error)

	// Matches returns an IntervalMatchesIterator for the given field, context and doc.
	// Returns nil if no intervals exist in the given document and field.
	Matches(field string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error)

	// Visit visits the tree of sources.
	Visit(field string, visitor search.QueryVisitor)

	// MinExtent returns the minimum possible width of an interval returned by this source.
	MinExtent() int

	// PullUpDisjunctions returns the set of disjunctions that make up this IntervalsSource.
	PullUpDisjunctions() []IntervalsSource

	// Equals reports whether this source equals another.
	Equals(other IntervalsSource) bool

	// HashCode returns a hash code.
	HashCode() int

	// String returns a string representation.
	String() string
}

// IntervalFilter is an abstract IntervalIterator that filters intervals from
// another IntervalIterator.
//
// Mirrors org.apache.lucene.queries.intervals.IntervalFilter.
type IntervalFilter struct {
	In     IntervalIterator
	accept func() bool
}

// NewIntervalFilter creates a new IntervalFilter.
func NewIntervalFilter(in IntervalIterator, accept func() bool) *IntervalFilter {
	return &IntervalFilter{In: in, accept: accept}
}

func (f *IntervalFilter) DocID() int             { return f.In.DocID() }
func (f *IntervalFilter) DocIDRunEnd() int        { return f.DocID() + 1 }
func (f *IntervalFilter) Start() int             { return f.In.Start() }
func (f *IntervalFilter) End() int               { return f.In.End() }
func (f *IntervalFilter) Gaps() int              { return f.In.Gaps() }
func (f *IntervalFilter) Width() int             { return f.In.Width() }
func (f *IntervalFilter) MatchCost() float32     { return f.In.MatchCost() }
func (f *IntervalFilter) Cost() int64            { return f.In.Cost() }

func (f *IntervalFilter) NextDoc() (int, error) {
	return f.In.NextDoc()
}

func (f *IntervalFilter) Advance(target int) (int, error) {
	return f.In.Advance(target)
}

func (f *IntervalFilter) NextInterval() (int, error) {
	for {
		next, err := f.In.NextInterval()
		if err != nil {
			return 0, err
		}
		if next == NoMoreIntervals || f.accept() {
			return next, nil
		}
	}
}
