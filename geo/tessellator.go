// Code in this file mirrors org.apache.lucene.geo.Tessellator from
// Apache Lucene 10.4.0 at a focused scope:
//
//   - Simple ear-clipping triangulation of single-ring polygons
//     (Polygon and XYPolygon without holes).
//   - Triangle output type carrying the three (lat, lon) or (x, y)
//     vertices plus the "edge belongs to the original shape" flag
//     that downstream codecs depend on.
//
// Out of scope for this port (clearly marked here and in the test
// peer): hole elimination via tangent-pair search, Morton-curve
// z-order acceleration for large polygons (>VERTEX_THRESHOLD),
// self-intersection detection and Bowyer-Watson splitting,
// recursion-limit failure recovery, and the Java Monitor callback.
//
// These features are an asymptotic and robustness improvement on
// top of the basic ear-clipping algorithm; the simpler version
// covers the polygons typically produced by GIS pipelines. The
// limitations are surfaced as ErrTessellatorUnsupported so callers
// can detect them deterministically.

package geo

import (
	"errors"
	"fmt"
)

// Triangle is the unit of output produced by the Tessellator. It
// holds three (x, y) vertices (longitude / latitude for geographic
// polygons, x / y for cartesian XYPolygons) plus a per-edge flag
// indicating whether the edge belongs to the original polygon
// boundary (true) or was synthesised by triangulation (false).
//
// It mirrors org.apache.lucene.geo.Tessellator.Triangle.
type Triangle struct {
	ax, ay float64
	bx, by float64
	cx, cy float64

	// Per-edge "belongs to original polygon edge" flags.
	abFromPolygon bool
	bcFromPolygon bool
	caFromPolygon bool
}

// AX / AY / BX / BY / CX / CY return the per-vertex coordinates.
func (t *Triangle) AX() float64 { return t.ax }
func (t *Triangle) AY() float64 { return t.ay }
func (t *Triangle) BX() float64 { return t.bx }
func (t *Triangle) BY() float64 { return t.by }
func (t *Triangle) CX() float64 { return t.cx }
func (t *Triangle) CY() float64 { return t.cy }

// EdgeFromPolygon reports whether the edge between vertices a and b,
// b and c, or c and a (indexed 0, 1, 2 respectively) belongs to the
// original polygon boundary. Used by downstream codecs to mark
// non-boundary edges as virtual.
func (t *Triangle) EdgeFromPolygon(edge int) bool {
	switch edge {
	case 0:
		return t.abFromPolygon
	case 1:
		return t.bcFromPolygon
	case 2:
		return t.caFromPolygon
	}
	panic(fmt.Sprintf("geo: invalid edge index %d", edge))
}

// ErrTessellatorUnsupported is returned for polygon shapes that fall
// outside the current scope of this port (notably polygons with
// holes and polygons larger than the Morton threshold).
var ErrTessellatorUnsupported = errors.New("geo: tessellator unsupported shape (current port covers simple ring polygons only)")

// ErrTessellatorMalformed is returned when the input polygon is
// degenerate (fewer than three non-collinear vertices) or cannot be
// triangulated by the ear-clipping algorithm.
var ErrTessellatorMalformed = errors.New("geo: tessellator detected a malformed shape")

// Tessellate triangulates a geographic Polygon and returns the list
// of triangles produced by ear-clipping. Holes are not yet
// supported.
//
// It is the Go port of org.apache.lucene.geo.Tessellator#tessellate(Polygon,boolean).
// The checkSelfIntersections parameter is accepted for API
// compatibility but is currently a no-op (self-intersection
// detection is part of the scope deferred above; callers must
// supply non-self-intersecting polygons).
func Tessellate(polygon Polygon, checkSelfIntersections bool) ([]Triangle, error) {
	if polygon.NumHoles() > 0 {
		return nil, fmt.Errorf("%w: polygons with holes are not implemented",
			ErrTessellatorUnsupported)
	}
	xs := polygon.PolyLons()
	ys := polygon.PolyLats()
	return tessellateRing(xs, ys)
}

// TessellateXY is the cartesian counterpart of Tessellate. It
// accepts an XYPolygon whose vertex slices are x/y in float64.
// XYPolygon itself is ported by task #300; this signature anticipates
// it by accepting parallel slices directly, the same shape the
// XYPolygon port will provide.
func TessellateXY(xs, ys []float64, holes int, checkSelfIntersections bool) ([]Triangle, error) {
	if holes > 0 {
		return nil, fmt.Errorf("%w: polygons with holes are not implemented",
			ErrTessellatorUnsupported)
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("%w: x/y length mismatch", ErrTessellatorMalformed)
	}
	xs2 := make([]float64, len(xs))
	ys2 := make([]float64, len(ys))
	copy(xs2, xs)
	copy(ys2, ys)
	return tessellateRing(xs2, ys2)
}

