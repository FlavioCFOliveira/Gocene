// Tests for XYLine, mirroring TestXYLine.java (Lucene 10.4.0).

package geo

import (
	"errors"
	"testing"
)

func TestXYLine_NilSlicesRejected(t *testing.T) {
	t.Parallel()
	if _, err := NewXYLine(nil, []float32{0, 1}); !errors.Is(err, ErrNilXYLineX) {
		t.Fatalf("err = %v; want ErrNilXYLineX", err)
	}
	if _, err := NewXYLine([]float32{0, 1}, nil); !errors.Is(err, ErrNilXYLineY) {
		t.Fatalf("err = %v; want ErrNilXYLineY", err)
	}
}

func TestXYLine_LengthMismatch(t *testing.T) {
	t.Parallel()
	_, err := NewXYLine([]float32{0, 1}, []float32{0, 1, 2})
	if !errors.Is(err, ErrXYLineLengthMismatch) {
		t.Fatalf("err = %v; want ErrXYLineLengthMismatch", err)
	}
}

func TestXYLine_TooFewPoints(t *testing.T) {
	t.Parallel()
	_, err := NewXYLine([]float32{0}, []float32{0})
	if !errors.Is(err, ErrTooFewXYLinePoints) {
		t.Fatalf("err = %v; want ErrTooFewXYLinePoints", err)
	}
}

func TestXYLine_BoundingBox(t *testing.T) {
	t.Parallel()
	l := MustNewXYLine([]float32{0, 5, -3}, []float32{10, -7, 2})
	if l.MinX() != -3 || l.MaxX() != 5 || l.MinY() != -7 || l.MaxY() != 10 {
		t.Errorf("bbox mismatched: (%v,%v,%v,%v)", l.MinX(), l.MaxX(), l.MinY(), l.MaxY())
	}
}

func TestXYLine_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewXYLine([]float32{0, 1}, []float32{2, 3})
	b := MustNewXYLine([]float32{0, 1}, []float32{2, 3})
	if !a.Equals(b) {
		t.Error("equal XYLines should be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal XYLines should share hash code")
	}
	c := MustNewXYLine([]float32{0, 1}, []float32{2, 4})
	if a.Equals(c) {
		t.Error("distinct XYLines should not be Equals")
	}
}

func TestXYLine_DefensiveCopies(t *testing.T) {
	t.Parallel()
	xs := []float32{0, 1}
	ys := []float32{2, 3}
	l, _ := NewXYLine(xs, ys)
	xs[0] = 99
	if l.X(0) == 99 {
		t.Fatal("constructor failed to copy inputs")
	}
	out := l.Xs()
	out[0] = 99
	if l.X(0) == 99 {
		t.Fatal("Xs() did not defensive-copy")
	}
}

func TestXYLine_String(t *testing.T) {
	t.Parallel()
	l := MustNewXYLine([]float32{1.5, 2.5}, []float32{-1, 3})
	want := "XYLine([1.5, -1.0][2.5, 3.0])"
	if got := l.String(); got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

func TestXYLine_ToComponent2DBoundingBox(t *testing.T) {
	t.Parallel()
	l := MustNewXYLine([]float32{0, 10}, []float32{0, 10})
	c := l.toComponent2D()
	if c.MinX() != 0 || c.MaxX() != 10 || c.MinY() != 0 || c.MaxY() != 10 {
		t.Errorf("component bbox = (%v,%v,%v,%v); want (0,10,0,10)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
}

func TestXYLine_SatisfiesXYGeometry(t *testing.T) {
	t.Parallel()
	l := MustNewXYLine([]float32{0, 1}, []float32{0, 1})
	if _, ok := any(l).(XYGeometry); !ok {
		t.Error("XYLine should satisfy XYGeometry")
	}
	if _, ok := any(l).(LatLonGeometry); ok {
		t.Error("XYLine should NOT satisfy LatLonGeometry")
	}
}
