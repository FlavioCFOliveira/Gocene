// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import (
	"math"
	"math/rand"
	"testing"
)

// circleXYZBounds builds the XYZBounds for a GeoStandardCircle via the ported
// single-plane Plane.RecordBounds path (GeoStandardCircle.GetBounds ->
// Bounds.AddPlane -> Plane.RecordBounds).
func circleXYZBounds(c *GeoStandardCircle) *XYZBounds {
	b := NewXYZBounds()
	c.GetBounds(b)
	return b
}

// randomCirclePoint returns a uniformly distributed point inside the cap of
// half-angle cutoff around the centre (lat,lon), projected onto the unit
// sphere. Uniform-in-area sampling of a spherical cap: the polar angle theta
// has cos(theta) uniform in [cos(cutoff), 1].
func randomCirclePoint(rng *rand.Rand, pm *PlanetModel, lat, lon, cutoff float64) *GeoPoint {
	cosTheta := 1.0 - rng.Float64()*(1.0-math.Cos(cutoff))
	theta := math.Acos(cosTheta)
	phi := rng.Float64() * 2.0 * math.Pi

	// Point on a cap around the +Z pole, then rotate to (lat,lon).
	sinTheta := math.Sin(theta)
	px := sinTheta * math.Cos(phi)
	py := sinTheta * math.Sin(phi)
	pz := cosTheta

	// Rotate the cap from the +Z pole to the centre direction.
	// First rotate about Y by (pi/2 - lat) so +Z maps to the centre's
	// colatitude, then about Z by lon.
	colat := math.Pi/2.0 - lat
	cosB, sinB := math.Cos(colat), math.Sin(colat)
	// Rotation about Y axis.
	rx := px*cosB + pz*sinB
	rz := -px*sinB + pz*cosB
	ry := py
	// Rotation about Z axis by lon.
	cosL, sinL := math.Cos(lon), math.Sin(lon)
	fx := rx*cosL - ry*sinL
	fy := rx*sinL + ry*cosL
	fz := rz

	// Project onto the planet surface (works for the sphere; for ellipsoids the
	// IsWithin gate below is still authoritative).
	return NewGeoPoint(fx*pm.XYScaling, fy*pm.XYScaling, fz*pm.ZScaling)
}

// TestCircleXYZBoundsIsSuperset is acceptance criterion (1): the XYZBounds
// computed for a GeoStandardCircle via the ported Plane.RecordBounds must be a
// proven complete superset of the circle. Every point that the circle reports
// as inside (IsWithin true) must fall inside the computed XYZBounds box.
func TestCircleXYZBoundsIsSuperset(t *testing.T) {
	pm := SPHERE
	rng := rand.New(rand.NewSource(0x1e3779b97f4a7c15))

	cases := []struct {
		lat, lon, cutoff float64
	}{
		{0.0, 0.0, 0.5},
		{0.0, 0.0, 0.1},
		{0.0, 0.0, 1.2},
		{0.8, 0.0, 0.5},
		{-0.8, 0.0, 0.5},
		{1.4, 0.0, 0.3},  // near north pole
		{-1.4, 0.0, 0.3}, // near south pole
		{0.3, 2.5, 0.6},  // off-prime-meridian
		{0.0, -1.7, 0.4}, // western hemisphere
		{0.2, 3.0, 0.9},  // wide, near antimeridian
		{0.0, 0.0, 1.5},  // very large cap
		{0.5, 1.0, 0.05}, // tiny cap
	}

	for _, tc := range cases {
		circle, err := NewGeoStandardCircle(pm, tc.lat, tc.lon, tc.cutoff)
		if err != nil {
			t.Fatalf("NewGeoStandardCircle(%v): %v", tc, err)
		}
		bounds := circleXYZBounds(circle)
		if !bounds.HasX() || !bounds.HasY() || !bounds.HasZ() {
			t.Fatalf("case %v: bounds incomplete: hasX=%v hasY=%v hasZ=%v",
				tc, bounds.HasX(), bounds.HasY(), bounds.HasZ())
		}

		const samples = 20000
		var inside, outside int
		for i := 0; i < samples; i++ {
			p := randomCirclePoint(rng, pm, tc.lat, tc.lon, tc.cutoff)
			if !circle.IsWithin(p.X, p.Y, p.Z) {
				continue // sampling noise near the boundary; only assert in-shape points
			}
			inside++
			if !bounds.IsWithin(p.X, p.Y, p.Z) {
				outside++
				t.Errorf("case %v: in-circle point (%g,%g,%g) lies OUTSIDE XYZBounds [x %g..%g y %g..%g z %g..%g]",
					tc, p.X, p.Y, p.Z,
					bounds.MinimumX, bounds.MaximumX,
					bounds.MinimumY, bounds.MaximumY,
					bounds.MinimumZ, bounds.MaximumZ)
			}
		}
		if inside == 0 {
			t.Fatalf("case %v: no in-circle samples generated", tc)
		}
		if outside != 0 {
			t.Fatalf("case %v: %d/%d in-circle points fell outside the XYZBounds superset",
				tc, outside, inside)
		}
		// The bounds must also be a genuine subset of the planet box (a sanity
		// check that RecordBounds did not blow up to nonsense).
		if bounds.MinimumX < pm.GetMinimumXValue()-xyzBoundsFudgeFactor ||
			bounds.MaximumX > pm.GetMaximumXValue()+xyzBoundsFudgeFactor {
			t.Errorf("case %v: X bounds exceed planet box: [%g..%g]", tc, bounds.MinimumX, bounds.MaximumX)
		}
	}
}
