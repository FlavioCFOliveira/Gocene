// Code in this file mirrors org.apache.lucene.geo.Rectangle2D from
// Apache Lucene 10.4.0. The Java type is package-private; the Go
// port keeps it unexported (rectangle2D / newRectangle2D).

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
// performs no domain checks.
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

// Contains reports whether (x, y) lies inside the rectangle.
func (r *rectangle2D) Contains(x, y float64) bool {
	return BoxContainsPoint(x, y, r.minX, r.maxX, r.minY, r.maxY)
}

// Relate returns the spatial relationship between the rectangle and
// the supplied query bounding box.
func (r *rectangle2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if Disjoint(r.minX, r.maxX, r.minY, r.maxY, minX, maxX, minY, maxY) {
		return CellOutsideQuery
	}
	if WithinBBox(minX, maxX, minY, maxY, r.minX, r.maxX, r.minY, r.maxY) {
		return CellInsideQuery
	}
	return CellCrossesQuery
}

// IntersectsLine reports whether the segment (a, b) crosses the
// rectangle.
func (r *rectangle2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if Disjoint(r.minX, r.maxX, r.minY, r.maxY, minX, maxX, minY, maxY) {
		return false
	}
	return r.Contains(aX, aY) || r.Contains(bX, bY) || r.edgesIntersect(aX, aY, bX, bY)
}

// IntersectsTriangle reports whether the triangle (a, b, c) crosses
// the rectangle.
func (r *rectangle2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if Disjoint(r.minX, r.maxX, r.minY, r.maxY, minX, maxX, minY, maxY) {
		return false
	}
	return r.Contains(aX, aY) || r.Contains(bX, bY) || r.Contains(cX, cY) ||
		PointInTriangle(minX, maxX, minY, maxY, r.minX, r.minY, aX, aY, bX, bY, cX, cY) ||
		r.edgesIntersect(aX, aY, bX, bY) ||
		r.edgesIntersect(bX, bY, cX, cY) ||
		r.edgesIntersect(cX, cY, aX, aY)
}

// ContainsLine reports whether the rectangle contains the segment;
// the cheapest sufficient condition is "segment bbox lies inside the
// rectangle". Matches the Java implementation exactly.
func (r *rectangle2D) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	return WithinBBox(minX, maxX, minY, maxY, r.minX, r.maxX, r.minY, r.maxY)
}

// ContainsTriangle reports whether the rectangle contains the
// triangle.
func (r *rectangle2D) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	return WithinBBox(minX, maxX, minY, maxY, r.minX, r.maxX, r.minY, r.maxY)
}

// WithinPoint returns NOTWITHIN if the point lies inside the
// rectangle (the rectangle is then not "within" the query),
// DISJOINT otherwise.
func (r *rectangle2D) WithinPoint(x, y float64) WithinRelation {
	if r.Contains(x, y) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

// WithinLine implements the rectangle-vs-segment within relation.
func (r *rectangle2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if Disjoint(r.minX, r.maxX, r.minY, r.maxY, minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if r.Contains(aX, aY) || r.Contains(bX, bY) {
		return WithinNotWithin
	}
	if ab && r.edgesIntersect(aX, aY, bX, bY) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

// WithinTriangle implements the rectangle-vs-triangle within
// relation. Faithful Go translation of Rectangle2D.withinTriangle.
func (r *rectangle2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if Disjoint(r.minX, r.maxX, r.minY, r.maxY, minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if r.Contains(aX, aY) || r.Contains(bX, bY) || r.Contains(cX, cY) {
		return WithinNotWithin
	}
	relation := WithinDisjoint
	if r.edgesIntersect(aX, aY, bX, bY) {
		if ab {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if r.edgesIntersect(bX, bY, cX, cY) {
		if bc {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if r.edgesIntersect(cX, cY, aX, aY) {
		if ca {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if relation == WithinCandidate {
		return WithinCandidate
	}
	if PointInTriangle(minX, maxX, minY, maxY, r.minX, r.minY, aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return relation
}

// minLonInclQuantize / maxLonInclQuantize mirror Java's
// MIN_LON_INCL_QUANTIZE / MAX_LON_INCL_QUANTIZE: the inclusive
// longitude bounds after a round-trip through the latlon encoder.
var (
	minLonInclQuantize = DecodeLongitude(MinLonEncoded)
	maxLonInclQuantize = DecodeLongitude(MaxLonEncoded)
)

// NewRectangle2DFromXY builds the Component2D for an XYRectangle.
// Mirrors Rectangle2D.create(XYRectangle).
func NewRectangle2DFromXY(rect XYRectangle) Component2D {
	return newRectangle2D(
		float64(rect.MinX()), float64(rect.MaxX()),
		float64(rect.MinY()), float64(rect.MaxY()),
	)
}

// NewRectangle2DFromLatLon builds the Component2D for a LatLon
// Rectangle, quantising the bounds through the latlon encoder and
// splitting dateline-crossing rectangles into a ComponentTree of two
// rectangles. Mirrors Rectangle2D.create(Rectangle).
func NewRectangle2DFromLatLon(rect Rectangle) Component2D {
	minLongitude := rect.MinLon()
	crossesDateline := rect.CrossesDateline()
	// Java edge case: a rectangle anchored exactly at +180 that crosses
	// the dateline collapses into a single component starting at -180.
	if minLongitude == 180.0 && crossesDateline {
		minLongitude = -180
		crossesDateline = false
	}
	qMinLat := DecodeLatitude(EncodeLatitudeCeil(rect.MinLat()))
	qMaxLat := DecodeLatitude(EncodeLatitude(rect.MaxLat()))
	qMinLon := DecodeLongitude(EncodeLongitudeCeil(minLongitude))
	qMaxLon := DecodeLongitude(EncodeLongitude(rect.MaxLon()))
	if crossesDateline {
		components := []Component2D{
			newRectangle2D(minLonInclQuantize, qMaxLon, qMinLat, qMaxLat),
			newRectangle2D(qMinLon, maxLonInclQuantize, qMinLat, qMaxLat),
		}
		return newComponentTree(components)
	}
	return newRectangle2D(qMinLon, qMaxLon, qMinLat, qMaxLat)
}

// edgesIntersect tests whether the segment (a, b) crosses any of the
// rectangle's four edges (boundary-inclusive). Mirrors the Java
// private helper of the same name.
func (r *rectangle2D) edgesIntersect(aX, aY, bX, bY float64) bool {
	// Bounding-box reject.
	if maxFloat(aX, bX) < r.minX || minFloat(aX, bX) > r.maxX ||
		minFloat(aY, bY) > r.maxY || maxFloat(aY, bY) < r.minY {
		return false
	}
	return LineCrossesLineWithBoundary(aX, aY, bX, bY, r.minX, r.maxY, r.maxX, r.maxY) || // top
		LineCrossesLineWithBoundary(aX, aY, bX, bY, r.maxX, r.maxY, r.maxX, r.minY) || // right
		LineCrossesLineWithBoundary(aX, aY, bX, bY, r.maxX, r.minY, r.minX, r.minY) || // bottom
		LineCrossesLineWithBoundary(aX, aY, bX, bY, r.minX, r.minY, r.minX, r.maxY) // left
}
