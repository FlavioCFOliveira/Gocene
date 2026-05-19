// Code in this file mirrors org.apache.lucene.geo.ComponentTree from
// Apache Lucene 10.4.0.
//
// ComponentTree is an interval-tree composite over a list of
// Component2D leaves, used by Lucene as a performance optimisation
// for set-union queries against multi-component geometries. The tree
// is built by alternately splitting on the X / Y coordinate at every
// level (a 2-d kd-tree built via quickselect over Component2D
// bounding boxes), and each query short-circuits whole subtrees by
// comparing the query bounding box against the per-node bounding box
// pulled up during construction.
//
// Lucene declares ComponentTree as package-private and treats its
// topology as a non-observable implementation detail; this port
// preserves that posture by keeping componentTree and the
// newComponentTree factory unexported. The default composite used by
// the geo factories (multiComponent2D, see multi_component2d.go) is
// intentionally NOT replaced by this tree — wiring it as the default
// composite is left to a follow-up task. componentTree is provided as
// a stand-alone Component2D port suitable for benchmarks and for
// callers that want the asymptotic profile of the Java reference.

package geo

import "github.com/FlavioCFOliveira/Gocene/util"

// rootSplitX is the splitting orientation chosen by Java's
// ComponentTree for the tree root; the first level splits on the Y
// coordinate, alternating downwards. Kept as a named constant for
// parity with the Java source.
const rootSplitX = false

// componentTree is the kd-tree composite that mirrors
// org.apache.lucene.geo.ComponentTree. It satisfies Component2D and
// short-circuits subtree traversal using per-node bounding boxes
// pulled up at construction time.
type componentTree struct {
	// minY, maxY, minX, maxX is the per-node bounding box. For leaf
	// nodes these start as the wrapped Component2D's bounding box;
	// for internal nodes the max values are pulled up from the
	// subtrees, and after createComponentTree completes the root's
	// min values are pulled down from the full input slice (see
	// newComponentTree).
	minY float64
	maxY float64
	minX float64
	maxX float64

	// left and right are the kd-tree children. Internal nodes may
	// have inconsistent bounding boxes by themselves and are not
	// exposed outside this file.
	left  *componentTree
	right *componentTree

	// component is the leaf Component2D wrapped at this node. Every
	// node — including internal ones — wraps one input Component2D,
	// matching the Java reference layout.
	component Component2D
}

// newComponentTreeNode allocates a leaf node wrapping component,
// initialising the per-node bounding box from the wrapped
// Component2D. Mirrors the private ComponentTree(Component2D) Java
// constructor.
func newComponentTreeNode(component Component2D) *componentTree {
	return &componentTree{
		minY:      component.MinY(),
		maxY:      component.MaxY(),
		minX:      component.MinX(),
		maxX:      component.MaxX(),
		component: component,
	}
}

// newComponentTree builds a kd-tree composite over components and
// returns it as a Component2D. When len(components) == 1 the single
// child is returned unchanged, matching ComponentTree.create. The
// input slice is permuted in place by the underlying quickselect
// (util.Select); callers that need to retain the original ordering
// must copy first.
//
// The tree alternates the splitting axis at every level starting
// from rootSplitX, so the root splits on the Y coordinate, its
// children on X, and so on. Construction takes O(n log n) time for
// the sequence of quickselects and the recursion.
func newComponentTree(components []Component2D) Component2D {
	if len(components) == 0 {
		panic("geo: newComponentTree requires at least one component")
	}
	if len(components) == 1 {
		return components[0]
	}
	root := createComponentTree(components, 0, len(components)-1, rootSplitX)
	// Pull up the min values for the root so that its bounding box
	// is a true superset of every input component's bounding box.
	// The recursive construction only pulls up max values; pulling
	// the min values requires a single linear scan over the inputs.
	for _, c := range components {
		if v := c.MinY(); v < root.minY {
			root.minY = v
		}
		if v := c.MinX(); v < root.minX {
			root.minX = v
		}
	}
	return root
}

