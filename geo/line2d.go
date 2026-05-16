// Code in this file mirrors org.apache.lucene.geo.Line2D from Apache
// Lucene 10.4.0. The Java reference uses an EdgeTree for O(log n)
// crossing tests; this Go port iterates linearly over the segments,
// preserving observable behaviour at slower asymptotic cost. The
// EdgeTree port can land later without changing this contract.

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

// newLine2DFromXY builds the Component2D for an XY Line. Mirrors
// Lucene's Line2D.create(XYLine) which casts the float coordinates
// up to double via XYEncodingUtils.floatArrayToDoubleArray.
func newLine2DFromXY(xs, ys []float64) *line2D {
	xs2 := make([]float64, len(xs))
	ys2 := make([]float64, len(ys))
	copy(xs2, xs)
	copy(ys2, ys)
	minX, maxX := xs[0], xs[0]
	minY, maxY := ys[0], ys[0]
	for i := 1; i < len(xs); i++ {
		if xs[i] < minX {
			minX = xs[i]
		}
		if xs[i] > maxX {
			maxX = xs[i]
		}
		if ys[i] < minY {
			minY = ys[i]
		}
		if ys[i] > maxY {
			maxY = ys[i]
		}
	}
	return &line2D{xs: xs2, ys: ys2, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// MinX / MaxX / MinY / MaxY accessors.
func (l *line2D) MinX() float64 { return l.minX }
func (l *line2D) MaxX() float64 { return l.maxX }
func (l *line2D) MinY() float64 { return l.minY }
func (l *line2D) MaxY() float64 { return l.maxY }

// Contains reports whether (x, y) lies exactly on one of the line's
// segments.
func (l *line2D) Contains(x, y float64) bool {
	if !BoxContainsPoint(x, y, l.minX, l.maxX, l.minY, l.maxY) {
		return false
	}
	for i := 1; i < len(l.xs); i++ {
		if pointOnSegment(l.xs[i-1], l.ys[i-1], l.xs[i], l.ys[i], x, y) {
			return true
		}
	}
	return false
}

// Relate matches Lucene's Line2D.relate semantics: zero-area shape,
// so INSIDE never appears; OUTSIDE on disjoint bbox; CROSSES when
// the line bbox is fully enclosed by the query or any segment
// crosses the query box.
func (l *line2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if Disjoint(l.minX, l.maxX, l.minY, l.maxY, minX, maxX, minY, maxY) {
		return CellOutsideQuery
	}
	if WithinBBox(l.minX, l.maxX, l.minY, l.maxY, minX, maxX, minY, maxY) {
		return CellCrossesQuery
	}
	if l.crossesBox(minX, maxX, minY, maxY) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}

// IntersectsLine reports whether any of the line's segments crosses
// the query segment (a, b).
func (l *line2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if Disjoint(l.minX, l.maxX, l.minY, l.maxY, minX, maxX, minY, maxY) {
		return false
	}
	for i := 1; i < len(l.xs); i++ {
		if LineCrossesLineWithBoundary(l.xs[i-1], l.ys[i-1], l.xs[i], l.ys[i], aX, aY, bX, bY) {
			return true
		}
	}
	return false
}

// IntersectsTriangle reports whether any of the line's segments
// crosses any edge of the triangle, or whether the line lies wholly
// inside the triangle.
func (l *line2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if Disjoint(l.minX, l.maxX, l.minY, l.maxY, minX, maxX, minY, maxY) {
		return false
	}
	// First vertex of the line inside the triangle is sufficient.
	if PointInTriangle(minX, maxX, minY, maxY, l.xs[0], l.ys[0], aX, aY, bX, bY, cX, cY) {
		return true
	}
	for i := 1; i < len(l.xs); i++ {
		x0, y0 := l.xs[i-1], l.ys[i-1]
		x1, y1 := l.xs[i], l.ys[i]
		if LineCrossesLineWithBoundary(x0, y0, x1, y1, aX, aY, bX, bY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, bX, bY, cX, cY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, cX, cY, aX, aY) {
			return true
		}
	}
	return false
}

// ContainsLine always returns false: a 1D line cannot strictly
// contain another non-degenerate line. Matches Java.
func (l *line2D) ContainsLine(_, _, _, _, _, _, _, _ float64) bool { return false }

// ContainsTriangle always returns false.
func (l *line2D) ContainsTriangle(_, _, _, _, _, _, _, _, _, _ float64) bool { return false }

// WithinPoint returns CANDIDATE if the point is on the line,
// DISJOINT otherwise.
func (l *line2D) WithinPoint(x, y float64) WithinRelation {
	if l.Contains(x, y) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// WithinLine is approximated as IntersectsLine in the Java reference
// ("can be improved?" comment in Lucene). We mirror that behaviour.
func (l *line2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if l.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// WithinTriangle is approximated as IntersectsTriangle in the Java
// reference for Line2D ("can be improved?" comment). We mirror it.
func (l *line2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if l.IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// crossesBox reports whether any segment crosses the query box.
func (l *line2D) crossesBox(minX, maxX, minY, maxY float64) bool {
	for i := 1; i < len(l.xs); i++ {
		ax, ay := l.xs[i-1], l.ys[i-1]
		bx, by := l.xs[i], l.ys[i]
		if segmentCrossesBox(ax, ay, bx, by, minX, maxX, minY, maxY) {
			return true
		}
	}
	return false
}

// ----- helpers used by line2D.Contains and tests -----

// pointOnSegment reports whether (px, py) lies on segment (a, b)
// within a tight float tolerance.
func pointOnSegment(ax, ay, bx, by, px, py float64) bool {
	if px < math.Min(ax, bx) || px > math.Max(ax, bx) ||
		py < math.Min(ay, by) || py > math.Max(ay, by) {
		return false
	}
	cross := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
	const eps = 1e-12
	return math.Abs(cross) <= eps
}

// segmentCrossesBox reports whether segment (a, b) intersects any
// edge of the axis-aligned box.
func segmentCrossesBox(ax, ay, bx, by, minX, maxX, minY, maxY float64) bool {
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

// segmentsIntersect uses the classic 4-orientation test.
func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	o1 := orient2d(ax, ay, bx, by, cx, cy)
	o2 := orient2d(ax, ay, bx, by, dx, dy)
	o3 := orient2d(cx, cy, dx, dy, ax, ay)
	o4 := orient2d(cx, cy, dx, dy, bx, by)
	if o1 != o2 && o3 != o4 {
		return true
	}
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

// orient2d returns -1, 0, or +1.
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

// onSegment reports whether (qx, qy) lies inside the bbox of (p, r).
func onSegment(px, py, qx, qy, rx, ry float64) bool {
	return qx <= math.Max(px, rx) && qx >= math.Min(px, rx) &&
		qy <= math.Max(py, ry) && qy >= math.Min(py, ry)
}
