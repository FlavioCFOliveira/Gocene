// Code in this file mirrors org.apache.lucene.geo.Circle2D from
// Apache Lucene 10.4.0. The Java type is package-private; the Go
// port keeps it unexported (circle2D / newCircle2DFromCircle).
//
// The Java reference exposes Circle2D via a DistanceCalculator
// strategy (HaversinDistance for geographic Circle, CartesianDistance
// for XYCircle). The Go port encodes the choice with the
// distanceCalculator interface so XYCircle (task #295) can reuse the
// same Component2D type by supplying its own calculator.
//
// Only the Component2D methods required by the current contract are
// implemented; the full Component2D surface arrives with task #277.

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
}

// haversinCalculator implements distanceCalculator for geographic
// circles using the same haversine distance the Lucene
// HaversinDistance strategy uses (via util.HaversinMeters, which
// matches SloppyMath bit-for-bit).
type haversinCalculator struct {
	lat     float64 // centre latitude in degrees
	lon     float64 // centre longitude in degrees
	radius  float64 // radius in metres
	bbox    Rectangle
	bboxErr error // captured at construction time; tested by callers
}

// newHaversinCalculator pre-computes the geographic bounding box via
// FromPointDistance. The bbox computation can fail only on invalid
// centre coordinates, which Circle has already validated.
func newHaversinCalculator(lat, lon, radius float64) *haversinCalculator {
	bbox, err := FromPointDistance(lat, lon, radius)
	return &haversinCalculator{lat: lat, lon: lon, radius: radius, bbox: bbox, bboxErr: err}
}

// Contains reports whether (x, y) = (lon, lat) lies inside the
// circle. Uses haversine distance.
func (h *haversinCalculator) Contains(x, y float64) bool {
	d := util.HaversinMeters(y, x, h.lat, h.lon)
	return d <= h.radius
}

// BoundingBox returns the (minX, maxX, minY, maxY) = (minLon, maxLon,
// minLat, maxLat) of the circle's geographic bounding box.
func (h *haversinCalculator) BoundingBox() (float64, float64, float64, float64) {
	return h.bbox.MinLon(), h.bbox.MaxLon(), h.bbox.MinLat(), h.bbox.MaxLat()
}

// cartesianCalculator implements distanceCalculator for cartesian
// circles, used by XYCircle (task #295). Kept here so the entire
// circle2D family lives in one file.
type cartesianCalculator struct {
	x      float64
	y      float64
	radius float64
}

// newCartesianCalculator builds the Euclidean strategy for a
// cartesian circle.
func newCartesianCalculator(x, y, radius float64) *cartesianCalculator {
	return &cartesianCalculator{x: x, y: y, radius: radius}
}

// Contains reports whether (x, y) lies within the cartesian circle.
func (c *cartesianCalculator) Contains(x, y float64) bool {
	dx := x - c.x
	dy := y - c.y
	return dx*dx+dy*dy <= c.radius*c.radius
}

// BoundingBox returns the axis-aligned bbox of the cartesian circle.
func (c *cartesianCalculator) BoundingBox() (float64, float64, float64, float64) {
	return c.x - c.radius, c.x + c.radius, c.y - c.radius, c.y + c.radius
}

// circle2D is the Component2D for a circle (geographic or cartesian)
// parameterised by its distance calculator.
type circle2D struct {
	calculator distanceCalculator
	minX       float64
	maxX       float64
	minY       float64
	maxY       float64
}

// newCircle2DFromCircle builds the Component2D for a geographic
// Circle using the haversin calculator.
func newCircle2DFromCircle(c Circle) *circle2D {
	calc := newHaversinCalculator(c.Lat(), c.Lon(), c.Radius())
	minX, maxX, minY, maxY := calc.BoundingBox()
	return &circle2D{calculator: calc, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// newCircle2DFromCalculator is the cross-cutting constructor used by
// XYCircle (task #295) once that type is ported.
func newCircle2DFromCalculator(calc distanceCalculator) *circle2D {
	minX, maxX, minY, maxY := calc.BoundingBox()
	return &circle2D{calculator: calc, minX: minX, maxX: maxX, minY: minY, maxY: maxY}
}

// MinX returns the inclusive minimum X coordinate of the bounding
// box.
func (c *circle2D) MinX() float64 { return c.minX }

// MaxX returns the inclusive maximum X coordinate of the bounding
// box.
func (c *circle2D) MaxX() float64 { return c.maxX }

// MinY returns the inclusive minimum Y coordinate of the bounding
// box.
func (c *circle2D) MinY() float64 { return c.minY }

// MaxY returns the inclusive maximum Y coordinate of the bounding
// box.
func (c *circle2D) MaxY() float64 { return c.maxY }

// Contains delegates to the distance calculator. Note that (x, y) is
// (longitude, latitude) for the haversin variant — matching Lucene's
// convention where Component2D consistently uses (x, y) = (lon, lat)
// for geographic shapes.
func (c *circle2D) Contains(x, y float64) bool {
	return c.calculator.Contains(x, y)
}

// Relate returns the spatial relation between the circle and the
// query bounding box. The implementation is the standard
// circle-vs-rectangle test:
//
//   - Box outside circle bbox -> OUTSIDE (cheap reject).
//   - All four corners of the box inside the circle and the box
//     bbox fully covered by the circle bbox -> CellInsideQuery.
//   - The circle centre is inside the box, or any corner is inside
//     the circle -> CellCrossesQuery.
//   - Otherwise compute the closest point on the box to the circle
//     centre and test whether the distance is within radius.
//
// This mirrors Lucene's Circle2D.relate for the Component2D contract
// methods currently exposed; the precise Bezier-edge test used in
// intersectsLine / intersectsTriangle is owned by task #277.
func (c *circle2D) Relate(minX, maxX, minY, maxY float64) Relation {
	if maxX < c.minX || minX > c.maxX || maxY < c.minY || minY > c.maxY {
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
	// Centre of the circle in (x, y) space — we recover it from the
	// bounding box rather than carrying it through the interface to
	// keep distanceCalculator slim.
	cx := (c.minX + c.maxX) * 0.5
	cy := (c.minY + c.maxY) * 0.5
	closestX := math.Max(minX, math.Min(cx, maxX))
	closestY := math.Max(minY, math.Min(cy, maxY))
	if c.calculator.Contains(closestX, closestY) {
		return CellCrossesQuery
	}
	return CellOutsideQuery
}
