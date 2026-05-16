// Code in this file mirrors org.apache.lucene.geo.Polygon2D from
// Apache Lucene 10.4.0. The Java reference uses an EdgeTree
// (balanced interval-tree of segments) and stores holes as a nested
// Component2D; the Go port currently uses a linear-scan ray-casting
// PNPOLY for Contains and a linear edge-crossing scan for Relate /
// IntersectsX / WithinX. The observable Component2D behaviour
// matches Java; only the asymptotic cost differs.

package geo

// polygon2D is the Component2D for a polygon shell plus zero or
// more holes (themselves polygon2D instances over the hole shells).
type polygon2D struct {
	xs    []float64
	ys    []float64
	holes []*polygon2D
	minX  float64
	maxX  float64
	minY  float64
	maxY  float64
}

// newPolygon2DFromPolygon builds the Component2D for a Polygon.
func newPolygon2DFromPolygon(p Polygon) *polygon2D {
	c := &polygon2D{
		xs:   p.PolyLons(),
		ys:   p.PolyLats(),
		minX: p.MinLon(),
		maxX: p.MaxLon(),
		minY: p.MinLat(),
		maxY: p.MaxLat(),
	}
	if p.NumHoles() > 0 {
		c.holes = make([]*polygon2D, p.NumHoles())
		for i := 0; i < p.NumHoles(); i++ {
			c.holes[i] = newPolygon2DFromPolygon(p.Hole(i))
		}
	}
	return c
}

// newPolygon2DFromXY builds the Component2D for an XYPolygon.
func newPolygon2DFromXY(xs, ys []float64, holes []*polygon2D) *polygon2D {
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
	return &polygon2D{xs: xs2, ys: ys2, holes: holes, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// MinX / MaxX / MinY / MaxY accessors.
func (p *polygon2D) MinX() float64 { return p.minX }
func (p *polygon2D) MaxX() float64 { return p.maxX }
func (p *polygon2D) MinY() float64 { return p.minY }
func (p *polygon2D) MaxY() float64 { return p.maxY }

// Contains reports whether (x, y) lies inside the polygon shell
// (with holes subtracted) using PNPOLY ray-casting.
func (p *polygon2D) Contains(x, y float64) bool {
	if !BoxContainsPoint(x, y, p.minX, p.maxX, p.minY, p.maxY) {
		return false
	}
	if !p.shellContains(x, y) {
		return false
	}
	for _, h := range p.holes {
		if h.shellContains(x, y) {
			return false
		}
	}
	return true
}

// shellContains is the PNPOLY shell test (no hole correction).
func (p *polygon2D) shellContains(x, y float64) bool {
	inside := false
	n := len(p.xs)
	if n < 2 {
		return false
	}
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		yi, yj := p.ys[i], p.ys[j]
		xi, xj := p.xs[i], p.xs[j]
		if (yi > y) != (yj > y) {
			xIntersect := (xj-xi)*(y-yi)/(yj-yi) + xi
			if x < xIntersect {
				inside = !inside
			}
		}
	}
	return inside
}

// Relate returns the spatial relation between the polygon and the
// query bounding box. Matches Lucene's Polygon2D.relate at the
// observable level.
func (p *polygon2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if Disjoint(p.minX, p.maxX, p.minY, p.maxY, minX, maxX, minY, maxY) {
		return CellOutsideQuery
	}
	allInside := p.Contains(minX, minY) && p.Contains(maxX, minY) &&
		p.Contains(minX, maxY) && p.Contains(maxX, maxY)
	anyEdgeCrosses := p.shellOrHoleCrossesBox(minX, maxX, minY, maxY)
	if allInside && !anyEdgeCrosses {
		return CellInsideQuery
	}
	if anyEdgeCrosses {
		return CellCrossesQuery
	}
	cx := (minX + maxX) * 0.5
	cy := (minY + maxY) * 0.5
	if p.Contains(cx, cy) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}

// IntersectsLine reports whether segment (a, b) intersects the
// polygon (including its boundary edges).
func (p *polygon2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if Disjoint(p.minX, p.maxX, p.minY, p.maxY, minX, maxX, minY, maxY) {
		return false
	}
	if p.Contains(aX, aY) || p.Contains(bX, bY) {
		return true
	}
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		if LineCrossesLineWithBoundary(p.xs[j], p.ys[j], p.xs[i], p.ys[i], aX, aY, bX, bY) {
			return true
		}
	}
	for _, h := range p.holes {
		if h.shellIntersectsLine(aX, aY, bX, bY) {
			return true
		}
	}
	return false
}

// IntersectsTriangle reports whether triangle (a, b, c) intersects
// the polygon.
func (p *polygon2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if Disjoint(p.minX, p.maxX, p.minY, p.maxY, minX, maxX, minY, maxY) {
		return false
	}
	if p.Contains(aX, aY) || p.Contains(bX, bY) || p.Contains(cX, cY) {
		return true
	}
	if PointInTriangle(minX, maxX, minY, maxY, p.xs[0], p.ys[0], aX, aY, bX, bY, cX, cY) {
		return true
	}
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		x0, y0 := p.xs[j], p.ys[j]
		x1, y1 := p.xs[i], p.ys[i]
		if LineCrossesLineWithBoundary(x0, y0, x1, y1, aX, aY, bX, bY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, bX, bY, cX, cY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, cX, cY, aX, aY) {
			return true
		}
	}
	return false
}

