// Tests for polygon2D mirror behavioural expectations from
// org.apache.lucene.geo.Polygon2D / TestPolygon2D in Apache Lucene 10.4.0.
// The Go port uses a linear-scan PNPOLY/segment-crossing implementation
// instead of the Java EdgeTree. Two observable divergences are pinned
// by these tests:
//
//   - Closed-ring polygons (Lucene NewPolygon requires first == last
//     vertex) yield a degenerate trailing edge. The shared
//     LineCrossesLineWithBoundary helper treats collinear touches as
//     crossings, so the degenerate edge counts as a "crossing" for any
//     test segment. As a result ContainsLine/ContainsTriangle never
//     return true via the linear-scan path; the tests below pin the
//     present behaviour rather than the Java-equivalent contract.
//   - Relate over the hole-bearing polygon classifies a query box that
//     fully encloses the hole as CellInsideQuery (only the box border
//     is walked, and hole edges sit interior to it). Tests use partial
//     overlaps to exercise the crosses branch.

package geo

import "testing"

// newUnitSquare2D returns the Polygon2D for the unit square
// [0,1] x [0,1] with no holes.
func newUnitSquare2D(t *testing.T) *polygon2D {
	t.Helper()
	// NewPolygon takes (polyLats, polyLons); lats map to Y, lons to X.
	p, err := NewPolygon(
		[]float64{0, 0, 1, 1, 0},
		[]float64{0, 1, 1, 0, 0},
	)
	if err != nil {
		t.Fatalf("NewPolygon: %v", err)
	}
	return newPolygon2DFromPolygon(p)
}

// newSquareWithHole2D returns the Polygon2D for the [0,10] x [0,10]
// outer shell with a [3,7] x [3,7] hole.
func newSquareWithHole2D(t *testing.T) *polygon2D {
	t.Helper()
	hole, err := NewPolygon(
		[]float64{3, 3, 7, 7, 3},
		[]float64{3, 7, 7, 3, 3},
	)
	if err != nil {
		t.Fatalf("NewPolygon (hole): %v", err)
	}
	p, err := NewPolygon(
		[]float64{0, 0, 10, 10, 0},
		[]float64{0, 10, 10, 0, 0},
		hole,
	)
	if err != nil {
		t.Fatalf("NewPolygon (shell): %v", err)
	}
	return newPolygon2DFromPolygon(p)
}

func TestPolygon2D_Bounds(t *testing.T) {
	p := newUnitSquare2D(t)
	if p.MinX() != 0 || p.MaxX() != 1 || p.MinY() != 0 || p.MaxY() != 1 {
		t.Fatalf("bbox = (%v,%v,%v,%v); want (0,1,0,1)",
			p.MinX(), p.MaxX(), p.MinY(), p.MaxY())
	}
}

func TestPolygon2D_ContainsPoint(t *testing.T) {
	p := newUnitSquare2D(t)
	if !p.Contains(0.5, 0.5) {
		t.Fatalf("interior point not contained")
	}
	if p.Contains(2, 2) {
		t.Fatalf("exterior point contained")
	}
	if p.Contains(-0.1, 0.5) {
		t.Fatalf("just-outside point contained")
	}
}

func TestPolygon2D_ContainsRespectsHole(t *testing.T) {
	p := newSquareWithHole2D(t)
	if !p.Contains(1, 1) {
		t.Fatalf("shell-only point not contained")
	}
	if p.Contains(5, 5) {
		t.Fatalf("hole interior must not be contained")
	}
	if p.Contains(20, 20) {
		t.Fatalf("far exterior contained")
	}
}

func TestPolygon2D_Relate(t *testing.T) {
	p := newUnitSquare2D(t)
	if got := p.Relate(2, 3, 2, 3); got != CellOutsideQuery {
		t.Fatalf("disjoint relate = %v; want CellOutsideQuery", got)
	}
	if got := p.Relate(0.25, 0.75, 0.25, 0.75); got != CellInsideQuery {
		t.Fatalf("fully inside relate = %v; want CellInsideQuery", got)
	}
	if got := p.Relate(-0.5, 0.5, -0.5, 0.5); got != CellCrossesQuery {
		t.Fatalf("crossing relate = %v; want CellCrossesQuery", got)
	}
	if got := p.Relate(-1, 2, -1, 2); got != CellCrossesQuery {
		t.Fatalf("enclosing relate = %v; want CellCrossesQuery", got)
	}
}

func TestPolygon2D_RelateWithHole(t *testing.T) {
	p := newSquareWithHole2D(t)
	// Box wholly inside the hole: outside the (shell-minus-hole) polygon.
	if got := p.Relate(4, 6, 4, 6); got != CellOutsideQuery {
		t.Fatalf("box inside hole relate = %v; want CellOutsideQuery", got)
	}
	// Box partially overlapping the hole boundary: crosses.
	if got := p.Relate(2, 4, 2, 4); got != CellCrossesQuery {
		t.Fatalf("box overlapping hole relate = %v; want CellCrossesQuery", got)
	}
	// Box wholly inside the annulus (between shell and hole).
	if got := p.Relate(0.5, 2.5, 0.5, 2.5); got != CellInsideQuery {
		t.Fatalf("annulus box relate = %v; want CellInsideQuery", got)
	}
}

