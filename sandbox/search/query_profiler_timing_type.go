// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerTimingType.
package search

import "strings"

// QueryProfilerTimingType enumerates the timer buckets the query profiler tracks.
// Leaf-level timers (Count through SetMinCompetitiveScore) run per-segment;
// CreateWeight runs at the top-level IndexReader.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerTimingType.
type QueryProfilerTimingType int

const (
	// TimingTypeCount is the leaf-level count timer.
	TimingTypeCount QueryProfilerTimingType = iota
	// TimingTypeBuildScorer is the leaf-level build-scorer timer.
	TimingTypeBuildScorer
	// TimingTypeNextDoc is the leaf-level nextDoc timer.
	TimingTypeNextDoc
	// TimingTypeAdvance is the leaf-level advance timer.
	TimingTypeAdvance
	// TimingTypeMatch is the leaf-level match timer.
	TimingTypeMatch
	// TimingTypeScore is the leaf-level score timer.
	TimingTypeScore
	// TimingTypeShallowAdvance is the leaf-level shallowAdvance timer.
	TimingTypeShallowAdvance
	// TimingTypeComputeMaxScore is the leaf-level computeMaxScore timer.
	TimingTypeComputeMaxScore
	// TimingTypeSetMinCompetitiveScore is the leaf-level setMinCompetitiveScore timer.
	TimingTypeSetMinCompetitiveScore

	// IMPORTANT: global (non-leaf) timers must come after all leaf-level timers
	// to preserve contiguous ordinal grouping (matching the Java enum ordering).

	// TimingTypeCreateWeight is the top-level createWeight timer.
	TimingTypeCreateWeight

	// timingTypeCount is the total number of timing types (internal sentinel).
	timingTypeCount
)

// leafLevelCount is the number of leaf-level timing types. All types with
// ordinal < leafLevelCount are leaf-level; the remainder are global.
const leafLevelCount = int(TimingTypeCreateWeight)

// IsLeafLevel reports whether the timing type tracks per-segment operations.
func (t QueryProfilerTimingType) IsLeafLevel() bool {
	return int(t) < leafLevelCount
}

// String returns the lowercase name matching the Java enum's toString().
func (t QueryProfilerTimingType) String() string {
	names := [...]string{
		"count",
		"build_scorer",
		"next_doc",
		"advance",
		"match",
		"score",
		"shallow_advance",
		"compute_max_score",
		"set_min_competitive_score",
		"create_weight",
	}
	if int(t) < len(names) {
		return names[t]
	}
	return strings.ToLower("unknown")
}

// allTimingTypes returns all defined QueryProfilerTimingType values.
func allTimingTypes() []QueryProfilerTimingType {
	types := make([]QueryProfilerTimingType, timingTypeCount)
	for i := range types {
		types[i] = QueryProfilerTimingType(i)
	}
	return types
}

// queryLevelTimingTypes returns only the non-leaf (global) timing types.
func queryLevelTimingTypes() []QueryProfilerTimingType {
	var out []QueryProfilerTimingType
	for _, t := range allTimingTypes() {
		if !t.IsLeafLevel() {
			out = append(out, t)
		}
	}
	return out
}

// leafLevelTimingTypes returns only the leaf-level timing types.
func leafLevelTimingTypes() []QueryProfilerTimingType {
	var out []QueryProfilerTimingType
	for _, t := range allTimingTypes() {
		if t.IsLeafLevel() {
			out = append(out, t)
		}
	}
	return out
}
