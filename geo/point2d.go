// Code in this file mirrors org.apache.lucene.geo.Point2D from Apache
// Lucene 10.4.0. The Java type is package-private; the Go port keeps
// it unexported (point2D) for the same reason.

package geo

// point2D is a Component2D representing a single (x, y) point.
// Coordinates are exact; the bounding box collapses to a single
// point.
type point2D struct {
	x float64
	y float64
}

// newPoint2D constructs a Point2D Component from an (x, y) pair.
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
func (p *point2D) Contains(x, y float64) bool { return x == p.x && y == p.y }

// Relate returns the spatial relationship between the point and the
// query bounding box. A single point can never report
// CellInsideQuery because it has zero area.
func (p *point2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if BoxContainsPoint(p.x, p.y, minX, maxX, minY, maxY) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}

// IntersectsLine reports whether the segment passes through the
// point. The Java reference combines a bbox containment check with
// an orientation test (the point must lie exactly on the line).
func (p *point2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	return BoxContainsPoint(p.x, p.y, minX, maxX, minY, maxY) &&
		Orient(aX, aY, bX, bY, p.x, p.y) == 0
}

// IntersectsTriangle delegates to PointInTriangle.
func (p *point2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	return PointInTriangle(minX, maxX, minY, maxY, p.x, p.y, aX, aY, bX, bY, cX, cY)
}

// ContainsLine always returns false — a single point cannot contain
// a non-degenerate line segment.
func (p *point2D) ContainsLine(_, _, _, _, _, _, _, _ float64) bool { return false }

// ContainsTriangle always returns false.
func (p *point2D) ContainsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool { return false }

// WithinPoint returns CANDIDATE if the point matches, DISJOINT
// otherwise.
func (p *point2D) WithinPoint(x, y float64) WithinRelation {
	if p.Contains(x, y) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// WithinLine returns CANDIDATE if the point lies on the segment,
// DISJOINT otherwise.
func (p *point2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if p.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// WithinTriangle returns CANDIDATE if the point lies inside the
// triangle, DISJOINT otherwise.
func (p *point2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if PointInTriangle(minX, maxX, minY, maxY, p.x, p.y, aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return WithinDisjoint
}
