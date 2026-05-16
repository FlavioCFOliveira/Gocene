// Code in this file mirrors org.apache.lucene.geo.Component2D from
// Apache Lucene 10.4.0. The interface is intentionally minimal at this
// point of the port and will be expanded by subsequent tasks in the
// geo Sprint (the full Component2D contract is owned by task #277).

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

// Component2D is the 2D geometry engine used to evaluate spatial
// relationships against bounding boxes, lines, triangles, and points.
//
// This interface is the Go port of org.apache.lucene.geo.Component2D.
// At this stage of the port, only the methods required to satisfy the
// public Geometry / LatLonGeometry / XYGeometry contracts are declared.
// The remaining methods (intersectsLine, intersectsTriangle,
// containsLine, containsTriangle, withinPoint, withinLine,
// withinTriangle) will be added together with the concrete
// implementations in task #277 (Port org.apache.lucene.geo.Component2D).
type Component2D interface {
	// MinX returns the minimum X (longitude or cartesian-x) value
	// covered by the component.
	MinX() float64

	// MaxX returns the maximum X value covered by the component.
	MaxX() float64

	// MinY returns the minimum Y (latitude or cartesian-y) value
	// covered by the component.
	MinY() float64

	// MaxY returns the maximum Y value covered by the component.
	MaxY() float64

	// Contains reports whether the given (x, y) point lies inside the
	// component.
	Contains(x, y float64) bool

	// Relate returns the relationship between the component and the
	// supplied bounding box.
	Relate(minX, maxX, minY, maxY float64) Relation
}
