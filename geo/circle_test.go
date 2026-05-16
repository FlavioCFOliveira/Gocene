// Tests for Circle, mirroring lucene/core/src/test/org/apache/lucene/
// geo/TestCircle.java (Lucene 10.4.0).
//
// Java test peer covers:
//
//   - testInvalidLat
//   - testInvalidLon
//   - testNegativeRadius
//   - testInfiniteRadius
//   - testEqualsAndHashCode
//
// All five are reproduced; Component2D-shape regressions are added.

package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestCircle_InvalidLat(t *testing.T) {
	t.Parallel()
	_, err := NewCircle(134.14, 45.23, 1000)
	if !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("err = %v, want wrap ErrInvalidLatitude", err)
	}
	if !strings.Contains(err.Error(), "invalid latitude 134.14; must be between -90.0 and 90.0") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestCircle_InvalidLon(t *testing.T) {
	t.Parallel()
	_, err := NewCircle(43.5, 180.5, 1000)
	if !errors.Is(err, ErrInvalidLongitude) {
		t.Fatalf("err = %v, want wrap ErrInvalidLongitude", err)
	}
	if !strings.Contains(err.Error(), "invalid longitude 180.5; must be between -180.0 and 180.0") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestCircle_NegativeRadius(t *testing.T) {
	t.Parallel()
	_, err := NewCircle(43.5, 45.23, -1000)
	if !errors.Is(err, ErrInvalidRadius) {
		t.Fatalf("err = %v, want wrap ErrInvalidRadius", err)
	}
	if !strings.Contains(err.Error(), "radiusMeters: '-1000.0' is invalid") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestCircle_InfiniteRadius(t *testing.T) {
	t.Parallel()
	_, err := NewCircle(43.5, 45.23, math.Inf(1))
	if !errors.Is(err, ErrInvalidRadius) {
		t.Fatalf("err = %v, want wrap ErrInvalidRadius", err)
	}
	if !strings.Contains(err.Error(), "radiusMeters: 'Infinity' is invalid") {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

func TestCircle_NaNRadius(t *testing.T) {
	t.Parallel()
	_, err := NewCircle(0, 0, math.NaN())
	if !errors.Is(err, ErrInvalidRadius) {
		t.Fatalf("err = %v, want wrap ErrInvalidRadius", err)
	}
	if !strings.Contains(err.Error(), "radiusMeters: 'NaN' is invalid") {
		t.Fatalf("unexpected NaN message: %q", err.Error())
	}
}

func TestCircle_ZeroRadiusAllowed(t *testing.T) {
	t.Parallel()
	// Java accepts radius=0; the resulting circle contains only the
	// centre point.
	if _, err := NewCircle(0, 0, 0); err != nil {
		t.Fatalf("zero radius should be allowed: %v", err)
	}
}

func TestCircle_AccessorsRoundTrip(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(48.8584, 2.2945, 1000)
	if c.Lat() != 48.8584 || c.Lon() != 2.2945 || c.Radius() != 1000 {
		t.Errorf("accessors mismatched: lat=%v lon=%v r=%v",
			c.Lat(), c.Lon(), c.Radius())
	}
}

func TestCircle_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewCircle(48.8584, 2.2945, 1000)
	b := MustNewCircle(48.8584, 2.2945, 1000)
	if !a.Equals(b) {
		t.Fatal("equal circles must be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal circles must share hash code")
	}
	c := MustNewCircle(48.8584, 2.2945, 2000)
	if a.Equals(c) {
		t.Fatal("circles with different radius must not be Equals")
	}
}

func TestCircle_StringMatchesJavaFormat(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(48.5, 2.5, 1000)
	got := c.String()
	want := "Circle([48.5,2.5] radius = 1000.0 meters)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCircle_ToComponent2D_ContainsCentre(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(0, 0, 10_000)
	comp := c.toComponent2D()
	if !comp.Contains(0, 0) {
		t.Error("Contains should accept the centre point")
	}
}

func TestCircle_ToComponent2D_BoundingBoxCoversCentre(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(10, 20, 5000)
	comp := c.toComponent2D()
	// Centre lon = 20, centre lat = 10 — these are X and Y in the
	// Component2D convention (X = lon, Y = lat).
	if comp.MinX() > 20 || comp.MaxX() < 20 {
		t.Errorf("bbox X = (%v,%v) does not span centre lon 20", comp.MinX(), comp.MaxX())
	}
	if comp.MinY() > 10 || comp.MaxY() < 10 {
		t.Errorf("bbox Y = (%v,%v) does not span centre lat 10", comp.MinY(), comp.MaxY())
	}
}

func TestCircle_ToComponent2D_RelateDisjoint(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(0, 0, 100)
	comp := c.toComponent2D()
	if got := comp.Relate(50, 60, 50, 60); got != CellOutsideQuery {
		t.Errorf("relate disjoint = %v, want OUTSIDE", got)
	}
}

func TestCircle_ToComponent2D_RelateInside(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(0, 0, 100_000) // 100 km radius
	comp := c.toComponent2D()
	// A 1m x 1m box centred at (0,0) is well inside the disk.
	if got := comp.Relate(-1e-5, 1e-5, -1e-5, 1e-5); got != CellInsideQuery {
		t.Errorf("relate inside = %v, want INSIDE", got)
	}
}

func TestCircle_ToComponent2D_RelateCrosses(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(0, 0, 100_000)
	comp := c.toComponent2D()
	// Box centred at the edge of the circle should partially
	// intersect.
	bx := comp.MaxX() - 0.001
	by := comp.MaxY() - 0.001
	if got := comp.Relate(bx-0.5, bx+0.5, by-0.5, by+0.5); got != CellCrossesQuery {
		t.Errorf("relate at edge = %v, want CROSSES", got)
	}
}

func TestCircle_SatisfiesLatLonGeometry(t *testing.T) {
	t.Parallel()
	c := MustNewCircle(0, 0, 100)
	if _, ok := any(c).(LatLonGeometry); !ok {
		t.Fatal("Circle should satisfy LatLonGeometry")
	}
	if _, ok := any(c).(XYGeometry); ok {
		t.Fatal("Circle should NOT satisfy XYGeometry")
	}
}
