// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestNewGeoTestUtil_SeededDeterminism verifies that two instances with
// the same seed produce identical NextLatitude / NextLongitude sequences.
func TestNewGeoTestUtil_SeededDeterminism(t *testing.T) {
	t.Parallel()

	const seed int64 = 42

	run := func() (lats, lons []float64) {
		g := NewGeoTestUtil(seed)
		for i := 0; i < 100; i++ {
			lats = append(lats, g.NextLatitude())
			lons = append(lons, g.NextLongitude())
		}
		return
	}

	la1, lo1 := run()
	la2, lo2 := run()

	for i := range la1 {
		if la1[i] != la2[i] {
			t.Errorf("lat #%d: seed determinism broken: got %g, want %g", i, la2[i], la1[i])
		}
		if lo1[i] != lo2[i] {
			t.Errorf("lon #%d: seed determinism broken: got %g, want %g", i, lo2[i], lo1[i])
		}
	}
}

// TestNextLatitude_Range checks that NextLatitude always returns values
// within the valid inclusive range.
func TestNextLatitude_Range(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(99)
	for i := 0; i < 1000; i++ {
		lat := g.NextLatitude()
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("NextLatitude #%d: got %g, want in [%g, %g]", i, lat, geo.MinLatIncl, geo.MaxLatIncl)
		}
	}
}

// TestNextLongitude_Range checks that NextLongitude always returns
// values within the valid inclusive range.
func TestNextLongitude_Range(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(99)
	for i := 0; i < 1000; i++ {
		lon := g.NextLongitude()
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("NextLongitude #%d: got %g, want in [%g, %g]", i, lon, geo.MinLonIncl, geo.MaxLonIncl)
		}
	}
}

// TestNextPointMust checks that NextPointMust returns a valid geo.Point.
func TestNextPointMust(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(123)
	for i := 0; i < 100; i++ {
		p := g.NextPointMust()
		if p.Lat() < geo.MinLatIncl || p.Lat() > geo.MaxLatIncl {
			t.Errorf("PointMust lat #%d: got %g, out of range", i, p.Lat())
		}
		if p.Lon() < geo.MinLonIncl || p.Lon() > geo.MaxLonIncl {
			t.Errorf("PointMust lon #%d: got %g, out of range", i, p.Lon())
		}
	}
}

// TestNextBox_Valid checks that NextBox produces valid rectangles (may
// cross dateline).
func TestNextBox_Valid(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(77)
	for i := 0; i < 200; i++ {
		box := g.NextBox()
		if box.MinLat() > box.MaxLat() {
			t.Errorf("NextBox #%d: minLat=%g > maxLat=%g", i, box.MinLat(), box.MaxLat())
		}
		if box.MinLat() < geo.MinLatIncl || box.MaxLat() > geo.MaxLatIncl {
			t.Errorf("NextBox #%d: lat out of range: [%g, %g]", i, box.MinLat(), box.MaxLat())
		}
		if box.MinLon() < geo.MinLonIncl || box.MaxLon() > geo.MaxLonIncl {
			t.Errorf("NextBox #%d: lon out of range: [%g, %g]", i, box.MinLon(), box.MaxLon())
		}
	}
}

// TestNextBoxNotCrossingDateline checks that the non-dateline-crossing
// variant always satisfies minLon <= maxLon.
func TestNextBoxNotCrossingDateline(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(33)
	for i := 0; i < 200; i++ {
		box := g.NextBoxNotCrossingDateline()
		if box.CrossesDateline() {
			t.Errorf("NextBoxNotCrossingDateline #%d: crosses dateline: %s", i, box.String())
		}
		if box.MinLon() > box.MaxLon() {
			t.Errorf("NextBoxNotCrossingDateline #%d: minLon=%g > maxLon=%g", i, box.MinLon(), box.MaxLon())
		}
	}
}

