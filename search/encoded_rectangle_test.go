// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
)

// TestEncodedRectangle_Contains_NonWrapping covers the simple
// contiguous-interval contains check: a point is inside iff both X
// and Y fall within the closed bounds.
func TestEncodedRectangle_Contains_NonWrapping(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(-10, 10, -5, 5, false)
	cases := []struct {
		name string
		x, y int32
		want bool
	}{
		{"centre inside", 0, 0, true},
		{"on minX edge", -10, 0, true},
		{"on maxX edge", 10, 0, true},
		{"on minY edge", 0, -5, true},
		{"on maxY edge", 0, 5, true},
		{"x below", -11, 0, false},
		{"x above", 11, 0, false},
		{"y below", 0, -6, false},
		{"y above", 0, 6, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Contains(c.x, c.y); got != c.want {
				t.Fatalf("Contains(%d, %d): got %v, want %v", c.x, c.y, got, c.want)
			}
		})
	}
}

// TestEncodedRectangle_Contains_Wrapping exercises the wrap case
// where the valid X interval is (-inf, maxX] ∪ [minX, +inf).
func TestEncodedRectangle_Contains_Wrapping(t *testing.T) {
	t.Parallel()
	// Wrap rectangle: X is "inside" when x >= 100 OR x <= -100.
	r := NewEncodedRectangle(100, -100, -5, 5, true)
	cases := []struct {
		name string
		x, y int32
		want bool
	}{
		{"in eastern strip", 150, 0, true},
		{"in western strip", -150, 0, true},
		{"in gap (rejected)", 0, 0, false},
		{"on east edge", 100, 0, true},
		{"on west edge", -100, 0, true},
		{"y outside east", 150, -100, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Contains(c.x, c.y); got != c.want {
				t.Fatalf("Contains(%d, %d): got %v, want %v", c.x, c.y, got, c.want)
			}
		})
	}
}

// TestEncodedRectangle_IntersectsLine covers the cheap-reject
// endpoint-contains path, the bounding-box-disjoint path and the
// per-edge fallback.
func TestEncodedRectangle_IntersectsLine(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	cases := []struct {
		name           string
		aX, aY, bX, bY int32
		want           bool
	}{
		{"endpoint inside", -5, 5, 5, 5, true},
		{"both endpoints inside", 2, 2, 8, 8, true},
		{"segment fully above", 0, 100, 10, 100, false},
		{"segment fully below", 0, -100, 10, -100, false},
		{"segment fully left", -100, 5, -1, 5, false},
		{"segment fully right", 11, 5, 100, 5, false},
		{"segment crosses through", -5, 5, 15, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.IntersectsLine(c.aX, c.aY, c.bX, c.bY); got != c.want {
				t.Fatalf("IntersectsLine(%d, %d, %d, %d): got %v, want %v",
					c.aX, c.aY, c.bX, c.bY, got, c.want)
			}
		})
	}
}

// TestEncodedRectangle_IntersectsTriangle covers vertex-inside and
// bounding-box-disjoint fast paths plus the per-edge fallback.
func TestEncodedRectangle_IntersectsTriangle(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	cases := []struct {
		name                   string
		aX, aY, bX, bY, cX, cY int32
		want                   bool
	}{
		{"vertex inside", -5, 5, 5, 5, -5, -5, true},
		{"triangle fully outside", 50, 50, 60, 60, 50, 60, false},
		{"triangle bounding outside Y", 0, 100, 10, 100, 5, 110, false},
		{"triangle bounding outside X", -100, 0, -110, 10, -120, 5, false},
		{"all vertices inside", 1, 1, 2, 2, 3, 3, true},
		{"rectangle inside triangle", -100, -100, 100, -100, 0, 100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.IntersectsTriangle(c.aX, c.aY, c.bX, c.bY, c.cX, c.cY); got != c.want {
				t.Fatalf("IntersectsTriangle: got %v, want %v", got, c.want)
			}
		})
	}
}

