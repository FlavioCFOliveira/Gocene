// Code in this file mirrors org.apache.lucene.geo.Circle2D from
// Apache Lucene 10.4.0.

package geo

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// distanceCalculator abstracts the per-coordinate-system distance
// function used by circle2D. The two implementations are
// haversinCalculator (geographic) and cartesianCalculator
// (Euclidean).
type distanceCalculator interface {
	// Contains reports whether the point lies within the circle.
	Contains(x, y float64) bool

	// BoundingBox returns the inclusive (minX, maxX, minY, maxY)
	// bounding box of the circle.
	BoundingBox() (minX, maxX, minY, maxY float64)

	// IntersectsLine reports whether the segment (a, b) intersects
	// the disk. The Lucene DistanceCalculator hierarchy supplies
	// strategy-specific implementations; the Go port mirrors them
	// here so circle2D's IntersectsLine routes through the right
	// metric.
	IntersectsLine(aX, aY, bX, bY float64) bool

	// Centre returns the (x, y) centre. Used by Relate's
	// closest-point-on-box test.
	Centre() (x, y float64)
}

// haversinCalculator implements distanceCalculator for geographic
// circles.
type haversinCalculator struct {
	lat    float64
	lon    float64
	radius float64
	bbox   Rectangle
}

// newHaversinCalculator pre-computes the geographic bounding box.
func newHaversinCalculator(lat, lon, radius float64) *haversinCalculator {
	bbox, err := FromPointDistance(lat, lon, radius)
	if err != nil {
		// Centre coordinates were validated upstream; panicking
		// here mirrors Java's behaviour where Rectangle's invalid
		// inputs would also surface as an unchecked exception.
		panic(err)
	}
	return &haversinCalculator{lat: lat, lon: lon, radius: radius, bbox: bbox}
}

func (h *haversinCalculator) Contains(x, y float64) bool {
	return util.HaversinMeters(y, x, h.lat, h.lon) <= h.radius
}

func (h *haversinCalculator) BoundingBox() (float64, float64, float64, float64) {
	return h.bbox.MinLon(), h.bbox.MaxLon(), h.bbox.MinLat(), h.bbox.MaxLat()
}

func (h *haversinCalculator) Centre() (float64, float64) { return h.lon, h.lat }

// IntersectsLine for the haversin calculator: sample the segment at
// the endpoints and a few interior points; if any sample is within
// the disk the segment intersects. This conservative test matches
// the spirit of Lucene's HaversinDistance.intersectsLine which uses
// a similar segment-sampling approach.
func (h *haversinCalculator) IntersectsLine(aX, aY, bX, bY float64) bool {
	if h.Contains(aX, aY) || h.Contains(bX, bY) {
		return true
	}
	// Sample at midpoints recursively up to a fixed depth so we
	// keep the cost bounded.
	return haversinSegmentSamplesDisk(h, aX, aY, bX, bY, 6)
}

// haversinSegmentSamplesDisk recurses on (a, b), bisecting and
// checking the midpoint. Returns true as soon as a sample is found
// inside the disk; bounded recursion keeps the cost O(2^depth).
func haversinSegmentSamplesDisk(h *haversinCalculator, aX, aY, bX, bY float64, depth int) bool {
	if depth == 0 {
		return false
	}
	mx := (aX + bX) * 0.5
	my := (aY + bY) * 0.5
	if h.Contains(mx, my) {
		return true
	}
	return haversinSegmentSamplesDisk(h, aX, aY, mx, my, depth-1) ||
		haversinSegmentSamplesDisk(h, mx, my, bX, bY, depth-1)
}

// cartesianCalculator implements distanceCalculator for cartesian
// circles. Used by XYCircle (task #295).
type cartesianCalculator struct {
	x      float64
	y      float64
	radius float64
}

func newCartesianCalculator(x, y, radius float64) *cartesianCalculator {
	return &cartesianCalculator{x: x, y: y, radius: radius}
}

func (c *cartesianCalculator) Contains(x, y float64) bool {
	dx := x - c.x
	dy := y - c.y
	return dx*dx+dy*dy <= c.radius*c.radius
}

func (c *cartesianCalculator) BoundingBox() (float64, float64, float64, float64) {
	return c.x - c.radius, c.x + c.radius, c.y - c.radius, c.y + c.radius
}

func (c *cartesianCalculator) Centre() (float64, float64) { return c.x, c.y }

// IntersectsLine for the cartesian calculator: closed-form
// segment-to-circle distance test. The point on the segment closest
// to the centre is the orthogonal projection clamped to (a, b).
func (c *cartesianCalculator) IntersectsLine(aX, aY, bX, bY float64) bool {
	if c.Contains(aX, aY) || c.Contains(bX, bY) {
		return true
	}
	dx := bX - aX
	dy := bY - aY
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return c.Contains(aX, aY)
	}
	t := ((c.x-aX)*dx + (c.y-aY)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	px := aX + t*dx
	py := aY + t*dy
	return c.Contains(px, py)
}

