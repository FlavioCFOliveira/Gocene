// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// xyDistanceFakeSortedNumeric is a minimal SortedNumericDocValues stub used
// to drive the comparator without standing up a full segment reader. It
// mirrors the distanceFakeSortedNumeric helper used by the LatLon test peer
// but lives separately to avoid coupling the two test files.
type xyDistanceFakeSortedNumeric struct {
	docs     []int
	values   [][]int64
	idx      int
	valueIdx int
}

func newXYDistanceFakeSortedNumeric(docs []int, values [][]int64) *xyDistanceFakeSortedNumeric {
	if len(docs) != len(values) {
		panic("xyDistanceFakeSortedNumeric: docs and values length mismatch")
	}
	return &xyDistanceFakeSortedNumeric{docs: docs, values: values, idx: -1}
}

func (f *xyDistanceFakeSortedNumeric) Cost() int64 { return int64(len(f.docs)) }

func (f *xyDistanceFakeSortedNumeric) LongValue() (int64, error) {
	if f.idx < 0 || f.idx >= len(f.docs) {
		return 0, nil
	}
	vs := f.values[f.idx]
	if len(vs) == 0 {
		return 0, nil
	}
	return vs[0], nil
}

func (f *xyDistanceFakeSortedNumeric) Advance(target int) (int, error) {
	if f.idx < 0 {
		f.idx = 0
	}
	for f.idx < len(f.docs) && f.docs[f.idx] < target {
		f.idx++
	}
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS, nil
	}
	f.valueIdx = 0
	return f.docs[f.idx], nil
}

func (f *xyDistanceFakeSortedNumeric) NextDoc() (int, error) {
	f.idx++
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS, nil
	}
	f.valueIdx = 0
	return f.docs[f.idx], nil
}

func (f *xyDistanceFakeSortedNumeric) DocID() int {
	if f.idx < 0 {
		return -1
	}
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS
	}
	return f.docs[f.idx]
}

func (f *xyDistanceFakeSortedNumeric) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (f *xyDistanceFakeSortedNumeric) NextValue() (int64, error) {
	if f.idx < 0 || f.idx >= len(f.docs) {
		return 0, nil
	}
	vs := f.values[f.idx]
	if f.valueIdx >= len(vs) {
		return 0, nil
	}
	v := vs[f.valueIdx]
	f.valueIdx++
	return v, nil
}

func (f *xyDistanceFakeSortedNumeric) DocValueCount() (int, error) {
	if f.idx < 0 || f.idx >= len(f.docs) {
		return 0, nil
	}
	return len(f.values[f.idx]), nil
}

// encodeXYPoint packs (x, y) into the same SortedNumeric layout produced by
// XYDocValuesField — upper 32 bits = encoded x, lower 32 bits = encoded y.
// Re-implemented locally to keep the search-package test free of a document
// package import (the production code uses the geo helpers directly).
func encodeXYPoint(x, y float32) int64 {
	xb := int64(geo.XYEncode(x)) & 0xFFFFFFFF
	yb := int64(geo.XYEncode(y)) & 0xFFFFFFFF
	return (xb << 32) | yb
}

func TestXYPointDistanceComparator_Constructor_InitialState(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 1.5, -2.25, 8)
	if cmp == nil {
		t.Fatalf("constructor returned nil")
	}
	if cmp.field != "xy" {
		t.Errorf("field = %q, want xy", cmp.field)
	}
	if cmp.x != 1.5 || cmp.y != -2.25 {
		t.Errorf("origin = (%v,%v), want (1.5,-2.25)", cmp.x, cmp.y)
	}
	if got := len(cmp.values); got != 8 {
		t.Errorf("values slot count = %d, want 8", got)
	}
	if cmp.minX != math.MinInt32 || cmp.maxX != math.MaxInt32 ||
		cmp.minY != math.MinInt32 || cmp.maxY != math.MaxInt32 {
		t.Errorf("initial bounding box must span the full int32 range")
	}
	if cmp.valuesDocID != -1 {
		t.Errorf("valuesDocID = %d, want -1", cmp.valuesDocID)
	}
}

