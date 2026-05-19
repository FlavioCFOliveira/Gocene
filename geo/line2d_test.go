// Tests for line2D mirror behavioural expectations from
// org.apache.lucene.geo.Line2D / TestLine2D in Apache Lucene 10.4.0.

package geo

import "testing"

func newDiagonalLine2D(t *testing.T) *line2D {
	t.Helper()
	l, err := NewLine([]float64{0, 1, 2}, []float64{0, 1, 2})
	if err != nil {
		t.Fatalf("NewLine: %v", err)
	}
	return newLine2DFromLine(l)
}

func TestLine2D_Bounds(t *testing.T) {
	l := newDiagonalLine2D(t)
	if l.MinX() != 0 || l.MaxX() != 2 || l.MinY() != 0 || l.MaxY() != 2 {
		t.Fatalf("bbox = (%v,%v,%v,%v); want (0,2,0,2)",
			l.MinX(), l.MaxX(), l.MinY(), l.MaxY())
	}
}

func TestLine2D_ContainsAndRelate(t *testing.T) {
	l := newDiagonalLine2D(t)
	if !l.Contains(1, 1) {
		t.Fatalf("expected (1,1) on the diagonal")
	}
	if l.Contains(1, 0) {
		t.Fatalf("did not expect (1,0) on the diagonal")
	}
	if got := l.Relate(-1, -0.5, -1, -0.5); got != CellOutsideQuery {
		t.Fatalf("disjoint relate = %v; want CellOutsideQuery", got)
	}
	if got := l.Relate(-1, 5, -1, 5); got != CellCrossesQuery {
		t.Fatalf("enclosing relate = %v; want CellCrossesQuery", got)
	}
	if got := l.Relate(0.5, 1.5, 0.5, 1.5); got != CellCrossesQuery {
		t.Fatalf("crossing relate = %v; want CellCrossesQuery", got)
	}
}

func TestLine2D_IntersectsAndContainsZeroArea(t *testing.T) {
	l := newDiagonalLine2D(t)
	if !l.IntersectsLine(0, 2, 0, 2, 0, 2, 2, 0) {
		t.Fatalf("expected anti-diagonal to intersect")
	}
	if l.IntersectsLine(-1, -0.5, -1, -0.5, -1, -1, -0.5, -0.5) {
		t.Fatalf("disjoint bbox must short-circuit IntersectsLine")
	}
	if !l.IntersectsTriangle(-1, 2, -1, 2, 0, 2, 2, 0, -1, -1) {
		t.Fatalf("expected triangle to intersect line")
	}
	if l.ContainsLine(0, 1, 0, 1, 0, 0, 1, 1) {
		t.Fatalf("ContainsLine must always be false")
	}
	if l.ContainsTriangle(0, 1, 0, 1, 0, 0, 1, 1, 0, 1) {
		t.Fatalf("ContainsTriangle must always be false")
	}
}

func TestLine2D_WithinPoint(t *testing.T) {
	l := newDiagonalLine2D(t)
	if got := l.WithinPoint(1, 1); got != WithinNotWithin {
		t.Fatalf("point on line WithinPoint = %v; want NOTWITHIN", got)
	}
	if got := l.WithinPoint(5, 5); got != WithinDisjoint {
		t.Fatalf("point off line WithinPoint = %v; want DISJOINT", got)
	}
}

func TestLine2D_WithinLineShapeEdgeFlag(t *testing.T) {
	l := newDiagonalLine2D(t)
	// Crossing segment, marked as shape edge -> NOTWITHIN.
	if got := WithinLineDefault(l, 0, 2, true, 2, 0); got != WithinNotWithin {
		t.Fatalf("ab=true crossing = %v; want NOTWITHIN", got)
	}
	// Same crossing, not a shape edge -> DISJOINT (line cannot be within
	// a non-shape segment).
	if got := WithinLineDefault(l, 0, 2, false, 2, 0); got != WithinDisjoint {
		t.Fatalf("ab=false crossing = %v; want DISJOINT", got)
	}
	if got := WithinLineDefault(l, 10, 10, true, 11, 11); got != WithinDisjoint {
		t.Fatalf("disjoint = %v; want DISJOINT", got)
	}
}

func TestLine2D_WithinTriangle(t *testing.T) {
	l := newDiagonalLine2D(t)

	// Disjoint triangle -> DISJOINT.
	if got := WithinTriangleDefault(l, 10, 10, true, 11, 10, true, 10, 11, true); got != WithinDisjoint {
		t.Fatalf("disjoint triangle = %v; want DISJOINT", got)
	}

	// Triangle whose edges all cross the line and are shape edges -> NOTWITHIN.
	if got := WithinTriangleDefault(l,
		-1, 3, true,
		3, 3, true,
		1, -1, true,
	); got != WithinNotWithin {
		t.Fatalf("shape-edge triangle = %v; want NOTWITHIN", got)
	}

	// Same triangle but no edge belongs to the shape -> CANDIDATE because
	// the line's first vertex lies inside the triangle.
	if got := WithinTriangleDefault(l,
		-1, 3, false,
		3, 3, false,
		1, -1, false,
	); got != WithinCandidate {
		t.Fatalf("non-shape-edge triangle = %v; want CANDIDATE", got)
	}
}
