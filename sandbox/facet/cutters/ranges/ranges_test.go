// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ranges

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets/rangefacets"
)

// ---- LongRangeNode tests ----

// TestLongRangeNode_AddOutputs verifies that AddOutputs correctly assigns
// range indices to tree nodes when a node is fully contained in the range.
func TestLongRangeNode_AddOutputs(t *testing.T) {
	// Build a leaf node [0, 10].
	leaf := NewLongRangeNode(0, 10, nil, nil)
	r := LongRangeAndPos{Range: &rangefacets.LongRange{Min: 0, Max: 20}, Pos: 0}
	leaf.AddOutputs(r)
	if len(leaf.Outputs) != 1 || leaf.Outputs[0] != 0 {
		t.Errorf("leaf outputs = %v; want [0]", leaf.Outputs)
	}
}

// TestLongRangeNode_AddOutputsNotIncluded verifies that no output is added
// when the node is not fully contained.
func TestLongRangeNode_AddOutputsNotIncluded(t *testing.T) {
	leaf := NewLongRangeNode(0, 20, nil, nil)
	r := LongRangeAndPos{Range: &rangefacets.LongRange{Min: 5, Max: 15}, Pos: 0}
	leaf.AddOutputs(r)
	if len(leaf.Outputs) != 0 {
		t.Errorf("expected no outputs, got %v", leaf.Outputs)
	}
}

