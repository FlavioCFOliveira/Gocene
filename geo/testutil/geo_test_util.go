// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testutil hosts geo-side test helpers ported from
// Apache Lucene 10.4.0's lucene-test-framework. Sprint 116 T4692
// introduces [GeoTestUtil], which provides seed-reproducible random
// generation of lat/lon coordinates and geo shapes (Point, Rectangle,
// Polygon, Line, Circle) for spatial query tests.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/geo/GeoTestUtil.java
package testutil

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GeoTestUtil wraps a seeded [math/rand.Rand] to generate reproducible
// random geo shapes.  It is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.geo.GeoTestUtil.
//
// All public methods use the internal *rand.Rand for decisions, so two
// GeoTestUtil instances created with NewGeoTestUtil(seed) and driven
// through the same call sequence produce identical output -- useful for
// reproducing test failures.
type GeoTestUtil struct {
	rng *rand.Rand
}

// NewGeoTestUtil creates a GeoTestUtil from the given seed.
func NewGeoTestUtil(seed int64) *GeoTestUtil {
	return &GeoTestUtil{rng: rand.New(rand.NewSource(seed))}
}

// NewGeoTestUtilFromRand creates a GeoTestUtil that draws randomness
// from rng.  Useful when an existing *rand.Rand (e.g. from a test
// harness) should be shared.
func NewGeoTestUtilFromRand(rng *rand.Rand) *GeoTestUtil {
	return &GeoTestUtil{rng: rng}
}

// ---------------------------------------------------------------------------
// Public API -- coordinate generation
// ---------------------------------------------------------------------------

// NextLatitude returns a uniformly distributed random latitude in
// [MinLatIncl, MaxLatIncl], with extra density at edges, zero, and
// other "interesting" values.
func (g *GeoTestUtil) NextLatitude() float64 {
	return g.nextDoubleInternal(geo.MinLatIncl, geo.MaxLatIncl)
}

// NextLongitude returns a uniformly distributed random longitude in
// [MinLonIncl, MaxLonIncl], with extra density at edges, zero, and
// other "interesting" values.
func (g *GeoTestUtil) NextLongitude() float64 {
	return g.nextDoubleInternal(geo.MinLonIncl, geo.MaxLonIncl)
}

// NextPoint returns a random (lat, lon) pair.
func (g *GeoTestUtil) NextPoint() (lat, lon float64) {
	return g.NextLatitude(), g.NextLongitude()
}

// NextPointMust wraps NextPoint and panics if the coordinate is invalid.
// Returns a geo.Point.
func (g *GeoTestUtil) NextPointMust() geo.Point {
	lat, lon := g.NextPoint()
	return geo.MustNewPoint(lat, lon)
}

// NextPointNearRectangle returns a (lat, lon) pair near rect.
// It honours dateline-crossing rectangles by picking a side first.
func (g *GeoTestUtil) NextPointNearRectangle(rect geo.Rectangle) (lat, lon float64) {
	if rect.CrossesDateline() {
		if g.rng.Intn(2) == 0 {
			r2, _ := geo.NewRectangle(rect.MinLat(), rect.MaxLat(), geo.MinLonIncl, rect.MaxLon())
			return g.NextPointNearRectangle(r2)
		}
		r2, _ := geo.NewRectangle(rect.MinLat(), rect.MaxLat(), rect.MinLon(), geo.MaxLonIncl)
		return g.NextPointNearRectangle(r2)
	}
	return g.NextPointNearPolygon(boxPolygon(rect))
}