// createComponentTree is the recursive kd-tree builder. low and high
// are inclusive bounds into components; splitX selects the splitting
// axis at this level. Mirrors the private createTree method of the
// Java ComponentTree.
func createComponentTree(components []Component2D, low, high int, splitX bool) *componentTree {
	if low > high {
		return nil
	}
	// Java uses an unsigned shift to avoid overflow on very large
	// arrays; with Go int the equivalent is the standard (low+high)/2
	// because indices into a slice cannot reach math.MaxInt/2.
	mid := (low + high) >> 1
	if low < high {
		if splitX {
			util.Select(components, low, high+1, mid, compareComponentX)
		} else {
			util.Select(components, low, high+1, mid, compareComponentY)
		}
	}

	newNode := newComponentTreeNode(components[mid])
	newNode.left = createComponentTree(components, low, mid-1, !splitX)
	newNode.right = createComponentTree(components, mid+1, high, !splitX)

	// Pull up max values from the subtrees so the per-node bounding
	// box covers all wrapped components in the subtree rooted here.
	if newNode.left != nil {
		if v := newNode.left.MaxX(); v > newNode.maxX {
			newNode.maxX = v
		}
		if v := newNode.left.MaxY(); v > newNode.maxY {
			newNode.maxY = v
		}
	}
	if newNode.right != nil {
		if v := newNode.right.MaxX(); v > newNode.maxX {
			newNode.maxX = v
		}
		if v := newNode.right.MaxY(); v > newNode.maxY {
			newNode.maxY = v
		}
	}
	return newNode
}

// compareComponentX is the Java X_COMPARATOR: order by MinX, break
// ties by MaxX. Returns the standard -1/0/+1 result expected by
// util.Select.
func compareComponentX(a, b Component2D) int {
	if v := compareFloat(a.MinX(), b.MinX()); v != 0 {
		return v
	}
	return compareFloat(a.MaxX(), b.MaxX())
}

// compareComponentY is the Java Y_COMPARATOR: order by MinY, break
// ties by MaxY.
func compareComponentY(a, b Component2D) int {
	if v := compareFloat(a.MinY(), b.MinY()); v != 0 {
		return v
	}
	return compareFloat(a.MaxY(), b.MaxY())
}

// compareFloat returns -1, 0, or +1 for a < b, a == b, or a > b.
// The Java Comparator.comparingDouble uses Double.compare semantics,
// which differs from `<` only on NaN / -0.0. Component2D bounding
// boxes from real geometries never carry those values, so the
// straightforward comparison preserves observable ordering.
func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (t *componentTree) MinX() float64 { return t.minX }
func (t *componentTree) MaxX() float64 { return t.maxX }
func (t *componentTree) MinY() float64 { return t.minY }
func (t *componentTree) MaxY() float64 { return t.maxY }

// Contains short-circuits whole subtrees using the per-node bounding
// box and the splitting axis at each level.
func (t *componentTree) Contains(x, y float64) bool {
	return t.containsRec(x, y, rootSplitX)
}

func (t *componentTree) containsRec(x, y float64, splitX bool) bool {
	if y <= t.maxY && x <= t.maxX {
		if t.component.Contains(x, y) {
			return true
		}
		if t.left != nil {
			if t.left.containsRec(x, y, !splitX) {
				return true
			}
		}
		if t.right != nil &&
			((!splitX && y >= t.component.MinY()) ||
				(splitX && x >= t.component.MinX())) {
			return t.right.containsRec(x, y, !splitX)
		}
	}
	return false
}

// IntersectsLine returns true if any wrapped component intersects
// the segment, pruning subtrees whose bounding box rules them out.
func (t *componentTree) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	return t.intersectsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, rootSplitX)
}

func (t *componentTree) intersectsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY float64, splitX bool) bool {
	if minY <= t.maxY && minX <= t.maxX {
		if t.component.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
			return true
		}
		if t.left != nil {
			if t.left.intersectsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, !splitX) {
				return true
			}
		}
		if t.right != nil &&
			((!splitX && maxY >= t.component.MinY()) ||
				(splitX && maxX >= t.component.MinX())) {
			return t.right.intersectsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, !splitX)
		}
	}
	return false
}

// IntersectsTriangle returns true if any wrapped component
// intersects the triangle.
func (t *componentTree) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	return t.intersectsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, rootSplitX)
}

