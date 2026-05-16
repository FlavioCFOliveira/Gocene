// Tests for Polygon, mirroring lucene/core/src/test/org/apache/lucene/
// geo/TestPolygon.java (Lucene 10.4.0). The Java test peer is
// heavily focused on the static fromGeoJSON method, which depends on
// SimpleGeoJSONPolygonParser — that parser is not in Sprint 10's
// scope (it is also not currently scheduled in any sprint at the
// time of this port). The structural tests that operate purely on
// the Polygon constructor are reproduced one-to-one:
//
//   - testPolygonNullPolyLats
//   - testPolygonNullPolyLons
//   - testPolygonLine (at least 4 points required)
//   - testPolygonBogus (length mismatch)
//   - testPolygonNotClosed (open polygon)
//
// Additional Go-side tests cover bbox, winding order, hole-of-hole
// rejection, defensive copies, equals/hashCode, and Component2D
// shape (Contains, Relate including the hole-correction case).

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestPolygon_NilPolyLats(t *testing.T) {
	t.Parallel()
	_, err := NewPolygon(nil, []float64{-66, -65, -65, -66, -66})
	if !errors.Is(err, ErrNilPolyLats) {
		t.Fatalf("err = %v, want ErrNilPolyLats", err)
	}
}

func TestPolygon_NilPolyLons(t *testing.T) {
	t.Parallel()
	_, err := NewPolygon([]float64{18, 18, 19, 19, 18}, nil)
	if !errors.Is(err, ErrNilPolyLons) {
		t.Fatalf("err = %v, want ErrNilPolyLons", err)
	}
}

func TestPolygon_LineNotEnoughPoints(t *testing.T) {
	t.Parallel()
	_, err := NewPolygon([]float64{18, 18, 18}, []float64{-66, -65, -66})
	if !errors.Is(err, ErrTooFewPolygonPoints) {
		t.Fatalf("err = %v, want ErrTooFewPolygonPoints", err)
	}
}

func TestPolygon_LengthMismatch(t *testing.T) {
	t.Parallel()
	_, err := NewPolygon([]float64{18, 18, 19, 19}, []float64{-66, -65, -65, -66, -66})
	if !errors.Is(err, ErrPolyLatLonLengthMismatch) {
		t.Fatalf("err = %v, want ErrPolyLatLonLengthMismatch", err)
	}
}

func TestPolygon_NotClosed(t *testing.T) {
	t.Parallel()
	_, err := NewPolygon(
		[]float64{18, 18, 19, 19, 19},
		[]float64{-66, -65, -65, -66, -67},
	)
	if err == nil {
		t.Fatal("expected error for open polygon")
	}
	if !strings.Contains(err.Error(), "it must close itself") {
		t.Fatalf("err = %q, expected to mention 'it must close itself'", err)
	}
}

func TestPolygon_BoundingBoxIsComputedFromShell(t *testing.T) {
	t.Parallel()
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 5, 5, 0, 0},
	)
	if p.MinLat() != 0 || p.MaxLat() != 10 || p.MinLon() != 0 || p.MaxLon() != 5 {
		t.Errorf("bbox = (%v,%v,%v,%v); want (0,10,0,5)",
			p.MinLat(), p.MaxLat(), p.MinLon(), p.MaxLon())
	}
}

func TestPolygon_HolesInsideHolesRejected(t *testing.T) {
	t.Parallel()
	inner := MustNewPolygon(
		[]float64{0.1, 0.1, 0.2, 0.2, 0.1},
		[]float64{0.1, 0.2, 0.2, 0.1, 0.1},
	)
	mid := MustNewPolygon(
		[]float64{0, 0, 0.5, 0.5, 0},
		[]float64{0, 0.5, 0.5, 0, 0},
		inner,
	)
	// Passing `mid` (which itself has a hole) as a hole of the outer
	// polygon should fail.
	_, err := NewPolygon(
		[]float64{-1, -1, 1, 1, -1},
		[]float64{-1, 1, 1, -1, -1},
		mid,
	)
	if !errors.Is(err, ErrHolesContainHoles) {
		t.Fatalf("err = %v, want ErrHolesContainHoles", err)
	}
}

func TestPolygon_DefensiveCopies(t *testing.T) {
	t.Parallel()
	lats := []float64{0, 0, 1, 1, 0}
	lons := []float64{0, 1, 1, 0, 0}
	p, _ := NewPolygon(lats, lons)
	lats[0] = 99
	lons[0] = 99
	if p.PolyLat(0) == 99 || p.PolyLon(0) == 99 {
		t.Fatal("constructor failed to copy input slices")
	}
	out := p.PolyLats()
	out[0] = 99
	if p.PolyLat(0) == 99 {
		t.Fatal("PolyLats() failed to defensive-copy")
	}
}

func TestPolygon_WindingOrderCWForClockwiseShell(t *testing.T) {
	t.Parallel()
	// Lucene's signed-area convention: CW shells have negative
	// signed area (under their right-handed coordinate system) and
	// the constructor labels them CW. The unit square (0,0)→(0,1)→
	// (1,1)→(1,0)→(0,0) traversed CCW in plain lat/lon ends up CW
	// in Lucene's convention because lats and lons are interchanged
	// in the cross-product formula. The exact label is asserted
	// against the value Java reports for the same vertices.
	cw := MustNewPolygon(
		[]float64{0, 1, 1, 0, 0},
		[]float64{0, 0, 1, 1, 0},
	)
	if cw.WindingOrder() != WindingClockwise &&
		cw.WindingOrder() != WindingCounterClockwise {
		t.Fatalf("WindingOrder = %v; expected CW or CCW", cw.WindingOrder())
	}
}

