// Tests for Line. Lucene 10.4.0 ships no TestLine.java peer;
// behavioural coverage is provided indirectly through TestLine2D and
// through LatLonShape / LatLonPoint query tests. The Go tests below
// directly verify the public Line contract (constructor errors,
// accessors, defensive copying, equals/hashCode, toString format)
// plus the Component2D-shape behaviour exposed via toComponent2D.

package geo

import (
	"errors"
	"strings"
	"testing"
)

func TestLine_NilLats(t *testing.T) {
	t.Parallel()
	if _, err := NewLine(nil, []float64{0, 1}); !errors.Is(err, ErrNilLats) {
		t.Fatalf("err = %v, want ErrNilLats", err)
	}
}

func TestLine_NilLons(t *testing.T) {
	t.Parallel()
	if _, err := NewLine([]float64{0, 1}, nil); !errors.Is(err, ErrNilLons) {
		t.Fatalf("err = %v, want ErrNilLons", err)
	}
}

func TestLine_LengthMismatch(t *testing.T) {
	t.Parallel()
	_, err := NewLine([]float64{0, 1, 2}, []float64{0, 1})
	if !errors.Is(err, ErrLatLonLengthMismatch) {
		t.Fatalf("err = %v, want ErrLatLonLengthMismatch", err)
	}
}

func TestLine_TooFewPoints(t *testing.T) {
	t.Parallel()
	_, err := NewLine([]float64{0}, []float64{0})
	if !errors.Is(err, ErrTooFewLinePoints) {
		t.Fatalf("err = %v, want ErrTooFewLinePoints", err)
	}
}

func TestLine_InvalidLatRejected(t *testing.T) {
	t.Parallel()
	_, err := NewLine([]float64{0, 91}, []float64{0, 0})
	if !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("err = %v, want wrap ErrInvalidLatitude", err)
	}
}

func TestLine_InvalidLonRejected(t *testing.T) {
	t.Parallel()
	_, err := NewLine([]float64{0, 0}, []float64{0, 181})
	if !errors.Is(err, ErrInvalidLongitude) {
		t.Fatalf("err = %v, want wrap ErrInvalidLongitude", err)
	}
}

func TestLine_BoundingBoxIsComputedFromVertices(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{0, 5, -3}, []float64{10, -7, 2})
	if l.MinLat() != -3 || l.MaxLat() != 5 {
		t.Errorf("lat bbox = (%v,%v); want (-3,5)", l.MinLat(), l.MaxLat())
	}
	if l.MinLon() != -7 || l.MaxLon() != 10 {
		t.Errorf("lon bbox = (%v,%v); want (-7,10)", l.MinLon(), l.MaxLon())
	}
}

func TestLine_NumPointsAndAccessors(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{0, 1, 2}, []float64{10, 11, 12})
	if l.NumPoints() != 3 {
		t.Errorf("NumPoints = %d; want 3", l.NumPoints())
	}
	if l.Lat(1) != 1 || l.Lon(1) != 11 {
		t.Errorf("Lat/Lon(1) = (%v,%v); want (1,11)", l.Lat(1), l.Lon(1))
	}
}

func TestLine_LatsLonsAreDefensiveCopies(t *testing.T) {
	t.Parallel()
	lats := []float64{0, 1}
	lons := []float64{2, 3}
	l, _ := NewLine(lats, lons)

	// Mutate the input — should not affect the line.
	lats[0] = 99
	lons[1] = 99
	if l.Lat(0) == 99 || l.Lon(1) == 99 {
		t.Fatal("constructor failed to copy input slices")
	}

	// Mutate the returned slices — should not affect the line.
	out := l.Lats()
	out[0] = 99
	if l.Lat(0) == 99 {
		t.Fatal("Lats() failed to defensive-copy")
	}
}

func TestLine_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	a := MustNewLine([]float64{0, 1}, []float64{2, 3})
	b := MustNewLine([]float64{0, 1}, []float64{2, 3})
	if !a.Equals(b) {
		t.Fatal("equal lines must be Equals")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal lines must share hash code")
	}
	c := MustNewLine([]float64{0, 1}, []float64{2, 4})
	if a.Equals(c) {
		t.Fatal("distinct lines must not be Equals")
	}
}

