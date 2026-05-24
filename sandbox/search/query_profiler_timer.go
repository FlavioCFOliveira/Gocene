// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerTimer.
package search

import "time"

// QueryProfilerTimer measures the time spent in a method body. Calls to Start
// and Stop should bracket the code being timed:
//
//	t.Start()
//	defer t.Stop()
//
// The timer uses an adaptive sampling strategy: it times every call for the
// first 256 invocations, then one in every gradually-increasing interval up to
// a maximum gap of 1024 calls. This mirrors the Java implementation to keep
// timing overhead proportional for hot paths.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerTimer.
type QueryProfilerTimer struct {
	doTiming               bool
	timing                 int64
	count                  int64
	lastCount              int64
	start                  int64
	earliestTimerStartTime int64
	// clock is the source of nanosecond timestamps. Defaults to
	// time.Now().UnixNano(). Override in tests via newQueryProfilerTimerWithClock.
	clock func() int64
}

// NewQueryProfilerTimer creates a zero-valued QueryProfilerTimer using the
// system clock.
func NewQueryProfilerTimer() *QueryProfilerTimer {
	return &QueryProfilerTimer{clock: defaultClock}
}

func defaultClock() int64 { return time.Now().UnixNano() }

// newQueryProfilerTimerWithClock creates a QueryProfilerTimer with an
// injectable clock. Used by tests to simulate deterministic time.
func newQueryProfilerTimerWithClock(clock func() int64) *QueryProfilerTimer {
	return &QueryProfilerTimer{clock: clock}
}

func (t *QueryProfilerTimer) nanoTime() int64 {
	if t.clock != nil {
		return t.clock()
	}
	return defaultClock()
}

// Start begins a timing window. Must be followed by a matching Stop call.
func (t *QueryProfilerTimer) Start() {
	// doTiming is true when (count - lastCount) >= min(lastCount >> 8, 1024).
	gap := t.count - t.lastCount
	threshold := t.lastCount >> 8
	if threshold > 1024 {
		threshold = 1024
	}
	t.doTiming = gap >= threshold
	if t.doTiming {
		t.start = t.nanoTime()
		if t.count == 0 {
			t.earliestTimerStartTime = t.start
		}
	}
	t.count++
}

// Stop ends a timing window and accumulates the elapsed time.
func (t *QueryProfilerTimer) Stop() {
	if t.doTiming {
		elapsed := t.nanoTime() - t.start
		if elapsed < 1 {
			elapsed = 1
		}
		t.timing += (t.count - t.lastCount) * elapsed
		t.lastCount = t.count
		t.start = 0
	}
}

// GetCount returns the number of times Start has been called.
// Panics if Start has been called without a matching Stop.
func (t *QueryProfilerTimer) GetCount() int64 {
	if t.start != 0 {
		panic("QueryProfilerTimer: #Start call misses a matching #Stop call")
	}
	return t.count
}

// GetEarliestTimerStartTime returns the nanosecond timestamp of the first
// timed Start call. Panics if Start has been called without a matching Stop.
func (t *QueryProfilerTimer) GetEarliestTimerStartTime() int64 {
	if t.start != 0 {
		panic("QueryProfilerTimer: #Start call misses a matching #Stop call")
	}
	return t.earliestTimerStartTime
}

// GetApproximateTiming returns an approximation of the total nanoseconds
// elapsed between paired Start/Stop calls. The approximation extrapolates
// the average timing from timed calls to the un-timed tail calls.
// Panics if Start has been called without a matching Stop.
func (t *QueryProfilerTimer) GetApproximateTiming() int64 {
	if t.start != 0 {
		panic("QueryProfilerTimer: #Start call misses a matching #Stop call")
	}
	timing := t.timing
	if t.count > t.lastCount && t.lastCount > 0 {
		timing += (t.count - t.lastCount) * timing / t.lastCount
	}
	return timing
}
