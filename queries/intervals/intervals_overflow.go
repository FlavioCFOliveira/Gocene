// Package intervals hosts the Sprint 29 overflow ports for
// org.apache.lucene.queries.intervals.
package intervals

// The Sprint 29 queries-module overflow surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// FilteredIntervalsSource mirrors org.apache.lucene.queries.intervals.FilteredIntervalsSource.
type FilteredIntervalsSource struct{}

// NewFilteredIntervalsSource builds a FilteredIntervalsSource.
func NewFilteredIntervalsSource() *FilteredIntervalsSource { return &FilteredIntervalsSource{} }

// IntervalFilter mirrors org.apache.lucene.queries.intervals.IntervalFilter.
type IntervalFilter struct{}

// NewIntervalFilter builds a IntervalFilter.
func NewIntervalFilter() *IntervalFilter { return &IntervalFilter{} }

// IntervalIterator mirrors org.apache.lucene.queries.intervals.IntervalIterator.
type IntervalIterator struct{}

// NewIntervalIterator builds a IntervalIterator.
func NewIntervalIterator() *IntervalIterator { return &IntervalIterator{} }

// IntervalMatchesIterator mirrors org.apache.lucene.queries.intervals.IntervalMatchesIterator.
type IntervalMatchesIterator struct{}

// NewIntervalMatchesIterator builds a IntervalMatchesIterator.
func NewIntervalMatchesIterator() *IntervalMatchesIterator { return &IntervalMatchesIterator{} }

// IntervalQuery mirrors org.apache.lucene.queries.intervals.IntervalQuery.
type IntervalQuery struct{}

// NewIntervalQuery builds a IntervalQuery.
func NewIntervalQuery() *IntervalQuery { return &IntervalQuery{} }

// Intervals mirrors org.apache.lucene.queries.intervals.Intervals.
type Intervals struct{}

// NewIntervals builds a Intervals.
func NewIntervals() *Intervals { return &Intervals{} }

// IntervalsSource mirrors org.apache.lucene.queries.intervals.IntervalsSource.
type IntervalsSource struct{}

// NewIntervalsSource builds a IntervalsSource.
func NewIntervalsSource() *IntervalsSource { return &IntervalsSource{} }

