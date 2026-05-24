// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerBreakdown.
package search

import (
	"math"
)

// QueryProfilerBreakdown records timings for all operations of a single query
// node across both the top-level (create-weight) and leaf (per-segment)
// operations. It holds one QueryProfilerTimer per non-leaf timing type and
// delegates leaf-level timers to a QueryLeafProfilerThreadAggregator.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerBreakdown.
type QueryProfilerBreakdown struct {
	// queryTimers holds timers for the query-level (non-leaf) timing types.
	queryTimers []*QueryProfilerTimer
	// leafAggregator handles the per-goroutine leaf-level timers.
	leafAggregator *QueryLeafProfilerThreadAggregator
}

// newQueryProfilerBreakdown creates a QueryProfilerBreakdown.
func newQueryProfilerBreakdown() *QueryProfilerBreakdown {
	globalTypes := queryLevelTimingTypes()
	timers := make([]*QueryProfilerTimer, int(timingTypeCount))
	for _, t := range globalTypes {
		timers[int(t)] = NewQueryProfilerTimer()
	}
	return &QueryProfilerBreakdown{
		queryTimers:    timers,
		leafAggregator: newQueryLeafProfilerThreadAggregator(),
	}
}

// GetTimer returns the QueryProfilerTimer for the given timing type.
// Leaf-level types are served from the per-goroutine aggregator.
func (b *QueryProfilerBreakdown) GetTimer(t QueryProfilerTimingType) *QueryProfilerTimer {
	if t.IsLeafLevel() {
		return b.leafAggregator.GetTimer(t)
	}
	return b.queryTimers[int(t)]
}

// GetQueryProfilerResult assembles a QueryProfilerResult from accumulated timings.
// queryName and description are derived from the query passed in; children are
// the profiled sub-queries.
func (b *QueryProfilerBreakdown) GetQueryProfilerResult(
	queryName, description string,
	children []QueryProfilerResult,
) QueryProfilerResult {
	queryLevelTypes := queryLevelTimingTypes()
	breakdownMap := make(map[string]int64, len(queryLevelTypes)*2)

	queryStartTime := int64(math.MaxInt64)
	queryTotalTime := int64(0)

	for _, t := range queryLevelTypes {
		timer := b.queryTimers[int(t)]
		if timer.GetCount() > 0 {
			start := timer.GetEarliestTimerStartTime()
			if start < queryStartTime {
				queryStartTime = start
			}
			queryTotalTime += timer.GetApproximateTiming()
		}
		breakdownMap[t.String()] = timer.GetApproximateTiming()
		breakdownMap[t.String()+"_count"] = timer.GetCount()
	}

	leafResults := b.leafAggregator.GetAggregatedQueryLeafProfilerResults()
	leafStart := b.leafAggregator.GetQueryStartTime()
	if leafStart < queryStartTime {
		queryStartTime = leafStart
	}
	queryTotalTime += b.leafAggregator.GetQueryTotalTime()

	if queryStartTime == math.MaxInt64 {
		queryStartTime = 0
	}

	return NewQueryProfilerResult(
		queryName, description,
		breakdownMap,
		leafResults,
		children,
		queryStartTime,
		queryTotalTime,
	)
}