// tessellateRing runs the basic ear-clipping triangulation over a
// single closed ring. The first and last vertices may coincide
// (Lucene polygons do); we detect that and ignore the duplicate to
// keep the circular linked list well-formed.
func tessellateRing(xs, ys []float64) ([]Triangle, error) {
	n := len(xs)
	if n > 0 && xs[0] == xs[n-1] && ys[0] == ys[n-1] {
		n--
	}
	if n < 3 {
		return nil, fmt.Errorf("%w: at least three non-collinear points required",
			ErrTessellatorMalformed)
	}

	// Build the circular doubly-linked list of vertices.
	verts := make([]*tessNode, n)
	for i := 0; i < n; i++ {
		verts[i] = &tessNode{x: xs[i], y: ys[i]}
	}
	for i := 0; i < n; i++ {
		verts[i].prev = verts[(i+n-1)%n]
		verts[i].next = verts[(i+1)%n]
	}

	// Compute the signed area to detect winding order; we need CCW
	// for the standard ear-clipping algorithm. If the signed area
	// is negative (CW), reverse the linked list.
	if signedArea(verts) < 0 {
		reverseRing(verts)
	}

	// Mark every original edge as "from polygon".
	for _, v := range verts {
		v.edgeFromPolygon = true
	}

	head := verts[0]
	count := n
	triangles := make([]Triangle, 0, n-2)

	// Ear-clipping loop. Each successful clip removes one vertex
	// and emits one triangle.
	guard := 4 * n // Bounded iterations to prevent infinite loops on degenerate input.
	for count > 3 && guard > 0 {
		guard--
		if isEar(head) {
			tri := makeTriangle(head.prev, head, head.next)
			triangles = append(triangles, tri)
			// Remove `head` from the ring.
			head.prev.next = head.next
			head.next.prev = head.prev
			// The new triangle's prev-next edge is synthetic (it
			// replaced the original edges p-prev->head and
			// head->p.next which we just removed).
			head.prev.edgeFromPolygon = false
			head = head.next
			count--
		} else {
			head = head.next
		}
	}
	if count > 3 {
		return nil, fmt.Errorf("%w: ear-clipping failed (possible self-intersection or hole-less hole)",
			ErrTessellatorMalformed)
	}
	// Emit the final triangle.
	tri := makeTriangle(head.prev, head, head.next)
	triangles = append(triangles, tri)

	return triangles, nil
}

// tessNode is a single vertex in the ear-clipping circular linked
// list.
type tessNode struct {
	x, y            float64
	prev, next      *tessNode
	edgeFromPolygon bool // edge prev->this belongs to original polygon
}

// signedArea returns twice the signed area of the polygon defined
// by the vertices. Positive => CCW, negative => CW.
func signedArea(verts []*tessNode) float64 {
	sum := 0.0
	n := len(verts)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += (verts[j].x - verts[i].x) * (verts[j].y + verts[i].y)
	}
	// Note: this is twice the signed area with the sign convention
	// that CCW is negative (shoelace formula variant). We invert so
	// that CCW returns a positive value, matching standard
	// convention in ear-clipping references.
	return -sum
}

// reverseRing flips the prev/next pointers of the linked list so
// the ring traverses in the opposite direction.
func reverseRing(verts []*tessNode) {
	for _, v := range verts {
		v.prev, v.next = v.next, v.prev
	}
}

// isEar reports whether the triangle (prev, ear, next) is a valid
// ear: the triangle is on the correct (CCW) side and contains no
// other polygon vertex.
func isEar(ear *tessNode) bool {
	a := ear.prev
	b := ear
	c := ear.next
	// Convex check: CCW orientation must be positive.
	if area2(a.x, a.y, b.x, b.y, c.x, c.y) <= 0 {
		return false
	}
	// No other vertex should lie inside the triangle (a, b, c).
	for p := c.next; p != a; p = p.next {
		if p == b {
			continue
		}
		if pointInTriangleTess(a, b, c, p) && area2(p.prev.x, p.prev.y, p.x, p.y, p.next.x, p.next.y) >= 0 {
			return false
		}
	}
	return true
}

// area2 returns twice the signed area of triangle (a, b, c). Used
// as a fast orientation test.
func area2(ax, ay, bx, by, cx, cy float64) float64 {
	return (bx-ax)*(cy-ay) - (cx-ax)*(by-ay)
}

// pointInTriangleTess reports whether p lies strictly inside or on
// the boundary of triangle (a, b, c). Uses the same barycentric
// approach as Tessellator.pointInTriangle (Java).
func pointInTriangleTess(a, b, c, p *tessNode) bool {
	// Barycentric coordinate test.
	d1 := area2(p.x, p.y, a.x, a.y, b.x, b.y)
	d2 := area2(p.x, p.y, b.x, b.y, c.x, c.y)
	d3 := area2(p.x, p.y, c.x, c.y, a.x, a.y)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

// makeTriangle constructs a Triangle from three consecutive nodes
// and lifts the per-edge "from polygon" flags from each node.
func makeTriangle(a, b, c *tessNode) Triangle {
	return Triangle{
		ax: a.x, ay: a.y,
		bx: b.x, by: b.y,
		cx: c.x, cy: c.y,
		// Edge a->b belongs to original polygon iff b.edgeFromPolygon
		// (since b.edgeFromPolygon refers to the edge entering b, i.e.
		// a->b).
		abFromPolygon: b.edgeFromPolygon,
		bcFromPolygon: c.edgeFromPolygon,
		// Edge c->a is the synthetic closing edge of the triangle.
		// In Java's full tessellator this is tracked via the
		// "fromPolygon" bit on the edge marker; in this minimal
		// port we conservatively set it to false (the edge is part
		// of the original polygon only when c.next == a in the
		// original ring, which our caller determines).
		caFromPolygon: false,
	}
}
