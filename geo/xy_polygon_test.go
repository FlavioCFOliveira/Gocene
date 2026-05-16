// Tests for XYPolygon, mirroring TestXYPolygon.java (Lucene 10.4.0).

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestXYPolygon_NilX(t *testing.T) {
	t.Parallel()
	_, err := NewXYPolygon(nil, []float32{0, 0, 1, 0})
	if !errors.Is(err, ErrNilXYPolygonX) {
		t.Fatalf("err = %v", err)
	}
}

func TestXYPolygon_NilY(t *testing.T) {
	t.Parallel()
	_, err := NewXYPolygon([]float32{0, 0, 1, 0}, nil)
	if !errors.Is(err, ErrNilXYPolygonY) {
		t.Fatalf("err = %v", err)
	}
}

func TestXYPolygon_LengthMismatch(t *testing.T) {
	t.Parallel()
	_, err := NewXYPolygon([]float32{0, 0, 1, 0}, []float32{0, 0, 1, 0, 0})
	if !errors.Is(err, ErrXYPolygonLengthMismatch) {
		t.Fatalf("err = %v", err)
	}
}

func TestXYPolygon_TooFewPoints(t *testing.T) {
	t.Parallel()
	_, err := NewXYPolygon([]float32{0, 0, 0}, []float32{0, 1, 0})
	if !errors.Is(err, ErrTooFewXYPolygonPoints) {
		t.Fatalf("err = %v", err)
	}
}

func TestXYPolygon_NotClosed(t *testing.T) {
	t.Parallel()
	_, err := NewXYPolygon([]float32{0, 0, 1, 1, 0}, []float32{0, 1, 1, 0, 1})
	if err == nil || !strings.Contains(err.Error(), "it must close itself") {
		t.Fatalf("expected closure error; got: %v", err)
	}
}

func TestXYPolygon_HolesContainHolesRejected(t *testing.T) {
	t.Parallel()
	inner := MustNewXYPolygon(
		[]float32{0.1, 0.1, 0.2, 0.2, 0.1},
		[]float32{0.1, 0.2, 0.2, 0.1, 0.1},
	)
	mid := MustNewXYPolygon(
		[]float32{0, 0, 0.5, 0.5, 0},
		[]float32{0, 0.5, 0.5, 0, 0},
		inner,
	)
	_, err := NewXYPolygon(
		[]float32{-1, -1, 1, 1, -1},
		[]float32{-1, 1, 1, -1, -1},
		mid,
	)
	if !errors.Is(err, ErrXYHolesContainHoles) {
		t.Fatalf("err = %v", err)
	}
}

func TestXYPolygon_AccessorsAndBoundingBox(t *testing.T) {
	t.Parallel()
	p := MustNewXYPolygon(
		[]float32{0, 0, 5, 5, 0},
		[]float32{0, 5, 5, 0, 0},
	)
	if p.MinX() != 0 || p.MaxX() != 5 || p.MinY() != 0 || p.MaxY() != 5 {
		t.Errorf("bbox = (%v,%v,%v,%v); want (0,5,0,5)",
			p.MinX(), p.MaxX(), p.MinY(), p.MaxY())
	}
	if p.NumPoints() != 5 {
		t.Errorf("NumPoints = %d; want 5", p.NumPoints())
	}
	if p.NumHoles() != 0 {
		t.Errorf("NumHoles = %d; want 0", p.NumHoles())
	}
}

func TestXYPolygon_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewXYPolygon([]float32{0, 0, 1, 1, 0}, []float32{0, 1, 1, 0, 0})
	b := MustNewXYPolygon([]float32{0, 0, 1, 1, 0}, []float32{0, 1, 1, 0, 0})
	if !a.Equals(b) {
		t.Error("equal XYPolygons should be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal XYPolygons should share hash code")
	}
	c := MustNewXYPolygon([]float32{0, 0, 2, 2, 0}, []float32{0, 1, 1, 0, 0})
	if a.Equals(c) {
		t.Error("distinct XYPolygons should not be Equals")
	}
}

func TestXYPolygon_VerticesToGeoJSON(t *testing.T) {
	t.Parallel()
	got := XYVerticesToGeoJSON(
		[]float32{0, 0, 1, 1, 0},
		[]float32{0, 1, 1, 0, 0},
	)
	want := "[[0.0, 0.0], [0.0, 1.0], [1.0, 1.0], [1.0, 0.0], [0.0, 0.0]]"
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestXYPolygon_ToComponent2DContainsCentre(t *testing.T) {
	t.Parallel()
	p := MustNewXYPolygon(
		[]float32{0, 0, 10, 10, 0},
		[]float32{0, 10, 10, 0, 0},
	)
	c := p.toComponent2D()
	if !c.Contains(5, 5) {
		t.Error("(5,5) should be inside the square")
	}
}

func TestXYPolygon_SatisfiesXYGeometry(t *testing.T) {
	t.Parallel()
	p := MustNewXYPolygon([]float32{0, 0, 1, 1, 0}, []float32{0, 1, 1, 0, 0})
	if _, ok := any(p).(XYGeometry); !ok {
		t.Error("XYPolygon should satisfy XYGeometry")
	}
	if _, ok := any(p).(LatLonGeometry); ok {
		t.Error("XYPolygon should NOT satisfy LatLonGeometry")
	}
}
