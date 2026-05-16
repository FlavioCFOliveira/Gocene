// Code in this file mirrors org.apache.lucene.geo.Component2D from
// Apache Lucene 10.4.0.

package geo

// Relation describes the spatial relationship between a bounding box
// (or cell) and a query shape. It mirrors
// org.apache.lucene.index.PointValues.Relation, which is the type
// returned by Component2D.relate in Java.
//
// The geo package declares its own Relation rather than importing the
// codecs package definition to preserve a clean dependency layer: geo
// is a low-level primitive package and must not depend on codecs.
type Relation int

const (
	// CellInsideQuery means the cell is fully contained inside the
	// query shape; every point in the cell satisfies the query.
	CellInsideQuery Relation = iota

	// CellOutsideQuery means the cell does not intersect the query
	// shape; no point in the cell satisfies the query.
	CellOutsideQuery

	// CellCrossesQuery means the cell partially intersects the query
	// shape; some points may satisfy the query and some may not.
	CellCrossesQuery
)

// String returns the symbolic name of the relation, matching the Java
// enum constant names from PointValues.Relation.
func (r Relation) String() string {
	switch r {
	case CellInsideQuery:
		return "CELL_INSIDE_QUERY"
	case CellOutsideQuery:
		return "CELL_OUTSIDE_QUERY"
	case CellCrossesQuery:
		return "CELL_CROSSES_QUERY"
	default:
		return "UNKNOWN"
	}
}

// WithinRelation is the result type of the withinX family of
// Component2D methods. It mirrors
// org.apache.lucene.geo.Component2D.WithinRelation.
type WithinRelation int

const (
	// WithinCandidate means the input shape is a candidate for
	// within: typically the query shape lies fully inside the
	// triangle, or it intersects only edges that do not belong to
	// the original shape.
	WithinCandidate WithinRelation = iota

	// WithinNotWithin means the input shape intersects an edge that
	// belongs to the original shape, or some point of the input
	// triangle is inside the shape.
	WithinNotWithin

	// WithinDisjoint means the input shape is disjoint with the
	// triangle.
	WithinDisjoint
)

// String returns the symbolic name of the WithinRelation.
func (w WithinRelation) String() string {
	switch w {
	case WithinCandidate:
		return "CANDIDATE"
	case WithinNotWithin:
		return "NOTWITHIN"
	case WithinDisjoint:
		return "DISJOINT"
	default:
		return "UNKNOWN"
	}
}

// Component2D is the 2D spatial-relationship engine used by all
// concrete geometries in the geo package. It is the Go port of
// org.apache.lucene.geo.Component2D.
//
// The interface is exported because external packages (search,
// codecs) need to consume it, but its implementations are kept
// inside the geo package: the concrete types satisfying it
// (point2D, rectangle2D, line2D, polygon2D, circle2D,
// multiComponent2D) are all unexported and reachable only via
// the geo factories. Custom Component2D implementations are
// possible but are explicitly out of scope for the byte-for-byte
// compatibility contract.
type Component2D interface {
	// MinX returns the inclusive minimum X (longitude or
	// cartesian-x) of the component's bounding box.
	MinX() float64

	// MaxX returns the inclusive maximum X of the bounding box.
	MaxX() float64

	// MinY returns the inclusive minimum Y of the bounding box.
	MinY() float64

	// MaxY returns the inclusive maximum Y of the bounding box.
	MaxY() float64

	// Contains reports whether (x, y) lies inside the component.
	Contains(x, y float64) bool

	// Relate returns the relationship between the component and the
	// supplied bounding box.
	Relate(minX, maxX, minY, maxY float64) Relation

	// IntersectsLine reports whether the component intersects the
	// segment (a, b) whose bounding box is (minX, maxX, minY, maxY).
	// Mirrors Component2D#intersectsLine(8 args).
	IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool

	// IntersectsTriangle reports whether the component intersects
	// the triangle (a, b, c) whose bounding box is (minX, maxX,
	// minY, maxY).
	IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool

	// ContainsLine reports whether the component contains the
	// segment (a, b).
	ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool

	// ContainsTriangle reports whether the component contains the
	// triangle (a, b, c).
	ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool

	// WithinPoint returns the within relation for the point (x, y).
	WithinPoint(x, y float64) WithinRelation

	// WithinLine returns the within relation for the segment (a, b)
	// whose bounding box is (minX, maxX, minY, maxY). The `ab` flag
	// is true when the segment is an edge of the original shape.
	WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation

	// WithinTriangle returns the within relation for the triangle
	// (a, b, c) whose bounding box is (minX, maxX, minY, maxY). The
	// ab / bc / ca flags indicate whether each edge belongs to the
	// original shape.
	WithinTriangle(
		minX, maxX, minY, maxY,
		aX, aY float64, ab bool,
		bX, bY float64, bc bool,
		cX, cY float64, ca bool,
	) WithinRelation
}