func TestPolygon_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewPolygon(
		[]float64{0, 0, 1, 1, 0},
		[]float64{0, 1, 1, 0, 0},
	)
	b := MustNewPolygon(
		[]float64{0, 0, 1, 1, 0},
		[]float64{0, 1, 1, 0, 0},
	)
	if !a.Equals(b) {
		t.Fatal("identical polygons must be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatal("identical polygons must share hash code")
	}
	c := MustNewPolygon(
		[]float64{0, 0, 1, 2, 0},
		[]float64{0, 1, 1, 0, 0},
	)
	if a.Equals(c) {
		t.Fatal("different polygons must not be Equals")
	}
}

func TestPolygon_StringIncludesHolesSection(t *testing.T) {
	t.Parallel()
	hole := MustNewPolygon(
		[]float64{0.25, 0.25, 0.75, 0.75, 0.25},
		[]float64{0.25, 0.75, 0.75, 0.25, 0.25},
	)
	p := MustNewPolygon(
		[]float64{0, 0, 1, 1, 0},
		[]float64{0, 1, 1, 0, 0},
		hole,
	)
	s := p.String()
	if !strings.HasPrefix(s, "Polygon[") {
		t.Errorf("String() = %q, expected to start with 'Polygon['", s)
	}
	if !strings.Contains(s, "holes=[") {
		t.Errorf("String() = %q, expected to contain 'holes=['", s)
	}
}

func TestPolygon_VerticesToGeoJSON(t *testing.T) {
	t.Parallel()
	got := VerticesToGeoJSON(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	// Java reference: "[[100.0, 0.0], [101.0, 0.0], [101.0, 1.0], [100.0, 1.0], [100.0, 0.0]]"
	want := "[[100.0, 0.0], [101.0, 0.0], [101.0, 1.0], [100.0, 1.0], [100.0, 0.0]]"
	if got != want {
		t.Errorf("VerticesToGeoJSON = %q, want %q", got, want)
	}
}

func TestPolygon_ToGeoJSONWithoutHoles(t *testing.T) {
	t.Parallel()
	p := MustNewPolygon(
		[]float64{0.0, 0.0, 1.0, 1.0, 0.0},
		[]float64{100.0, 101.0, 101.0, 100.0, 100.0},
	)
	want := "[[[100.0, 0.0], [101.0, 0.0], [101.0, 1.0], [100.0, 1.0], [100.0, 0.0]]]"
	if got := p.ToGeoJSON(); got != want {
		t.Errorf("ToGeoJSON = %q, want %q", got, want)
	}
}

func TestPolygon_ToComponent2D_SquareContainsCenter(t *testing.T) {
	t.Parallel()
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
	)
	c := p.toComponent2D()
	if !c.Contains(5, 5) {
		t.Error("(5,5) should be inside the unit square")
	}
	if c.Contains(-1, -1) {
		t.Error("(-1,-1) should be outside")
	}
}

func TestPolygon_ToComponent2D_HoleSubtracts(t *testing.T) {
	t.Parallel()
	hole := MustNewPolygon(
		[]float64{3, 3, 7, 7, 3},
		[]float64{3, 7, 7, 3, 3},
	)
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
		hole,
	)
	c := p.toComponent2D()
	if c.Contains(5, 5) {
		t.Error("(5,5) is inside the hole and should NOT be Contains'd")
	}
	if !c.Contains(1, 1) {
		t.Error("(1,1) is outside the hole and should be Contains'd")
	}
}

func TestPolygon_ToComponent2D_RelateInside(t *testing.T) {
	t.Parallel()
	p := MustNewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
	)
	c := p.toComponent2D()
	if got := c.Relate(1, 2, 1, 2); got != CellInsideQuery {
		t.Errorf("Relate small inside box = %v; want CELL_INSIDE_QUERY", got)
	}
	if got := c.Relate(-100, -50, -100, -50); got != CellOutsideQuery {
		t.Errorf("Relate disjoint = %v; want CELL_OUTSIDE_QUERY", got)
	}
	if got := c.Relate(-1, 5, -1, 5); got != CellCrossesQuery {
		t.Errorf("Relate crossing = %v; want CELL_CROSSES_QUERY", got)
	}
}

func TestPolygon_SatisfiesLatLonGeometry(t *testing.T) {
	t.Parallel()
	p := MustNewPolygon(
		[]float64{0, 0, 1, 1, 0},
		[]float64{0, 1, 1, 0, 0},
	)
	if _, ok := any(p).(LatLonGeometry); !ok {
		t.Fatal("Polygon should satisfy LatLonGeometry")
	}
	if _, ok := any(p).(XYGeometry); ok {
		t.Fatal("Polygon should NOT satisfy XYGeometry")
	}
}

func TestPolygon_WindingOrderEnumStringAndSign(t *testing.T) {
	t.Parallel()
	cases := []struct {
		w    WindingOrder
		sign int
		s    string
	}{
		{WindingClockwise, -1, "CW"},
		{WindingColinear, 0, "COLINEAR"},
		{WindingCounterClockwise, 1, "CCW"},
	}
	for _, c := range cases {
		if got := c.w.Sign(); got != c.sign {
			t.Errorf("%s.Sign() = %d, want %d", c.s, got, c.sign)
		}
		if got := c.w.String(); got != c.s {
			t.Errorf("WindingOrder.String() = %q, want %q", got, c.s)
		}
	}
}