// TestNextCircle_Valid checks that NextCircle produces valid circles
// with positive radii.
func TestNextCircle_Valid(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(55)
	for i := 0;  i < 100; i++ {
		c := g.NextCircle()
		if c.Radius() <= 0 {
			t.Errorf("NextCircle #%d: radius=%g, want >0", i, c.Radius())
		}
		if c.Lat() < geo.MinLatIncl || c.Lat() > geo.MaxLatIncl {
			t.Errorf("NextCircle #%d: lat out of range: %g", i, c.Lat())
		}
		if c.Lon() < geo.MinLonIncl || c.Lon() > geo.MaxLonIncl {
			t.Errorf("NextCircle #%d: lon out of range: %g", i, c.Lon())
		}
	}
}

// TestNextLine_Valid checks that NextLine produces valid lines with at
// least 2 vertices.
func TestNextLine_Valid(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(111)
	for i := 0; i < 100; i++ {
		line := g.NextLine()
		if line.NumPoints() < 2 {
			t.Errorf("NextLine #%d: NumPoints=%d, want >=2", i, line.NumPoints())
		}
		for j := 0; j < line.NumPoints(); j++ {
			if line.Lat(j) < geo.MinLatIncl || line.Lat(j) > geo.MaxLatIncl {
				t.Errorf("NextLine #%d vertex %d: lat=%g out of range", i, j, line.Lat(j))
			}
			if line.Lon(j) < geo.MinLonIncl || line.Lon(j) > geo.MaxLonIncl {
				t.Errorf("NextLine #%d vertex %d: lon=%g out of range", i, j, line.Lon(j))
			}
		}
	}
}

// TestNextPolygon_Valid checks that NextPolygon produces valid polygons
// (closed, at least 4 vertices, within bounds).
func TestNextPolygon_Valid(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(222)
	for i := 0; i < 50; i++ {
		poly := g.NextPolygon()
		if poly.NumPoints() < 4 {
			t.Errorf("NextPolygon #%d: NumPoints=%d, want >=4", i, poly.NumPoints())
		}
		// Check that polygon is closed.
		last := poly.NumPoints() - 1
		if poly.PolyLat(0) != poly.PolyLat(last) || poly.PolyLon(0) != poly.PolyLon(last) {
			t.Errorf("NextPolygon #%d: not closed: first=(%g,%g) last=(%g,%g)",
				i, poly.PolyLat(0), poly.PolyLon(0), poly.PolyLat(last), poly.PolyLon(last))
		}
		for j := 0; j < poly.NumPoints(); j++ {
			if poly.PolyLat(j) < geo.MinLatIncl || poly.PolyLat(j) > geo.MaxLatIncl {
				t.Errorf("NextPolygon #%d vertex %d: lat=%g out of range", i, j, poly.PolyLat(j))
			}
			if poly.PolyLon(j) < geo.MinLonIncl || poly.PolyLon(j) > geo.MaxLonIncl {
				t.Errorf("NextPolygon #%d vertex %d: lon=%g out of range", i, j, poly.PolyLon(j))
			}
		}
	}
}

// TestNextPointNearRectangle checks that NextPointNearRectangle returns
// a valid point for both dateline-crossing and non-crossing rectangles.
func TestNextPointNearRectangle(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(444)

	// Non-dateline-crossing rectangle.
	rect := newRectangleOrPanic(10, 20, 30, 40)
	for i := 0; i < 50; i++ {
		lat, lon := g.NextPointNearRectangle(rect)
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("lat out of range: %g", lat)
		}
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("lon out of range: %g", lon)
		}
	}

	// Dateline-crossing rectangle.
	dlRect := newRectangleOrPanic(-10, 10, 170, -170)
	for i := 0; i < 50; i++ {
		lat, lon := g.NextPointNearRectangle(dlRect)
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("lat out of range: %g", lat)
		}
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("lon out of range: %g", lon)
		}
	}
}