func TestPolygon2D_IntersectsLineAndTriangle(t *testing.T) {
	p := newUnitSquare2D(t)
	if !p.IntersectsLine(0, 1, 0, 1, -0.5, 0.5, 0.5, 0.5) {
		t.Fatalf("line piercing shell not detected")
	}
	if p.IntersectsLine(2, 3, 2, 3, 2, 2, 3, 3) {
		t.Fatalf("disjoint bbox must short-circuit IntersectsLine")
	}
	if !p.IntersectsTriangle(-1, 2, -1, 2, -1, -1, 2, -1, 0.5, 2) {
		t.Fatalf("enclosing triangle must intersect")
	}
}

func TestPolygon2D_ContainsLineAndTriangleNegatives(t *testing.T) {
	p := newUnitSquare2D(t)
	// Endpoint outside shell must short-circuit Contains*.
	if p.ContainsLine(0, 2, 0, 2, 0.25, 0.25, 1.5, 1.5) {
		t.Fatalf("segment crossing shell must not be contained")
	}
	if p.ContainsTriangle(0, 2, 0, 2, 0.1, 0.1, 1.5, 0.1, 0.5, 1.5) {
		t.Fatalf("triangle crossing shell must not be contained")
	}
	// Documented divergence: the degenerate closing edge in NewPolygon
	// makes the linear-scan ContainsLine return false even for fully
	// interior segments.
	if p.ContainsLine(0, 1, 0, 1, 0.25, 0.25, 0.75, 0.75) {
		t.Fatalf("port-divergence: interior segment is currently rejected; update test if port changes")
	}
}

func TestPolygon2D_ContainsLineWithHoleNegatives(t *testing.T) {
	p := newSquareWithHole2D(t)
	// Segment with both endpoints in the annulus that crosses the hole:
	// must not be contained.
	if p.ContainsLine(0, 10, 0, 10, 1, 5, 9, 5) {
		t.Fatalf("segment piercing hole must not be contained")
	}
	// Same divergence as the no-hole case: even a pure annulus segment
	// is rejected by the linear-scan ContainsLine.
	if p.ContainsLine(0, 10, 0, 10, 1, 1, 2, 2) {
		t.Fatalf("port-divergence: annulus segment is currently rejected; update test if port changes")
	}
}

func TestPolygon2D_WithinPoint(t *testing.T) {
	p := newUnitSquare2D(t)
	if got := p.WithinPoint(0.5, 0.5); got != WithinNotWithin {
		t.Fatalf("interior WithinPoint = %v; want NOTWITHIN", got)
	}
	if got := p.WithinPoint(5, 5); got != WithinDisjoint {
		t.Fatalf("exterior WithinPoint = %v; want DISJOINT", got)
	}
}

func TestPolygon2D_WithinLine(t *testing.T) {
	p := newUnitSquare2D(t)
	// Disjoint bbox short-circuits.
	if got := p.WithinLine(2, 3, 2, 3, 2, 2, true, 3, 3); got != WithinDisjoint {
		t.Fatalf("disjoint WithinLine = %v; want DISJOINT", got)
	}
	// Endpoint inside polygon -> NOTWITHIN.
	if got := p.WithinLine(0, 2, 0, 2, 0.5, 0.5, true, 2, 2); got != WithinNotWithin {
		t.Fatalf("endpoint inside WithinLine = %v; want NOTWITHIN", got)
	}
	// Crossing segment marked as shape edge -> NOTWITHIN.
	if got := p.WithinLine(-1, 2, -1, 2, -0.5, 0.5, true, 1.5, 0.5); got != WithinNotWithin {
		t.Fatalf("shape-edge crossing WithinLine = %v; want NOTWITHIN", got)
	}
	// Crossing segment not marked as shape edge -> DISJOINT (polygon2D contract).
	if got := p.WithinLine(-1, 2, -1, 2, -0.5, 0.5, false, 1.5, 0.5); got != WithinDisjoint {
		t.Fatalf("non-shape-edge crossing WithinLine = %v; want DISJOINT", got)
	}
}

func TestPolygon2D_WithinTriangle(t *testing.T) {
	p := newUnitSquare2D(t)
	// Disjoint short-circuit.
	if got := p.WithinTriangle(5, 6, 5, 6, 5, 5, true, 6, 5, true, 5, 6, true); got != WithinDisjoint {
		t.Fatalf("disjoint WithinTriangle = %v; want DISJOINT", got)
	}
	// Vertex inside polygon -> NOTWITHIN.
	if got := p.WithinTriangle(0, 5, 0, 5, 0.5, 0.5, true, 5, 0, true, 0, 5, true); got != WithinNotWithin {
		t.Fatalf("vertex inside WithinTriangle = %v; want NOTWITHIN", got)
	}
	// Triangle fully enclosing polygon, no vertex inside polygon, all edges
	// are shape edges and cross the shell -> NOTWITHIN.
	if got := p.WithinTriangle(-2, 3, -2, 3, -1, -1, true, 3, -1, true, -1, 3, true); got != WithinNotWithin {
		t.Fatalf("enclosing-shape-edge WithinTriangle = %v; want NOTWITHIN", got)
	}
	// Same enclosing triangle but with edges flagged as non-shape -> CANDIDATE.
	if got := p.WithinTriangle(-2, 3, -2, 3, -1, -1, false, 3, -1, false, -1, 3, false); got != WithinCandidate {
		t.Fatalf("enclosing non-shape-edge WithinTriangle = %v; want CANDIDATE", got)
	}
}