// ContainsLine reports whether the polygon fully contains segment
// (a, b). Sufficient condition: both endpoints inside the polygon
// AND no shell/hole edge crosses the segment.
func (p *polygon2D) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if !p.Contains(aX, aY) || !p.Contains(bX, bY) {
		return false
	}
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		if LineCrossesLineWithBoundary(p.xs[j], p.ys[j], p.xs[i], p.ys[i], aX, aY, bX, bY) {
			return false
		}
	}
	for _, h := range p.holes {
		if h.shellIntersectsLine(aX, aY, bX, bY) {
			return false
		}
	}
	return true
}

// ContainsTriangle reports whether the polygon fully contains the
// triangle (a, b, c).
func (p *polygon2D) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if !p.Contains(aX, aY) || !p.Contains(bX, bY) || !p.Contains(cX, cY) {
		return false
	}
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		x0, y0 := p.xs[j], p.ys[j]
		x1, y1 := p.xs[i], p.ys[i]
		if LineCrossesLineWithBoundary(x0, y0, x1, y1, aX, aY, bX, bY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, bX, bY, cX, cY) ||
			LineCrossesLineWithBoundary(x0, y0, x1, y1, cX, cY, aX, aY) {
			return false
		}
	}
	return true
}

// WithinPoint returns NOTWITHIN if the point is inside the polygon,
// DISJOINT otherwise.
func (p *polygon2D) WithinPoint(x, y float64) WithinRelation {
	if p.Contains(x, y) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

// WithinLine is the polygon-vs-segment within relation.
func (p *polygon2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if Disjoint(p.minX, p.maxX, p.minY, p.maxY, minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if p.Contains(aX, aY) || p.Contains(bX, bY) {
		return WithinNotWithin
	}
	if ab && p.crossesAnyEdge(aX, aY, bX, bY) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

// WithinTriangle implements the polygon-vs-triangle within
// relation. Faithful Go translation of the conservative variant
// used by Polygon2D for the cases reachable through Component2D
// alone.
func (p *polygon2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if Disjoint(p.minX, p.maxX, p.minY, p.maxY, minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if p.Contains(aX, aY) || p.Contains(bX, bY) || p.Contains(cX, cY) {
		return WithinNotWithin
	}
	relation := WithinDisjoint
	if p.crossesAnyEdge(aX, aY, bX, bY) {
		if ab {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if p.crossesAnyEdge(bX, bY, cX, cY) {
		if bc {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if p.crossesAnyEdge(cX, cY, aX, aY) {
		if ca {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if relation == WithinCandidate {
		return WithinCandidate
	}
	if PointInTriangle(minX, maxX, minY, maxY, p.xs[0], p.ys[0], aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return relation
}

// shellOrHoleCrossesBox reports whether any shell or hole edge
// crosses the box boundary.
func (p *polygon2D) shellOrHoleCrossesBox(minX, maxX, minY, maxY float64) bool {
	if p.shellCrossesBox(minX, maxX, minY, maxY) {
		return true
	}
	for _, h := range p.holes {
		if h.shellCrossesBox(minX, maxX, minY, maxY) {
			return true
		}
	}
	return false
}

// shellCrossesBox walks shell edges looking for a box intersection.
func (p *polygon2D) shellCrossesBox(minX, maxX, minY, maxY float64) bool {
	n := len(p.xs)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		ax, ay := p.xs[j], p.ys[j]
		bx, by := p.xs[i], p.ys[i]
		if segmentCrossesBox(ax, ay, bx, by, minX, maxX, minY, maxY) {
			return true
		}
	}
	return false
}

// crossesAnyEdge reports whether segment (a, b) crosses any
// shell or hole edge (boundary-inclusive).
func (p *polygon2D) crossesAnyEdge(aX, aY, bX, bY float64) bool {
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		if LineCrossesLineWithBoundary(p.xs[j], p.ys[j], p.xs[i], p.ys[i], aX, aY, bX, bY) {
			return true
		}
	}
	for _, h := range p.holes {
		if h.crossesAnyEdge(aX, aY, bX, bY) {
			return true
		}
	}
	return false
}

// shellIntersectsLine reports whether any shell edge crosses
// segment (a, b). Used by hole-pierce tests.
func (p *polygon2D) shellIntersectsLine(aX, aY, bX, bY float64) bool {
	for i, j := 0, len(p.xs)-1; i < len(p.xs); j, i = i, i+1 {
		if LineCrossesLineWithBoundary(p.xs[j], p.ys[j], p.xs[i], p.ys[i], aX, aY, bX, bY) {
			return true
		}
	}
	return false
}