// TestLongRangeNode_String smoke-tests the String method.
func TestLongRangeNode_String(t *testing.T) {
	leaf := NewLongRangeNode(0, 10, nil, nil)
	s := leaf.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

// ---- IntervalTracker tests ----

// TestMultiIntervalTracker_SetGetFreeze verifies basic set/get/freeze/nextOrd
// cycle.
func TestMultiIntervalTracker_SetGetFreeze(t *testing.T) {
	tr := NewMultiIntervalTracker(5)
	tr.Set(1)
	tr.Set(3)
	tr.Freeze()

	got := []int{}
	for {
		ord := tr.NextOrd()
		if ord == NoMoreOrds {
			break
		}
		got = append(got, ord)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("NextOrd sequence = %v; want [1 3]", got)
	}
}

// TestMultiIntervalTracker_Clear verifies that Clear resets the state.
func TestMultiIntervalTracker_Clear(t *testing.T) {
	tr := NewMultiIntervalTracker(5)
	tr.Set(0)
	tr.Set(4)
	tr.Freeze()
	tr.Clear()
	tr.Set(2)
	tr.Freeze()

	got := []int{}
	for {
		ord := tr.NextOrd()
		if ord == NoMoreOrds {
			break
		}
		got = append(got, ord)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("after Clear, NextOrd sequence = %v; want [2]", got)
	}
}

// ---- NonOverlappingLongRangeFacetCutter tests ----

// TestNonOverlappingBuilder verifies the elementary intervals produced for
// non-overlapping ranges.
func TestNonOverlappingBuilder(t *testing.T) {
	ranges := []*rangefacets.LongRange{
		{Min: 0, Max: 9},
		{Min: 10, Max: 19},
	}
	cutter := NewNonOverlappingLongRangeFacetCutter(nil, nil, ranges)

	// With two non-overlapping ranges [0,9] and [10,19]:
	// sorted order is same; prev starts at MinInt64.
	// Gap: [MinInt64, -1], [0, 9], [10, 19], [20, MaxInt64]
	if len(cutter.elementaryIntervals) != 4 {
		t.Errorf("expected 4 elementary intervals, got %d: %v",
			len(cutter.elementaryIntervals), cutter.elementaryIntervals)
	}
}

// TestNonOverlapping_SingleValue exercises single-valued document matching.
func TestNonOverlapping_SingleValue(t *testing.T) {
	ranges := []*rangefacets.LongRange{
		{Min: 0, Max: 9},
		{Min: 10, Max: 19},
	}
	cutter := NewNonOverlappingLongRangeFacetCutter(nil, nil, ranges)

	sv := &stubLongValues{values: []int64{5, 15, 100}}
	leaf := cutter.CreateLeafCutter(nil, sv)

	// doc 0: value 5 → range 0
	ok, err := leaf.AdvanceExact(0)
	if err != nil || !ok {
		t.Fatalf("AdvanceExact(0): ok=%v err=%v", ok, err)
	}
	if ord := leaf.NextOrd(); ord != 0 {
		t.Errorf("doc 0: ord = %d; want 0", ord)
	}
	if ord := leaf.NextOrd(); ord != NoMoreOrds {
		t.Errorf("doc 0: expected NoMoreOrds, got %d", ord)
	}

	// doc 1: value 15 → range 1
	ok, err = leaf.AdvanceExact(1)
	if err != nil || !ok {
		t.Fatalf("AdvanceExact(1): ok=%v err=%v", ok, err)
	}
	if ord := leaf.NextOrd(); ord != 1 {
		t.Errorf("doc 1: ord = %d; want 1", ord)
	}

	// doc 2: value 100 → no range → NoMoreOrds immediately
	ok, err = leaf.AdvanceExact(2)
	if err != nil || !ok {
		t.Fatalf("AdvanceExact(2): ok=%v err=%v", ok, err)
	}
	if ord := leaf.NextOrd(); ord != NoMoreOrds {
		t.Errorf("doc 2: expected NoMoreOrds, got %d", ord)
	}
}

// ---- OverlappingLongRangeFacetCutter tests ----

// TestOverlapping_SingleValue verifies that a doc value landing in two
// overlapping ranges emits both ordinals.
func TestOverlapping_SingleValue(t *testing.T) {
	// Ranges [0, 10] and [5, 15] overlap.
	ranges := []*rangefacets.LongRange{
		{Min: 0, Max: 10},
		{Min: 5, Max: 15},
	}
	cutter := NewOverlappingLongRangeFacetCutter(nil, nil, ranges)

	sv := &stubLongValues{values: []int64{7}} // falls in both ranges
	leaf := cutter.CreateLeafCutter(nil, sv)

	ok, err := leaf.AdvanceExact(0)
	if err != nil || !ok {
		t.Fatalf("AdvanceExact(0): ok=%v err=%v", ok, err)
	}
	got := collectOrds(leaf)
	if len(got) != 2 {
		t.Errorf("expected 2 ordinals for value 7 in overlapping [0,10]/[5,15], got %v", got)
	}
}

// TestAreOverlappingRanges verifies the overlap detection helper.
func TestAreOverlappingRanges(t *testing.T) {
	cases := []struct {
		ranges []*rangefacets.LongRange
		want   bool
	}{
		{[]*rangefacets.LongRange{{Min: 0, Max: 9}, {Min: 10, Max: 19}}, false},
		{[]*rangefacets.LongRange{{Min: 0, Max: 10}, {Min: 5, Max: 15}}, true},
		{[]*rangefacets.LongRange{{Min: 0, Max: 5}, {Min: 5, Max: 10}}, true}, // touching = overlapping
		{[]*rangefacets.LongRange{}, false},
	}
	for _, tc := range cases {
		if got := areOverlappingRanges(tc.ranges); got != tc.want {
			t.Errorf("areOverlappingRanges(%v) = %v; want %v", tc.ranges, got, tc.want)
		}
	}
}

// ---- helpers ----

// stubLongValues is a stub search.LongValues for testing.
type stubLongValues struct {
	values []int64
	cur    int
}

func (s *stubLongValues) AdvanceExact(doc int) (bool, error) {
	if doc < len(s.values) {
		s.cur = doc
		return true, nil
	}
	return false, nil
}

func (s *stubLongValues) LongValue() (int64, error) {
	return s.values[s.cur], nil
}

// collectOrds drains all ordinals from a leaf cutter.
func collectOrds(leaf interface{ NextOrd() int }) []int {
	var ords []int
	for {
		ord := leaf.NextOrd()
		if ord == NoMoreOrds {
			break
		}
		ords = append(ords, ord)
	}
	return ords
}
