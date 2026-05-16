// Tests for Point, mirroring lucene/core/src/test/org/apache/lucene/
// geo/TestPoint.java (Lucene 10.4.0). The Java test peer covers:
//
//   - testInvalidLat: invalid latitude triggers IllegalArgumentException
//     containing "invalid latitude X; must be between -90.0 and 90.0".
//   - testInvalidLon: invalid longitude triggers
//     IllegalArgumentException containing "invalid longitude X; must
//     be between -180.0 and 180.0".
//   - testEqualsAndHashCode: a Point and a copy must be equal and
//     share their hash code; two random points either differ or
//     compare equal consistently.
//
// The Go port replaces Java's exception-based control flow with
// returned errors and uses errors.Is for the sentinel comparison.

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestPoint_InvalidLat(t *testing.T) {
	t.Parallel()

	_, err := NewPoint(134.14, 45.23)
	if err == nil {
		t.Fatal("expected error for invalid latitude")
	}
	if !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("err = %v, want wrapping ErrInvalidLatitude", err)
	}
	want := "invalid latitude 134.14; must be between -90.0 and 90.0"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want substring %q", err.Error(), want)
	}
}

func TestPoint_InvalidLon(t *testing.T) {
	t.Parallel()

	_, err := NewPoint(43.5, 180.5)
	if err == nil {
		t.Fatal("expected error for invalid longitude")
	}
	if !errors.Is(err, ErrInvalidLongitude) {
		t.Fatalf("err = %v, want wrapping ErrInvalidLongitude", err)
	}
	want := "invalid longitude 180.5; must be between -180.0 and 180.0"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want substring %q", err.Error(), want)
	}
}

func TestPoint_BoundsAreInclusive(t *testing.T) {
	t.Parallel()

	// MIN_LAT_INCL, MAX_LAT_INCL, MIN_LON_INCL, MAX_LON_INCL are all
	// accepted as valid inputs (the Lucene constants are `INCL`-suffixed
	// for a reason).
	cases := [][2]float64{
		{MinLatIncl, MinLonIncl},
		{MaxLatIncl, MaxLonIncl},
		{0, 0},
		{MinLatIncl, MaxLonIncl},
		{MaxLatIncl, MinLonIncl},
	}
	for _, c := range cases {
		if _, err := NewPoint(c[0], c[1]); err != nil {
			t.Errorf("NewPoint(%v,%v) = err %v, want nil", c[0], c[1], err)
		}
	}
}

func TestPoint_RejectsNaN(t *testing.T) {
	t.Parallel()
	nan := func() float64 {
		var z float64
		return z / z
	}()
	if _, err := NewPoint(nan, 0); err == nil {
		t.Error("NewPoint(NaN, 0) returned nil err, want invalid latitude")
	}
	if _, err := NewPoint(0, nan); err == nil {
		t.Error("NewPoint(0, NaN) returned nil err, want invalid longitude")
	}
}

func TestPoint_EqualsAndHashCode(t *testing.T) {
	t.Parallel()

	p := MustNewPoint(12.5, 45.25)
	copyP := MustNewPoint(p.Lat(), p.Lon())
	if !p.Equals(copyP) {
		t.Fatal("a Point and an exact copy must be equal")
	}
	if p.HashCode() != copyP.HashCode() {
		t.Fatal("equal Points must share a hash code")
	}

	other := MustNewPoint(-5.0, 100.0)
	if p.Equals(other) {
		t.Fatal("distinct Points must not be equal")
	}
}

func TestPoint_LatLonAccessors(t *testing.T) {
	t.Parallel()
	p := MustNewPoint(12.5, -45.0)
	if got, want := p.Lat(), 12.5; got != want {
		t.Errorf("Lat = %v, want %v", got, want)
	}
	if got, want := p.Lon(), -45.0; got != want {
		t.Errorf("Lon = %v, want %v", got, want)
	}
}

func TestPoint_StringMatchesJava(t *testing.T) {
	t.Parallel()
	// Java emits longitude first: "Point(<lon>,<lat>)".
	p := MustNewPoint(12.5, -45.0)
	if got, want := p.String(), "Point(-45.0,12.5)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPoint_StringRoundTrip(t *testing.T) {
	t.Parallel()
	p := MustNewPoint(12.5, -45.0)
	parsed, err := parsePointString(p.String())
	if err != nil {
		t.Fatalf("parse round trip: %v", err)
	}
	if !parsed.Equals(p) {
		t.Errorf("round trip mismatch: parsed %s, original %s", parsed, p)
	}
}

func TestPoint_ToComponent2DBehaviour(t *testing.T) {
	t.Parallel()
	p := MustNewPoint(10, 20)
	c := p.toComponent2D()
	if got, want := c.MinX(), p.Lon(); got != want {
		t.Errorf("Component2D.MinX = %v, want %v (lon)", got, want)
	}
	if got, want := c.MinY(), p.Lat(); got != want {
		t.Errorf("Component2D.MinY = %v, want %v (lat)", got, want)
	}
	if !c.Contains(p.Lon(), p.Lat()) {
		t.Errorf("Component2D.Contains rejected its own point")
	}
	if c.Contains(p.Lon()+1, p.Lat()) {
		t.Errorf("Component2D.Contains accepted a different point")
	}
	if got := c.Relate(p.Lon()-1, p.Lon()+1, p.Lat()-1, p.Lat()+1); got != CellCrossesQuery {
		t.Errorf("Relate (point in box) = %v, want CellCrossesQuery", got)
	}
	if got := c.Relate(p.Lon()+10, p.Lon()+11, p.Lat()+10, p.Lat()+11); got != CellOutsideQuery {
		t.Errorf("Relate (box outside point) = %v, want CellOutsideQuery", got)
	}
}

func TestPoint_SatisfiesLatLonGeometry(t *testing.T) {
	t.Parallel()
	p := MustNewPoint(0, 0)
	if _, ok := any(p).(LatLonGeometry); !ok {
		t.Fatal("Point should satisfy LatLonGeometry")
	}
	if _, ok := any(p).(XYGeometry); ok {
		t.Fatal("Point should NOT satisfy XYGeometry")
	}
}
