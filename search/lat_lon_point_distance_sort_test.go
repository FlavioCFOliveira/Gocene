// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

// TestLatLonDocValuesDistanceSort_BasicConstruction verifies that
// NewLatLonDocValuesDistanceSort builds a SortField with the correct
// field, reverse=false (ascending), missing-last sentinel (+Inf), and
// a non-nil comparator source. Mirrors the Java factory
// LatLonDocValuesField.newDistanceSort.
func TestLatLonDocValuesDistanceSort_BasicConstruction(t *testing.T) {
	t.Parallel()
	sf, err := NewLatLonDocValuesDistanceSort("location", 37.0, -122.0)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesDistanceSort: %v", err)
	}
	if sf.GetField() != "location" {
		t.Fatalf("GetField: got %q, want %q", sf.GetField(), "location")
	}
	if sf.GetReverse() {
		t.Fatalf("GetReverse: expected ascending (false), got true")
	}
	if sf.MissingValue != math.Inf(1) {
		t.Fatalf("MissingValue: want +Inf sentinel, got %v", sf.MissingValue)
	}
	if sf.GetComparatorSource() == nil {
		t.Fatalf("GetComparatorSource: must not be nil")
	}
}

// TestLatLonDocValuesDistanceSort_RejectsEmptyField verifies that
// an empty field name triggers an error, matching the Java
// IllegalArgumentException.
func TestLatLonDocValuesDistanceSort_RejectsEmptyField(t *testing.T) {
	t.Parallel()
	if _, err := NewLatLonDocValuesDistanceSort("", 0, 0); err == nil {
		t.Fatalf("expected error on empty field")
	}
}

// TestLatLonDocValuesDistanceSort_OriginExtremes verifies that
// constructing a distance sort with extreme origin values (poles,
// dateline) does not panic or error.
func TestLatLonDocValuesDistanceSort_OriginExtremes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		lat  float64
		lon  float64
	}{
		{"northPole", 90, 0},
		{"southPole", -90, 0},
		{"dateline", 0, 180},
		{"greenwich", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewLatLonDocValuesDistanceSort("loc", tc.lat, tc.lon); err != nil {
				t.Fatalf("NewLatLonDocValuesDistanceSort(%v,%v): %v", tc.lat, tc.lon, err)
			}
		})
	}
}

// TestLatLonDocValuesDistanceSort_MissingLastSemantic confirms the
// factory honours missing-last by setting MissingValueLast as the
// Missing field policy.
func TestLatLonDocValuesDistanceSort_MissingLastSemantic(t *testing.T) {
	t.Parallel()
	sf, err := NewLatLonDocValuesDistanceSort("loc", 10, 20)
	if err != nil {
		t.Fatalf("NewLatLonDocValuesDistanceSort: %v", err)
	}
	if sf.Missing != MissingValueLast {
		t.Fatalf("Missing: expected MissingValueLast, got %v", sf.Missing)
	}
}
