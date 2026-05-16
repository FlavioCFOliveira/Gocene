// Tests for XYPoint, mirroring TestXYPoint.java (Lucene 10.4.0). The
// Java test peer covers invalid value rejection and
// equals/hashCode; both are reproduced here.

package geo

import (
	"errors"
	"math"
	"testing"
)

func TestXYPoint_InvalidValueRejected(t *testing.T) {
	t.Parallel()
	_, err := NewXYPoint(float32(math.NaN()), 0)
	if !errors.Is(err, ErrInvalidXYValue) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYValue", err)
	}
	_, err = NewXYPoint(0, float32(math.Inf(1)))
	if !errors.Is(err, ErrInvalidXYValue) {
		t.Fatalf("err = %v; want wrap ErrInvalidXYValue", err)
	}
}

func TestXYPoint_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewXYPoint(1.5, 2.5)
	b := MustNewXYPoint(1.5, 2.5)
	if !a.Equals(b) {
		t.Error("equal XYPoints should be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal XYPoints should share hash code")
	}
	c := MustNewXYPoint(1.5, 3.5)
	if a.Equals(c) {
		t.Error("distinct XYPoints should not be Equals")
	}
}

func TestXYPoint_AccessorsAndString(t *testing.T) {
	t.Parallel()
	p := MustNewXYPoint(1.5, -2.5)
	if p.X() != 1.5 || p.Y() != -2.5 {
		t.Errorf("accessors mismatched: (%v,%v)", p.X(), p.Y())
	}
	if got, want := p.String(), "XYPoint(1.5,-2.5)"; got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

func TestXYPoint_SatisfiesXYGeometry(t *testing.T) {
	t.Parallel()
	p := MustNewXYPoint(0, 0)
	if _, ok := any(p).(XYGeometry); !ok {
		t.Error("XYPoint should satisfy XYGeometry")
	}
	if _, ok := any(p).(LatLonGeometry); ok {
		t.Error("XYPoint should NOT satisfy LatLonGeometry")
	}
}

func TestXYPoint_ToComponent2D(t *testing.T) {
	t.Parallel()
	p := MustNewXYPoint(5, 10)
	c := p.toComponent2D()
	if c.MinX() != 5 || c.MinY() != 10 {
		t.Errorf("component bounds = (%v,%v); want (5,10)", c.MinX(), c.MinY())
	}
	if !c.Contains(5, 10) {
		t.Error("component should contain its own point")
	}
}
