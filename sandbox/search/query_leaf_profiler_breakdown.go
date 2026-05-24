// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryLeafProfilerBreakdown.
package search

import (
	"math"
	"runtime"
)

// QueryLeafProfilerBreakdown records timings for all leaf-level operations of
// a query node on a single goroutine/thread. It holds one QueryProfilerTimer
// per leaf-level QueryProfilerTimingType.
//
// Mirrors org.apache.lucene.sandbox.search.QueryLeafProfilerBreakdown.
type QueryLeafProfilerBreakdown struct {
	timers []*QueryProfilerTimer
}

// newQueryLeafProfilerBreakdown creates a zero-valued breakdown.
func newQueryLeafProfilerBreakdown() *QueryLeafProfilerBreakdown {
	leafTypes := leafLevelTimingTypes()
	timers := make([]*QueryProfilerTimer, len(leafTypes))
	for i := range timers {
		timers[i] = NewQueryProfilerTimer()
	}
	return &QueryLeafProfilerBreakdown{timers: timers}
}

// GetTimer returns the timer for the given leaf-level timing type.
func (b *QueryLeafProfilerBreakdown) GetTimer(t QueryProfilerTimingType) *QueryProfilerTimer {
	return b.timers[int(t)]
}

// ToBreakdownMap builds a map of type_name → timing and type_name_count → count
// for all leaf-level timers.
func (b *QueryLeafProfilerBreakdown) ToBreakdownMap() map[string]int64 {
	leafTypes := leafLevelTimingTypes()
	m := make(map[string]int64, len(leafTypes)*2)
	for _, t := range leafTypes {
		timer := b.timers[int(t)]
		m[t.String()] = timer.GetApproximateTiming()
		m[t.String()+"_count"] = timer.GetCount()
	}
	return m
}

// GetLeafProfilerResult builds an AggregatedQueryLeafProfilerResult from this
// breakdown. threadID is set to the current goroutine's approximate ID.
func (b *QueryLeafProfilerBreakdown) GetLeafProfilerResult() AggregatedQueryLeafProfilerResult {
	leafTypes := leafLevelTimingTypes()
	m := make(map[string]int64, len(leafTypes)*2)
	sliceStartTime := int64(math.MaxInt64)
	sliceEndTime := int64(math.MinInt64)

	for _, t := range leafTypes {
		timer := b.timers[int(t)]
		if timer.GetCount() > 0 {
			start := timer.GetEarliestTimerStartTime()
			end := start + timer.GetApproximateTiming()
			if start < sliceStartTime {
				sliceStartTime = start
			}
			if end > sliceEndTime {
				sliceEndTime = end
			}
		}
		m[t.String()] = timer.GetApproximateTiming()
		m[t.String()+"_count"] = timer.GetCount()
	}

	totalTime := int64(0)
	if sliceEndTime > sliceStartTime {
		totalTime = sliceEndTime - sliceStartTime
	}
	if sliceStartTime == math.MaxInt64 {
		sliceStartTime = 0
	}

	return NewAggregatedQueryLeafProfilerResult(goroutineID(), m, sliceStartTime, totalTime)
}

// ToTotalTime returns the sum of approximate timings for all leaf-level timers.
func (b *QueryLeafProfilerBreakdown) ToTotalTime() int64 {
	total := int64(0)
	for _, timer := range b.timers {
		total += timer.GetApproximateTiming()
	}
	return total
}

// goroutineID returns a rough goroutine identifier using runtime.Stack.
// This is a best-effort approximation of the Java Thread object used as a map key.
func goroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	id := int64(0)
	// Format: "goroutine <id> [..."
	for i := 10; i < n; i++ {
		c := buf[i]
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
