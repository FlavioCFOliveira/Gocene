// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package rangefacets

import "testing"

// TestExclusiveCounter verifies basic single-valued counting with non-overlapping ranges.
func TestExclusiveCounter(t *testing.T) {
	ranges := []*LongRange{
		NewLongRange("0-9", 0, true, 9, true),
		NewLongRange("10-19", 10, true, 19, true),
		NewLongRange("20-29", 20, true, 29, true),
	}
	counts := make([]int, 3)
	c := NewLongRangeCounter(ranges, counts)

	for v := int64(0); v < 30; v++ {
		c.AddSingleValued(v)
	}
	c.AddSingleValued(100) // outside all ranges
	c.Finish()

	if counts[0] != 10 || counts[1] != 10 || counts[2] != 10 {
		t.Errorf("exclusive counts = %v; want [10 10 10]", counts)
	}
}

// TestOverlappingCounter verifies single-valued counting with overlapping ranges.
func TestOverlappingCounter(t *testing.T) {
	ranges := []*LongRange{
		NewLongRange("0-9", 0, true, 9, true),
		NewLongRange("5-14", 5, true, 14, true),
	}
	counts := make([]int, 2)
	c := NewLongRangeCounter(ranges, counts)

	for v := int64(0); v < 15; v++ {
		c.AddSingleValued(v)
	}
	c.Finish()

	// 0-4: only range[0]; 5-9: both; 10-14: only range[1]
	if counts[0] != 10 || counts[1] != 10 {
		t.Errorf("overlapping counts = %v; want [10 10]", counts)
	}
}

// TestMultiValuedCounter verifies multi-valued doc counting with non-overlapping ranges.
func TestMultiValuedCounter(t *testing.T) {
	ranges := []*LongRange{
		NewLongRange("low", 0, true, 4, true),
		NewLongRange("high", 5, true, 9, true),
	}
	counts := make([]int, 2)
	c := NewLongRangeCounter(ranges, counts)

	// Doc with values [1, 6]: matches both ranges — each range gets +1 once.
	c.StartMultiValuedDoc()
	c.AddMultiValued(1)
	c.AddMultiValued(6)
	c.EndMultiValuedDoc()

	c.Finish()

	if counts[0] != 1 || counts[1] != 1 {
		t.Errorf("multivalued counts = %v; want [1 1]", counts)
	}
}