// TestEncodedRectangle_IntersectsRectangle covers contiguous and
// wrap-around cases.
func TestEncodedRectangle_IntersectsRectangle(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	if !r.IntersectsRectangle(5, 15, 5, 15) {
		t.Fatalf("IntersectsRectangle overlap: got false, want true")
	}
	if r.IntersectsRectangle(20, 30, 0, 10) {
		t.Fatalf("IntersectsRectangle disjoint: got true, want false")
	}
	if r.IntersectsRectangle(0, 10, 20, 30) {
		t.Fatalf("IntersectsRectangle Y disjoint: got true, want false")
	}
}

// TestEncodedRectangle_ContainsRectangle covers the strict
// containment check.
func TestEncodedRectangle_ContainsRectangle(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(-10, 10, -10, 10, false)
	if !r.ContainsRectangle(-5, 5, -5, 5) {
		t.Fatalf("ContainsRectangle inside: got false, want true")
	}
	if r.ContainsRectangle(-15, 5, -5, 5) {
		t.Fatalf("ContainsRectangle extending left: got true, want false")
	}
}

// TestEncodedRectangle_ContainsLine_NonWrapping covers contiguous
// containment.
func TestEncodedRectangle_ContainsLine_NonWrapping(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	if !r.ContainsLine(1, 1, 9, 9) {
		t.Fatalf("ContainsLine inside: got false, want true")
	}
	if r.ContainsLine(-1, 1, 5, 5) {
		t.Fatalf("ContainsLine partially out: got true, want false")
	}
}

// TestEncodedRectangle_ContainsTriangle_NonWrapping covers
// contiguous triangle containment.
func TestEncodedRectangle_ContainsTriangle_NonWrapping(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	if !r.ContainsTriangle(1, 1, 5, 5, 9, 1) {
		t.Fatalf("ContainsTriangle inside: got false, want true")
	}
	if r.ContainsTriangle(-1, 1, 5, 5, 9, 1) {
		t.Fatalf("ContainsTriangle partially out: got true, want false")
	}
}

// TestEncodedRectangle_WithinLine covers the three return values
// of the within predicate for a single segment.
func TestEncodedRectangle_WithinLine(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	// Segment with both endpoints outside the rectangle and not
	// crossing it → DISJOINT.
	if got := r.WithinLine(-10, 0, true, -5, 0); got != geo.WithinDisjoint {
		t.Fatalf("WithinLine disjoint: got %v, want %v", got, geo.WithinDisjoint)
	}
	// Segment with endpoint inside → NOTWITHIN.
	if got := r.WithinLine(5, 5, true, 15, 5); got != geo.WithinNotWithin {
		t.Fatalf("WithinLine endpoint inside: got %v, want %v", got, geo.WithinNotWithin)
	}
}

// TestEncodedRectangle_WithinTriangle covers vertex-inside ->
// NOTWITHIN and the rectangle-inside-triangle CANDIDATE case.
func TestEncodedRectangle_WithinTriangle(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(0, 10, 0, 10, false)
	// Triangle vertex inside → NOTWITHIN.
	if got := r.WithinTriangle(5, 5, true, 100, 100, true, -100, 100, true); got != geo.WithinNotWithin {
		t.Fatalf("WithinTriangle vertex inside: got %v, want %v", got, geo.WithinNotWithin)
	}
	// Rectangle inside a huge triangle → CANDIDATE.
	if got := r.WithinTriangle(-100, -100, false, 100, -100, false, 0, 100, false); got != geo.WithinCandidate {
		t.Fatalf("WithinTriangle rect inside: got %v, want %v", got, geo.WithinCandidate)
	}

// TestEncodedRectangle_Accessors confirms the accessor methods
// report the constructor-supplied values without mutation.
}
func TestEncodedRectangle_Accessors(t *testing.T) {
	t.Parallel()
	r := NewEncodedRectangle(1, 2, 3, 4, true)
	if r.MinX() != 1 || r.MaxX() != 2 || r.MinY() != 3 || r.MaxY() != 4 {
		t.Fatalf("Accessors: got (%d, %d, %d, %d), want (1, 2, 3, 4)",
			r.MinX(), r.MaxX(), r.MinY(), r.MaxY())
	}
	if !r.WrapsCoordinateSystem() {
		t.Fatalf("WrapsCoordinateSystem: got false, want true")
	}
}