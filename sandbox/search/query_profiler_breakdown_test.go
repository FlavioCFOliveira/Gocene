// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of relevant portions of
// org.apache.lucene.sandbox.search.QueryProfilerBreakdown tests
// (tested implicitly via TestQueryProfilerWeight in Java).
package search

import (
	"testing"
)

// TestQueryProfilerBreakdown_GetTimer verifies that GetTimer returns a non-nil
// timer for every defined timing type.
func TestQueryProfilerBreakdown_GetTimer(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	for _, typ := range allTimingTypes() {
		timer := bd.GetTimer(typ)
		if timer == nil {
			t.Errorf("GetTimer(%v) returned nil", typ)
		}
	}
}

// TestQueryProfilerBreakdown_LeafTimerIsPerGoroutine verifies that leaf-level
// timers are isolated to the calling goroutine.
func TestQueryProfilerBreakdown_LeafTimerIsPerGoroutine(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	timer := bd.GetTimer(TimingTypeScore)
	timer.Start()
	timer.Stop()
	if timer.GetCount() != 1 {
		t.Errorf("expected count 1 after one Start/Stop, got %d", timer.GetCount())
	}
}

// TestQueryProfilerBreakdown_CreateWeightTimer verifies the non-leaf
// (createWeight) timer behaves correctly.
func TestQueryProfilerBreakdown_CreateWeightTimer(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	timer := bd.GetTimer(TimingTypeCreateWeight)
	if timer == nil {
		t.Fatal("GetTimer(TimingTypeCreateWeight) returned nil")
	}
	timer.Start()
	timer.Stop()
	if timer.GetCount() != 1 {
		t.Errorf("expected count 1, got %d", timer.GetCount())
	}
}

// TestQueryProfilerBreakdown_GetQueryProfilerResult verifies the result
// contains breakdown keys for all query-level timing types.
func TestQueryProfilerBreakdown_GetQueryProfilerResult(t *testing.T) {
	bd := newQueryProfilerBreakdown()
	result := bd.GetQueryProfilerResult("TestQuery", "description", nil)
	breakdown := result.GetTimeBreakdown()
	for _, typ := range queryLevelTimingTypes() {
		if _, ok := breakdown[typ.String()]; !ok {
			t.Errorf("breakdown missing key %q", typ.String())
		}
		if _, ok := breakdown[typ.String()+"_count"]; !ok {
			t.Errorf("breakdown missing key %q", typ.String()+"_count")
		}
	}
}

// TestQueryProfilerTimingType_String verifies that String() returns lowercase
// names matching the Java enum.toString().
func TestQueryProfilerTimingType_String(t *testing.T) {
	cases := []struct {
		typ  QueryProfilerTimingType
		want string
	}{
		{TimingTypeCount, "count"},
		{TimingTypeBuildScorer, "build_scorer"},
		{TimingTypeNextDoc, "next_doc"},
		{TimingTypeAdvance, "advance"},
		{TimingTypeMatch, "match"},
		{TimingTypeScore, "score"},
		{TimingTypeShallowAdvance, "shallow_advance"},
		{TimingTypeComputeMaxScore, "compute_max_score"},
		{TimingTypeSetMinCompetitiveScore, "set_min_competitive_score"},
		{TimingTypeCreateWeight, "create_weight"},
	}
	for _, tc := range cases {
		if got := tc.typ.String(); got != tc.want {
			t.Errorf("%d.String() = %q; want %q", tc.typ, got, tc.want)
		}
	}
}

// TestQueryProfilerTimingType_IsLeafLevel verifies the leaf/global partitioning.
func TestQueryProfilerTimingType_IsLeafLevel(t *testing.T) {
	leafTypes := map[QueryProfilerTimingType]bool{
		TimingTypeCount:                   true,
		TimingTypeBuildScorer:             true,
		TimingTypeNextDoc:                 true,
		TimingTypeAdvance:                 true,
		TimingTypeMatch:                   true,
		TimingTypeScore:                   true,
		TimingTypeShallowAdvance:          true,
		TimingTypeComputeMaxScore:         true,
		TimingTypeSetMinCompetitiveScore:  true,
		TimingTypeCreateWeight:            false,
	}
	for typ, wantLeaf := range leafTypes {
		if got := typ.IsLeafLevel(); got != wantLeaf {
			t.Errorf("%v.IsLeafLevel() = %v; want %v", typ, got, wantLeaf)
		}
	}
}
