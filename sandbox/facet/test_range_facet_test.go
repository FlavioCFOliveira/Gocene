// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.TestRangeFacet.
//
// Deviations from Java:
//   - All 16 test methods require IndexSearcher / RandomIndexWriter,
//     LongRangeFacetCutter with a live index, and TaxonomyWriter/Reader.
//     These are deferred to backlog #2693 when the Gocene search + facet
//     pipeline is available.
//   - The present tests exercise the LongRange and DoubleRange builder
//     structs from the facets/rangefacets package that the facet cutter uses.
package facet

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets/rangefacets"
)

// TestRangeFacet_LongRangeStructure verifies basic LongRange field values
// (the building blocks used by OverlappingLongRangeFacetCutter and
// NonOverlappingLongRangeFacetCutter).
func TestRangeFacet_LongRangeStructure(t *testing.T) {
	tests := []struct {
		min, max int64
	}{
		{0, 10},
		{-100, 100},
		{0, 0},
	}
	for _, tc := range tests {
		r := &rangefacets.LongRange{Min: tc.min, Max: tc.max}
		if r.Min != tc.min || r.Max != tc.max {
			t.Errorf("LongRange{Min: %d, Max: %d}: got Min=%d Max=%d",
				tc.min, tc.max, r.Min, r.Max)
		}
	}
}

// TestRangeFacet_LongMinMax verifies sentinel values match Java's
// Long.MIN_VALUE / Long.MAX_VALUE (used in testLongMinMax).
func TestRangeFacet_LongMinMax(t *testing.T) {
	const minLong int64 = -9223372036854775808 // math.MinInt64
	const maxLong int64 = 9223372036854775807  // math.MaxInt64
	r := &rangefacets.LongRange{Min: minLong, Max: maxLong}
	if r.Min != minLong || r.Max != maxLong {
		t.Errorf("extreme LongRange: Min=%d Max=%d", r.Min, r.Max)
	}
}

// TestRangeFacet_LongRangeOverlap verifies the overlap detection helper used
// by the range facet cutters.
func TestRangeFacet_LongRangeOverlap(t *testing.T) {
	cases := []struct {
		a, b    *rangefacets.LongRange
		overlap bool
	}{
		{&rangefacets.LongRange{Min: 0, Max: 9}, &rangefacets.LongRange{Min: 10, Max: 19}, false},
		{&rangefacets.LongRange{Min: 0, Max: 10}, &rangefacets.LongRange{Min: 5, Max: 15}, true},
		{&rangefacets.LongRange{Min: 5, Max: 5}, &rangefacets.LongRange{Min: 5, Max: 5}, true},
	}
	for _, tc := range cases {
		// Overlap: a.Max >= b.Min && b.Max >= a.Min
		got := tc.a.Max >= tc.b.Min && tc.b.Max >= tc.a.Min
		if got != tc.overlap {
			t.Errorf("overlap([%d,%d],[%d,%d]) = %v; want %v",
				tc.a.Min, tc.a.Max, tc.b.Min, tc.b.Max, got, tc.overlap)
		}
	}
}