func TestXYPointDistanceComparator_CopyAndCompare_OrderClosestFirst(t *testing.T) {
	t.Parallel()

	// Origin (0, 0). Pythagorean triples give exact integer distances after
	// round-trip through the sortable-int encoding.
	cmp := NewXYPointDistanceComparator("xy", 0, 0, 4)

	docs := []int{0, 1, 2, 3}
	values := [][]int64{
		{encodeXYPoint(0, 0)},  // distance 0
		{encodeXYPoint(3, 4)},  // distance 5
		{encodeXYPoint(6, 8)},  // distance 10
		{encodeXYPoint(9, 12)}, // distance 15
	}
	cmp.currentDocs = newXYDistanceFakeSortedNumeric(docs, values)

	for slot, doc := range docs {
		if err := cmp.Copy(slot, doc); err != nil {
			t.Fatalf("Copy(slot=%d, doc=%d): %v", slot, doc, err)
		}
	}

	wantDistances := []float64{0, 5, 10, 15}
	for slot, want := range wantDistances {
		got := cmp.Value(slot)
		if math.Abs(got-want) > 1e-3 {
			t.Errorf("Value(slot=%d) = %v, want %v", slot, got, want)
		}
	}

	// Each closer slot must beat the next farther slot.
	for i := 0; i < len(docs)-1; i++ {
		if cmp.Compare(i, i+1) >= 0 {
			t.Errorf("Compare(slot=%d, slot=%d) = %d, want <0", i, i+1, cmp.Compare(i, i+1))
		}
	}
	if cmp.Compare(2, 2) != 0 {
		t.Errorf("Compare(self, self) = %d, want 0", cmp.Compare(2, 2))
	}
}

func TestXYPointDistanceComparator_MultiValued_PicksClosestPoint(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	// Single doc with two points: one at (100, 100) and one at the origin.
	// The minimum (origin) must win the sort key.
	docs := []int{0}
	values := [][]int64{{
		encodeXYPoint(100, 100), // far
		encodeXYPoint(0, 0),     // origin (closer; wins)
	}}
	cmp.currentDocs = newXYDistanceFakeSortedNumeric(docs, values)
	if err := cmp.Copy(0, 0); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := cmp.Value(0); got > 1e-3 {
		t.Errorf("multi-valued doc Value = %v, want ~0 (closest point wins)", got)
	}
}

func TestXYPointDistanceComparator_MissingDoc_ReturnsInfinity(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	cmp.currentDocs = newXYDistanceFakeSortedNumeric(
		[]int{2}, [][]int64{{encodeXYPoint(0, 0)}})
	// Probe a doc id past the only entry.
	if err := cmp.Copy(0, 5); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := cmp.Value(0); !math.IsInf(got, 1) {
		t.Errorf("Value(missing) = %v, want +Inf", got)
	}
}

func TestXYPointDistanceComparator_NilDocs_ReturnsInfinity(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	if err := cmp.Copy(0, 0); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := cmp.Value(0); !math.IsInf(got, 1) {
		t.Errorf("Value(nil docs) = %v, want +Inf", got)
	}
}

func TestXYPointDistanceComparator_SetBottom_BuildsBoundingBox(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	// Seed slot 0 with a 10-unit distance from origin (which is the bottom).
	cmp.values[0] = 10
	if err := cmp.SetBottom(0); err != nil {
		t.Fatalf("SetBottom: %v", err)
	}
	if cmp.bottom != 10 {
		t.Errorf("bottom = %v, want 10", cmp.bottom)
	}
	if cmp.setBottomCounter != 1 {
		t.Errorf("setBottomCounter = %d, want 1", cmp.setBottomCounter)
	}
	// The bounding box must now be a proper subset of the int32 range. The
	// exact bytes are governed by geo.XYEncode and geo.FromXYPointDistance;
	// we only assert tightening, not specific encoded boundaries.
	if cmp.minX == math.MinInt32 || cmp.maxX == math.MaxInt32 {
		t.Errorf("setBottom did not tighten X bounds: minX=%d maxX=%d", cmp.minX, cmp.maxX)
	}
	if cmp.minY == math.MinInt32 || cmp.maxY == math.MaxInt32 {
		t.Errorf("setBottom did not tighten Y bounds: minY=%d maxY=%d", cmp.minY, cmp.maxY)
	}
	// Origin (0,0) means the encoded box must straddle zero: minX < 0 < maxX
	// (likewise for Y). The exact bounds are not symmetric bit-for-bit
	// because geo.FromXYPointDistance dilates the radius via
	// math.Nextafter32 and the sortable-int encoding is not parity-preserving
	// around zero; we only assert the straddle property.
	if cmp.minX >= 0 || cmp.maxX <= 0 {
		t.Errorf("setBottom X box does not straddle origin: minX=%d maxX=%d", cmp.minX, cmp.maxX)
	}
	if cmp.minY >= 0 || cmp.maxY <= 0 {
		t.Errorf("setBottom Y box does not straddle origin: minY=%d maxY=%d", cmp.minY, cmp.maxY)
	}
}

