// Tests for rectangle2D mirror the parts of
// org.apache.lucene.geo.TestRectangle2D and Component2D semantics
// from Apache Lucene 10.4.0 that are relevant to the unexported Go
// port. The Java type is package-private and is exercised
// transitively through LatLonShape / XYShape tests; the Go test
// keeps a focused unit surface so the port's invariants do not
// drift.

package geo

import "testing"

func TestRectangle2D_Bounds(t *testing.T) {
	r := newRectangle2D(-1, 2, -3, 4)
	if r.MinX() != -1 || r.MaxX() != 2 || r.MinY() != -3 || r.MaxY() != 4 {
		t.Fatalf("bbox = (%v,%v,%v,%v); want (-1, 2, -3, 4)",
			r.MinX(), r.MaxX(), r.MinY(), r.MaxY())
	}
}

func TestRectangle2D_Contains(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	cases := []struct {
		name   string
		x, y   float64
		inside bool
	}{
		{"interior", 5, 5, true},
		{"min corner", 0, 0, true},
		{"max corner", 10, 10, true},
		{"on edge", 0, 5, true},
		{"outside left", -1, 5, false},
		{"outside above", 5, 11, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Contains(tc.x, tc.y); got != tc.inside {
				t.Fatalf("Contains(%v, %v) = %v; want %v", tc.x, tc.y, got, tc.inside)
			}
		})
	}
}

func TestRectangle2D_Relate(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	cases := []struct {
		name                   string
		minX, maxX, minY, maxY float64
		want                   Relation
	}{
		// The rectangle IS the query; the four arguments are the cell.
		{"disjoint right", 20, 30, 0, 10, CellOutsideQuery},
		{"disjoint above", 0, 10, 20, 30, CellOutsideQuery},
		{"cell inside rect", 2, 4, 2, 4, CellInsideQuery},
		{"cell encloses rect", -5, 15, -5, 15, CellCrossesQuery},
		{"touch corner", 10, 20, 10, 20, CellCrossesQuery},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Relate(tc.minX, tc.maxX, tc.minY, tc.maxY); got != tc.want {
				t.Fatalf("Relate(%v,%v,%v,%v) = %s; want %s",
					tc.minX, tc.maxX, tc.minY, tc.maxY, got, tc.want)
			}
		})
	}
}

