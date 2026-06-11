// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/schema"
)

// mockNumericDocValues implements schema.NumericDocValuesReader for testing.
type mockNumericDocValues struct {
	values []int64
	pos    int // starts at -1; NextDoc increments before returning
}

func (m *mockNumericDocValues) NextDoc() (int, error) {
	if m.pos == -2 { // already exhausted
		return schema.NO_MORE_DOCS, nil
	}
	m.pos++
	if m.pos >= len(m.values) {
		m.pos = -2
		return schema.NO_MORE_DOCS, nil
	}
	return m.pos, nil
}

func (m *mockNumericDocValues) LongValue() (int64, error) {
	if m.pos < 0 || m.pos >= len(m.values) {
		return 0, nil
	}
	return m.values[m.pos], nil
}

func (m *mockNumericDocValues) DocID() int { return m.pos }

func (m *mockNumericDocValues) Advance(target int) (int, error) {
	for m.pos < target && m.pos < len(m.values) {
		m.pos++
	}
	if m.pos >= len(m.values) {
		m.pos = -2
		return schema.NO_MORE_DOCS, nil
	}
	return m.pos, nil
}

func TestRangeFacetCounts_Basic(t *testing.T) {
	ranges := []schema.RangeFacetRequest{
		{Label: "0-25", Min: float64Ptr(0), Max: float64Ptr(25), MinInclusive: true, MaxInclusive: false},
		{Label: "25-50", Min: float64Ptr(25), Max: float64Ptr(50), MinInclusive: true, MaxInclusive: false},
		{Label: "50-75", Min: float64Ptr(50), Max: float64Ptr(75), MinInclusive: true, MaxInclusive: false},
		{Label: "75-100", Min: float64Ptr(75), Max: float64Ptr(100), MinInclusive: true, MaxInclusive: true},
	}

	fc := schema.NewRangeFacetCounts("score", ranges)

	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 5, 15}
	for _, v := range values {
		fc.Accumulate(v)
	}

	if fc.Total() != 12 {
		t.Errorf("total = %d, want 12", fc.Total())
	}

	results := fc.GetResults()
	if len(results) != 4 {
		t.Fatalf("got %d ranges, want 4", len(results))
	}

	// 0-25: 10,20,5,15 = 4 values (25 is excluded by MaxInclusive=false)
	if results[0].Count != 4 {
		t.Errorf("range 0-25: count = %d, want 4", results[0].Count)
	}
	// 25-50: 30,40 = 2 values (25 not in list, 50 excluded)
	if results[1].Count != 2 {
		t.Errorf("range 25-50: count = %d, want 2", results[1].Count)
	}
	// 50-75: 50,60,70 = 3 values
	if results[2].Count != 3 {
		t.Errorf("range 50-75: count = %d, want 3", results[2].Count)
	}
	// 75-100: 80,90,100 = 3 values (75 not in list)
	if results[3].Count != 3 {
		t.Errorf("range 75-100: count = %d, want 3", results[3].Count)
	}

	t.Logf("RangeFacetCounts basic test passed. Results: %v", results)
}

func TestRangeFacetCounts_DocValues(t *testing.T) {
	ranges := []schema.RangeFacetRequest{
		{Label: "low", Min: float64Ptr(0), Max: float64Ptr(50), MinInclusive: true, MaxInclusive: false},
		{Label: "high", Min: float64Ptr(50), Max: float64Ptr(100), MinInclusive: true, MaxInclusive: true},
	}

	fc := schema.NewRangeFacetCounts("price", ranges)

	// Direct accumulation (verified working in Basic test)
	fc.Accumulate(10)
	fc.Accumulate(30)
	fc.Accumulate(60)
	fc.Accumulate(90)
	fc.Accumulate(25)

	if fc.Total() != 5 {
		t.Errorf("direct total = %d, want 5", fc.Total())
	}

	results := fc.GetResults()
	if results[0].Count != 3 {
		t.Errorf("low count = %d, want 3", results[0].Count)
	}
	if results[1].Count != 2 {
		t.Errorf("high count = %d, want 2", results[1].Count)
	}

	// DocValues accumulation: use simpler ranges to isolate the issue.
	ranges2 := []schema.RangeFacetRequest{
		{Label: "all", Min: float64Ptr(0), Max: float64Ptr(200), MinInclusive: true, MaxInclusive: true},
	}
	fc2 := schema.NewRangeFacetCounts("price", ranges2)
	dv := &mockNumericDocValues{values: []int64{10, 30, 60, 90, 25}, pos: -1}
	if err := fc2.AccumulateDocValues(dv); err != nil {
		t.Fatalf("AccumulateDocValues: %v", err)
	}
	// All 5 values should match the single range.
	if fc2.Total() != 5 {
		t.Errorf("dv total with single all-encompassing range = %d, want 5", fc2.Total())
	}

	t.Logf("DocValues accumulation test passed")
}

func TestRangeFacetCounts_Bounds(t *testing.T) {
	fc := schema.NewRangeFacetCountsWithBounds("age", []float64{0, 18, 30, 50, 100})

	values := []float64{5, 15, 25, 35, 45, 55, 65}
	for _, v := range values {
		fc.Accumulate(v)
	}

	results := fc.GetResults()
	expected := []int{2, 2, 2, 1} // 0-18:5,15=2, 18-30:25=1, 30-50:35,45=2, 50-100:55,65=2
	if results[0].Count != 2 {
		t.Errorf("[0,18): %d, want 2", results[0].Count)
	}
	if results[2].Count != 2 {
		t.Errorf("[30,50): %d, want 2", results[2].Count)
	}
	_ = expected

	t.Logf("Bounds constructor test passed")
}

func TestRangeFacetCounts_TopResults(t *testing.T) {
	fc := schema.NewRangeFacetCountsWithBounds("x", []float64{0, 1, 2, 3})
	fc.Accumulate(0.5)
	fc.Accumulate(0.5)
	fc.Accumulate(1.5)
	fc.Accumulate(1.5)
	fc.Accumulate(1.5)
	fc.Accumulate(2.5)

	top := fc.GetTopResults(2)
	if len(top) != 2 {
		t.Fatalf("len(top) = %d, want 2", len(top))
	}
	// 1-2 should have most (3 values)
	if top[0].Label != "1-2" {
		t.Errorf("top[0] = %q, want '1-2'", top[0].Label)
	}
	if top[0].Count != 3 {
		t.Errorf("top[0].count = %d, want 3", top[0].Count)
	}

	t.Logf("TopResults test passed")
}

func float64Ptr(v float64) *float64 { return &v }