// NextPointNearPolygon returns a (lat, lon) pair near polygon using
// various strategies (target vertices, edges, bounding box, or a fully
// random point).
func (g *GeoTestUtil) NextPointNearPolygon(poly geo.Polygon) (lat, lon float64) {
	holes := poly.Holes()
	if len(holes) > 0 && g.rng.Intn(3) == 0 {
		return g.NextPointNearPolygon(holes[g.rng.Intn(len(holes))])
	}

	polyLats := poly.PolyLats()
	polyLons := poly.PolyLons()

	surpriseMe := g.rng.Intn(97)
	switch {
	case surpriseMe == 0:
		return g.NextPoint()
	case surpriseMe < 5:
		return g.nextLatitudeBetween(poly.MinLat(), poly.MaxLat()),
			g.nextLongitudeBetween(poly.MinLon(), poly.MaxLon())
	case surpriseMe < 20:
		vertex := g.rng.Intn(len(polyLats) - 1)
		return g.nextLatitudeNear(polyLats[vertex], polyLats[vertex+1]-polyLats[vertex]),
			g.nextLongitudeNear(polyLons[vertex], polyLons[vertex+1]-polyLons[vertex])
	case surpriseMe < 30:
		container := boxPolygon(newRectangleOrPanic(poly.MinLat(), poly.MaxLat(), poly.MinLon(), poly.MaxLon()))
		contLats := container.PolyLats()
		contLons := container.PolyLons()
		startVertex := g.rng.Intn(len(contLats) - 1)
		return g.nextPointAroundLine(
			contLats[startVertex], contLons[startVertex],
			contLats[startVertex+1], contLons[startVertex+1])
	default:
		startVertex := g.rng.Intn(len(polyLats) - 1)
		var endVertex int
		if g.rng.Intn(2) == 0 {
			endVertex = startVertex + 1
		} else {
			endVertex = g.rng.Intn(len(polyLats) - 1)
		}
		return g.nextPointAroundLine(
			polyLats[startVertex], polyLons[startVertex],
			polyLats[endVertex], polyLons[endVertex])
	}
}

// NextBoxNearPolygon returns a random [geo.Rectangle] near the given polygon.
func (g *GeoTestUtil) NextBoxNearPolygon(poly geo.Polygon) geo.Rectangle {
	holes := poly.Holes()
	if len(holes) > 0 && g.rng.Intn(3) == 0 {
		return g.NextBoxNearPolygon(holes[g.rng.Intn(len(holes))])
	}

	surpriseMe := g.rng.Intn(97)
	var point1, point2 [2]float64
	if surpriseMe == 0 {
		point1[0], point1[1] = g.NextPointNearPolygon(poly)
		point2[0], point2[1] = g.NextPointNearPolygon(poly)
	} else {
		point1[0], point1[1] = g.NextPointNearPolygon(poly)
		polyLats := poly.PolyLats()
		polyLons := poly.PolyLons()
		vertex := g.rng.Intn(len(polyLats) - 1)
		deltaX := polyLons[vertex+1] - polyLons[vertex]
		deltaY := polyLats[vertex+1] - polyLats[vertex]
		edgeLength := math.Sqrt(deltaX*deltaX + deltaY*deltaY)
		point2[0] = g.nextLatitudeNear(point1[0], edgeLength)
		point2[1] = g.nextLongitudeNear(point1[1], edgeLength)
	}

	minLat := math.Min(point1[0], point2[0])
	maxLat := math.Max(point1[0], point2[0])
	minLon := math.Min(point1[1], point2[1])
	maxLon := math.Max(point1[1], point2[1])
	return newRectangleOrPanic(minLat, maxLat, minLon, maxLon)
}

// ---------------------------------------------------------------------------
// Public API -- shape generation
// ---------------------------------------------------------------------------

// NextBox returns a random [geo.Rectangle] that may cross the dateline.
func (g *GeoTestUtil) NextBox() geo.Rectangle {
	return g.nextBoxInternal(true)
}

// NextBoxNotCrossingDateline returns a random [geo.Rectangle] that
// does NOT cross the 180th meridian.
func (g *GeoTestUtil) NextBoxNotCrossingDateline() geo.Rectangle {
	return g.nextBoxInternal(false)
}

// NextCircle returns a random [geo.Circle] with radius between 1 and
// (earth quadrant) meters.
func (g *GeoTestUtil) NextCircle() geo.Circle {
	lat := g.NextLatitude()
	lon := g.NextLongitude()
	radiusMeters := g.rng.Float64()*geo.EarthMeanRadiusMeters*math.Pi/2.0 + 1.0
	return geo.MustNewCircle(lat, lon, radiusMeters)
}

// NextLine returns a random [geo.Line] by computing a polygon and
// dropping the closing vertex.
func (g *GeoTestUtil) NextLine() geo.Line {
	poly := g.NextPolygon()
	lats := make([]float64, poly.NumPoints()-1)
	lons := make([]float64, len(lats))
	for i := 0; i < len(lats); i++ {
		lats[i] = poly.PolyLat(i)
		lons[i] = poly.PolyLon(i)
	}
	return geo.MustNewLine(lats, lons)
}

