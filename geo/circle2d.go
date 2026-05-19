// Code in this file mirrors org.apache.lucene.geo.Circle2D from
// Apache Lucene 10.4.0.

package geo

import "github.com/FlavioCFOliveira/Gocene/util"

// distanceCalculator abstracts the per-coordinate-system distance
// engine consumed by circle2D. The two implementations are
// haversinCalculator (geographic) and cartesianCalculator
// (Euclidean), mirroring Java's DistanceCalculator interface and its
// HaversinDistance / CartesianDistance subclasses.
type distanceCalculator interface {
	// Contains reports whether the point lies within the disc.
	Contains(x, y float64) bool

	// IntersectsLine reports whether the segment (a, b) intersects
	// the disc.
	IntersectsLine(aX, aY, bX, bY float64) bool

	// Relate returns the relationship between the calculator's
	// bounding box and the supplied query bounding box.
	Relate(minX, maxX, minY, maxY float64) Relation

	// Disjoint reports whether the supplied bounding box is disjoint
	// from the calculator's bounding box.
	Disjoint(minX, maxX, minY, maxY float64) bool

	// Within reports whether the calculator's bounding box is fully
	// inside the supplied bounding box.
	Within(minX, maxX, minY, maxY float64) bool

	// MinX/MaxX/MinY/MaxY expose the calculator's bounding box.
	MinX() float64
	MaxX() float64
	MinY() float64
	MaxY() float64

	// X / Y return the centre coordinates.
	X() float64
	Y() float64
}

// circle2D is the Component2D for a circle. Mirrors Java's
// org.apache.lucene.geo.Circle2D.
type circle2D struct {
	calculator distanceCalculator
}

// newCircle2DFromCircle builds the Component2D for a geographic
// Circle. Mirrors Java's Circle2D.create(Circle).
func newCircle2DFromCircle(c Circle) *circle2D {
	return &circle2D{
		calculator: newHaversinCalculator(c.Lon(), c.Lat(), c.Radius()),
	}
}

// newCircle2DFromCalculator is the cross-cutting constructor used by
// XYCircle to wire the cartesian distance engine.
func newCircle2DFromCalculator(calc distanceCalculator) *circle2D {
	return &circle2D{calculator: calc}
}

func (c *circle2D) MinX() float64 { return c.calculator.MinX() }
func (c *circle2D) MaxX() float64 { return c.calculator.MaxX() }
func (c *circle2D) MinY() float64 { return c.calculator.MinY() }
func (c *circle2D) MaxY() float64 { return c.calculator.MaxY() }

func (c *circle2D) Contains(x, y float64) bool {
	return c.calculator.Contains(x, y)
}

func (c *circle2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return CellOutsideQuery
	}
	if c.calculator.Within(minX, maxX, minY, maxY) {
		return CellCrossesQuery
	}
	return c.calculator.Relate(minX, maxX, minY, maxY)
}

func (c *circle2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return false
	}
	return c.Contains(aX, aY) || c.Contains(bX, bY) ||
		c.calculator.IntersectsLine(aX, aY, bX, bY)
}

func (c *circle2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return false
	}
	if c.Contains(aX, aY) || c.Contains(bX, bY) || c.Contains(cX, cY) {
		return true
	}
	if PointInTriangle(minX, maxX, minY, maxY, c.calculator.X(), c.calculator.Y(),
		aX, aY, bX, bY, cX, cY) {
		return true
	}
	return c.calculator.IntersectsLine(aX, aY, bX, bY) ||
		c.calculator.IntersectsLine(bX, bY, cX, cY) ||
		c.calculator.IntersectsLine(cX, cY, aX, aY)
}

func (c *circle2D) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return false
	}
	return c.Contains(aX, aY) && c.Contains(bX, bY)
}

func (c *circle2D) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return false
	}
	return c.Contains(aX, aY) && c.Contains(bX, bY) && c.Contains(cX, cY)
}