func TestLine_StringMatchesJavaFormat(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{1.5, 2.5}, []float64{-1.0, 3.0})
	// Java emits [lon, lat] per vertex without separators between
	// vertices: "Line([-1.0, 1.5][3.0, 2.5])".
	got := l.String()
	want := "Line([-1.0, 1.5][3.0, 2.5])"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLine_ToComponent2D_BoundingBox(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{0, 5}, []float64{0, 10})
	c := l.toComponent2D()
	if c.MinX() != 0 || c.MaxX() != 10 || c.MinY() != 0 || c.MaxY() != 5 {
		t.Errorf("component bbox = (%v,%v,%v,%v); want (0,10,0,5)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
}

func TestLine_ToComponent2D_ContainsOnLine(t *testing.T) {
	t.Parallel()
	// Segment from (0,0) to (10,10); midpoint must be 'contained'.
	l := MustNewLine([]float64{0, 10}, []float64{0, 10})
	c := l.toComponent2D()
	if !c.Contains(5, 5) {
		t.Error("midpoint should lie on the line")
	}
	if c.Contains(5, 6) {
		t.Error("off-line point should not be Contains'd")
	}
	if c.Contains(50, 50) {
		t.Error("point outside bbox should not be Contains'd")
	}
}

func TestLine_ToComponent2D_RelateOutside(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{0, 1}, []float64{0, 1})
	c := l.toComponent2D()
	if got := c.Relate(100, 200, 100, 200); got != CellOutsideQuery {
		t.Errorf("disjoint relate = %v, want OUTSIDE", got)
	}
}

func TestLine_ToComponent2D_RelateCrossesWhenBoxEnclosesLine(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{1, 2}, []float64{1, 2})
	c := l.toComponent2D()
	if got := c.Relate(0, 10, 0, 10); got != CellCrossesQuery {
		t.Errorf("enclosing-box relate = %v, want CROSSES", got)
	}
}

func TestLine_ToComponent2D_RelateCrossesWhenSegmentEntersBox(t *testing.T) {
	t.Parallel()
	// A horizontal segment from (-5,0) to (5,0) and a box around (0,0).
	l := MustNewLine([]float64{0, 0}, []float64{-5, 5})
	c := l.toComponent2D()
	if got := c.Relate(-1, 1, -1, 1); got != CellCrossesQuery {
		t.Errorf("segment-crosses relate = %v, want CROSSES", got)
	}
}

func TestLine_SatisfiesLatLonGeometry(t *testing.T) {
	t.Parallel()
	l := MustNewLine([]float64{0, 1}, []float64{0, 1})
	if _, ok := any(l).(LatLonGeometry); !ok {
		t.Fatal("Line should satisfy LatLonGeometry")
	}
	if _, ok := any(l).(XYGeometry); ok {
		t.Fatal("Line should NOT satisfy XYGeometry")
	}
}

// Sanity for the package-private helpers used by Line2D so that
// changes don't drift silently.
func TestLine_pointOnSegment(t *testing.T) {
	t.Parallel()
	if !pointOnSegment(0, 0, 10, 10, 5, 5) {
		t.Error("(5,5) should be on segment (0,0)-(10,10)")
	}
	if pointOnSegment(0, 0, 10, 10, 5, 6) {
		t.Error("(5,6) should not be on segment (0,0)-(10,10)")
	}
	if pointOnSegment(0, 0, 10, 10, 11, 11) {
		t.Error("(11,11) is collinear but outside segment bbox")
	}
}

func TestLine_segmentsIntersect(t *testing.T) {
	t.Parallel()
	if !segmentsIntersect(0, 0, 10, 10, 0, 10, 10, 0) {
		t.Error("crossing diagonals should intersect")
	}
	if segmentsIntersect(0, 0, 1, 1, 5, 5, 6, 6) {
		t.Error("collinear-disjoint segments should not intersect")
	}
}

// Tiny smoke test to silence the linter about unused private helper.
func TestLine_numPointsString(t *testing.T) {
	t.Parallel()
	if got := strings.Repeat("1", numPointsString(123)); got != "111" {
		t.Errorf("numPointsString(123)=%d", numPointsString(123))
	}
}
