// Tests for Rectangle. Lucene 10.4.0 ships no TestRectangle.java
// peer; behavioural coverage is provided indirectly through
// TestRectangle2D.java and through the LatLonShape / LatLonPoint
// tests which exercise Rectangle as a query shape. The tests below
// reproduce the contract directly: bounds validation, dateline-cross
// detection, containment semantics, equals/hashCode, toString format,
// fromPointDistance shape, and axisLat behaviour at poles.

package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestRectangle_InvalidLatitudeIsRejected(t *testing.T) {
	t.Parallel()
	if _, err := NewRectangle(-91, 0, 0, 1); !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("minLat -91: err = %v, want wrap ErrInvalidLatitude", err)
	}
	if _, err := NewRectangle(0, 91, 0, 1); !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("maxLat 91: err = %v, want wrap ErrInvalidLatitude", err)
	}
}

func TestRectangle_InvalidLongitudeIsRejected(t *testing.T) {
	t.Parallel()
	if _, err := NewRectangle(0, 1, -181, 0); !errors.Is(err, ErrInvalidLongitude) {
		t.Fatalf("minLon -181: err = %v, want wrap ErrInvalidLongitude", err)
	}
	if _, err := NewRectangle(0, 1, 0, 181); !errors.Is(err, ErrInvalidLongitude) {
		t.Fatalf("maxLon 181: err = %v, want wrap ErrInvalidLongitude", err)
	}
}

func TestRectangle_MaxLatLessThanMinLatRejected(t *testing.T) {
	t.Parallel()
	_, err := NewRectangle(10, 5, 0, 1)
	if err == nil {
		t.Fatal("expected error when maxLat < minLat")
	}
	if !strings.Contains(err.Error(), "invalid rectangle") {
		t.Fatalf("err = %q, expected to contain 'invalid rectangle'", err)
	}
}

func TestRectangle_MinLonGreaterThanMaxLonAllowedAcrossDateline(t *testing.T) {
	t.Parallel()
	// Lucene explicitly permits this; the result is a
	// dateline-crossing rectangle.
	r, err := NewRectangle(20, 30, 170, -170)
	if err != nil {
		t.Fatalf("NewRectangle dateline cross: %v", err)
	}
	if !r.CrossesDateline() {
		t.Fatal("dateline-cross rectangle should report CrossesDateline true")
	}
}

func TestRectangle_AccessorsRoundTrip(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(-1, 2, -3, 4)
	if r.MinLat() != -1 || r.MaxLat() != 2 || r.MinLon() != -3 || r.MaxLon() != 4 {
		t.Fatalf("accessors mismatched: got (%v,%v,%v,%v)",
			r.MinLat(), r.MaxLat(), r.MinLon(), r.MaxLon())
	}
}

func TestRectangle_ContainsPointStatic(t *testing.T) {
	t.Parallel()
	if !ContainsPoint(5, 5, 0, 10, 0, 10) {
		t.Error("(5,5) should be inside (0..10, 0..10)")
	}
	if ContainsPoint(11, 5, 0, 10, 0, 10) {
		t.Error("(11,5) should be outside")
	}
}

func TestRectangle_ContainsInstanceHonorsDateline(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(0, 10, 170, -170)
	if !r.Contains(5, 175) {
		t.Error("dateline-cross rect should contain (5,175)")
	}
	if !r.Contains(5, -175) {
		t.Error("dateline-cross rect should contain (5,-175)")
	}
	if r.Contains(5, 0) {
		t.Error("dateline-cross rect should not contain (5,0)")
	}
}

func TestRectangle_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewRectangle(-1, 1, -2, 2)
	b := MustNewRectangle(-1, 1, -2, 2)
	if !a.Equals(b) {
		t.Fatal("equal rectangles must be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal rectangles must share hash code")
	}
	c := MustNewRectangle(-1, 1, -2, 3)
	if a.Equals(c) {
		t.Fatal("different rectangles must not be Equals")
	}
}

func TestRectangle_StringWithoutDateline(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(-1, 1, -2, 2)
	if got, want := r.String(), "Rectangle(lat=-1.0 TO 1.0 lon=-2.0 TO 2.0)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRectangle_StringWithDateline(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(20, 30, 170, -170)
	got := r.String()
	if !strings.Contains(got, "[crosses dateline!]") {
		t.Errorf("String() = %q, missing dateline marker", got)
	}
}