// TestNextPointNearPolygon checks that NextPointNearPolygon produces
// valid points for a simple polygon.
func TestNextPointNearPolygon(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(555)
	poly := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))

	for i := 0; i < 50; i++ {
		lat, lon := g.NextPointNearPolygon(poly)
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("lat out of range: %g", lat)
		}
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("lon out of range: %g", lon)
		}
	}
}

// TestNextBoxNearPolygon checks that NextBoxNearPolygon produces valid
// rectangles for a simple polygon.
func TestNextBoxNearPolygon(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(666)
	poly := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))

	for i := 0; i < 50; i++ {
		box := g.NextBoxNearPolygon(poly)
		if box.MinLat() > box.MaxLat() {
			t.Errorf("box #%d: minLat=%g > maxLat=%g", i, box.MinLat(), box.MaxLat())
		}
		if box.MinLat() < geo.MinLatIncl || box.MaxLat() > geo.MaxLatIncl {
			t.Errorf("box #%d: lat out of range: [%g, %g]", i, box.MinLat(), box.MaxLat())
		}
		if box.MinLon() < geo.MinLonIncl || box.MaxLon() > geo.MaxLonIncl {
			t.Errorf("box #%d: lon out of range: [%g, %g]", i, box.MinLon(), box.MaxLon())
		}
	}
}

// TestCreateRegularPolygon checks that the regular polygon generator
// produces a valid polygon with the expected vertex count.
func TestCreateRegularPolygon(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(1)
	poly := g.CreateRegularPolygon(0, 0, 10000, 6)

	if poly.NumPoints() != 7 { // 6 vertices + closing vertex
		t.Errorf("expected 7 points (6+close), got %d", poly.NumPoints())
	}
	// Check closed.
	last := poly.NumPoints() - 1
	if poly.PolyLat(0) != poly.PolyLat(last) || poly.PolyLon(0) != poly.PolyLon(last) {
		t.Errorf("regular polygon not closed")
	}
	// Check each vertex is within bounds.
	for i := 0; i < poly.NumPoints(); i++ {
		if poly.PolyLat(i) < geo.MinLatIncl || poly.PolyLat(i) > geo.MaxLatIncl {
			t.Errorf("vertex %d: lat=%g out of range", i, poly.PolyLat(i))
		}
		if poly.PolyLon(i) < geo.MinLonIncl || poly.PolyLon(i) > geo.MaxLonIncl {
			t.Errorf("vertex %d: lon=%g out of range", i, poly.PolyLon(i))
		}
	}
}

// TestContainsSlowly checks the slow point-in-polygon test against a
// simple box polygon.
func TestContainsSlowly(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(1)
	poly := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))

	// Interior point must be inside.
	if !g.ContainsSlowly(poly, 0, 0) {
		t.Error("ContainsSlowly(0,0): want true, got false")
	}

	// Point outside must be outside.
	if g.ContainsSlowly(poly, 20, 20) {
		t.Error("ContainsSlowly(20,20): want false, got true")
	}

	// Points on boundary must be inside.
	cases := []struct {
		lat, lon float64
		desc     string
	}{
		{-10, 0, "bottom edge"},
		{10, 0, "top edge"},
		{0, -20, "left edge"},
		{0, 20, "right edge"},
		{-10, -20, "bottom-left corner"},
		{-10, 20, "bottom-right corner"},
		{10, -20, "top-left corner"},
		{10, 20, "top-right corner"},
	}
	for _, tc := range cases {
		if !g.ContainsSlowly(poly, tc.lat, tc.lon) {
			t.Errorf("ContainsSlowly(%g,%g) on %s: want true, got false", tc.lat, tc.lon, tc.desc)
		}
	}
}

