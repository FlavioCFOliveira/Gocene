// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryLeafProfilerThreadAggregator.
package search

import (
	"math"
	"sync"
)

// QueryLeafProfilerThreadAggregator aggregates leaf-level profiling breakdowns
// keyed by goroutine ID. It is the Go counterpart of the Java implementation
// that keys on Thread objects.
//
// Mirrors org.apache.lucene.sandbox.search.QueryLeafProfilerThreadAggregator.
type QueryLeafProfilerThreadAggregator struct {
	mu             sync.Mutex
	breakdowns     map[int64]*QueryLeafProfilerBreakdown
	queryStartTime int64
	queryTotalTime int64
}

// newQueryLeafProfilerThreadAggregator creates an aggregator.
func newQueryLeafProfilerThreadAggregator() *QueryLeafProfilerThreadAggregator {
	return &QueryLeafProfilerThreadAggregator{
		breakdowns:     make(map[int64]*QueryLeafProfilerBreakdown),
		queryStartTime: math.MaxInt64,
	}
}

// getOrCreateBreakdown returns the breakdown for the current goroutine,
// creating one if it doesn't exist.
func (a *QueryLeafProfilerThreadAggregator) getOrCreateBreakdown() *QueryLeafProfilerBreakdown {
	id := goroutineID()
	a.mu.Lock()
	b, ok := a.breakdowns[id]
	if !ok {
		b = newQueryLeafProfilerBreakdown()
		a.breakdowns[id] = b
	}
	a.mu.Unlock()
	return b
}

// GetTimer returns the QueryProfilerTimer for the given leaf-level timing type
// for the current goroutine.
func (a *QueryLeafProfilerThreadAggregator) GetTimer(t QueryProfilerTimingType) *QueryProfilerTimer {
	return a.getOrCreateBreakdown().GetTimer(t)
}

// GetQueryStartTime returns the earliest leaf start time across all goroutines.
// Must be called after GetAggregatedQueryLeafProfilerResults.
func (a *QueryLeafProfilerThreadAggregator) GetQueryStartTime() int64 {
	return a.queryStartTime
}

// GetQueryTotalTime returns the total leaf time across all goroutines.
// Must be called after GetAggregatedQueryLeafProfilerResults.
func (a *QueryLeafProfilerThreadAggregator) GetQueryTotalTime() int64 {
	return a.queryTotalTime
}

// GetAggregatedQueryLeafProfilerResults collects and aggregates the per-goroutine
// leaf results. Calling this method also updates queryStartTime and queryTotalTime.
func (a *QueryLeafProfilerThreadAggregator) GetAggregatedQueryLeafProfilerResults() []AggregatedQueryLeafProfilerResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	results := make([]AggregatedQueryLeafProfilerResult, 0, len(a.breakdowns))
	for _, bd := range a.breakdowns {
		result := bd.GetLeafProfilerResult()
		if result.GetStartTime() < a.queryStartTime {
			a.queryStartTime = result.GetStartTime()
		}
		a.queryTotalTime += result.GetTotalTime()
		results = append(results, result)
	}
	return results
}