func TestRectangle2D_IntersectsLine(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	cases := []struct {
		name           string
		aX, aY, bX, bY float64
		want           bool
	}{
		{"crosses through", -5, 5, 15, 5, true},
		{"endpoint inside", 5, 5, 20, 20, true},
		{"disjoint", 20, 20, 30, 30, false},
		{"touches edge", 0, 5, -5, 5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			minX, maxX := minFloat(tc.aX, tc.bX), maxFloat(tc.aX, tc.bX)
			minY, maxY := minFloat(tc.aY, tc.bY), maxFloat(tc.aY, tc.bY)
			if got := r.IntersectsLine(minX, maxX, minY, maxY, tc.aX, tc.aY, tc.bX, tc.bY); got != tc.want {
				t.Fatalf("IntersectsLine = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestRectangle2D_IntersectsTriangle(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	// Triangle with one vertex inside (a=(5,5)) and two outside.
	if !r.IntersectsTriangle(-5, 20, -5, 20, 5, 5, -5, -5, 20, -5) {
		t.Fatalf("IntersectsTriangle vertex-in: want true")
	}
	// Stable interior triangle.
	if !r.IntersectsTriangle(1, 4, 1, 4, 1, 1, 4, 1, 4, 4) {
		t.Fatalf("IntersectsTriangle interior: want true")
	}
	// Triangle disjoint from rect.
	if r.IntersectsTriangle(20, 30, 20, 30, 20, 20, 30, 20, 30, 30) {
		t.Fatalf("IntersectsTriangle disjoint: want false")
	}
	// Triangle enclosing the rect.
	if !r.IntersectsTriangle(-100, 100, -100, 100, -100, -100, 100, -100, 0, 100) {
		t.Fatalf("IntersectsTriangle enclosing: want true")
	}
}

func TestRectangle2D_ContainsLineAndTriangle(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	// Segment fully inside.
	if !r.ContainsLine(2, 8, 2, 8, 2, 2, 8, 8) {
		t.Fatalf("ContainsLine inside: want true")
	}
	// Segment partly outside.
	if r.ContainsLine(2, 20, 2, 8, 2, 2, 20, 8) {
		t.Fatalf("ContainsLine partly outside: want false")
	}
	// Triangle fully inside.
	if !r.ContainsTriangle(1, 4, 1, 4, 1, 1, 4, 1, 4, 4) {
		t.Fatalf("ContainsTriangle inside: want true")
	}
}

func TestRectangle2D_WithinPoint(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	if got := r.WithinPoint(5, 5); got != WithinNotWithin {
		t.Fatalf("WithinPoint inside = %s; want NOTWITHIN", got)
	}
	if got := r.WithinPoint(20, 20); got != WithinDisjoint {
		t.Fatalf("WithinPoint outside = %s; want DISJOINT", got)
	}
}

func TestRectangle2D_WithinLine(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	// Endpoint inside -> NOTWITHIN.
	if got := r.WithinLine(5, 20, 5, 20, 5, 5, true, 20, 20); got != WithinNotWithin {
		t.Fatalf("WithinLine endpoint-in = %s; want NOTWITHIN", got)
	}
	// Disjoint -> DISJOINT.
	if got := r.WithinLine(20, 30, 20, 30, 20, 20, true, 30, 30); got != WithinDisjoint {
		t.Fatalf("WithinLine disjoint = %s; want DISJOINT", got)
	}
	// Shape edge crossing rect, ab==true -> NOTWITHIN.
	if got := r.WithinLine(-5, 15, 5, 5, -5, 5, true, 15, 5); got != WithinNotWithin {
		t.Fatalf("WithinLine crossing shape-edge = %s; want NOTWITHIN", got)
	}
	// Same crossing but ab==false -> DISJOINT (no candidate because the
	// "candidate" promotion happens only for triangles).
	if got := r.WithinLine(-5, 15, 5, 5, -5, 5, false, 15, 5); got != WithinDisjoint {
		t.Fatalf("WithinLine crossing non-shape-edge = %s; want DISJOINT", got)
	}
}

func TestRectangle2D_WithinTriangle(t *testing.T) {
	r := newRectangle2D(0, 10, 0, 10)
	// Triangle vertex inside rect -> NOTWITHIN.
	if got := r.WithinTriangle(-5, 15, -5, 15, 5, 5, true, -5, -5, true, 15, -5, true); got != WithinNotWithin {
		t.Fatalf("WithinTriangle vertex-in = %s; want NOTWITHIN", got)
	}
	// Triangle disjoint -> DISJOINT.
	if got := r.WithinTriangle(20, 30, 20, 30, 20, 20, true, 30, 20, true, 30, 30, true); got != WithinDisjoint {
		t.Fatalf("WithinTriangle disjoint = %s; want DISJOINT", got)
	}
	// Triangle enclosing the rect: no rect edge crosses a triangle edge
	// (rect is strictly interior), so the result is decided by the
	// final pointInTriangle branch and is CANDIDATE regardless of the
	// ab/bc/ca flags. Mirrors Java Rectangle2D.withinTriangle exactly.
	if got := r.WithinTriangle(-100, 100, -100, 100, -100, -100, true, 100, -100, true, 0, 100, true); got != WithinCandidate {
		t.Fatalf("WithinTriangle enclosing (rect strictly interior) = %s; want CANDIDATE", got)
	}
	// Triangle whose edge actually crosses the rectangle, ab==true ->
	// NOTWITHIN (shape edge cuts the query rect).
	if got := r.WithinTriangle(-5, 15, 5, 5, -5, 5, true, 15, 5, true, 5, -50, true); got != WithinNotWithin {
		t.Fatalf("WithinTriangle shape-edge crossing = %s; want NOTWITHIN", got)
	}
	// Same crossing but ab==false -> CANDIDATE (non-shape edge hits).
	if got := r.WithinTriangle(-5, 15, 5, 5, -5, 5, false, 15, 5, false, 5, -50, false); got != WithinCandidate {
		t.Fatalf("WithinTriangle non-shape-edge crossing = %s; want CANDIDATE", got)
	}
}

func TestNewRectangle2DFromXY(t *testing.T) {
	xy, err := NewXYRectangle(-1, 2, -3, 4)
	if err != nil {
		t.Fatalf("NewXYRectangle: %v", err)
	}
	c := NewRectangle2DFromXY(xy)
	if c.MinX() != -1 || c.MaxX() != 2 || c.MinY() != -3 || c.MaxY() != 4 {
		t.Fatalf("bbox = (%v,%v,%v,%v); want (-1, 2, -3, 4)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
}

func TestNewRectangle2DFromLatLon_Simple(t *testing.T) {
	rect, err := NewRectangle(-10, 10, -20, 20)
	if err != nil {
		t.Fatalf("NewRectangle: %v", err)
	}
	c := NewRectangle2DFromLatLon(rect)
	// The encoder quantises to a deterministic grid; the bbox lands
	// just inside the requested bounds (ceil on min, floor on max in
	// each axis). Assert the origin is inside and that the quantised
	// bbox stays strictly inside the requested envelope by at most
	// ~1e-7 degrees (the encoder step).
	if !c.Contains(0, 0) {
		t.Fatalf("expected Component to contain the origin; bbox=(%v,%v,%v,%v)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
	const tol = 1e-6
	if c.MinX() < -20-tol || c.MaxX() > 20+tol || c.MinY() < -10-tol || c.MaxY() > 10+tol {
		t.Fatalf("quantised bbox grew past input: got (%v,%v,%v,%v)",
			c.MinX(), c.MaxX(), c.MinY(), c.MaxY())
	}
}

func TestNewRectangle2DFromLatLon_DatelineSplit(t *testing.T) {
	// minLon > maxLon signals a dateline crossing; Java splits into two
	// rectangles wrapped in a ComponentTree.
	rect, err := NewRectangle(-10, 10, 170, -170)
	if err != nil {
		t.Fatalf("NewRectangle: %v", err)
	}
	c := NewRectangle2DFromLatLon(rect)
	// Both sides of the dateline must be reachable.
	if !c.Contains(175, 0) {
		t.Fatalf("expected lon=175 to be inside the wrapped component")
	}
	if !c.Contains(-175, 0) {
		t.Fatalf("expected lon=-175 to be inside the wrapped component")
	}
	// A clearly outside point must be rejected.
	if c.Contains(0, 0) {
		t.Fatalf("lon=0 must be outside a 170..-170 dateline-crossing component")
	}
}

func TestNewRectangle2DFromLatLon_DatelineAt180Collapses(t *testing.T) {
	// minLon == 180 with crossesDateline collapses to a single
	// component anchored at -180.
	rect, err := NewRectangle(-10, 10, 180, -170)
	if err != nil {
		t.Fatalf("NewRectangle: %v", err)
	}
	c := NewRectangle2DFromLatLon(rect)
	if _, ok := c.(*rectangle2D); !ok {
		t.Fatalf("expected a single *rectangle2D, got %T", c)
	}
	if !c.Contains(-175, 0) {
		t.Fatalf("expected lon=-175 to be inside the collapsed component")
	}
}
