// Code in this file mirrors org.apache.lucene.geo.EdgeTree from Apache
// Lucene 10.4.0. The Java type is package-private and final; the Go
// port keeps it unexported (edgeTree) and exposes only the same
// package-internal surface.
//
// Construction is O(n log n) for sorting and tree assembly; query
// methods are O(n) worst case but, for realistic lines and polygons,
// the y-interval pruning makes them much faster than brute force.

package geo

import (
	"math"
	"sort"
)

// containsResult mirrors the byte-valued tri-state returned by
// containsPnPoly in the Java reference. The numeric values are kept
// identical so that the XOR-based parity arithmetic continues to work.
type containsResult byte

const (
	edgeFalse  containsResult = 0x00
	edgeTrue   containsResult = 0x01
	edgeOnEdge containsResult = 0x02
)

// edgeTree is the internal interval-tree node used by line2D and
// polygon2D to accelerate point-in-polygon, edge-on-line and crossing
// queries. Each node represents a single edge from (x1, y1) to
// (x2, y2); low is the minimum y of the edge and max is the maximum y
// of the edge or any of its descendants.
type edgeTree struct {
	// X-Y pair (in original order) of the two vertices.
	y1, y2 float64
	x1, x2 float64

	// low is the minimum y of this edge; max is the maximum y of this
	// edge or any of its children (updated during tree assembly).
	low float64
	max float64

	// left and right children, or nil.
	left  *edgeTree
	right *edgeTree
}

// contains reports whether the (x, y) point lies on an edge of the
// subtree or crosses it an odd number of times. Mirrors EdgeTree
// #contains in the Java reference.
func (e *edgeTree) contains(x, y float64) bool {
	return e.containsPnPoly(x, y) > edgeFalse
}

// containsPnPoly returns edgeFalse if the point crosses this subtree
// an even number of times, edgeTrue if it crosses an odd number of
// times, and edgeOnEdge if the point lies exactly on one of the edges.
//
// Ported from the W. Randolph Franklin PNPOLY routine via the Java
// reference; see EdgeTree#containsPnPoly for the original BSD-licensed
// algorithm.
func (e *edgeTree) containsPnPoly(x, y float64) containsResult {
	res := edgeFalse
	if y <= e.max {
		if (y == e.y1 && y == e.y2) ||
			((y <= e.y1 && y >= e.y2) != (y >= e.y1 && y <= e.y2)) {
			if (x == e.x1 && x == e.x2) ||
				((x <= e.x1 && x >= e.x2) != (x >= e.x1 && x <= e.x2) &&
					Orient(e.x1, e.y1, e.x2, e.y2, x, y) == 0) {
				return edgeOnEdge
			} else if (e.y1 > y) != (e.y2 > y) {
				if x < (e.x2-e.x1)*(y-e.y1)/(e.y2-e.y1)+e.x1 {
					res = edgeTrue
				} else {
					res = edgeFalse
				}
			}
		}
		if e.left != nil {
			res ^= e.left.containsPnPoly(x, y)
			if res&0x02 == 0x02 {
				return edgeOnEdge
			}
		}
		if e.right != nil && y >= e.low {
			res ^= e.right.containsPnPoly(x, y)
			if res&0x02 == 0x02 {
				return edgeOnEdge
			}
		}
	}
	return res
}

// isPointOnLine reports whether (x, y) lies on any edge of this
// subtree. Mirrors EdgeTree#isPointOnLine in the Java reference.
func (e *edgeTree) isPointOnLine(x, y float64) bool {
	if y <= e.max {
		a1x, a1y := e.x1, e.y1
		b1x, b1y := e.x2, e.y2
		outside := (a1y < y && b1y < y) ||
			(a1y > y && b1y > y) ||
			(a1x < x && b1x < x) ||
			(a1x > x && b1x > x)
		if !outside && Orient(a1x, a1y, b1x, b1y, x, y) == 0 {
			return true
		}
		if e.left != nil && e.left.isPointOnLine(x, y) {
			return true
		}
		if e.right != nil && y >= e.low && e.right.isPointOnLine(x, y) {
			return true
		}
	}
	return false
}