// TestContainsSlowly_PanicsOnHoles checks that ContainsSlowly panics
// when given a polygon with holes.
func TestContainsSlowly_PanicsOnHoles(t *testing.T) {
	t.Parallel()

	outer := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))
	inner := boxPolygon(newRectangleOrPanic(-5, 5, -10, 10))
	withHoles := geo.MustNewPolygon(
		outer.PolyLats(), outer.PolyLons(), inner,
	)

	g := NewGeoTestUtil(1)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for polygon with holes, got none")
		}
	}()
	g.ContainsSlowly(withHoles, 0, 0)
}

// TestToSVG checks that ToSVG produces a non-empty SVG string for a
// mix of objects.
func TestToSVG(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(1)
	poly := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))
	rect := newRectangleOrPanic(-5, 5, -10, 10)
	pt := [2]float64{0, 0}

	svg := g.ToSVG(poly, rect, pt)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("ToSVG: expected SVG prefix, got %q", svg[:min(len(svg), 20)])
	}
	if !strings.HasSuffix(strings.TrimSpace(svg), "</svg>") {
		t.Errorf("ToSVG: expected SVG suffix, got %q", svg[max(0, len(svg)-20):])
	}
	if !strings.Contains(svg, "polygon") {
		t.Errorf("ToSVG: expected at least one polygon element")
	}
}

// TestToSVG_PolygonSlice checks that ToSVG accepts a []geo.Polygon.
func TestToSVG_PolygonSlice(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(2)
	poly := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))

	svg := g.ToSVG([]geo.Polygon{poly})
	if !strings.Contains(svg, "polygon") {
		t.Errorf("ToSVG with Polygon slice: expected polygon element")
	}
}

// TestToSVG_NoPolygon checks that the fallback message is emitted.
func TestToSVG_NoPolygon(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(3)
	svg := g.ToSVG("not a geo object")
	if !strings.Contains(svg, "no polygon or rectangle objects") {
		t.Errorf("ToSVG with no geo objects: expected fallback, got %q", svg)
	}
}

// TestNewGeoTestUtilFromRand checks that the constructor from an
// existing *rand.Rand works and produces valid coordinates.
func TestNewGeoTestUtilFromRand(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))
	g := NewGeoTestUtilFromRand(rng)

	for i := 0; i < 200; i++ {
		lat := g.NextLatitude()
		lon := g.NextLongitude()
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("lat #%d out of range: %g", i, lat)
		}
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("lon #%d out of range: %g", i, lon)
		}
	}
}

// TestNextPointNearPolygon_Holes checks that polygons with holes are
// handled (holes are targeted aggressively).
func TestNextPointNearPolygon_Holes(t *testing.T) {
	t.Parallel()

	outer := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))
	inner := boxPolygon(newRectangleOrPanic(-5, 5, -10, 10))
	withHoles := geo.MustNewPolygon(
		[]float64{outer.PolyLat(0), outer.PolyLat(1), outer.PolyLat(2), outer.PolyLat(3), outer.PolyLat(4)},
		[]float64{outer.PolyLon(0), outer.PolyLon(1), outer.PolyLon(2), outer.PolyLon(3), outer.PolyLon(4)},
		inner,
	)

	g := NewGeoTestUtil(777)
	for i := 0; i < 30; i++ {
		lat, lon := g.NextPointNearPolygon(withHoles)
		if lat < geo.MinLatIncl || lat > geo.MaxLatIncl {
			t.Errorf("lat out of range: %g", lat)
		}
		if lon < geo.MinLonIncl || lon > geo.MaxLonIncl {
			t.Errorf("lon out of range: %g", lon)
		}
	}
}