func TestXYPointDistanceComparator_SetBottom_SkipsAtMaxFloat(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	// A bottom of +Inf or >= MaxFloat32 must leave the box untouched (Java
	// guard: `bottom < Float.MAX_VALUE`).
	cmp.values[0] = math.Inf(1)
	if err := cmp.SetBottom(0); err != nil {
		t.Fatalf("SetBottom: %v", err)
	}
	if cmp.minX != math.MinInt32 || cmp.maxX != math.MaxInt32 ||
		cmp.minY != math.MinInt32 || cmp.maxY != math.MaxInt32 {
		t.Errorf("box was tightened despite +Inf bottom; got x=[%d,%d] y=[%d,%d]",
			cmp.minX, cmp.maxX, cmp.minY, cmp.maxY)
	}
	if cmp.setBottomCounter != 1 {
		t.Errorf("setBottomCounter = %d, want 1 (counter must increment even when skipped)",
			cmp.setBottomCounter)
	}
}

func TestXYPointDistanceComparator_CompareBottom_RejectsOutsideBox(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	// Bottom = 5-unit radius around origin (3-4-5 triangle hits the edge).
	cmp.values[0] = 5
	if err := cmp.SetBottom(0); err != nil {
		t.Fatalf("SetBottom: %v", err)
	}
	// doc 0 sits at the origin (clearly competitive); doc 1 at (1000, 1000)
	// is far outside the competitive box.
	cmp.currentDocs = newXYDistanceFakeSortedNumeric(
		[]int{0, 1},
		[][]int64{
			{encodeXYPoint(0, 0)},
			{encodeXYPoint(1000, 1000)},
		},
	)
	got, err := cmp.CompareBottom(0)
	if err != nil {
		t.Fatalf("CompareBottom(0): %v", err)
	}
	if got <= 0 {
		t.Errorf("CompareBottom(origin) = %d, want positive (competitive)", got)
	}
	got, err = cmp.CompareBottom(1)
	if err != nil {
		t.Fatalf("CompareBottom(1): %v", err)
	}
	if got > 0 {
		t.Errorf("CompareBottom(far) = %d, want non-positive (rejected)", got)
	}
}

func TestXYPointDistanceComparator_GetLeafComparator_NilContextErrors(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	if _, err := cmp.GetLeafComparator(nil); err == nil {
		t.Fatalf("GetLeafComparator(nil) must error")
	}
}

func TestXYPointDistanceComparator_SetTopValue_CompareTop(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	cmp.currentDocs = newXYDistanceFakeSortedNumeric(
		[]int{0}, [][]int64{{encodeXYPoint(0, 0)}})
	// Doc distance ≈ 0; topValue = 10 → topValue > doc distance → positive.
	cmp.SetTopValue(10)
	got, err := cmp.CompareTop(0)
	if err != nil {
		t.Fatalf("CompareTop: %v", err)
	}
	if got <= 0 {
		t.Errorf("CompareTop = %d, want positive (top reference greater than doc distance)", got)
	}
}

func TestXYPointDistanceComparator_CompetitiveIterator_NilAndNoOpHooks(t *testing.T) {
	t.Parallel()

	cmp := NewXYPointDistanceComparator("xy", 0, 0, 1)
	it, err := cmp.CompetitiveIterator()
	if err != nil {
		t.Fatalf("CompetitiveIterator: %v", err)
	}
	if it != nil {
		t.Errorf("CompetitiveIterator() = %T, want nil", it)
	}
	// SetHitsThresholdReached + SetScorer must not panic or error.
	cmp.SetHitsThresholdReached()
	if err := cmp.SetScorer(nil); err != nil {
		t.Errorf("SetScorer(nil) = %v, want nil", err)
	}
}
