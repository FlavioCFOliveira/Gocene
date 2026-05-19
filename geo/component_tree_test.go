// Tests for componentTree, the kd-tree composite ported from
// org.apache.lucene.geo.ComponentTree. Lucene 10.4.0 does not ship a
// dedicated TestComponentTree.java (its behaviour is covered by the
// per-shape integration tests and by the LatLonShape / XYShape
// random tests). The tests here therefore exercise the contract
// indirectly: every Component2D method must produce the same
// observable answer as the linear-scan multiComponent2D composite
// over the same set of leaves, and the single-component pass-
// through and IllegalArgumentException-equivalent paths must match
// the Java reference.

package geo

import "testing"

// testTreeRectangles builds a stable set of overlapping and
// disjoint rectangles used as Component2D leaves for the parity
// tests. The set is intentionally larger than 2 so the tree depth
// is at least 2 and both X- and Y-splitting layers exercise.
func testTreeRectangles() []Component2D {
	return []Component2D{
		newRectangle2D(0, 10, 0, 10),
		newRectangle2D(5, 15, 5, 15),
		newRectangle2D(20, 30, 20, 30),
		newRectangle2D(40, 50, 0, 10),
		newRectangle2D(-5, 5, -5, 5),
		newRectangle2D(100, 110, 100, 110),
		newRectangle2D(7, 9, 7, 9),
	}
}

// cloneComponents returns a fresh slice with the same elements so
// that newComponentTree's in-place permutation cannot leak across
// the parity test fixtures.
func cloneComponents(in []Component2D) []Component2D {
	out := make([]Component2D, len(in))
	copy(out, in)
	return out
}

func TestComponentTree_SingleComponentIsPassThrough(t *testing.T) {
	t.Parallel()
	leaf := newRectangle2D(0, 10, 0, 10)
	got := newComponentTree([]Component2D{leaf})
	if got != Component2D(leaf) {
		t.Fatalf("newComponentTree with one input must return the leaf unchanged; got %T", got)
	}
}

func TestComponentTree_EmptyInputPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("newComponentTree with no inputs must panic")
		}
	}()
	_ = newComponentTree(nil)
}

func TestComponentTree_BoundingBoxMatchesMultiComposite(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	if tree.MinX() != multi.MinX() ||
		tree.MaxX() != multi.MaxX() ||
		tree.MinY() != multi.MinY() ||
		tree.MaxY() != multi.MaxY() {
		t.Fatalf("tree bbox %v..%v / %v..%v; want %v..%v / %v..%v",
			tree.MinX(), tree.MaxX(), tree.MinY(), tree.MaxY(),
			multi.MinX(), multi.MaxX(), multi.MinY(), multi.MaxY())
	}
}

// containsCases samples points across the union, inside individual
// leaves, on shared boundaries, and outside every leaf.
func containsCases() []struct{ x, y float64 } {
	return []struct{ x, y float64 }{
		{0, 0}, {5, 5}, {7, 8}, {9, 9}, {15, 15},
		{25, 25}, {45, 5}, {-5, -5}, {-10, -10},
		{100, 100}, {105, 105}, {110, 110}, {111, 111},
		{20, 5}, {17, 17}, {3, 12}, {12, 3},
	}
}

func TestComponentTree_ContainsParity(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	for _, p := range containsCases() {
		want := multi.Contains(p.x, p.y)
		got := tree.Contains(p.x, p.y)
		if got != want {
			t.Errorf("Contains(%v,%v) tree=%v multi=%v", p.x, p.y, got, want)
		}
	}
}

// relateCases samples query rectangles that fall fully inside,
// fully outside, and straddle the leaves.
func relateCases() []struct{ minX, maxX, minY, maxY float64 } {
	return []struct{ minX, maxX, minY, maxY float64 }{
		{1, 2, 1, 2},      // fully inside leaf 0
		{6, 8, 6, 8},      // inside leaf 0 + leaf 1 + leaf 6
		{22, 28, 22, 28},  // fully inside leaf 2
		{200, 210, 0, 10}, // disjoint from every leaf
		{0, 100, 0, 100},  // covers many leaves
		{-3, 3, -3, 3},    // inside leaf 4
		{105, 108, 105, 108},
		{8, 12, 8, 12},
		{50, 60, 50, 60},
	}
}

