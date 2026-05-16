// Code in this file mirrors org.apache.lucene.geo.Point2D from Apache
// Lucene 10.4.0. The Java type is package-private; the Go port keeps
// it unexported (point2D) for the same reason.
//
// Only the methods required by the current Component2D contract are
// implemented at this stage; the full Component2D surface
// (withinPoint, intersectsLine, containsTriangle, ...) is owned by
// task #277.

package geo

// point2D is a Component2D representing a single (x, y) point.
// Coordinates are exact; the bounding box collapses to a single
// point.
type point2D struct {
	x float64
	y float64
}

// newPoint2D constructs a Point2D Component from an (x, y) pair.
// Inputs are expected to have been validated upstream; this type
// does not perform domain checks.
func newPoint2D(x, y float64) *point2D {
	return &point2D{x: x, y: y}
}

// MinX returns the X coordinate of the point.
func (p *point2D) MinX() float64 { return p.x }

// MaxX returns the X coordinate of the point.
func (p *point2D) MaxX() float64 { return p.x }

// MinY returns the Y coordinate of the point.
func (p *point2D) MinY() float64 { return p.y }

// MaxY returns the Y coordinate of the point.
func (p *point2D) MaxY() float64 { return p.y }

// Contains reports whether (x, y) coincides exactly with the point.
// Mirrors Lucene's Point2D.contains which uses strict equality (no
// tolerance).
func (p *point2D) Contains(x, y float64) bool { return x == p.x && y == p.y }

// Relate returns the spatial relationship between the point and the
// query bounding box. Mirrors Lucene's Point2D.relate:
//
//   - If the point lies strictly outside the box -> CellOutsideQuery.
//   - Otherwise -> CellCrossesQuery (a single point can never fully
//     contain a non-degenerate box).
//
// Note that Lucene's Point2D.relate never returns CellInsideQuery
// because a 0-area point cannot fully contain a query cell.
func (p *point2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if p.x < minX || p.x > maxX || p.y < minY || p.y > maxY {
		return CellOutsideQuery
	}
	return CellCrossesQuery
}
