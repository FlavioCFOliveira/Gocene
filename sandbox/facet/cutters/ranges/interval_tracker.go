// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.cutters.ranges.IntervalTracker.
package ranges

import (
	"github.com/FlavioCFOliveira/Gocene/join"
)

// NoMoreOrds is the sentinel value returned by IntervalTracker.NextOrd when
// there are no more ordinals.
//
// Mirrors OrdinalIterator.NO_MORE_ORDS.
const NoMoreOrds = -1

// IntervalTracker is a specialised ordinal iterator that supports write
// (Set and Clear) operations. Clients write data, call Freeze, and then
// read data using NextOrd.
//
// Mirrors org.apache.lucene.sandbox.facet.cutters.ranges.IntervalTracker.
type IntervalTracker interface {
	// NextOrd returns the next ordinal or NoMoreOrds.
	NextOrd() int
	// Set records that interval i was observed.
	Set(i int)
	// Clear resets all recorded information.
	Clear()
	// Get reports whether any data for interval index has been recorded.
	Get(index int) bool
	// Freeze finalises state before NextOrd can be called.
	Freeze()
}

// MultiIntervalTracker is an IntervalTracker that tracks multiple intervals
// using a bit set.
//
// Mirrors IntervalTracker.MultiIntervalTracker.
type MultiIntervalTracker struct {
	tracker          *join.FixedBitSet
	trackerState     int
	bitFrom          int
	intervalsWithHit int
}

// NewMultiIntervalTracker creates a MultiIntervalTracker for size intervals.
func NewMultiIntervalTracker(size int) *MultiIntervalTracker {
	return &MultiIntervalTracker{tracker: join.NewFixedBitSet(size)}
}

// Set records that interval i was observed.
func (m *MultiIntervalTracker) Set(i int) {
	m.tracker.Set(i)
}

// Clear resets all recorded state.
func (m *MultiIntervalTracker) Clear() {
	m.tracker = join.NewFixedBitSet(m.tracker.Size())
	m.bitFrom = 0
	m.trackerState = 0
	m.intervalsWithHit = 0
}

// Get reports whether interval index has been observed.
func (m *MultiIntervalTracker) Get(index int) bool {
	return m.tracker.Get(index)
}

// Freeze computes the cardinality so that NextOrd can bound iteration.
func (m *MultiIntervalTracker) Freeze() {
	m.intervalsWithHit = m.tracker.Cardinality()
}

// NextOrd returns the next set ordinal or NoMoreOrds.
func (m *MultiIntervalTracker) NextOrd() int {
	if m.trackerState == m.intervalsWithHit {
		return NoMoreOrds
	}
	m.trackerState++
	next := m.tracker.NextSetBit(m.bitFrom)
	if next < 0 {
		return NoMoreOrds
	}
	m.bitFrom = next + 1
	return next
}

var _ IntervalTracker = (*MultiIntervalTracker)(nil)