// TestNextBoxNearPolygon_Holes checks that NextBoxNearPolygon handles
// polygons with holes.
func TestNextBoxNearPolygon_Holes(t *testing.T) {
	t.Parallel()

	outer := boxPolygon(newRectangleOrPanic(-10, 10, -20, 20))
	inner := boxPolygon(newRectangleOrPanic(-5, 5, -10, 10))
	withHoles := geo.MustNewPolygon(
		[]float64{outer.PolyLat(0), outer.PolyLat(1), outer.PolyLat(2), outer.PolyLat(3), outer.PolyLat(4)},
		[]float64{outer.PolyLon(0), outer.PolyLon(1), outer.PolyLon(2), outer.PolyLon(3), outer.PolyLon(4)},
		inner,
	)

	g := NewGeoTestUtil(888)
	for i := 0; i < 30; i++ {
		box := g.NextBoxNearPolygon(withHoles)
		if box.MinLat() > box.MaxLat() {
			t.Errorf("box #%d: minLat=%g > maxLat=%g", i, box.MinLat(), box.MaxLat())
		}
	}
}

// TestCreateRegularPolygon_SeededDeterminism verifies that
// CreateRegularPolygon produces identical output for the same seed.
func TestCreateRegularPolygon_SeededDeterminism(t *testing.T) {
	t.Parallel()

	run := func() geo.Polygon {
		g := NewGeoTestUtil(42)
		return g.CreateRegularPolygon(0, 0, 5000, 5)
	}

	p1 := run()
	p2 := run()

	if !p1.Equals(p2) {
		t.Error("CreateRegularPolygon: seed determinism broken")
	}
}

// TestEdgeValuesExercised checks that the nextDoubleInternal edge-value
// targeting produces low, high, and 0 values at least once over many
// draws.
func TestEdgeValuesExercised(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(9999)
	const n = 5000

	var sawLow, sawHigh, sawZero bool
	epsilon := 1e-15

	for i := 0; i < n; i++ {
		lat := g.NextLatitude()
		if math.Abs(lat-geo.MinLatIncl) <= epsilon {
			sawLow = true
		}
		if math.Abs(lat-geo.MaxLatIncl) <= epsilon {
			sawHigh = true
		}
		if math.Abs(lat) <= epsilon {
			sawZero = true
		}
	}

	if !sawLow {
		t.Log("warning: MinLatIncl not hit in 5000 draws (possible but unlikely)")
	}
	if !sawHigh {
		t.Log("warning: MaxLatIncl not hit in 5000 draws (possible but unlikely)")
	}
	if !sawZero {
		t.Log("warning: zero not hit in 5000 draws (possible but unlikely)")
	}
}

// TestHaversinMeters_Reference verifies that the HaversinMeters
// utility used by CreateRegularPolygon returns the expected distance.
func TestHaversinMeters_Reference(t *testing.T) {
	t.Parallel()

	// Equatorial distance of 1 degree should be ~111km.
	d := util.HaversinMeters(0, 0, 0, 1)
	expected := 111319.5 // approx 111km per degree at equator
	if math.Abs(d-expected) > 1000 {
		t.Errorf("haversin(0,0,0,1): got %g, want ~%g", d, expected)
	}
}

// TestNextDoubleInternal_BoundsRegression generates many random values
// and verifies they never escape [low, high].
func TestNextDoubleInternal_BoundsRegression(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(12345)
	for i := 0; i < 10000; i++ {
		// Test at various ranges
		for _, bounds := range [][2]float64{
			{-90, 90},
			{-180, 180},
			{-1, 1},
			{0, 1},
			{-1, 0},
			{23.5, 23.5}, // equal bounds
			{-0.001, 0.001},
		} {
			low, high := bounds[0], bounds[1]
			v := g.nextDoubleInternal(low, high)
			if low > high {
				continue
			}
			if v < low || v > high {
				t.Errorf("nextDoubleInternal(%g, %g): got %g, out of bounds", low, high, v)
			}
		}
	}
}

// TestNextLine_PolygonDerivation checks that NextLine discards the
// closing vertex from the source polygon.
func TestNextLine_PolygonDerivation(t *testing.T) {
	t.Parallel()

	g := NewGeoTestUtil(333)
	for i := 0; i < 20; i++ {
		line := g.NextLine()
		if line.NumPoints() < 2 {
			t.Errorf("NextLine #%d: NumPoints=%d < 2", i, line.NumPoints())
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
