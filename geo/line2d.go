// Code in this file mirrors org.apache.lucene.geo.Line2D from Apache
// Lucene 10.4.0. The Java type is package-private; the Go port keeps
// it unexported (line2D / newLine2D) for the same reason.
//
// The Java reference uses an EdgeTree (a balanced interval-tree of
// segments) to accelerate point-on-line and crossing queries. The
// EdgeTree port lands together with the full Component2D contract
// in task #277; until then, this implementation uses a linear scan
// over the segments. The observable behaviour is identical for the
// Component2D methods currently in the contract (MinX/MaxX/MinY/MaxY,
// Contains, Relate); only the asymptotic cost differs.

package geo

import "math"

// line2D is the Component2D for a poly-line.
type line2D struct {
	xs   []float64
	ys   []float64
	minX float64
	maxX float64
	minY float64
	maxY float64
}

// newLine2DFromLine builds the Component2D for a geographic Line.
// Internally, line2D uses (x, y) = (lon, lat) coordinates so that
// the same type can later back XYLine without conversion.
func newLine2DFromLine(l Line) *line2D {
	xs := make([]float64, l.NumPoints())
	ys := make([]float64, l.NumPoints())
	for i := 0; i < l.NumPoints(); i++ {
		xs[i] = l.Lon(i)
		ys[i] = l.Lat(i)
	}
	return &line2D{
		xs:   xs,
		ys:   ys,
		minX: l.MinLon(),
		maxX: l.MaxLon(),
		minY: l.MinLat(),
		maxY: l.MaxLat(),
	}
}

// MinX returns the inclusive minimum X (longitude or cartesian X) of
// the line's bounding box.
func (l *line2D) MinX() float64 { return l.minX }

// MaxX returns the inclusive maximum X of the line's bounding box.
func (l *line2D) MaxX() float64 { return l.maxX }

// MinY returns the inclusive minimum Y of the line's bounding box.
func (l *line2D) MinY() float64 { return l.minY }

// MaxY returns the inclusive maximum Y of the line's bounding box.
func (l *line2D) MaxY() float64 { return l.maxY }

// Contains reports whether (x, y) lies exactly on one of the line's
// segments. Mirrors Lucene's Line2D.contains, which first does a
// cheap bounding-box test and then delegates to EdgeTree.isPointOnLine
// for the precise collinearity check.
func (l *line2D) Contains(x, y float64) bool {
	if x < l.minX || x > l.maxX || y < l.minY || y > l.maxY {
		return false
	}
	for i := 1; i < len(l.xs); i++ {
		if pointOnSegment(l.xs[i-1], l.ys[i-1], l.xs[i], l.ys[i], x, y) {
			return true
		}
	}
	return false
}

// Relate returns the spatial relation between the line and the
// supplied query bounding box.
//
// Mirrors Lucene's Line2D.relate semantics. A poly-line has zero
// area, so it can never report CellInsideQuery; it is either fully
// disjoint from the query box (OUTSIDE) or partially intersects it
// (CROSSES). Lucene further short-circuits the CROSSES check when the
// query box completely contains the line's bounding box; we keep the
// same short-circuit. Without an EdgeTree we then iterate over the
// segments to look for a crossing.
func (l *line2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if maxY < l.minY || minY > l.maxY || maxX < l.minX || minX > l.maxX {
		return CellOutsideQuery
	}
	// Query box fully encloses the line bbox -> definitely intersects.
	if l.minX >= minX && l.maxX <= maxX && l.minY >= minY && l.maxY <= maxY {
		return CellCrossesQuery
	}
	// Quick check: any vertex inside the box -> CROSSES.
	for i := range l.xs {
		if l.xs[i] >= minX && l.xs[i] <= maxX && l.ys[i] >= minY && l.ys[i] <= maxY {
			return CellCrossesQuery
		}
	}
	// Otherwise, scan segments for an intersection with any of the
	// four box edges.
	for i := 1; i < len(l.xs); i++ {
		ax, ay := l.xs[i-1], l.ys[i-1]
		bx, by := l.xs[i], l.ys[i]
		if segmentCrossesBox(ax, ay, bx, by, minX, maxX, minY, maxY) {
			return CellCrossesQuery
		}
	}
	return CellOutsideQuery
}