// crossesTriangle reports whether the triangle (ax, ay)-(bx, by)-
// (cx, cy), pre-bounded by [minX, maxX] x [minY, maxY], crosses any
// edge in this subtree. When includeBoundary is true the test uses
// the boundary-inclusive variant of the segment intersection routine.
// Mirrors EdgeTree#crossesTriangle in the Java reference.
func (e *edgeTree) crossesTriangle(
	minX, maxX, minY, maxY float64,
	ax, ay, bx, by, cx, cy float64,
	includeBoundary bool,
) bool {
	if minY <= e.max {
		dy, ey := e.y1, e.y2
		dx, ex := e.x1, e.x2

		// optimization: skip if the rectangle is entirely outside the
		// edge's bounding box.
		outside := (dy < minY && ey < minY) ||
			(dy > maxY && ey > maxY) ||
			(dx < minX && ex < minX) ||
			(dx > maxX && ex > maxX)

		if !outside {
			if includeBoundary {
				if LineCrossesLineWithBoundary(dx, dy, ex, ey, ax, ay, bx, by) ||
					LineCrossesLineWithBoundary(dx, dy, ex, ey, bx, by, cx, cy) ||
					LineCrossesLineWithBoundary(dx, dy, ex, ey, cx, cy, ax, ay) {
					return true
				}
			} else {
				if LineCrossesLine(dx, dy, ex, ey, ax, ay, bx, by) ||
					LineCrossesLine(dx, dy, ex, ey, bx, by, cx, cy) ||
					LineCrossesLine(dx, dy, ex, ey, cx, cy, ax, ay) {
					return true
				}
			}
		}

		if e.left != nil &&
			e.left.crossesTriangle(minX, maxX, minY, maxY, ax, ay, bx, by, cx, cy, includeBoundary) {
			return true
		}
		if e.right != nil && maxY >= e.low &&
			e.right.crossesTriangle(minX, maxX, minY, maxY, ax, ay, bx, by, cx, cy, includeBoundary) {
			return true
		}
	}
	return false
}

// crossesBox reports whether the axis-aligned box defined by
// (minX, maxX, minY, maxY) crosses any edge in this subtree.
// Mirrors EdgeTree#crossesBox in the Java reference.
func (e *edgeTree) crossesBox(
	minX, maxX, minY, maxY float64,
	includeBoundary bool,
) bool {
	if minY <= e.max {
		cy, dy := e.y1, e.y2
		cx, dx := e.x1, e.x2

		// optimization: an edge endpoint inside the rectangle is an
		// immediate yes.
		// ContainsPoint takes (lat, lon, minLat, maxLat, minLon, maxLon),
		// matching the (y, x, minY, maxY, minX, maxX) order used here.
		if ContainsPoint(cy, cx, minY, maxY, minX, maxX) ||
			ContainsPoint(dy, dx, minY, maxY, minX, maxX) {
			return true
		}

		outside := (cy < minY && dy < minY) ||
			(cy > maxY && dy > maxY) ||
			(cx < minX && dx < minX) ||
			(cx > maxX && dx > maxX)

		if !outside {
			if includeBoundary {
				if LineCrossesLineWithBoundary(cx, cy, dx, dy, minX, minY, maxX, minY) ||
					LineCrossesLineWithBoundary(cx, cy, dx, dy, maxX, minY, maxX, maxY) ||
					LineCrossesLineWithBoundary(cx, cy, dx, dy, maxX, maxY, minX, maxY) ||
					LineCrossesLineWithBoundary(cx, cy, dx, dy, minX, maxY, minX, minY) {
					return true
				}
			} else {
				if LineCrossesLine(cx, cy, dx, dy, minX, minY, maxX, minY) ||
					LineCrossesLine(cx, cy, dx, dy, maxX, minY, maxX, maxY) ||
					LineCrossesLine(cx, cy, dx, dy, maxX, maxY, minX, maxY) ||
					LineCrossesLine(cx, cy, dx, dy, minX, maxY, minX, minY) {
					return true
				}
			}
		}

		if e.left != nil && e.left.crossesBox(minX, maxX, minY, maxY, includeBoundary) {
			return true
		}
		if e.right != nil && maxY >= e.low &&
			e.right.crossesBox(minX, maxX, minY, maxY, includeBoundary) {
			return true
		}
	}
	return false
}

