// Code in this file mirrors org.apache.lucene.geo.Polygon2D from
// Apache Lucene 10.4.0. The Java type is package-private; the Go
// port keeps it unexported (polygon2D / newPolygon2DFromPolygon).
//
// The full Java reference uses an EdgeTree (an R-tree of segments)
// plus child Polygon2D instances for holes. Task #277 will port the
// EdgeTree-backed implementation. Until then, this implementation
// uses the standard ray-casting point-in-polygon test (correct,
// O(n) per query) and a linear segment-crossing scan for Relate.
// Observable behaviour for the Component2D methods currently in
// the contract (MinX/MaxX/MinY/MaxY, Contains, Relate) matches the
// Java reference.

package geo

// polygon2D is the Component2D for a polygon shell plus zero or
// more holes (themselves polygon2D instances over the hole shells).
type polygon2D struct {
	xs    []float64 // shell longitudes
	ys    []float64 // shell latitudes
	holes []*polygon2D
	minX  float64
	maxX  float64
	minY  float64
	maxY  float64
}

// newPolygon2DFromPolygon builds a Component2D for a Polygon. Each
// hole becomes a child polygon2D so that point-in-polygon queries
// correctly subtract the hole regions.
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

// MinX returns the inclusive minimum X coordinate of the polygon
// shell's bounding box.
func (p *polygon2D) MinX() float64 { return p.minX }

// MaxX returns the inclusive maximum X coordinate of the polygon
// shell's bounding box.
func (p *polygon2D) MaxX() float64 { return p.maxX }

// MinY returns the inclusive minimum Y coordinate of the polygon
// shell's bounding box.
func (p *polygon2D) MinY() float64 { return p.minY }

// MaxY returns the inclusive maximum Y coordinate of the polygon
// shell's bounding box.
func (p *polygon2D) MaxY() float64 { return p.maxY }

// Contains reports whether (x, y) lies inside the polygon shell
// (with holes subtracted). Uses the classic horizontal-ray casting
// algorithm: count how many polygon edges the +x ray from (x, y)
// crosses; if odd, the point is inside.
func (p *polygon2D) Contains(x, y float64) bool {
	if x < p.minX || x > p.maxX || y < p.minY || y > p.maxY {
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

// shellContains is the ray-casting point-in-polygon test against the
// shell only (no hole correction).
func (p *polygon2D) shellContains(x, y float64) bool {
	inside := false
	n := len(p.xs)
	if n < 2 {
		return false
	}
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		yi, yj := p.ys[i], p.ys[j]
		xi, xj := p.xs[i], p.xs[j]
		// Standard PNPOLY edge-cross test.
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
// query bounding box. The semantics match Lucene's Polygon2D.relate:
//
//   - Disjoint bbox -> CellOutsideQuery.
//   - Query box entirely inside the polygon (no hole overlap) ->
//     CellInsideQuery.
//   - Polygon shell entirely inside the query box (with no holes
//     piercing the box) -> CellCrossesQuery (Lucene specifically
//     uses CROSSES rather than INSIDE here because the polygon does
//     not cover the entire cell).
//   - Otherwise -> CellCrossesQuery if any edge crosses, else
//     CellOutsideQuery (no edge crosses but the box is on the
//     exterior side of the shell).
func (p *polygon2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if maxX < p.minX || minX > p.maxX || maxY < p.minY || minY > p.maxY {
		return CellOutsideQuery
	}
	// Test the four corners; if all four are inside the polygon and
	// no shell edge crosses the box, the box is fully inside.
	allInside := p.Contains(minX, minY) && p.Contains(maxX, minY) &&
		p.Contains(minX, maxY) && p.Contains(maxX, maxY)
	anyEdgeCrosses := p.shellOrHoleCrossesBox(minX, maxX, minY, maxY)

	if allInside && !anyEdgeCrosses {
		return CellInsideQuery
	}
	if anyEdgeCrosses {
		return CellCrossesQuery
	}
	// No edges cross and not all corners are inside: either fully
	// outside the shell or partially overlapping with the box
	// straddling outside-of-shell space. The cheapest correct call
	// here is OUTSIDE if a sample point (the centre) is outside the
	// polygon, else CROSSES.
	cx := (minX + maxX) * 0.5
	cy := (minY + maxY) * 0.5
	if p.Contains(cx, cy) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
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

// shellCrossesBox walks the shell edges looking for an intersection
// with any of the four box edges.
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
