// multiComponent2D is the geo-package replacement for Lucene's
// package-private ComponentTree. ComponentTree is an interval tree
// used for performance when multiple Component2D instances are joined
// via LatLonGeometry.create / XYGeometry.create; its topology is not
// part of any public or serialised contract, so a functionally
// equivalent linear composite is acceptable while the full
// ComponentTree port is deferred to task #277.

package geo

// multiComponent2D answers Component2D queries as the set-union over
// a fixed list of child components. It is constructed once from the
// input slice and is safe for concurrent reads; the child slice is
// copied defensively at construction time.
type multiComponent2D struct {
	components []Component2D
	minX       float64
	maxX       float64
	minY       float64
	maxY       float64
}

// newMultiComponent2D builds a composite over the provided children.
// The caller must guarantee components is non-empty and free of nil
// entries; callers in this package satisfy that precondition by
// validating their inputs before calling.
func newMultiComponent2D(components []Component2D) *multiComponent2D {
	if len(components) == 0 {
		// Defensive: this should never happen because all callers
		// inside the geo package validate first, but a panic here is
		// preferable to a silent zero-value Component2D.
		panic("geo: newMultiComponent2D requires at least one component")
	}
	// Defensive copy so the composite is immune to caller-side
	// mutation of the original slice.
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
		minX:       minX,
		maxX:       maxX,
		minY:       minY,
		maxY:       maxY,
	}
}

// MinX returns the union minimum X across all child components.
func (m *multiComponent2D) MinX() float64 { return m.minX }

// MaxX returns the union maximum X across all child components.
func (m *multiComponent2D) MaxX() float64 { return m.maxX }

// MinY returns the union minimum Y across all child components.
func (m *multiComponent2D) MinY() float64 { return m.minY }

// MaxY returns the union maximum Y across all child components.
func (m *multiComponent2D) MaxY() float64 { return m.maxY }

// Contains reports whether any child component contains the point.
func (m *multiComponent2D) Contains(x, y float64) bool {
	for _, c := range m.components {
		if c.Contains(x, y) {
			return true
		}
	}
	return false
}

// Relate returns the union relation: INSIDE wins as soon as any
// child reports INSIDE; otherwise CROSSES wins if any child crosses
// or is inside; otherwise OUTSIDE.
//
// This matches the union semantics implemented by Lucene's
// ComponentTree, which short-circuits on INSIDE and aggregates
// CROSSES/OUTSIDE the same way.
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
