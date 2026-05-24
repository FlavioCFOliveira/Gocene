// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerResult and
// org.apache.lucene.sandbox.search.AggregatedQueryLeafProfilerResult.
package search

// QueryProfilerResult is the internal representation of a profiled query node.
// Each node may have child results, corresponding to sub-queries in the tree.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerResult.
type QueryProfilerResult struct {
	queryName       string
	description     string
	startTime       int64
	totalTime       int64
	queryBreakdowns map[string]int64
	leafBreakdowns  []AggregatedQueryLeafProfilerResult
	children        []QueryProfilerResult
}

// NewQueryProfilerResult constructs a result node.
func NewQueryProfilerResult(
	queryName, description string,
	queryBreakdowns map[string]int64,
	leafBreakdowns []AggregatedQueryLeafProfilerResult,
	children []QueryProfilerResult,
	startTime, totalTime int64,
) QueryProfilerResult {
	childCopy := append([]QueryProfilerResult(nil), children...)
	leafCopy := append([]AggregatedQueryLeafProfilerResult(nil), leafBreakdowns...)
	bd := make(map[string]int64, len(queryBreakdowns))
	for k, v := range queryBreakdowns {
		bd[k] = v
	}
	return QueryProfilerResult{
		queryName:       queryName,
		description:     description,
		startTime:       startTime,
		totalTime:       totalTime,
		queryBreakdowns: bd,
		leafBreakdowns:  leafCopy,
		children:        childCopy,
	}
}

// GetQueryName returns the query type name.
func (r QueryProfilerResult) GetQueryName() string { return r.queryName }

// GetDescription returns the query description.
func (r QueryProfilerResult) GetDescription() string { return r.description }

// GetTimeBreakdown returns a copy of the timing breakdown map.
func (r QueryProfilerResult) GetTimeBreakdown() map[string]int64 {
	out := make(map[string]int64, len(r.queryBreakdowns))
	for k, v := range r.queryBreakdowns {
		out[k] = v
	}
	return out
}

// GetStartTime returns the earliest start time in nanoseconds.
func (r QueryProfilerResult) GetStartTime() int64 { return r.startTime }

// GetTotalTime returns the total elapsed nanoseconds.
func (r QueryProfilerResult) GetTotalTime() int64 { return r.totalTime }

// GetAggregatedQueryLeafBreakdowns returns the per-thread leaf breakdowns.
func (r QueryProfilerResult) GetAggregatedQueryLeafBreakdowns() []AggregatedQueryLeafProfilerResult {
	return append([]AggregatedQueryLeafProfilerResult(nil), r.leafBreakdowns...)
}

// GetProfiledChildren returns the child profiler results.
func (r QueryProfilerResult) GetProfiledChildren() []QueryProfilerResult {
	return append([]QueryProfilerResult(nil), r.children...)
}

// AggregatedQueryLeafProfilerResult holds the per-thread leaf breakdown for
// one query node.
//
// Mirrors org.apache.lucene.sandbox.search.AggregatedQueryLeafProfilerResult.
type AggregatedQueryLeafProfilerResult struct {
	threadID  int64 // goroutine ID approximation; Java uses Thread
	breakdown map[string]int64
	startTime int64
	totalTime int64
}

// NewAggregatedQueryLeafProfilerResult constructs a leaf result.
// threadID is a goroutine-ID-like identifier (the Java original uses Thread).
func NewAggregatedQueryLeafProfilerResult(threadID int64, breakdown map[string]int64, startTime, totalTime int64) AggregatedQueryLeafProfilerResult {
	bd := make(map[string]int64, len(breakdown))
	for k, v := range breakdown {
		bd[k] = v
	}
	return AggregatedQueryLeafProfilerResult{
		threadID:  threadID,
		breakdown: bd,
		startTime: startTime,
		totalTime: totalTime,
	}
}

// GetTimeBreakdown returns a copy of the leaf timing breakdown.
func (r AggregatedQueryLeafProfilerResult) GetTimeBreakdown() map[string]int64 {
	out := make(map[string]int64, len(r.breakdown))
	for k, v := range r.breakdown {
		out[k] = v
	}
	return out
}

// GetStartTime returns the start time in nanoseconds.
func (r AggregatedQueryLeafProfilerResult) GetStartTime() int64 { return r.startTime }

// GetTotalTime returns the total time in nanoseconds.
func (r AggregatedQueryLeafProfilerResult) GetTotalTime() int64 { return r.totalTime }
