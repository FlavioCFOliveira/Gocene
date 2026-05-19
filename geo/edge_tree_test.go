// Tests for the Go port of org.apache.lucene.geo.EdgeTree. The Java
// reference ships no dedicated EdgeTreeTest; coverage is exercised
// indirectly through Polygon2D / Line2D tests. The cases below pin
// the observable behaviour of every package-internal method against
// a small, hand-checked square ring and poly-line so future changes
// to the algorithm cannot drift silently.

package geo

import "testing"

// unitSquareRing returns the closed ring [(0,0), (1,0), (1,1), (0,1), (0,0)].
func unitSquareRing() (xs, ys []float64) {
	return []float64{0, 1, 1, 0, 0}, []float64{0, 0, 1, 1, 0}
}

// staircaseLine returns three connected segments forming a step shape.
func staircaseLine() (xs, ys []float64) {
	return []float64{0, 1, 1, 2}, []float64{0, 0, 1, 1}
}

func TestCreateEdgeTree_BuildsBalancedTreeWithCorrectMax(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)
	if tree == nil {
		t.Fatal("createEdgeTree returned nil for 4-edge ring")
	}
	// Four edges -> a balanced BST of size 4 has depth 3 (root + two
	// children + one grandchild). The root's max must dominate the
	// whole subtree.
	maxOfSubtree := func(n *edgeTree) float64 {
		var walk func(*edgeTree) float64
		walk = func(n *edgeTree) float64 {
			if n == nil {
				return 0
			}
			m := n.max
			if l := walk(n.left); l > m {
				m = l
			}
			if r := walk(n.right); r > m {
				m = r
			}
			return m
		}
		return walk(n)
	}
	if got, want := tree.max, maxOfSubtree(tree); got != want {
		t.Fatalf("root max = %v, expected dominating value %v", got, want)
	}
	// Highest y in the ring is 1; root max must equal it.
	if tree.max != 1 {
		t.Fatalf("root max = %v, want 1 for unit square", tree.max)
	}
}

func TestCreateEdgeTree_DegenerateInputReturnsNil(t *testing.T) {
	t.Parallel()
	if got := createEdgeTree(nil, nil); got != nil {
		t.Fatalf("nil inputs: got %+v, want nil", got)
	}
	if got := createEdgeTree([]float64{0}, []float64{0}); got != nil {
		t.Fatalf("single vertex: got %+v, want nil", got)
	}
}

func TestEdgeTree_Contains_PointInsideOutsideAndOnEdge(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)

	cases := []struct {
		name string
		x, y float64
		want bool
	}{
		{"strict interior", 0.5, 0.5, true},
		{"on bottom edge", 0.5, 0, true},
		{"on top edge", 0.5, 1, true},
		{"on left edge", 0, 0.5, true},
		{"on right edge", 1, 0.5, true},
		{"vertex", 0, 0, true},
		{"strictly outside (right)", 1.5, 0.5, false},
		{"strictly outside (above)", 0.5, 1.5, false},
		{"strictly outside (left)", -0.5, 0.5, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tree.contains(tc.x, tc.y); got != tc.want {
				t.Fatalf("contains(%v,%v) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

func TestEdgeTree_ContainsPnPoly_OnEdgeReturnsOnEdgeSentinel(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)
	if got := tree.containsPnPoly(0.5, 0); got != edgeOnEdge {
		t.Fatalf("containsPnPoly on edge = %v, want edgeOnEdge", got)
	}
	if got := tree.containsPnPoly(2, 2); got != edgeFalse {
		t.Fatalf("containsPnPoly outside = %v, want edgeFalse", got)
	}
}

func TestEdgeTree_IsPointOnLine(t *testing.T) {
	t.Parallel()
	xs, ys := staircaseLine()
	tree := createEdgeTree(xs, ys)

	cases := []struct {
		name string
		x, y float64
		want bool
	}{
		{"midpoint of first segment", 0.5, 0, true},
		{"corner vertex", 1, 0, true},
		{"midpoint of vertical segment", 1, 0.5, true},
		{"midpoint of top segment", 1.5, 1, true},
		{"off the line", 0.5, 0.5, false},
		{"past the end", 3, 1, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tree.isPointOnLine(tc.x, tc.y); got != tc.want {
				t.Fatalf("isPointOnLine(%v,%v) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}

func TestEdgeTree_CrossesBox(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)

	cases := []struct {
		name                   string
		minX, maxX, minY, maxY float64
		includeBoundary        bool
		want                   bool
	}{
		{"box straddling left edge", -0.5, 0.5, 0.25, 0.75, false, true},
		{"box fully outside", 2, 3, 2, 3, false, false},
		{"box fully inside", 0.25, 0.75, 0.25, 0.75, false, false},
		// A box whose right side lies exactly on x=0 only crosses an
		// edge when boundary points are counted; without boundary the
		// shared segment is not considered an intersection.
		{"flush with left edge no boundary", -1, 0, 0.25, 0.75, false, false},
		{"flush with left edge with boundary", -1, 0, 0.25, 0.75, true, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tree.crossesBox(tc.minX, tc.maxX, tc.minY, tc.maxY, tc.includeBoundary)
			if got != tc.want {
				t.Fatalf("crossesBox = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEdgeTree_CrossesLine(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)

	// Horizontal segment slicing the square across y=0.5.
	if got := tree.crossesLine(-1, 2, 0.5, 0.5, -1, 0.5, 2, 0.5, false); !got {
		t.Fatal("crossing segment not detected")
	}
	// Segment well above the square.
	if got := tree.crossesLine(-1, 2, 2, 2, -1, 2, 2, 2, false); got {
		t.Fatal("non-crossing segment reported as crossing")
	}
}

func TestEdgeTree_CrossesTriangle(t *testing.T) {
	t.Parallel()
	xs, ys := unitSquareRing()
	tree := createEdgeTree(xs, ys)

	// Triangle that pokes into the unit square from the left.
	if got := tree.crossesTriangle(
		-1, 0.5, 0, 1,
		-1, 0.5, 0.5, 1, 0.5, 0,
		false,
	); !got {
		t.Fatal("intersecting triangle not detected")
	}
	// Triangle entirely outside.
	if got := tree.crossesTriangle(
		3, 5, 3, 5,
		3, 3, 5, 3, 4, 5,
		false,
	); got {
		t.Fatal("disjoint triangle reported as crossing")
	}
}
