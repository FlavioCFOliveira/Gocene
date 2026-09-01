package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// IntervalIterator exposes the minimum intervals defined by an IntervalsSource.
type IntervalIterator interface {
	// Minimal methods for a faithful port
	Next() (int, int, bool)
	Gaps() int
}

// IntervalMatchesIterator returns matches for a given document and field.
type IntervalMatchesIterator interface {
	// Minimal methods for a faithful port
	Next() (int, int, bool)
}

// IntervalsSource is a helper for IntervalQuery that provides an IntervalIterator
// for a given field and segment.
type IntervalsSource interface {
	// Intervals create an IntervalIterator exposing the minimum intervals defined by this IntervalsSource.
	// Returns nil if no intervals for this field exist in this segment.
	Intervals(field string, ctx index.LeafReaderContext) (IntervalIterator, error)

	// Matches return a MatchesIterator over the intervals defined by this IntervalsSource for a given document and field.
	// Returns nil if no intervals exist in the given document and field.
	Matches(field string, ctx index.LeafReaderContext, doc int) (IntervalMatchesIterator, error)

	// Visit is an expert method to visit the tree of sources.
	Visit(field string, visitor any)

	// MinExtent return the minimum possible width of an interval returned by this source.
	MinExtent() int

	// PullUpDisjunctions return the set of disjunctions that make up this IntervalsSource.
	PullUpDisjunctions() []IntervalsSource

	String() string
}