// NextPolygon returns a random [geo.Polygon].  Half the time it
// produces an irregular blob (surpriseMePolygon), 10 % a regular
// polygon, and 40 % a box or triangle.
func (g *GeoTestUtil) NextPolygon() geo.Polygon {
	if g.rng.Intn(2) == 0 {
		return g.surpriseMePolygon()
	}
	if g.rng.Intn(10) == 1 {
		for {
			gons := g.nextIntBetween(4, 500)
			radiusMeters := g.rng.Float64()*geo.EarthMeanRadiusMeters*math.Pi/2.0 + 1.0
			lat := g.NextLatitude()
			lon := g.NextLongitude()
			p, err := tryCreateRegularPolygon(lat, lon, radiusMeters, gons)
			if err == nil {
				return p
			}
		}
	}

	box := g.nextBoxInternal(false)
	if g.rng.Intn(2) == 0 {
		return boxPolygon(box)
	}
	return trianglePolygon(box)
}

// ---------------------------------------------------------------------------
// Public API -- utilities
// ---------------------------------------------------------------------------

// CreateRegularPolygon returns an n-gon centred at (centerLat, centerLon)
// whose vertices are approximately radiusMeters from the centre.
// It cannot cross the dateline or a pole.
func (g *GeoTestUtil) CreateRegularPolygon(centerLat, centerLon, radiusMeters float64, gons int) geo.Polygon {
	resultLats := make([]float64, gons+1)
	resultLons := make([]float64, gons+1)
	for i := 0; i < gons; i++ {
		angle := 360.0 - float64(i)*(360.0/float64(gons))
		x := math.Cos(angle * math.Pi / 180)
		y := math.Sin(angle * math.Pi / 180)

		factor := 2.0
		step := 1.0
		last := 0

		for {
			lat := centerLat + y*factor
			lon := centerLon + x*factor
			distanceMeters := util.HaversinMeters(centerLat, centerLon, lat, lon)
			if math.Abs(distanceMeters-radiusMeters) < 0.1 {
				resultLats[i] = lat
				resultLons[i] = lon
				break
			}
			if distanceMeters > radiusMeters {
				factor -= step
				if last == 1 {
					step /= 2.0
				}
				last = -1
			} else {
				factor += step
				if last == -1 {
					step /= 2.0
				}
				last = 1
			}
		}
	}
	resultLats[gons] = resultLats[0]
	resultLons[gons] = resultLons[0]
	return geo.MustNewPolygon(resultLats, resultLons)
}

// ContainsSlowly is a simple (but slow) ray-casting point-in-polygon
// test for a polygon without holes.  It is a direct port of the BSD-
// licensed PNPOLY algorithm used by Lucene's GeoTestUtil.
//
// It returns true when (lat, lon) is inside or on the boundary of p.
// Panics if p has holes.
func (g *GeoTestUtil) ContainsSlowly(p geo.Polygon, lat, lon float64) bool {
	if len(p.Holes()) > 0 {
		panic("containsSlowly does not support holes")
	}

	if lat < p.MinLat() || lat > p.MaxLat() || lon < p.MinLon() || lon > p.MaxLon() {
		return false
	}

	polyLats := p.PolyLats()
	polyLons := p.PolyLons()
	nvert := len(polyLats)

	c := false
	for i, j := 0, 1; j < nvert; i, j = i+1, j+1 {
		if ((lat == polyLats[j] && lat == polyLats[i]) ||
			((lat <= polyLats[j] && lat >= polyLats[i]) !=
				(lat >= polyLats[j] && lat <= polyLats[i]))) &&
			((lon == polyLons[j] && lon == polyLons[i]) ||
				(((lon <= polyLons[j] && lon >= polyLons[i]) !=
					(lon >= polyLons[j] && lon <= polyLons[i])) &&
					geo.Orient(polyLons[i], polyLats[i], polyLons[j], polyLats[j], lon, lat) == 0)) {
			return true
		}

		if ((polyLats[i] > lat) != (polyLats[j] > lat)) &&
			(lon < (polyLons[j]-polyLons[i])*(lat-polyLats[i])/(polyLats[j]-polyLats[i])+polyLons[i]) {
			c = !c
		}
	}

	return c
}