func (t *componentTree) intersectsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64, splitX bool) bool {
	if minY <= t.maxY && minX <= t.maxX {
		if t.component.IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY) {
			return true
		}
		if t.left != nil {
			if t.left.intersectsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, !splitX) {
				return true
			}
		}
		if t.right != nil &&
			((!splitX && maxY >= t.component.MinY()) ||
				(splitX && maxX >= t.component.MinX())) {
			return t.right.intersectsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, !splitX)
		}
	}
	return false
}

// ContainsLine returns true if any single wrapped component contains
// the segment. Mirrors the Java semantics: containment is short-
// circuited as soon as one leaf reports true.
func (t *componentTree) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	return t.containsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, rootSplitX)
}

func (t *componentTree) containsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY float64, splitX bool) bool {
	if minY <= t.maxY && minX <= t.maxX {
		if t.component.ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
			return true
		}
		if t.left != nil {
			if t.left.containsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, !splitX) {
				return true
			}
		}
		if t.right != nil &&
			((!splitX && maxY >= t.component.MinY()) ||
				(splitX && maxX >= t.component.MinX())) {
			return t.right.containsLineRec(minX, maxX, minY, maxY, aX, aY, bX, bY, !splitX)
		}
	}
	return false
}

// ContainsTriangle returns true if any single wrapped component
// contains the triangle.
func (t *componentTree) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	return t.containsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, rootSplitX)
}

func (t *componentTree) containsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64, splitX bool) bool {
	if minY <= t.maxY && minX <= t.maxX {
		if t.component.ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY) {
			return true
		}
		if t.left != nil {
			if t.left.containsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, !splitX) {
				return true
			}
		}
		if t.right != nil &&
			((!splitX && maxY >= t.component.MinY()) ||
				(splitX && maxX >= t.component.MinX())) {
			return t.right.containsTriangleRec(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY, !splitX)
		}
	}
	return false
}

// WithinPoint is only defined for single-component trees: the Java
// reference throws IllegalArgumentException once any subtree exists.
// The Go port panics with a parallel message, since the within
// family has no error channel on the Component2D contract.
func (t *componentTree) WithinPoint(x, y float64) WithinRelation {
	if t.left != nil || t.right != nil {
		panic("geo: withinPoint is not supported for shapes with more than one component")
	}
	return t.component.WithinPoint(x, y)
}

// WithinLine is only defined for single-component trees.
func (t *componentTree) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if t.left != nil || t.right != nil {
		panic("geo: withinLine is not supported for shapes with more than one component")
	}
	return t.component.WithinLine(minX, maxX, minY, maxY, aX, aY, ab, bX, bY)
}

// WithinTriangle is only defined for single-component trees.
func (t *componentTree) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if t.left != nil || t.right != nil {
		panic("geo: withinTriangle is not supported for shapes with more than one component")
	}
	return t.component.WithinTriangle(minX, maxX, minY, maxY, aX, aY, ab, bX, bY, bc, cX, cY, ca)
}

// Relate walks the tree returning the first non-OUTSIDE relation
// found among the wrapped components. INSIDE wins immediately;
// CROSSES is returned the moment it is observed; OUTSIDE is only
// returned when every reachable subtree is OUTSIDE.
func (t *componentTree) Relate(minX, maxX, minY, maxY float64) Relation {
	return t.relateRec(minX, maxX, minY, maxY, rootSplitX)
}

func (t *componentTree) relateRec(minX, maxX, minY, maxY float64, splitX bool) Relation {
	if minY <= t.maxY && minX <= t.maxX {
		relation := t.component.Relate(minX, maxX, minY, maxY)
		if relation != CellOutsideQuery {
			return relation
		}
		if t.left != nil {
			relation = t.left.relateRec(minX, maxX, minY, maxY, !splitX)
			if relation != CellOutsideQuery {
				return relation
			}
		}
		if t.right != nil &&
			((!splitX && maxY >= t.component.MinY()) ||
				(splitX && maxX >= t.component.MinX())) {
			return t.right.relateRec(minX, maxX, minY, maxY, !splitX)
		}
	}
	return CellOutsideQuery
}
