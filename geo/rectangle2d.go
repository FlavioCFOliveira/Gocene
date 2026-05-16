// Code in this file mirrors org.apache.lucene.geo.Rectangle2D from
// Apache Lucene 10.4.0. The Java type is package-private; the Go
// port keeps it unexported (rectangle2D / newRectangle2D) for the
// same reason.
//
// Only the Component2D methods required by the current contract are
// implemented; the full Component2D surface (intersectsLine,
// containsTriangle, ...) is owned by task #277.

package geo

// rectangle2D is the cartesian Component2D for an axis-aligned
// bounding box defined by its inclusive bounds.
type rectangle2D struct {
	minX float64
	maxX float64
	minY float64
	maxY float64
}

// newRectangle2D constructs a rectangle Component2D. Inputs are
// assumed to have been validated by the caller; this constructor
// performs no domain checks. The semantics mirror the Java
// Rectangle2D private constructor.
func newRectangle2D(minX, maxX, minY, maxY float64) *rectangle2D {
	return &rectangle2D{minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// MinX returns the inclusive minimum X coordinate.
func (r *rectangle2D) MinX() float64 { return r.minX }

// MaxX returns the inclusive maximum X coordinate.
func (r *rectangle2D) MaxX() float64 { return r.maxX }

// MinY returns the inclusive minimum Y coordinate.
func (r *rectangle2D) MinY() float64 { return r.minY }

// MaxY returns the inclusive maximum Y coordinate.
func (r *rectangle2D) MaxY() float64 { return r.maxY }

// Contains reports whether (x, y) lies inside the rectangle. Edges
// are inclusive on all four sides, matching Component2D.containsPoint
// in the Java reference.
func (r *rectangle2D) Contains(x, y float64) bool {
	return x >= r.minX && x <= r.maxX && y >= r.minY && y <= r.maxY
}

// Relate returns the spatial relationship between the rectangle and
// the supplied query bounding box. Mirrors Rectangle2D.relate:
//
//   - If the rectangle is disjoint from the query box -> OUTSIDE.
//   - If the query box is fully contained inside the rectangle ->
//     INSIDE (this is the only case where INSIDE is returned; note
//     that the within-check is "query inside this", not the other
//     way round).
//   - Otherwise -> CROSSES.
func (r *rectangle2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if maxY < r.minY || minY > r.maxY || maxX < r.minX || minX > r.maxX {
		return CellOutsideQuery
	}
	if minY >= r.minY && maxY <= r.maxY && minX >= r.minX && maxX <= r.maxX {
		return CellInsideQuery
	}
	return CellCrossesQuery
}