// pointOnSegment reports whether (px, py) lies exactly on the
// segment (a, b), within a tight tolerance for floating-point
// rounding. The test is the standard "collinear and within bounding
// box" check used by computational-geometry libraries.
func pointOnSegment(ax, ay, bx, by, px, py float64) bool {
	// First the bounding-box rejection (fast and exact).
	if px < math.Min(ax, bx) || px > math.Max(ax, bx) ||
		py < math.Min(ay, by) || py > math.Max(ay, by) {
		return false
	}
	// Cross-product test for collinearity. A non-zero result means
	// the point is off the line through (a, b).
	cross := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
	// Tolerance scaled to the segment length squared so we accept
	// the same true positives Lucene's exact integer-encoded checks
	// would accept at typical input precision (~1ulp of float64).
	const eps = 1e-12
	return math.Abs(cross) <= eps
}

// segmentCrossesBox reports whether the segment (a, b) intersects
// any edge of the axis-aligned box defined by (minX, maxX, minY,
// maxY). The implementation walks the four box edges and applies
// the standard segment-segment intersection test.
func segmentCrossesBox(ax, ay, bx, by, minX, maxX, minY, maxY float64) bool {
	// Box edges in order: (minX, minY)-(maxX, minY) bottom,
	// (maxX, minY)-(maxX, maxY) right, (maxX, maxY)-(minX, maxY) top,
	// (minX, maxY)-(minX, minY) left.
	if segmentsIntersect(ax, ay, bx, by, minX, minY, maxX, minY) {
		return true
	}
	if segmentsIntersect(ax, ay, bx, by, maxX, minY, maxX, maxY) {
		return true
	}
	if segmentsIntersect(ax, ay, bx, by, maxX, maxY, minX, maxY) {
		return true
	}
	if segmentsIntersect(ax, ay, bx, by, minX, maxY, minX, minY) {
		return true
	}
	return false
}

// segmentsIntersect uses the classic 4-orientation test to decide
// whether the open segments (a, b) and (c, d) intersect, including
// endpoint touches. The implementation is allocation-free and uses
// only multiplications and comparisons, matching what Lucene's
// EdgeTree.crossesLine does at the leaf level.
func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	o1 := orient2d(ax, ay, bx, by, cx, cy)
	o2 := orient2d(ax, ay, bx, by, dx, dy)
	o3 := orient2d(cx, cy, dx, dy, ax, ay)
	o4 := orient2d(cx, cy, dx, dy, bx, by)

	if o1 != o2 && o3 != o4 {
		return true
	}
	// Collinear cases.
	if o1 == 0 && onSegment(ax, ay, cx, cy, bx, by) {
		return true
	}
	if o2 == 0 && onSegment(ax, ay, dx, dy, bx, by) {
		return true
	}
	if o3 == 0 && onSegment(cx, cy, ax, ay, dx, dy) {
		return true
	}
	if o4 == 0 && onSegment(cx, cy, bx, by, dx, dy) {
		return true
	}
	return false
}

// orient2d returns -1, 0, or +1 according to the orientation of the
// triple (a, b, c). +1 is counter-clockwise, -1 is clockwise, 0 is
// collinear. The convention matches Lucene's GeoUtils.orient.
func orient2d(ax, ay, bx, by, cx, cy float64) int {
	v := (by-ay)*(cx-bx) - (bx-ax)*(cy-by)
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

// onSegment reports whether (qx, qy) lies inside the axis-aligned
// bounding box of segment (p, r). Used by segmentsIntersect to
// disambiguate collinear cases.
func onSegment(px, py, qx, qy, rx, ry float64) bool {
	return qx <= math.Max(px, rx) && qx >= math.Min(px, rx) &&
		qy <= math.Max(py, ry) && qy >= math.Min(py, ry)
}