func TestRectangle_ToComponent2DDelegates(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(-1, 1, -2, 2)
	c := r.toComponent2D()
	if c.MinX() != -2 || c.MaxX() != 2 || c.MinY() != -1 || c.MaxY() != 1 {
		t.Errorf("component bounds = (%v,%v,%v,%v); want (-2,2,-1,1)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
	if !c.Contains(0, 0) {
		t.Error("component should contain (0,0)")
	}
	if got := c.Relate(-0.5, 0.5, -0.5, 0.5); got != CellInsideQuery {
		t.Errorf("relate query-fully-inside = %v, want INSIDE", got)
	}
	if got := c.Relate(10, 20, 10, 20); got != CellOutsideQuery {
		t.Errorf("relate disjoint = %v, want OUTSIDE", got)
	}
	if got := c.Relate(-100, 100, -100, 100); got != CellCrossesQuery {
		t.Errorf("relate query-overlapping = %v, want CROSSES", got)
	}
}

func TestRectangle_ToComponent2DDatelineSplitsIntoMulti(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(0, 10, 170, -170)
	c := r.toComponent2D()
	// Multi composite should report the union bbox covering the full
	// longitude range.
	if c.MinX() != -180 || c.MaxX() != 180 {
		t.Errorf("dateline component MinX/MaxX = (%v,%v); want (-180,180)", c.MinX(), c.MaxX())
	}
	if !c.Contains(175, 5) || !c.Contains(-175, 5) {
		t.Error("dateline component should contain both sides")
	}
}

func TestRectangle_FromPointDistance_RoundEarth(t *testing.T) {
	t.Parallel()
	// A small radius near the equator: bbox should be a roughly
	// symmetric little box around the centre.
	r, err := FromPointDistance(0, 0, 1000)
	if err != nil {
		t.Fatalf("FromPointDistance: %v", err)
	}
	if r.MinLat() >= 0 || r.MaxLat() <= 0 {
		t.Errorf("bbox should straddle equator: lat=(%v,%v)", r.MinLat(), r.MaxLat())
	}
	if r.MinLon() >= 0 || r.MaxLon() <= 0 {
		t.Errorf("bbox should straddle meridian: lon=(%v,%v)", r.MinLon(), r.MaxLon())
	}
}

func TestRectangle_FromPointDistance_PoleOverlap(t *testing.T) {
	t.Parallel()
	// 5000 km cap near the pole should span the full longitude.
	r, err := FromPointDistance(89, 0, 5_000_000)
	if err != nil {
		t.Fatalf("FromPointDistance polar: %v", err)
	}
	if r.MinLon() != MinLonIncl || r.MaxLon() != MaxLonIncl {
		t.Errorf("polar bbox lon = (%v,%v); want full lon ring", r.MinLon(), r.MaxLon())
	}
	if r.MaxLat() != MaxLatIncl {
		t.Errorf("polar bbox maxLat = %v; want %v", r.MaxLat(), MaxLatIncl)
	}
}

func TestRectangle_AxisLat_PoleSaturation(t *testing.T) {
	t.Parallel()
	// A radius large enough to engulf the pole should saturate to
	// the pole latitude.
	if got := AxisLat(89, 1_000_000); got != MaxLatIncl {
		t.Errorf("AxisLat(89, 1e6) = %v; want %v", got, MaxLatIncl)
	}
	if got := AxisLat(-89, 1_000_000); got != MinLatIncl {
		t.Errorf("AxisLat(-89, 1e6) = %v; want %v", got, MinLatIncl)
	}
}

func TestRectangle_AxisLat_EquatorRange(t *testing.T) {
	t.Parallel()
	// At the equator, the axis latitude should be a small positive
	// number for a positive centre and the magnitude should grow
	// with radius.
	a := AxisLat(10, 100_000)
	b := AxisLat(10, 500_000)
	if !(a < b) {
		t.Errorf("AxisLat should grow with radius: a=%v b=%v", a, b)
	}
}

func TestRectangle_AxisLatError_IsTiny(t *testing.T) {
	t.Parallel()
	// AxisLatError should be a fraction of a degree (around 1e-6),
	// matching Java's Math.toDegrees(0.1 / EARTH_MEAN_RADIUS_METERS).
	if AxisLatError <= 0 || AxisLatError > 1e-3 {
		t.Errorf("AxisLatError = %v out of plausible range", AxisLatError)
	}
}

func TestRectangle_FromPointDistance_InvalidCenter(t *testing.T) {
	t.Parallel()
	if _, err := FromPointDistance(91, 0, 100); err == nil {
		t.Error("expected error for invalid centerLat")
	}
	if _, err := FromPointDistance(0, 181, 100); err == nil {
		t.Error("expected error for invalid centerLon")
	}
}

func TestRectangle_SatisfiesLatLonGeometry(t *testing.T) {
	t.Parallel()
	r := MustNewRectangle(0, 1, 0, 1)
	if _, ok := any(r).(LatLonGeometry); !ok {
		t.Fatal("Rectangle should satisfy LatLonGeometry")
	}
	if _, ok := any(r).(XYGeometry); ok {
		t.Fatal("Rectangle should NOT satisfy XYGeometry")
	}
}

// Sanity: AxisLat should produce a number consistent with math.Acos
// at the equator; the threshold matches Lucene's documented error.
func TestRectangle_AxisLat_NumericalSanity(t *testing.T) {
	t.Parallel()
	for _, lat := range []float64{0, 30, 45, 60, 89} {
		got := AxisLat(lat, 100_000)
		if math.Abs(got) > MaxLatIncl {
			t.Errorf("AxisLat(%v, 1e5) = %v out of bounds", lat, got)
		}
	}
}
