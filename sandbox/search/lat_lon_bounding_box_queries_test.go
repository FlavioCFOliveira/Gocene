// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.search.TestLatLonBoundingBoxQueries.
//
// Deviations from Java:
//   - testBasics and the BaseRangeFieldQueryTestCase-derived random tests
//     require IndexSearcher / RandomIndexWriter and LatLonBoundingBox query
//     factories (newIntersectsQuery, newContainsQuery, newWithinQuery,
//     newCrossesQuery); deferred to backlog #2693.
//   - testToString requires geo-quantized encoding in the LatLonBoundingBox
//     constructor; the sandbox/document stub does not yet encode coordinates.
//     Deferred to backlog #2693 alongside the query factories.
//   - The present tests cover the GeoBBox predicate logic (isDisjoint, isWithin,
//     contains) which is pure in-memory and self-contained.
package search

import (
	"testing"
)

// geoBBox mirrors the inner GeoBBox class from the Java test.
type geoBBox struct {
	minLat, maxLat, minLon, maxLon float64
}

func (b geoBBox) isDisjoint(o geoBBox) bool {
	if b.minLat > o.maxLat || b.maxLat < o.minLat {
		return true
	}
	if b.minLon > o.maxLon || b.maxLon < o.minLon {
		return true
	}
	return false
}

func (b geoBBox) contains(o geoBBox) bool {
	if b.minLat > o.minLat || b.maxLat < o.maxLat {
		return false
	}
	if b.minLon > o.minLon || b.maxLon < o.maxLon {
		return false
	}
	return true
}

func (b geoBBox) isWithin(o geoBBox) bool { return o.contains(b) }

// TestLatLonBoundingBoxQueries_GeoBBoxIsDisjoint verifies the disjoint predicate.
func TestLatLonBoundingBoxQueries_GeoBBoxIsDisjoint(t *testing.T) {
	tests := []struct {
		a, b    geoBBox
		want    bool
		comment string
	}{
		{
			a:       geoBBox{-10, 10, -10, 10},
			b:       geoBBox{-10, 10, -10, 10},
			want:    false,
			comment: "identical boxes are not disjoint",
		},
		{
			a:       geoBBox{-20, -10, -180, -100},
			b:       geoBBox{0, 10, 0, 14},
			want:    true,
			comment: "lat ranges don't overlap → disjoint",
		},
		{
			a:       geoBBox{0, 10, 14, 20},
			b:       geoBBox{-10, 0, -1, 14},
			want:    false,
			comment: "boxes share an edge point → not disjoint",
		},
	}
	for _, tc := range tests {
		got := tc.a.isDisjoint(tc.b)
		if got != tc.want {
			t.Errorf("%s: isDisjoint = %v; want %v", tc.comment, got, tc.want)
		}
	}
}

// TestLatLonBoundingBoxQueries_GeoBBoxContains verifies the contains predicate.
func TestLatLonBoundingBoxQueries_GeoBBoxContains(t *testing.T) {
	outer := geoBBox{-20, 20, -180, 180}
	inner := geoBBox{-5, 5, -10, 10}

	if !outer.contains(inner) {
		t.Error("outer should contain inner")
	}
	if inner.contains(outer) {
		t.Error("inner should not contain outer")
	}
	if !outer.contains(outer) {
		t.Error("a box should contain itself")
	}
}

// TestLatLonBoundingBoxQueries_GeoBBoxIsWithin verifies the isWithin predicate
// as the inverse of contains.
func TestLatLonBoundingBoxQueries_GeoBBoxIsWithin(t *testing.T) {
	outer := geoBBox{-20, 20, -180, 180}
	inner := geoBBox{-5, 5, -10, 10}

	if !inner.isWithin(outer) {
		t.Error("inner should be within outer")
	}
	if outer.isWithin(inner) {
		t.Error("outer should not be within inner")
	}
}