// crossesLine reports whether the segment (a2x, a2y)-(b2x, b2y),
// pre-bounded by [minX, maxX] x [minY, maxY], crosses any edge in
// this subtree. Mirrors EdgeTree#crossesLine in the Java reference.
func (e *edgeTree) crossesLine(
	minX, maxX, minY, maxY float64,
	a2x, a2y, b2x, b2y float64,
	includeBoundary bool,
) bool {
	if minY <= e.max {
		a1x, a1y := e.x1, e.y1
		b1x, b1y := e.x2, e.y2

		outside := (a1y < minY && b1y < minY) ||
			(a1y > maxY && b1y > maxY) ||
			(a1x < minX && b1x < minX) ||
			(a1x > maxX && b1x > maxX)
		if !outside {
			if includeBoundary {
				if LineCrossesLineWithBoundary(a1x, a1y, b1x, b1y, a2x, a2y, b2x, b2y) {
					return true
				}
			} else {
				if LineCrossesLine(a1x, a1y, b1x, b1y, a2x, a2y, b2x, b2y) {
					return true
				}
			}
		}
		if e.left != nil &&
			e.left.crossesLine(minX, maxX, minY, maxY, a2x, a2y, b2x, b2y, includeBoundary) {
			return true
		}
		if e.right != nil && maxY >= e.low &&
			e.right.crossesLine(minX, maxX, minY, maxY, a2x, a2y, b2x, b2y, includeBoundary) {
			return true
		}
	}
	return false
}

// createEdgeTree builds a balanced edge interval tree from the
// (x, y) vertex arrays of a poly-line or polygon ring. The two
// arrays must have the same length and at least two vertices so
// that one or more edges exist. Mirrors EdgeTree#createTree.
func createEdgeTree(x, y []float64) *edgeTree {
	n := len(x) - 1
	if n <= 0 {
		return nil
	}
	edges := make([]*edgeTree, n)
	for i := 1; i <= n; i++ {
		x1 := x[i-1]
		y1 := y[i-1]
		x2 := x[i]
		y2 := y[i]
		edges[i-1] = &edgeTree{
			x1:  x1,
			y1:  y1,
			x2:  x2,
			y2:  y2,
			low: math.Min(y1, y2),
			max: math.Max(y1, y2),
		}
	}
	// Sort by (low asc, max asc) to mirror Java's Double.compare-based
	// comparator. sort.SliceStable preserves input order on ties,
	// which keeps tree assembly deterministic for equal edges.
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].low != edges[j].low {
			return edges[i].low < edges[j].low
		}
		return edges[i].max < edges[j].max
	})
	return buildEdgeTree(edges, 0, len(edges)-1)
}

// buildEdgeTree recursively assembles a balanced tree from the
// pre-sorted edges and propagates max upwards. Mirrors the private
// createTree(edges, low, high) overload in the Java reference.
func buildEdgeTree(edges []*edgeTree, low, high int) *edgeTree {
	if low > high {
		return nil
	}
	// Unsigned right shift to avoid overflow on huge inputs; with
	// non-negative low/high the int form below is equivalent.
	mid := int(uint(low+high) >> 1)
	node := edges[mid]
	node.left = buildEdgeTree(edges, low, mid-1)
	node.right = buildEdgeTree(edges, mid+1, high)
	if node.left != nil && node.left.max > node.max {
		node.max = node.left.max
	}
	if node.right != nil && node.right.max > node.max {
		node.max = node.right.max
	}
	return node
}