// ToSVG produces an SVG debug visualisation of geo objects (Polygon,
// Rectangle, or [2]float64 points).  It is modelled after Lucene's
// GeoTestUtil.toSVG.
func (g *GeoTestUtil) ToSVG(objects ...any) string {
	var b strings.Builder

	minLat := math.MaxFloat64
	maxLat := -math.MaxFloat64
	minLon := math.MaxFloat64
	maxLon := -math.MaxFloat64
	hasObjects := false

	flattened := g.flattenObjects(objects)
	for _, obj := range flattened {
		switch v := obj.(type) {
		case interface {
			PolyLats() []float64
			PolyLons() []float64
		}:
			hasObjects = true
			plats := v.PolyLats()
			plons := v.PolyLons()
			for i := range plats {
				if plats[i] < minLat {
					minLat = plats[i]
				}
				if plats[i] > maxLat {
					maxLat = plats[i]
				}
				if plons[i] < minLon {
					minLon = plons[i]
				}
				if plons[i] > maxLon {
					maxLon = plons[i]
				}
			}
		case rectish:
			hasObjects = true
			if v.MinLat() < minLat {
				minLat = v.MinLat()
			}
			if v.MaxLat() > maxLat {
				maxLat = v.MaxLat()
			}
			if v.MinLon() < minLon {
				minLon = v.MinLon()
			}
			if v.MaxLon() > maxLon {
				maxLon = v.MaxLon()
			}
		case [2]float64:
			hasObjects = true
			if v[0] < minLat {
				minLat = v[0]
			}
			if v[0] > maxLat {
				maxLat = v[0]
			}
			if v[1] < minLon {
				minLon = v[1]
			}
			if v[1] > maxLon {
				maxLon = v[1]
			}
		}
	}

	if !isFiniteFloat(minLat) || !isFiniteFloat(maxLat) || !hasObjects {
		return "<!-- ToSVG: no polygon or rectangle objects provided -->\n"
	}

	xpadding := (maxLon - minLon) / 64
	ypadding := (maxLat - minLat) / 64
	pointX := xpadding * 0.1
	pointY := ypadding * 0.1

	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" height="640" width="480" viewBox="`)
	b.WriteString(fmt.Sprintf("%v %v %v %v",
		minLon-xpadding, 90-maxLat-ypadding,
		maxLon-minLon+(2*xpadding), maxLat-minLat+(2*ypadding)))
	b.WriteString("\">\n")

	for _, obj := range flattened {
		switch v := obj.(type) {
		case interface {
			PolyLats() []float64
			PolyLons() []float64
		}:
			poly, err := geo.NewPolygon(v.PolyLats(), v.PolyLons())
			if err != nil {
				continue
			}
			polyLats := poly.PolyLats()
			polyLons := poly.PolyLons()
			style := "fill:lawngreen;stroke:black;stroke-width:0.3%;"
			opacity := "0.5"
			b.WriteString(fmt.Sprintf("<polygon fill-opacity=\"%s\" points=\"", opacity))
			for i := range polyLats {
				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(fmt.Sprintf("%v,%v", polyLons[i], 90-polyLats[i]))
			}
			b.WriteString(fmt.Sprintf("\" style=\"%s\"/>\n", style))
		case rectish:
			gon := boxPolygonFromRect(v)
			polyLats := gon.PolyLats()
			polyLons := gon.PolyLons()
			style := "fill:lightskyblue;stroke:black;stroke-width:0.2%;stroke-dasharray:0.5%,1%;"
			opacity := "0.3"
			b.WriteString(fmt.Sprintf("<polygon fill-opacity=\"%s\" points=\"", opacity))
			for i := range polyLats {
				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(fmt.Sprintf("%v,%v", polyLons[i], 90-polyLats[i]))
			}
			b.WriteString(fmt.Sprintf("\" style=\"%s\"/>\n", style))
		case [2]float64:
			pt := v
			gon := boxPolygonFromRect(newRectish(
				math.Max(-90, pt[0]-pointY),
				math.Min(90, pt[0]+pointY),
				math.Max(-180, pt[1]-pointX),
				math.Min(180, pt[1]+pointX),
			))
			polyLats := gon.PolyLats()
			polyLons := gon.PolyLons()
			style := "fill:red;stroke:red;stroke-width:0.1%;"
			opacity := "0.7"
			b.WriteString(fmt.Sprintf("<polygon fill-opacity=\"%s\" points=\"", opacity))
			for i := range polyLats {
				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(fmt.Sprintf("%v,%v", polyLons[i], 90-polyLats[i]))
			}
			b.WriteString(fmt.Sprintf("\" style=\"%s\"/>\n", style))
		}
	}

	b.WriteString("</svg>\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Private: core random double generation
// ---------------------------------------------------------------------------

// nextDoubleInternal is the heart of GeoTestUtil's random number
// generation.  It produces a double in [low, high] with extra density at
// interesting values (edges, zero, discrete 360-block samples) and may
// perturb the result by 1 ulp to catch edge-case bugs.
func (g *GeoTestUtil) nextDoubleInternal(low, high float64) float64 {
	if low == high {
		return low
	}

	var baseValue float64
	surpriseMe := g.rng.Intn(17)

	switch {
	case surpriseMe == 0:
		lowBits := util.DoubleToSortableLong(low)
		highBits := util.DoubleToSortableLong(high)
		baseValue = util.SortableLongToDouble(g.nextLongBetween(lowBits, highBits))
	case surpriseMe == 1:
		baseValue = low
	case surpriseMe == 2:
		baseValue = high
	case surpriseMe == 3 && low <= 0 && high >= 0:
		baseValue = 0.0
	case surpriseMe == 4:
		delta := (high - low) / 360
		block := g.rng.Intn(360)
		baseValue = low + delta*float64(block)
	default:
		baseValue = low + (high-low)*g.rng.Float64()
	}

	adjustMe := g.rng.Intn(17)
	switch {
	case adjustMe == 0:
		return math.Nextafter(baseValue, high)
	case adjustMe == 1:
		return math.Nextafter(baseValue, low)
	default:
		return baseValue
	}
}

// nextLongBetween returns a uniformly random int64 in [low, high].
func (g *GeoTestUtil) nextLongBetween(low, high int64) int64 {
	if low >= high {
		return low
	}
	range_ := uint64(high) - uint64(low)
	n := range_ + 1
	rem := ^uint64(0) % n
	threshold := ^uint64(0) - rem
	for {
		val := g.rng.Uint64()
		if val < threshold {
			return low + int64(val%n)
		}
	}
}

// ---------------------------------------------------------------------------
// Private: specialised lat/lon generation
// ---------------------------------------------------------------------------

// nextLatitudeNear returns a latitude near otherLatitude within delta.
func (g *GeoTestUtil) nextLatitudeNear(otherLatitude, delta float64) float64 {
	delta = math.Abs(delta)
	surpriseMe := g.rng.Intn(97)
	switch {
	case surpriseMe == 0:
		return g.NextLatitude()
	case surpriseMe < 49:
		return g.nextDoubleInternal(otherLatitude, math.Min(90, otherLatitude+delta))
	default:
		return g.nextDoubleInternal(math.Max(-90, otherLatitude-delta), otherLatitude)
	}
}

// nextLongitudeNear returns a longitude near otherLongitude within delta.
func (g *GeoTestUtil) nextLongitudeNear(otherLongitude, delta float64) float64 {
	delta = math.Abs(delta)
	surpriseMe := g.rng.Intn(97)
	switch {
	case surpriseMe == 0:
		return g.NextLongitude()
	case surpriseMe < 49:
		return g.nextDoubleInternal(otherLongitude, math.Min(180, otherLongitude+delta))
	default:
		return g.nextDoubleInternal(math.Max(-180, otherLongitude-delta), otherLongitude)
	}
}

// nextLatitudeBetween returns a latitude between minLatitude and
// maxLatitude, with occasional excursions outside (to test edge cases).
func (g *GeoTestUtil) nextLatitudeBetween(minLatitude, maxLatitude float64) float64 {
	if g.rng.Intn(47) == 0 {
		return g.NextLatitude()
	}
	difference := (maxLatitude - minLatitude) / 100
	lower := math.Max(-90, minLatitude-difference)
	upper := math.Min(90, maxLatitude+difference)
	return g.nextDoubleInternal(lower, upper)
}

// nextLongitudeBetween returns a longitude between minLongitude and
// maxLongitude, with occasional excursions outside.
func (g *GeoTestUtil) nextLongitudeBetween(minLongitude, maxLongitude float64) float64 {
	if g.rng.Intn(47) == 0 {
		return g.NextLongitude()
	}
	difference := (maxLongitude - minLongitude) / 100
	lower := math.Max(-180, minLongitude-difference)
	upper := math.Min(180, maxLongitude+difference)
	return g.nextDoubleInternal(lower, upper)
}

// nextPointAroundLine returns a (lat, lon) near the segment
// (lat1,lon1)--(lat2,lon2).
func (g *GeoTestUtil) nextPointAroundLine(lat1, lon1, lat2, lon2 float64) (float64, float64) {
	minX := math.Min(lon1, lon2)
	maxX := math.Max(lon1, lon2)
	minY := math.Min(lat1, lat2)
	maxY := math.Max(lat1, lat2)

	switch {
	case minX == maxX:
		return g.nextLatitudeBetween(minY, maxY),
			g.nextLongitudeNear(minX, 0.01*(maxY-minY))
	case minY == maxY:
		return g.nextLatitudeNear(minY, 0.01*(maxX-minX)),
			g.nextLongitudeBetween(minX, maxX)
	default:
		x := g.nextLongitudeBetween(minX, maxX)
		y := (lat1-lat2)/(lon1-lon2)*(x-lon1) + lat1
		if !isFiniteFloat(y) {
			y = math.Copysign(90, lon1)
		}
		delta := (maxY - minY) * 0.01
		y = math.Min(90, y)
		y = math.Max(-90, y)
		return g.nextLatitudeNear(y, delta), x
	}
}

// ---------------------------------------------------------------------------
// Private: shape helpers
// ---------------------------------------------------------------------------

// nextBoxInternal returns a random rectangle that prevents degenerate
// zero-width boxes.  When canCrossDateline is false, minLon <= maxLon.
func (g *GeoTestUtil) nextBoxInternal(canCrossDateline bool) geo.Rectangle {
	lat0 := g.NextLatitude()
	lat1 := g.NextLatitude()
	for lat0 == lat1 {
		lat1 = g.NextLatitude()
	}
	lon0 := g.NextLongitude()
	lon1 := g.NextLongitude()
	for lon0 == lon1 {
		lon1 = g.NextLongitude()
	}

	if lat1 < lat0 {
		lat0, lat1 = lat1, lat0
	}
	if !canCrossDateline && lon1 < lon0 {
		lon0, lon1 = lon1, lon0
	}

	return newRectangleOrPanic(lat0, lat1, lon0, lon1)
}

// boxPolygon converts a non-dateline-crossing rectangle to a 5-vertex
// closed polygon.
func boxPolygon(rect geo.Rectangle) geo.Polygon {
	polyLats := []float64{rect.MinLat(), rect.MaxLat(), rect.MaxLat(), rect.MinLat(), rect.MinLat()}
	polyLons := []float64{rect.MinLon(), rect.MinLon(), rect.MaxLon(), rect.MaxLon(), rect.MinLon()}
	return geo.MustNewPolygon(polyLats, polyLons)
}

// trianglePolygon converts a non-dateline-crossing rectangle to a
// 4-vertex closed triangle.
func trianglePolygon(rect geo.Rectangle) geo.Polygon {
	polyLats := []float64{rect.MinLat(), rect.MaxLat(), rect.MaxLat(), rect.MinLat()}
	polyLons := []float64{rect.MinLon(), rect.MinLon(), rect.MaxLon(), rect.MinLon()}
	return geo.MustNewPolygon(polyLats, polyLons)
}

// surpriseMePolygon generates a random irregular polygon blob.
func (g *GeoTestUtil) surpriseMePolygon() geo.Polygon {
newPoly:
	for {
		centerLat := g.NextLatitude()
		centerLon := g.NextLongitude()
		radius := 0.1 + 20*g.rng.Float64()
		radiusDelta := g.rng.Float64()

		var lats, lons []float64
		angle := 0.0
		for {
			angle += g.rng.Float64() * 40.0
			if angle > 360 {
				break
			}
			length := radius * (1.0 - radiusDelta + radiusDelta*g.rng.Float64())
			lat := centerLat + length*math.Cos(angle*math.Pi/180)
			lon := centerLon + length*math.Sin(angle*math.Pi/180)
			if lon <= geo.MinLonIncl || lon >= geo.MaxLonIncl || lat > 90 || lat < -90 {
				continue newPoly
			}
			lats = append(lats, lat)
			lons = append(lons, lon)
		}

		if len(lats) < 3 {
			continue newPoly
		}

		lats = append(lats, lats[0])
		lons = append(lons, lons[0])

		return geo.MustNewPolygon(lats, lons)
	}
}

// tryCreateRegularPolygon is like CreateRegularPolygon but returns an
// error when the polygon would cross the dateline or a pole.
func tryCreateRegularPolygon(centerLat, centerLon, radiusMeters float64, gons int) (geo.Polygon, error) {
	resultLats := make([]float64, gons+1)
	resultLons := make([]float64, gons+1)
	for i := 0; i < gons; i++ {
		angle := 360.0 - float64(i)*(360.0/float64(gons))
		x := math.Cos(angle * math.Pi / 180)
		y := math.Sin(angle * math.Pi / 180)

		factor := 2.0
		step := 1.0
		last := 0

		for {
			lat := centerLat + y*factor
			lon := centerLon + x*factor
			if err := geo.CheckLatitude(lat); err != nil {
				return geo.Polygon{}, err
			}
			if err := geo.CheckLongitude(lon); err != nil {
				return geo.Polygon{}, err
			}
			distanceMeters := util.HaversinMeters(centerLat, centerLon, lat, lon)
			if math.Abs(distanceMeters-radiusMeters) < 0.1 {
				resultLats[i] = lat
				resultLons[i] = lon
				break
			}
			if distanceMeters > radiusMeters {
				factor -= step
				if last == 1 {
					step /= 2.0
				}
				last = -1
			} else {
				factor += step
				if last == -1 {
					step /= 2.0
				}
				last = 1
			}
		}
	}
	resultLats[gons] = resultLats[0]
	resultLons[gons] = resultLons[0]
	return geo.NewPolygon(resultLats, resultLons)
}

// ---------------------------------------------------------------------------
// Private: ToSVG helpers
// ---------------------------------------------------------------------------

type rectish interface {
	MinLat() float64
	MaxLat() float64
	MinLon() float64
	MaxLon() float64
}

func (g *GeoTestUtil) flattenObjects(objects []any) []any {
	var out []any
	for _, o := range objects {
		switch v := o.(type) {
		case []geo.Polygon:
			for i := range v {
				out = append(out, v[i])
			}
		default:
			out = append(out, o)
		}
	}
	return out
}

type simpleRect struct {
	minLat, maxLat, minLon, maxLon float64
}

func (s simpleRect) MinLat() float64 { return s.minLat }
func (s simpleRect) MaxLat() float64 { return s.maxLat }
func (s simpleRect) MinLon() float64 { return s.minLon }
func (s simpleRect) MaxLon() float64 { return s.maxLon }

func newRectish(minLat, maxLat, minLon, maxLon float64) rectish {
	return simpleRect{minLat: minLat, maxLat: maxLat, minLon: minLon, maxLon: maxLon}
}

func boxPolygonFromRect(r rectish) geo.Polygon {
	polyLats := []float64{r.MinLat(), r.MaxLat(), r.MaxLat(), r.MinLat(), r.MinLat()}
	polyLons := []float64{r.MinLon(), r.MinLon(), r.MaxLon(), r.MaxLon(), r.MinLon()}
	return geo.MustNewPolygon(polyLats, polyLons)
}

// ---------------------------------------------------------------------------
// Misc helpers
// ---------------------------------------------------------------------------

func (g *GeoTestUtil) nextIntBetween(min, max int) int {
	return min + g.rng.Intn(max-min+1)
}

func newRectangleOrPanic(minLat, maxLat, minLon, maxLon float64) geo.Rectangle {
	return geo.MustNewRectangle(minLat, maxLat, minLon, maxLon)
}

func isFiniteFloat(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}