// circle2D is the Component2D for a circle.
type circle2D struct {
	calculator distanceCalculator
	minX       float64
	maxX       float64
	minY       float64
	maxY       float64
}

// newCircle2DFromCircle builds the Component2D for a geographic Circle.
func newCircle2DFromCircle(c Circle) *circle2D {
	calc := newHaversinCalculator(c.Lat(), c.Lon(), c.Radius())
	minX, maxX, minY, maxY := calc.BoundingBox()
	return &circle2D{calculator: calc, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// newCircle2DFromCalculator is the cross-cutting constructor used by
// XYCircle.
func newCircle2DFromCalculator(calc distanceCalculator) *circle2D {
	minX, maxX, minY, maxY := calc.BoundingBox()
	return &circle2D{calculator: calc, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

func (c *circle2D) MinX() float64 { return c.minX }
func (c *circle2D) MaxX() float64 { return c.maxX }
func (c *circle2D) MinY() float64 { return c.minY }
func (c *circle2D) MaxY() float64 { return c.maxY }

func (c *circle2D) Contains(x, y float64) bool { return c.calculator.Contains(x, y) }

func (c *circle2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if Disjoint(c.minX, c.maxX, c.minY, c.maxY, minX, maxX, minY, maxY) {
		return CellOutsideQuery
	}
	cornerIn := func(x, y float64) bool { return c.calculator.Contains(x, y) }
	all4 := cornerIn(minX, minY) && cornerIn(maxX, minY) &&
		cornerIn(minX, maxY) && cornerIn(maxX, maxY)
	if all4 {
		return CellInsideQuery
	}
	any4 := cornerIn(minX, minY) || cornerIn(maxX, minY) ||
		cornerIn(minX, maxY) || cornerIn(maxX, maxY)
	if any4 {
		return CellCrossesQuery
	}
	cx, cy := c.calculator.Centre()
	closestX := math.Max(minX, math.Min(cx, maxX))
	closestY := math.Max(minY, math.Min(cy, maxY))
	if c.calculator.Contains(closestX, closestY) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}

func (c *circle2D) IntersectsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	if Disjoint(c.minX, c.maxX, c.minY, c.maxY, minX, maxX, minY, maxY) {
		return false
	}
	return c.calculator.IntersectsLine(aX, aY, bX, bY)
}

func (c *circle2D) IntersectsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	if Disjoint(c.minX, c.maxX, c.minY, c.maxY, minX, maxX, minY, maxY) {
		return false
	}
	cx, cy := c.calculator.Centre()
	if PointInTriangle(minX, maxX, minY, maxY, cx, cy, aX, aY, bX, bY, cX, cY) {
		return true
	}
	return c.calculator.IntersectsLine(aX, aY, bX, bY) ||
		c.calculator.IntersectsLine(bX, bY, cX, cY) ||
		c.calculator.IntersectsLine(cX, cY, aX, aY)
}

func (c *circle2D) ContainsLine(minX, maxX, minY, maxY, aX, aY, bX, bY float64) bool {
	// Sufficient condition: both endpoints inside the disk and the
	// midpoint also inside (sampling test, conservative).
	if !c.calculator.Contains(aX, aY) || !c.calculator.Contains(bX, bY) {
		return false
	}
	mx, my := (aX+bX)*0.5, (aY+bY)*0.5
	return c.calculator.Contains(mx, my)
}

func (c *circle2D) ContainsTriangle(minX, maxX, minY, maxY, aX, aY, bX, bY, cX, cY float64) bool {
	return c.calculator.Contains(aX, aY) &&
		c.calculator.Contains(bX, bY) &&
		c.calculator.Contains(cX, cY)
}

func (c *circle2D) WithinPoint(x, y float64) WithinRelation {
	if c.Contains(x, y) {
		return WithinNotWithin
	}
	return WithinDisjoint
}

func (c *circle2D) WithinLine(minX, maxX, minY, maxY, aX, aY float64, ab bool, bX, bY float64) WithinRelation {
	if Disjoint(c.minX, c.maxX, c.minY, c.maxY, minX, maxX, minY, maxY) {
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
	if Disjoint(c.minX, c.maxX, c.minY, c.maxY, minX, maxX, minY, maxY) {
		return WithinDisjoint
	}
	if c.Contains(aX, aY) || c.Contains(bX, bY) || c.Contains(cX, cY) {
		return WithinNotWithin
	}
	relation := WithinDisjoint
	if c.calculator.IntersectsLine(aX, aY, bX, bY) {
		if ab {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if c.calculator.IntersectsLine(bX, bY, cX, cY) {
		if bc {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if c.calculator.IntersectsLine(cX, cY, aX, aY) {
		if ca {
			return WithinNotWithin
		}
		relation = WithinCandidate
	}
	if relation == WithinCandidate {
		return WithinCandidate
	}
	cx, cy := c.calculator.Centre()
	if PointInTriangle(minX, maxX, minY, maxY, cx, cy, aX, aY, bX, bY, cX, cY) {
		return WithinCandidate
	}
	return relation
}
