// Tests for XYRectangle, mirroring TestXYRectangle.java
// (Lucene 10.4.0).

package geo

import (
	"errors"
	"math"
	"testing"
)

func TestXYRectangle_InvalidBoundsRejected(t *testing.T) {
	t.Parallel()
	if _, err := NewXYRectangle(10, 5, 0, 1); !errors.Is(err, ErrInvalidXYRectangleBounds) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRectangleBounds", err)
	}
	if _, err := NewXYRectangle(0, 1, 10, 5); !errors.Is(err, ErrInvalidXYRectangleBounds) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYRectangleBounds", err)
	}
}

func TestXYRectangle_NaNRejected(t *testing.T) {
	t.Parallel()
	if _, err := NewXYRectangle(float32(math.NaN()), 1, 0, 1); !errors.Is(err, ErrInvalidXYValue) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYValue", err)
	}
}

func TestXYRectangle_AccessorsAndString(t *testing.T) {
	t.Parallel()
	r := MustNewXYRectangle(-1, 1, -2, 2)
	if r.MinX() != -1 || r.MaxX() != 1 || r.MinY() != -2 || r.MaxY() != 2 {
		t.Errorf("accessors mismatched")
	}
	got := r.String()
	want := "XYRectangle(x=-1.0 TO 1.0 y=-2.0 TO 2.0)"
	if got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

func TestXYRectangle_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewXYRectangle(0, 1, 0, 1)
	b := MustNewXYRectangle(0, 1, 0, 1)
	if !a.Equals(b) {
		t.Error("equal rectangles should be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal rectangles should share hash code")
	}
	c := MustNewXYRectangle(0, 2, 0, 1)
	if a.Equals(c) {
		t.Error("distinct rectangles should not be Equals")
	}
}

func TestXYRectangle_FromPointDistance(t *testing.T) {
	t.Parallel()
	r, err := FromXYPointDistance(0, 0, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MinX() > 0 || r.MaxX() < 0 || r.MinY() > 0 || r.MaxY() < 0 {
		t.Errorf("bbox should straddle origin: %s", r.String())
	}
}

func TestXYRectangle_FromPointDistance_NegativeRadius(t *testing.T) {
	t.Parallel()
	if _, err := FromXYPointDistance(0, 0, -1); err == nil {
		t.Error("expected error for negative radius")
	}
}

func TestXYRectangle_FromPointDistance_InfiniteRadius(t *testing.T) {
	t.Parallel()
	if _, err := FromXYPointDistance(0, 0, float32(math.Inf(1))); err == nil {
		t.Error("expected error for infinite radius")
	}
}

func TestXYRectangle_ToComponent2D(t *testing.T) {
	t.Parallel()
	r := MustNewXYRectangle(0, 10, 0, 10)
	c := r.toComponent2D()
	if !c.Contains(5, 5) {
		t.Error("component should contain centre")
	}
	if got := c.Relate(1, 2, 1, 2); got != CellInsideQuery {
		t.Errorf("inside relate = %v; want INSIDE", got)
	}
}

func TestXYRectangle_SatisfiesXYGeometry(t *testing.T) {
	t.Parallel()
	r := MustNewXYRectangle(0, 1, 0, 1)
	if _, ok := any(r).(XYGeometry); !ok {
		t.Error("XYRectangle should satisfy XYGeometry")
	}
	if _, ok := any(r).(LatLonGeometry); ok {
		t.Error("XYRectangle should NOT satisfy LatLonGeometry")
	}
}