func (c *circle2D) WithinPoint(x, y float64) WithinRelation {
	if c.Contains(x, y) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

func (c *circle2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if c.Contains(aX, aY) || c.Contains(bX, bY) {
		return WithinNotWithin
	}
	if ab && c.calculator.IntersectsLine(aX, aY, bX, bY) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

func (c *circle2D) WithinTriangle(
	minX, maxX, minY, maxY,
	aX, aY float64, ab bool,
	bX, bY float64, bc bool,
	cX, cY float64, ca bool,
) WithinRelation {
	if c.calculator.Disjoint(minX, maxX, minY, maxY) {
		return WithinDisjoint
	}

	// If any of the triangle's vertices is inside the circle, the
	// indexed shape cannot be within this triangle.
	if c.Contains(aX, aY) || c.Contains(bX, bY) || c.Contains(cX, cY) {
		return WithinNotWithin
	}

	// We only check edges that belong to the original polygon. If we
	// intersect any of them then we are not within.
	if ab && c.calculator.IntersectsLine(aX, aY, bX, bY) {
		return WithinNotWithin
	}
	if bc && c.calculator.IntersectsLine(bX, bY, cX, cY) {
		return WithinNotWithin
	}
	if ca && c.calculator.IntersectsLine(cX, cY, aX, aY) {
		return WithinNotWithin
	}

	// The remaining test: if the circle's centre is inside the
	// triangle, the triangle is a candidate.
	if PointInTriangle(minX, maxX, minY, maxY, c.calculator.X(), c.calculator.Y(),
		aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return WithinDisjoint
}

// circle2DIntersectsLine is the shared closest-point-on-segment test
// used by both calculators. Mirrors Circle2D.intersectsLine in Java
// (the private static helper). Returns true when the perpendicular
// projection of (centerX, centerY) onto segment (a, b) falls inside
// the segment and is itself inside the disc.
func circle2DIntersectsLine(centerX, centerY, aX, aY, bX, bY float64, calc distanceCalculator) bool {
	vectorAPX := centerX - aX
	vectorAPY := centerY - aY
	vectorABX := bX - aX
	vectorABY := bY - aY
	magnitudeAB := vectorABX*vectorABX + vectorABY*vectorABY
	dotProduct := vectorAPX*vectorABX + vectorAPY*vectorABY
	distance := dotProduct / magnitudeAB
	if distance < 0 || distance > 1 {
		return false
	}
	pX := aX + vectorABX*distance
	pY := aY + vectorABY*distance
	minX := minFloat(aX, bX)
	maxX := maxFloat(aX, bX)
	minY := minFloat(aY, bY)
	maxY := maxFloat(aY, bY)
	if pX >= minX && pX <= maxX && pY >= minY && pY <= maxY {
		return calc.Contains(pX, pY)
	}
	return false
}

// cartesianCalculator implements distanceCalculator for cartesian
// circles. Mirrors CartesianDistance in Java.
type cartesianCalculator struct {
	centerX       float64
	centerY       float64
	radiusSquared float64
	rectangle     XYRectangle
}

// newCartesianCalculator pre-computes the cartesian bounding box.
// The float32 -> float64 promotion matches the Java reference, where
// CartesianDistance stores float centre coordinates as doubles after
// the rectangle has been computed from the floats.
func newCartesianCalculator(x, y, radius float64) *cartesianCalculator {
	rect, err := FromXYPointDistance(float32(x), float32(y), float32(radius))
	if err != nil {
		// Validated upstream by XYCircle; surface invariants via
		// panic to match Java's unchecked-exception behaviour.
		panic(err)
	}
	return &cartesianCalculator{
		centerX:       x,
		centerY:       y,
		radiusSquared: radius * radius,
		rectangle:     rect,
	}
}

func (c *cartesianCalculator) Contains(x, y float64) bool {
	if BoxContainsPoint(x, y,
		float64(c.rectangle.MinX()), float64(c.rectangle.MaxX()),
		float64(c.rectangle.MinY()), float64(c.rectangle.MaxY())) {
		dx := x - c.centerX
		dy := y - c.centerY
		return dx*dx+dy*dy <= c.radiusSquared
	}
	return false
}

func (c *cartesianCalculator) IntersectsLine(aX, aY, bX, bY float64) bool {
	return circle2DIntersectsLine(c.centerX, c.centerY, aX, aY, bX, bY, c)
}

func (c *cartesianCalculator) Relate(minX, maxX, minY, maxY float64) Relation {
	if BoxContainsPoint(c.centerX, c.centerY, minX, maxX, minY, maxY) {
		if c.Contains(minX, minY) && c.Contains(maxX, minY) &&
			c.Contains(maxX, maxY) && c.Contains(minX, maxY) {
			return CellInsideQuery
		}
	} else {
		sumSq := 0.0
		if c.centerX < minX {
			d := minX - c.centerX
			sumSq += d * d
		} else if c.centerX > maxX {
			d := maxX - c.centerX
			sumSq += d * d
		}
		if c.centerY < minY {
			d := minY - c.centerY
			sumSq += d * d
		} else if c.centerY > maxY {
			d := maxY - c.centerY
			sumSq += d * d
		}
		if sumSq > c.radiusSquared {
			return CellOutsideQuery
		}
	}
	return CellCrossesQuery
}

func (c *cartesianCalculator) Disjoint(minX, maxX, minY, maxY float64) bool {
	return Disjoint(
		float64(c.rectangle.MinX()), float64(c.rectangle.MaxX()),
		float64(c.rectangle.MinY()), float64(c.rectangle.MaxY()),
		minX, maxX, minY, maxY)
}

func (c *cartesianCalculator) Within(minX, maxX, minY, maxY float64) bool {
	return WithinBBox(
		float64(c.rectangle.MinX()), float64(c.rectangle.MaxX()),
		float64(c.rectangle.MinY()), float64(c.rectangle.MaxY()),
		minX, maxX, minY, maxY)
}

func (c *cartesianCalculator) MinX() float64 { return float64(c.rectangle.MinX()) }
func (c *cartesianCalculator) MaxX() float64 { return float64(c.rectangle.MaxX()) }
func (c *cartesianCalculator) MinY() float64 { return float64(c.rectangle.MinY()) }
func (c *cartesianCalculator) MaxY() float64 { return float64(c.rectangle.MaxY()) }
func (c *cartesianCalculator) X() float64    { return c.centerX }
func (c *cartesianCalculator) Y() float64    { return c.centerY }

// haversinCalculator implements distanceCalculator for geographic
// circles. Mirrors HaversinDistance in Java.
type haversinCalculator struct {
	centerLat       float64
	centerLon       float64
	sortKey         float64
	axisLat         float64
	rectangle       Rectangle
	crossesDateline bool
}

// newHaversinCalculator pre-computes the geographic bounding box,
// the sort-key threshold and the auxiliary axis latitude. Argument
// order mirrors Java: HaversinDistance(centerLon, centerLat, radius).
func newHaversinCalculator(centerLon, centerLat, radius float64) *haversinCalculator {
	rect, err := FromPointDistance(centerLat, centerLon, radius)
	if err != nil {
		panic(err)
	}
	return &haversinCalculator{
		centerLat:       centerLat,
		centerLon:       centerLon,
		sortKey:         DistanceQuerySortKey(radius),
		axisLat:         AxisLat(centerLat, radius),
		rectangle:       rect,
		crossesDateline: rect.MinLon() > rect.MaxLon(),
	}
}

func (h *haversinCalculator) Relate(minX, maxX, minY, maxY float64) Relation {
	return Relate(minY, maxY, minX, maxX, h.centerLat, h.centerLon, h.sortKey, h.axisLat)
}

func (h *haversinCalculator) Contains(x, y float64) bool {
	if h.crossesDateline {
		if BoxContainsPoint(x, y, h.rectangle.MinLon(), MaxLonIncl, h.rectangle.MinLat(), h.rectangle.MaxLat()) ||
			BoxContainsPoint(x, y, MinLonIncl, h.rectangle.MaxLon(), h.rectangle.MinLat(), h.rectangle.MaxLat()) {
			return util.HaversinSortKey(y, x, h.centerLat, h.centerLon) <= h.sortKey
		}
		return false
	}
	if BoxContainsPoint(x, y, h.rectangle.MinLon(), h.rectangle.MaxLon(), h.rectangle.MinLat(), h.rectangle.MaxLat()) {
		return util.HaversinSortKey(y, x, h.centerLat, h.centerLon) <= h.sortKey
	}
	return false
}

func (h *haversinCalculator) IntersectsLine(aX, aY, bX, bY float64) bool {
	if circle2DIntersectsLine(h.centerLon, h.centerLat, aX, aY, bX, bY, h) {
		return true
	}
	if h.crossesDateline {
		newCenterLon := h.centerLon + 360
		if h.centerLon > 0 {
			newCenterLon = h.centerLon - 360
		}
		return circle2DIntersectsLine(newCenterLon, h.centerLat, aX, aY, bX, bY, h)
	}
	return false
}

func (h *haversinCalculator) Disjoint(minX, maxX, minY, maxY float64) bool {
	if h.crossesDateline {
		return Disjoint(h.rectangle.MinLon(), MaxLonIncl, h.rectangle.MinLat(), h.rectangle.MaxLat(),
			minX, maxX, minY, maxY) &&
			Disjoint(MinLonIncl, h.rectangle.MaxLon(), h.rectangle.MinLat(), h.rectangle.MaxLat(),
				minX, maxX, minY, maxY)
	}
	return Disjoint(h.rectangle.MinLon(), h.rectangle.MaxLon(),
		h.rectangle.MinLat(), h.rectangle.MaxLat(),
		minX, maxX, minY, maxY)
}

func (h *haversinCalculator) Within(minX, maxX, minY, maxY float64) bool {
	if h.crossesDateline {
		return WithinBBox(h.rectangle.MinLon(), MaxLonIncl, h.rectangle.MinLat(), h.rectangle.MaxLat(),
			minX, maxX, minY, maxY) ||
			WithinBBox(MinLonIncl, h.rectangle.MaxLon(), h.rectangle.MinLat(), h.rectangle.MaxLat(),
				minX, maxX, minY, maxY)
	}
	return WithinBBox(h.rectangle.MinLon(), h.rectangle.MaxLon(),
		h.rectangle.MinLat(), h.rectangle.MaxLat(),
		minX, maxX, minY, maxY)
}

func (h *haversinCalculator) MinX() float64 {
	if h.crossesDateline {
		// Component2D does not support boxes that cross the dateline.
		return MinLonIncl
	}
	return h.rectangle.MinLon()
}

func (h *haversinCalculator) MaxX() float64 {
	if h.crossesDateline {
		return MaxLonIncl
	}
	return h.rectangle.MaxLon()
}

func (h *haversinCalculator) MinY() float64 { return h.rectangle.MinLat() }
func (h *haversinCalculator) MaxY() float64 { return h.rectangle.MaxLat() }
func (h *haversinCalculator) X() float64    { return h.centerLon }
func (h *haversinCalculator) Y() float64    { return h.centerLat }
