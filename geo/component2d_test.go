// Tests for the Component2D contract and its static helpers,
// mirroring TestComponent2D.java behaviour where applicable (the
// Java test peer is split across TestPoint2D / TestRectangle2D /
// TestLine2D / TestPolygon2D / TestCircle2D, all of which exercise
// the same contract). The tests in this file cover the helpers
// declared in component2d.go directly and the WithinRelation /
// Relation enum semantics.

package geo

import "testing"

func TestComponent2D_Relation_StringNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   Relation
		want string
	}{
		{CellInsideQuery, "CELL_INSIDE_QUERY"},
		{CellOutsideQuery, "CELL_OUTSIDE_QUERY"},
		{CellCrossesQuery, "CELL_CROSSES_QUERY"},
		{Relation(99), "UNKNOWN"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Relation(%d).String() = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestComponent2D_WithinRelation_StringNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   WithinRelation
		want string
	}{
		{WithinCandidate, "CANDIDATE"},
		{WithinNotWithin, "NOTWITHIN"},
		{WithinDisjoint, "DISJOINT"},
		{WithinRelation(99), "UNKNOWN"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("WithinRelation(%d).String() = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestComponent2D_Disjoint(t *testing.T) {
	t.Parallel()
	if !Disjoint(0, 1, 0, 1, 2, 3, 2, 3) {
		t.Error("disjoint boxes should be Disjoint=true")
	}
	if Disjoint(0, 1, 0, 1, 0.5, 1.5, 0.5, 1.5) {
		t.Error("overlapping boxes should be Disjoint=false")
	}
	if Disjoint(0, 1, 0, 1, 0, 1, 0, 1) {
		t.Error("identical boxes should be Disjoint=false")
	}
}

func TestComponent2D_WithinBBox(t *testing.T) {
	t.Parallel()
	if !WithinBBox(0.1, 0.9, 0.1, 0.9, 0, 1, 0, 1) {
		t.Error("inner box should be Within the outer")
	}
	if WithinBBox(0, 1, 0, 1, 0.1, 0.9, 0.1, 0.9) {
		t.Error("outer should not be Within the inner")
	}
}

func TestComponent2D_BoxContainsPoint(t *testing.T) {
	t.Parallel()
	if !BoxContainsPoint(0.5, 0.5, 0, 1, 0, 1) {
		t.Error("(0.5,0.5) should be Contains'd by (0..1,0..1)")
	}
	if BoxContainsPoint(2, 2, 0, 1, 0, 1) {
		t.Error("(2,2) should not be Contains'd")
	}
	// Inclusive edges.
	if !BoxContainsPoint(0, 1, 0, 1, 0, 1) {
		t.Error("edge point should be Contains'd (inclusive)")
	}
}

func TestComponent2D_PointInTriangle(t *testing.T) {
	t.Parallel()
	// Triangle (0,0), (10,0), (5,10); midpoint (5, 5/3).
	if !PointInTriangle(0, 10, 0, 10, 5, 5, 0, 0, 10, 0, 5, 10) {
		t.Error("(5,5) should be inside the triangle")
	}
	if PointInTriangle(0, 10, 0, 10, 20, 20, 0, 0, 10, 0, 5, 10) {
		t.Error("(20,20) should be outside the triangle")
	}
	// Edge case: vertex.
	if !PointInTriangle(0, 10, 0, 10, 0, 0, 0, 0, 10, 0, 5, 10) {
		t.Error("(0,0) is a vertex and should be Contains'd")
	}
}

func TestComponent2D_DefaultHelpers(t *testing.T) {
	t.Parallel()
	// Build a rectangle2D and exercise the default helpers
	// (IntersectsLineDefault etc.) — they compute the bbox of the
	// input and call the 8-arg method.
	r := newRectangle2D(0, 10, 0, 10)
	if !IntersectsLineDefault(r, 1, 1, 5, 5) {
		t.Error("segment fully inside should intersect")
	}
	if IntersectsLineDefault(r, 20, 20, 25, 25) {
		t.Error("segment fully outside should not intersect")
	}
	if !IntersectsTriangleDefault(r, 1, 1, 2, 2, 3, 3) {
		t.Error("triangle inside rectangle should intersect")
	}
	if !ContainsLineDefault(r, 1, 1, 2, 2) {
		t.Error("rectangle should contain a fully-inside segment")
	}
	if !ContainsTriangleDefault(r, 1, 1, 2, 2, 3, 3) {
		t.Error("rectangle should contain a fully-inside triangle")
	}
	if WithinLineDefault(r, 1, 1, false, 2, 2) != WithinNotWithin {
		t.Error("segment inside should be NOTWITHIN for rectangle2D")
	}
	if WithinTriangleDefault(r, 1, 1, false, 2, 2, false, 3, 3, false) != WithinNotWithin {
		t.Error("triangle inside should be NOTWITHIN for rectangle2D")
	}
}

func TestComponent2D_RectangleWithinTriangle_CandidateOnEdge(t *testing.T) {
	t.Parallel()
	// Rectangle at (5..6, 5..6) sits fully inside a big triangle
	// whose edges do not belong to the original shape — the
	// expected within relation is CANDIDATE.
	r := newRectangle2D(5, 6, 5, 6)
	got := r.WithinTriangle(0, 100, 0, 100,
		0, 0, false,
		100, 0, false,
		50, 100, false)
	if got != WithinCandidate {
		t.Errorf("WithinTriangle = %v; want CANDIDATE", got)
	}
}

func TestComponent2D_MultiAggregatesInsideRelations(t *testing.T) {
	t.Parallel()
	// Two disjoint rectangles; INSIDE on one child must bubble up.
	a := newRectangle2D(0, 10, 0, 10)
	b := newRectangle2D(20, 30, 20, 30)
	multi := newMultiComponent2D([]Component2D{a, b})
	if got := multi.Relate(2, 3, 2, 3); got != CellInsideQuery {
		t.Errorf("multi.Relate inside-of-a = %v; want INSIDE", got)
	}
}

func TestComponent2D_MultiWithinPrecedence(t *testing.T) {
	t.Parallel()
	a := newRectangle2D(0, 10, 0, 10)
	b := newRectangle2D(20, 30, 20, 30)
	multi := newMultiComponent2D([]Component2D{a, b})
	// Point in a is NOTWITHIN for that rectangle; should bubble up.
	if got := multi.WithinPoint(5, 5); got != WithinNotWithin {
		t.Errorf("multi.WithinPoint inside-of-a = %v; want NOTWITHIN", got)
	}
	// Point outside both -> DISJOINT.
	if got := multi.WithinPoint(100, 100); got != WithinDisjoint {
		t.Errorf("multi.WithinPoint outside-both = %v; want DISJOINT", got)
	}
}