// IntersectsLine is the default 4-arg helper that computes the
// segment bounding box and delegates to the 8-arg method on the
// component. Mirrors Component2D.intersectsLine(4 args).
func IntersectsLineDefault(c Component2D, aX, aY, bX, bY float64) bool {
	minX := minFloat(aX, bX)
	maxX := maxFloat(aX, bX)
	minY := minFloat(aY, bY)
	maxY := maxFloat(aY, bY)
	return c.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY)
}

// IntersectsTriangleDefault is the default 6-arg helper.
func IntersectsTriangleDefault(c Component2D, aX, aY, bX, bY, cX, cY float64) bool {
	minX := minFloat3(aX, bX, cX)
	maxX := maxFloat3(aX, bX, cX)
	minY := minFloat3(aY, bY, cY)
	maxY := maxFloat3(aY, bY, cY)
	return c.IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY)
}

// ContainsLineDefault is the default 4-arg helper.
func ContainsLineDefault(c Component2D, aX, aY, bX, bY float64) bool {
	minX := minFloat(aX, bX)
	maxX := maxFloat(aX, bX)
	minY := minFloat(aY, bY)
	maxY := maxFloat(aY, bY)
	return c.ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY)
}

// ContainsTriangleDefault is the default 6-arg helper.
func ContainsTriangleDefault(c Component2D, aX, aY, bX, bY, cX, cY float64) bool {
	minX := minFloat3(aX, bX, cX)
	maxX := maxFloat3(aX, bX, cX)
	minY := minFloat3(aY, bY, cY)
	maxY := maxFloat3(aY, bY, cY)
	return c.ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY)
}

// WithinLineDefault is the default 5-arg helper.
func WithinLineDefault(c Component2D, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	minX := minFloat(aX, bX)
	maxX := maxFloat(aX, bX)
	minY := minFloat(aY, bY)
	maxY := maxFloat(aY, bY)
	return c.WithinLine(minX, maxX, minY, maxY, aX, aY, ab, bX, bY)
}

// WithinTriangleDefault is the default 9-arg helper.
func WithinTriangleDefault(c Component2D,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	minX := minFloat3(aX, bX, cX)
	maxX := maxFloat3(aX, bX, cX)
	minY := minFloat3(aY, bY, cY)
	maxY := maxFloat3(aY, bY, cY)
	return c.WithinTriangle(minX, maxX, minY, maxY, aX, aY, ab, bX, bY, bc, cX, cY, ca)
}

// Disjoint reports whether two axis-aligned bounding boxes are
// disjoint. Mirrors Component2D.disjoint.
func Disjoint(minX1, maxX1, minY1, maxY1, minX2, maxX2, minY2, maxY2 float64) bool {
	return maxY1 < minY2 || minY1 > maxY2 || maxX1 < minX2 || minX1 > maxX2
}

// WithinBBox reports whether the first bounding box is within the
// second. Mirrors Component2D.within.
func WithinBBox(minX1, maxX1, minY1, maxY1, minX2, maxX2, minY2, maxY2 float64) bool {
	return minY2 <= minY1 && maxY2 >= maxY1 && minX2 <= minX1 && maxX2 >= maxX1
}

// BoxContainsPoint reports whether the rectangle (minX..maxX,
// minY..maxY) contains (x, y). Mirrors Component2D.containsPoint.
func BoxContainsPoint(x, y, minX, maxX, minY, maxY float64) bool {
	return x >= minX && x <= maxX && y >= minY && y <= maxY
}

// PointInTriangle reports whether (x, y) lies inside the triangle
// (a, b, c). The supplied (minX, maxX, minY, maxY) is the triangle's
// bounding box, used as a fast reject for degenerate triangles.
// Mirrors Component2D.pointInTriangle.
func PointInTriangle(minX, maxX, minY, maxY, x, y, aX, aY, bX, bY, cX, cY float64) bool {
	if !(x >= minX && x <= maxX && y >= minY && y <= maxY) {
		return false
	}
	a := Orient(x, y, aX, aY, bX, bY)
	b := Orient(x, y, bX, bY, cX, cY)
	if a == 0 || b == 0 || (a < 0) == (b < 0) {
		cc := Orient(x, y, cX, cY, aX, aY)
		return cc == 0 || ((cc < 0) == ((b < 0) || (a < 0)))
	}
	return false
}

// Small helpers kept local to avoid pulling math into the component
// hot path for everything.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat3(a, b, c float64) float64 { return minFloat(minFloat(a, b), c) }
func maxFloat3(a, b, c float64) float64 { return maxFloat(maxFloat(a, b), c) }