// TestComponentTree_RelateConsistency verifies the tree's Relate
// against the per-leaf scan that mirrors the Java contract:
//
//   - OUTSIDE is only returned when every leaf is OUTSIDE;
//   - any other answer must equal the relation of at least one leaf.
//
// The tree is not required to match multiComponent2D's union answer
// (which always reports INSIDE when any leaf reports INSIDE): the
// Java ComponentTree.relate short-circuits on the first non-OUTSIDE
// leaf in traversal order, so a CROSSES leaf visited before an
// INSIDE leaf legitimately yields CROSSES.
func TestComponentTree_RelateConsistency(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	scan := cloneComponents(leaves)
	tree := newComponentTree(cloneComponents(leaves))
	for _, r := range relateCases() {
		perLeaf := make([]Relation, 0, len(scan))
		allOutside := true
		for _, c := range scan {
			rel := c.Relate(r.minX, r.maxX, r.minY, r.maxY)
			perLeaf = append(perLeaf, rel)
			if rel != CellOutsideQuery {
				allOutside = false
			}
		}
		got := tree.Relate(r.minX, r.maxX, r.minY, r.maxY)
		if allOutside {
			if got != CellOutsideQuery {
				t.Errorf("Relate(%v,%v,%v,%v) tree=%v; all leaves OUTSIDE, want OUTSIDE",
					r.minX, r.maxX, r.minY, r.maxY, got)
			}
			continue
		}
		matched := false
		for _, rel := range perLeaf {
			if rel == got {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("Relate(%v,%v,%v,%v) tree=%v; expected one of leaves=%v",
				r.minX, r.maxX, r.minY, r.maxY, got, perLeaf)
		}
	}
}

// segmentCases provides line endpoints whose bounding box covers
// every interesting subregion of the rectangle fixture.
func segmentCases() []struct{ aX, aY, bX, bY float64 } {
	return []struct{ aX, aY, bX, bY float64 }{
		{0, 0, 10, 10},
		{5, 5, 25, 25},
		{45, 0, 45, 10},
		{-10, -10, -1, -1},
		{0, 0, 100, 100},
		{15, 15, 20, 20},
		{105, 105, 108, 108},
		{50, 50, 60, 60},
	}
}

func TestComponentTree_IntersectsLineParity(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	for _, s := range segmentCases() {
		want := IntersectsLineDefault(multi, s.aX, s.aY, s.bX, s.bY)
		got := IntersectsLineDefault(tree, s.aX, s.aY, s.bX, s.bY)
		if got != want {
			t.Errorf("IntersectsLine(%v,%v -> %v,%v) tree=%v multi=%v",
				s.aX, s.aY, s.bX, s.bY, got, want)
		}
	}
}

func TestComponentTree_ContainsLineParity(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	for _, s := range segmentCases() {
		want := ContainsLineDefault(multi, s.aX, s.aY, s.bX, s.bY)
		got := ContainsLineDefault(tree, s.aX, s.aY, s.bX, s.bY)
		if got != want {
			t.Errorf("ContainsLine(%v,%v -> %v,%v) tree=%v multi=%v",
				s.aX, s.aY, s.bX, s.bY, got, want)
		}
	}
}

// triangleCases reuses the segment fixture and pins a third vertex
// to expose triangle interiors that cross the kd-tree splits.
func triangleCases() []struct{ aX, aY, bX, bY, cX, cY float64 } {
	segs := segmentCases()
	out := make([]struct{ aX, aY, bX, bY, cX, cY float64 }, 0, len(segs))
	for _, s := range segs {
		out = append(out, struct{ aX, aY, bX, bY, cX, cY float64 }{
			s.aX, s.aY, s.bX, s.bY, s.aX + 1, s.aY + 1,
		})
	}
	return out
}

func TestComponentTree_IntersectsTriangleParity(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	for _, tr := range triangleCases() {
		want := IntersectsTriangleDefault(multi, tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY)
		got := IntersectsTriangleDefault(tree, tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY)
		if got != want {
			t.Errorf("IntersectsTriangle(%v,%v %v,%v %v,%v) tree=%v multi=%v",
				tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY, got, want)
		}
	}
}

func TestComponentTree_ContainsTriangleParity(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	multi := newMultiComponent2D(cloneComponents(leaves))
	tree := newComponentTree(cloneComponents(leaves))
	for _, tr := range triangleCases() {
		want := ContainsTriangleDefault(multi, tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY)
		got := ContainsTriangleDefault(tree, tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY)
		if got != want {
			t.Errorf("ContainsTriangle(%v,%v %v,%v %v,%v) tree=%v multi=%v",
				tr.aX, tr.aY, tr.bX, tr.bY, tr.cX, tr.cY, got, want)
		}
	}
}

// The within family is only defined for single-component trees in
// the Java reference. The Go port surfaces a panic that mirrors the
// IllegalArgumentException, since Component2D has no error channel.
func TestComponentTree_WithinPointPanicsForMulti(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	tree := newComponentTree(cloneComponents(leaves))
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("WithinPoint on multi-component tree must panic")
		}
	}()
	_ = tree.WithinPoint(0, 0)
}

func TestComponentTree_WithinLinePanicsForMulti(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	tree := newComponentTree(cloneComponents(leaves))
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("WithinLine on multi-component tree must panic")
		}
	}()
	_ = WithinLineDefault(tree, 0, 0, false, 1, 1)
}

func TestComponentTree_WithinTrianglePanicsForMulti(t *testing.T) {
	t.Parallel()
	leaves := testTreeRectangles()
	tree := newComponentTree(cloneComponents(leaves))
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("WithinTriangle on multi-component tree must panic")
		}
	}()
	_ = WithinTriangleDefault(tree,
		0, 0, false,
		1, 0, false,
		0, 1, false)
}

// For a single-leaf tree, the within family must delegate to the
// underlying Component2D and produce identical answers.
func TestComponentTree_WithinPassThroughForSingleLeaf(t *testing.T) {
	t.Parallel()
	leaf := newRectangle2D(0, 10, 0, 10)
	tree := newComponentTree([]Component2D{leaf})
	// pass-through returns the leaf directly per
	// TestComponentTree_SingleComponentIsPassThrough, so the within
	// family naturally delegates. Confirm a representative answer.
	if tree.WithinPoint(5, 5) != leaf.WithinPoint(5, 5) {
		t.Errorf("WithinPoint pass-through mismatch")
	}
}
