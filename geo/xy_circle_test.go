// Tests for XYCircle, mirroring TestXYCircle.java (Lucene 10.4.0).

package geo

import (
	"errors"
	"math"
	"testing"
)

func TestXYCircle_NegativeRadiusRejected(t *testing.T) {
	t.Parallel()
	_, err := NewXYCircle(0, 0, -1)
	if !errors.Is(err, ErrInvalidXYRadius) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRadius", err)
	}
}

func TestXYCircle_ZeroRadiusRejected(t *testing.T) {
	t.Parallel()
	// XYCircle requires strictly-positive radius (Java uses `<=
	// 0`); Circle (geographic) accepts zero. The tests match the
	// Java implementations.
	_, err := NewXYCircle(0, 0, 0)
	if !errors.Is(err, ErrInvalidXYRadius) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRadius", err)
	}
}

func TestXYCircle_InfiniteRadiusRejected(t *testing.T) {
	t.Parallel()
	_, err := NewXYCircle(0, 0, float32(math.Inf(1)))
	if !errors.Is(err, ErrInvalidXYRadius) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRadius", err)
	}
}

func TestXYCircle_NaNRadiusRejected(t *testing.T) {
	t.Parallel()
	_, err := NewXYCircle(0, 0, float32(math.NaN()))
	if !errors.Is(err, ErrInvalidXYRadius) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRadius", err)
	}
}

func TestXYCircle_InvalidCenterRejected(t *testing.T) {
	t.Parallel()
	_, err := NewXYCircle(float32(math.NaN()), 0, 5)
	if !errors.Is(err, ErrInvalidXYValue) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYValue", err)
	}
}

func TestXYCircle_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewXYCircle(1, 2, 5)
	b := MustNewXYCircle(1, 2, 5)
	if !a.Equals(b) {
		t.Error("equal XYCircles should be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal XYCircles should share hash code")
	}
	c := MustNewXYCircle(1, 2, 6)
	if a.Equals(c) {
		t.Error("distinct XYCircles should not be Equals")
	}
}

func TestXYCircle_String(t *testing.T) {
	t.Parallel()
	c := MustNewXYCircle(1.5, 2.5, 5)
	got := c.String()
	want := "XYCircle([1.5,2.5] radius = 5.0)"
	if got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

func TestXYCircle_ToComponent2DContains(t *testing.T) {
	t.Parallel()
	c := MustNewXYCircle(0, 0, 5)
	comp := c.toComponent2D()
	if !comp.Contains(0, 0) {
		t.Error("centre should be inside")
	}
	if !comp.Contains(3, 4) {
		t.Error("(3,4) at distance 5 should be inside (boundary inclusive)")
	}
	if comp.Contains(10, 10) {
		t.Error("(10,10) should be outside")
	}
}

func TestXYCircle_ToComponent2DRelate(t *testing.T) {
	t.Parallel()
	c := MustNewXYCircle(0, 0, 10)
	comp := c.toComponent2D()
	if got := comp.Relate(-1, 1, -1, 1); got != CellInsideQuery {
		t.Errorf("Relate inside = %v; want INSIDE", got)
	}
	if got := comp.Relate(50, 60, 50, 60); got != CellOutsideQuery {
		t.Errorf("Relate outside = %v; want OUTSIDE", got)
	}
}

func TestXYCircle_SatisfiesXYGeometry(t *testing.T) {
	t.Parallel()
	c := MustNewXYCircle(0, 0, 1)
	if _, ok := any(c).(XYGeometry); !ok {
		t.Error("XYCircle should satisfy XYGeometry")
	}
	if _, ok := any(c).(LatLonGeometry); ok {
		t.Error("XYCircle should NOT satisfy LatLonGeometry")
	}
}
