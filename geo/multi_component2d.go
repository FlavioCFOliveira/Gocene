// multiComponent2D is the geo-package replacement for Lucene's
// package-private ComponentTree. ComponentTree is an interval tree
// used for performance when multiple Component2D instances are joined
// via LatLonGeometry.create / XYGeometry.create; its topology is not
// part of any public or serialised contract, so a functionally
// equivalent linear composite is acceptable.

package geo

// multiComponent2D answers Component2D queries as the set-union over
// a fixed list of child components.
type multiComponent2D struct {
	components []Component2D
	minX       float64
	maxX       float64
	minY       float64
	maxY       float64
}

// newMultiComponent2D builds a composite over the provided children.
func newMultiComponent2D(components []Component2D) *multiComponent2D {
	if len(components) == 0 {
		panic("geo: newMultiComponent2D requires at least one component")
	}
	copied := make([]Component2D, len(components))
	copy(copied, components)

	minX, maxX := copied[0].MinX(), copied[0].MaxX()
	minY, maxY := copied[0].MinY(), copied[0].MaxY()
	for i := 1; i < len(copied); i++ {
		c := copied[i]
		if v := c.MinX(); v < minX {
			minX = v
		}
		if v := c.MaxX(); v > maxX {
			maxX = v
		}
		if v := c.MinY(); v < minY {
			minY = v
		}
		if v := c.MaxY(); v > maxY {
			maxY = v
		}
	}
	return &multiComponent2D{
		components: copied,
		minX:       minX, maxX: maxX, minY: minY, maxY: maxY,
	}
}

func (m *multiComponent2D) MinX() float64 { return m.minX }
func (m *multiComponent2D) MaxX() float64 { return m.maxX }
func (m *multiComponent2D) MinY() float64 { return m.minY }
func (m *multiComponent2D) MaxY() float64 { return m.maxY }

// Contains returns true if any child contains the point.
func (m *multiComponent2D) Contains(x, y float64) bool {
	for _, c := range m.components {
		if c.Contains(x, y) {
			return true
		}
	}
	return false
}

// Relate aggregates per-child relations: INSIDE on any child wins;
// otherwise CROSSES if any child crosses; OUTSIDE only when every
// child is OUTSIDE.
func (m *multiComponent2D) Relate(minX, maxX, minY, maxY float64) Relation {
	hasCrosses := false
	for _, c := range m.components {
		switch c.Relate(minX, maxX, minY, maxY) {
		case CellInsideQuery:
			return CellInsideQuery
		case CellCrossesQuery:
			hasCrosses = true
		}
	}
	if hasCrosses {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}

// IntersectsLine returns true if any child intersects the segment.
func (m *multiComponent2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	for _, c := range m.components {
		if c.IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
			return true
		}
	}
	return false
}

// IntersectsTriangle returns true if any child intersects the
// triangle.
func (m *multiComponent2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	for _, c := range m.components {
		if c.IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY) {
			return true
		}
	}
	return false
}

// ContainsLine returns true if any single child contains the segment.
// The Java ComponentTree applies the same semantics: the segment
// counts as contained once any leaf says so.
func (m *multiComponent2D) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	for _, c := range m.components {
		if c.ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY) {
			return true
		}
	}
	return false
}

// ContainsTriangle returns true if any single child contains the
// triangle.
func (m *multiComponent2D) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	for _, c := range m.components {
		if c.ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY) {
			return true
		}
	}
	return false
}

// WithinPoint aggregates per-child within relations following the
// standard precedence: NOTWITHIN dominates, then CANDIDATE, then
// DISJOINT. Mirrors how ComponentTree walks its children.
func (m *multiComponent2D) WithinPoint(x, y float64) WithinRelation {
	relation := WithinDisjoint
	for _, c := range m.components {
		switch c.WithinPoint(x, y) {
		case WithinNotWithin:
			return WithinNotWithin
		case WithinCandidate:
			relation = WithinCandidate
		}
	}
	return relation
}

// WithinLine aggregates per-child relations as above.
func (m *multiComponent2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	relation := WithinDisjoint
	for _, c := range m.components {
		switch c.WithinLine(minX, maxX, minY, maxY, aX, aY, ab, bX, bY) {
		case WithinNotWithin:
			return WithinNotWithin
		case WithinCandidate:
			relation = WithinCandidate
		}
	}
	return relation
}

// WithinTriangle aggregates per-child relations as above.
func (m *multiComponent2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	relation := WithinDisjoint
	for _, c := range m.components {
		switch c.WithinTriangle(minX, maxX, minY, maxY, aX, aY, ab, bX, bY, bc, cX, cY, ca) {
		case WithinNotWithin:
			return WithinNotWithin
		case WithinCandidate:
			relation = WithinCandidate
		}
	}
	return relation
}
